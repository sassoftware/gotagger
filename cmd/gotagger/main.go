// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/sassoftware/gotagger"
	"github.com/sassoftware/gotagger/mapper"
)

const (
	successExitCode      = 0
	genericErrorExitCode = 1

	versionOutput = `gotagger:
 version     : %s
 build date  : %s
 git hash    : %s
 go version  : %s
 go compiler : %s
 platform    : %s/%s
`

	defaultConfigFlag  = "gotagger.json"
	defaultDirtyFlag   = "none"
	defaultModulesFlag = true
	defaultPrefixFlag  = "v"
	defaultRemoteFlag  = "origin"
)

var (
	AppName    = "gotagger"
	AppVersion = "dev"
	Commit     = "unknown"
	BuildDate  = "none"
)

// GoTagger represents a specific execution of the gotagger cli
type GoTagger struct {
	Args           []string  // The command-line arguments
	Env            []string  // The os environment
	Stdout, Stderr io.Writer // Output writers
	WorkingDir     string    // The directory the process is run from

	// output loggers
	out *log.Logger
	err *log.Logger

	// command-line options
	configFile     string
	debug          bool
	dirtyIncrement string
	force          bool
	modules        bool
	pathFilter     string
	pushTag        bool
	remoteName     string
	showVersion    bool
	tagRelease     bool
	versionPrefix  string
}

// Runs GoTagger.
func (g *GoTagger) Run() int {
	// setup loggers to write to stdout/stderr
	g.out = log.New(g.Stdout, "", 0)
	g.err = log.New(g.Stderr, "", 0)

	flags := flag.NewFlagSet(AppName, flag.ContinueOnError)
	flags.SetOutput(g.Stderr)

	flags.StringVar(&g.configFile, "config", g.stringEnv("config", defaultConfigFlag), "path to the gotagger configuration file.")
	flags.StringVar(&g.dirtyIncrement, "dirty", g.stringEnv("dirty", defaultDirtyFlag), "how to increment the version for a dirty checkout [minor, patch, none]")
	flags.BoolVar(&g.debug, "debug", false, "enable debug output")
	flags.BoolVar(&g.force, "force", g.boolEnv("force", false), "force creation of a tag")
	flags.BoolVar(&g.modules, "modules", g.boolEnv("modules", defaultModulesFlag), "enable go module versioning")
	flags.StringVar(&g.pathFilter, "path", "", "filter commits by path")
	flags.BoolVar(&g.pushTag, "push", g.boolEnv("push", false), "push the just created tag, implies -release")
	flags.StringVar(&g.remoteName, "remote", g.stringEnv("remote", defaultRemoteFlag), "name of the remote to push tags to")
	flags.BoolVar(&g.showVersion, "version", false, "show version information")
	flags.BoolVar(&g.tagRelease, "release", g.boolEnv("release", false), "tag HEAD with the current version if it is a release commit")
	flags.StringVar(&g.versionPrefix, "prefix", g.stringEnv("prefix", defaultPrefixFlag), "set a prefix for versions")

	// profiling options
	cpuprofile := flags.String("cpuprofile", "", "write cpu profile to file")
	memprofile := flags.String("memprofile", "", "write memory profile to file")

	g.setUsage(flags)
	if err := flags.Parse(g.Args); err != nil {
		return genericErrorExitCode
	}

	zerolog.SetGlobalLevel(zerolog.Disabled)
	if g.debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerologr.NameFieldName = "logger"
	zerologr.NameSeparator = "/"
	zl := zerolog.New(zerolog.ConsoleWriter{Out: g.Stderr, TimeFormat: time.StampMicro})
	zl = zl.With().Caller().Timestamp().Logger()

	rootLogger := zerologr.New(&zl)

	// we only really log debug messages,
	// so force the V-level to 1
	logger := rootLogger.WithName("main").V(1)

	if *cpuprofile != "" {
		logger.Info("enabling cpu profiling", "path", *cpuprofile)
		f, err := os.Create(filepath.Join(g.WorkingDir, *cpuprofile))
		if err != nil {
			g.err.Println("error: could not create CPU profile:", err)
			return genericErrorExitCode
		}
		defer f.Close()

		logger.Info("starting cpu profiling")
		if err := pprof.StartCPUProfile(f); err != nil {
			g.err.Println("error: could not start CPU profile:", err)
			return genericErrorExitCode
		}
		defer pprof.StopCPUProfile()
	}

	if *memprofile != "" {
		logger.Info("enabling memory profiling", "path", *memprofile)
		f, err := os.Create(filepath.Join(g.WorkingDir, *memprofile))
		if err != nil {
			g.err.Println("error: could not create memory profile:", err)
			return genericErrorExitCode
		}
		defer f.Close()

		defer func() {
			runtime.GC()
			logger.Info("writing out memory profile")
			if err := pprof.WriteHeapProfile(f); err != nil {
				g.err.Fatal("error: could not write memory profile:", err)
			}
		}()
	}

	if g.showVersion {
		g.out.Print(versionInfo(AppVersion, Commit, BuildDate))
		return successExitCode
	}

	// Find the git repo
	path := flags.Arg(0)
	if path == "" {
		path = g.WorkingDir
	}

	// validate that path filter is a directory in the git repo
	info, err := os.Stat(filepath.Join(path, g.pathFilter))
	if err != nil {
		g.err.Printf("error: invalid path filter %s: %v", g.pathFilter, err)
		return genericErrorExitCode
	}

	if !info.IsDir() {
		g.err.Printf("error: invalid path filter %s: not a directory", g.pathFilter)
		return genericErrorExitCode
	}

	r, err := gotagger.New(path)
	if err != nil {
		g.err.Println("error:", err)
		return genericErrorExitCode
	}

	r.SetLogger(rootLogger)

	if g.configFile != "" {
		logger.Info("reading config file", "path", g.configFile)
		data, err := os.ReadFile(g.configFile)
		// ignore a missing "default" config file
		if g.configFile != defaultConfigFlag || !errors.Is(err, os.ErrNotExist) {
			if err != nil {
				g.err.Println("error:", err)
				return genericErrorExitCode
			}

			logger.Info("parsing config data", "path", g.configFile)
			err = r.Config.ParseJSON(data)
			if err != nil {
				g.err.Println("error:", err)
				return genericErrorExitCode
			}
		}
	}

	r.Config.CreateTag = g.tagRelease || g.pushTag || g.force
	r.Config.Force = g.force
	r.Config.PushTag = g.pushTag
	r.Config.RemoteName = g.remoteName

	if !g.modules {
		r.Config.IgnoreModules = !g.modules
	}
	if g.versionPrefix != defaultPrefixFlag {
		r.Config.VersionPrefix = g.versionPrefix
	}
	if g.dirtyIncrement != defaultDirtyFlag {
		inc, err := mapper.Convert(g.dirtyIncrement)
		if err != nil {
			g.err.Println("error:", err)
			return genericErrorExitCode
		}

		if inc == mapper.IncrementMajor {
			g.err.Println("error: -dirty value must be minor, patch, or none")
			return genericErrorExitCode
		}
		r.Config.DirtyWorktreeIncrement = inc
	}
	if g.pathFilter != "" {
		r.Config.Paths = []string{g.pathFilter}
	}

	start := time.Now()
	logger.Info("calculating version", "start", start)
	versions, err := r.TagRepo()
	dur := time.Since(start)
	logger.Info("done calculating version", "duration", dur)

	if err != nil {
		g.err.Println("error:", err)
		return genericErrorExitCode
	}

	for _, version := range versions {
		g.out.Println(version)
	}

	return successExitCode
}

func (g *GoTagger) boolEnv(env string, def bool) bool {
	if val, ok := getEnv(env); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			// We use fatal here since we cannot return an error.
			g.err.Fatalf("error: cannot parse GOTAGGER_%s as a boolean value: %v\n", strings.ToUpper(env), err)
		}
		return b
	}

	return def
}

func (g *GoTagger) stringEnv(env, def string) string {
	if val, ok := getEnv(env); ok {
		return val
	}

	return def
}

func getEnv(env string) (string, bool) {
	env = "GOTAGGER_" + strings.ToUpper(env)
	return os.LookupEnv(env)
}

const (
	usagePrefix = `Usage: %s [OPTION]... [PATH]
Print the current version of the project to standard output.

With no PATH the current directory is used.

Options:
  -help
        show this help message
`
	usageSuffix = `
The current version is determined by finding the commit tagged with highest
version in the current branch and then determining what type of commits were
made since that commit by parsing the commit messages using the conventional
commit standard.

If the -release flag is set and the HEAD commit uses the 'release' type, then
gotagger will create a tag using the version it calculates. For projects that
contain multiple go modules, tag specific modules by including them in the
release commit using the Modules footer:

    release: some submodules

	Modules: github.com/example/repo/module, github.com/example/repo/other/module

The -path flag causes gotagger to filter commit history by paths. This is useful
for using gotagger with git repositories that contain multiple pieces that
should be versioned separately. A path filter must exist and must be a
directory.
`
)

func (g *GoTagger) setUsage(fs *flag.FlagSet) {
	fs.Usage = func() {
		g.err.Printf(usagePrefix, AppName)
		fs.PrintDefaults()
		g.err.Print(usageSuffix)
	}
}

func versionInfo(version, commit, date string) string {
	return fmt.Sprintf(versionOutput, version, date, commit, runtime.Version(),
		runtime.Compiler, runtime.GOOS, runtime.GOARCH)
}

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: failed to get current working directory: ", err)
		os.Exit(genericErrorExitCode)
	}

	exc := &GoTagger{
		Args:       os.Args[1:],
		Env:        os.Environ(),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		WorkingDir: wd,
	}

	os.Exit(exc.Run())
}
