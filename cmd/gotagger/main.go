// Copyright (c) SAS Institute, Inc.

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"

	"sassoftware.io/clis/gotagger"
	"sassoftware.io/clis/gotagger/git"
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
	pushTag       bool
	remoteName    string
	showVersion   bool
	tagRelease    bool
	versionPrefix string
}

// Runs GoTagger.
func (g *GoTagger) Run() int {
	// setup logggers to write to stdout/stderr
	g.out = log.New(g.Stdout, "", 0)
	g.err = log.New(g.Stderr, "", 0)

	flags := flag.NewFlagSet(AppName, flag.ContinueOnError)
	flags.SetOutput(g.Stderr)
	flags.BoolVar(&g.pushTag, "push", g.boolEnv("push", false), "push the just created tag, implies -release")
	flags.StringVar(&g.remoteName, "remote", g.stringEnv("remote", "origin"), "name of the remote to push tags to")
	flags.BoolVar(&g.showVersion, "version", false, "show version information")
	flags.BoolVar(&g.tagRelease, "release", g.boolEnv("release", false), "tag HEAD with the current version if it is a release commit")
	flags.StringVar(&g.versionPrefix, "prefix", g.stringEnv("prefix", "v"), "set a prefix for versions")

	g.setUsage(flags)
	if err := flags.Parse(g.Args[1:]); err != nil {
		return genericErrorExitCode
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
	r, err := git.New(path)
	if err != nil {
		g.err.Println("error: ", err)
		return genericErrorExitCode
	}

	cfg := &gotagger.Config{
		RemoteName:    g.remoteName,
		CreateTag:     g.tagRelease,
		PushTag:       g.pushTag,
		VersionPrefix: g.versionPrefix,
	}

	latest, err := gotagger.TagRepo(cfg, r)
	if err != nil {
		g.err.Println("error: ", err)
		return genericErrorExitCode
	}
	g.out.Println(latest)
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
since that commit.
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
		fmt.Fprintf(os.Stderr, "failed to get current working directory: %s", err)
		os.Exit(genericErrorExitCode)
	}
	exc := &GoTagger{
		Args:       os.Args,
		Env:        os.Environ(),
		Stdout:     os.Stdout,
		Stderr:     os.Stdin,
		WorkingDir: wd,
	}
	os.Exit(exc.Run())
}
