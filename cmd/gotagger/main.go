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
	dirtyIncrement string
	force          bool
	modules        bool
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

	flags.BoolVar(&g.force, "force", g.boolEnv("force", false), "force creation of a tag")
	flags.BoolVar(&g.modules, "modules", g.boolEnv("modules", defaultModulesFlag), "enable go module versioning")
	flags.BoolVar(&g.pushTag, "push", g.boolEnv("push", false), "push the just created tag, implies -release")
	flags.StringVar(&g.remoteName, "remote", g.stringEnv("remote", defaultRemoteFlag), "name of the remote to push tags to")
	flags.BoolVar(&g.showVersion, "version", false, "show version information")
	flags.BoolVar(&g.tagRelease, "release", g.boolEnv("release", false), "tag HEAD with the current version if it is a release commit")
	flags.StringVar(&g.versionPrefix, "prefix", g.stringEnv("prefix", defaultPrefixFlag), "set a prefix for versions")
	flags.StringVar(&g.dirtyIncrement, "dirty", g.stringEnv("dirty", defaultDirtyFlag), "how to increment the version for a dirty checkout [minor, patch, none]")
	flags.StringVar(&g.configFile, "config", g.stringEnv("config", defaultConfigFlag), "path to the gotagger configuration file.")

	if g.configFile == defaultConfigFlag {
		// If there's no config file provided, check for one locally.
		defaultConfig := filepath.Join(g.WorkingDir, defaultConfigFlag)
		if _, err := os.Stat(defaultConfig); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				g.err.Println("error: unable to read config file:", err)
				return genericErrorExitCode
			}

			// no config file available
			g.configFile = ""
		}
	}

	// profiling options
	cpuprofile := flags.String("cpuprofile", "", "write cpu profile to file")
	memprofile := flags.String("memprofile", "", "write memory profile to file")

	g.setUsage(flags)
	if err := flags.Parse(g.Args); err != nil {
		return genericErrorExitCode
	}

	if *cpuprofile != "" {
		f, err := os.Create(filepath.Join(g.WorkingDir, *cpuprofile))
		if err != nil {
			g.err.Println("error: could not create CPU profile:", err)
			return genericErrorExitCode
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			g.err.Println("error: could not start CPU profile:", err)
			return genericErrorExitCode
		}
		defer pprof.StopCPUProfile()
	}

	if *memprofile != "" {
		f, err := os.Create(filepath.Join(g.WorkingDir, *memprofile))
		if err != nil {
			g.err.Println("error: could not create memory profile:", err)
			return genericErrorExitCode
		}
		defer f.Close()

		defer func() {
			runtime.GC()
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
	r, err := gotagger.New(path)
	if err != nil {
		g.err.Println("error:", err)
		return genericErrorExitCode
	}

	if g.configFile != "" {
		data, err := os.ReadFile(g.configFile)
		if err != nil {
			g.err.Println("error:", err)
			return genericErrorExitCode
		}

		err = r.Config.ParseJSON(data)
		if err != nil {
			g.err.Println("error:", err)
			return genericErrorExitCode
		}
	}

	r.Config.Force = g.force
	r.Config.CreateTag = g.tagRelease || g.pushTag || g.force
	r.Config.PushTag = g.pushTag
	r.Config.RemoteName = g.remoteName

	//nolint: gosimple // makes this consistent with other flags,
	// and avoids hard to understand double negatives
	if g.modules != defaultModulesFlag {
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

	versions, err := r.TagRepo()
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
version in the current branch and then determing what type of commits were made
since that commit. Go submodules can be tagged by including the module name in a
Modules footer in the release commit message.
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
