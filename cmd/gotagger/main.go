// Copyright (c) SAS Institute, Inc.

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"

	"sassoftware.io/clis/gotagger/git"
	"sassoftware.io/clis/gotagger/marker"
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
}

// Runs GoTagger.
func (g GoTagger) Run() int {
	// setup logggers to write to stdout/stderr
	outLogger := log.New(g.Stdout, "", 0)
	errLogger := log.New(g.Stderr, "", 0)

	// Register flags
	var (
		showVersion bool
		pushTag     bool
		tagRelease  bool
	)

	flags := flag.NewFlagSet(AppName, flag.ContinueOnError)
	flags.SetOutput(g.Stderr)
	flags.BoolVar(&pushTag, "push", boolEnv("push"), "push the just created tag, implies -release")
	flags.BoolVar(&tagRelease, "release", boolEnv("release"), "tag HEAD with the current version if it is a release commit")
	flags.BoolVar(&showVersion, "version", false, "show version information")

	setUsage(errLogger, flags)
	if err := flags.Parse(g.Args[1:]); err != nil {
		return genericErrorExitCode
	}

	if showVersion {
		outLogger.Print(versionInfo(AppVersion, Commit, BuildDate))
		return successExitCode
	}

	// Find the git repo
	path := flags.Arg(0)
	if path == "" {
		path = g.WorkingDir
	}
	r, err := git.New(path)
	if err != nil {
		errLogger.Println("error: ", err)
		return genericErrorExitCode
	}

	// Find the latest semver and the commit hash it references.
	latest, commitHash, err := getLatest(r)
	if err != nil {
		errLogger.Println("error: ", err)
		return genericErrorExitCode
	}

	// Find the most significant marker between HEAD and the latest tagged commit.
	commits, err := r.RevList("HEAD", commitHash)
	if err != nil {
		errLogger.Printf("error: could not fetch commits HEAD..%s: %s", commitHash, err)
		return genericErrorExitCode
	}

	// If HEAD is already tagged, just display the latest version
	if len(commits) == 0 {
		outLogger.Println(latest)
		return successExitCode
	}

	changeType, isBreaking := scanForMarkers(commits)
	switch {
	case isBreaking:
		*latest = latest.IncMajor()
	case changeType == marker.Feature:
		*latest = latest.IncMinor()
	default:
		*latest = latest.IncPatch()
	}
	if len(commits) > 0 {
		head := commits[0]
		if (pushTag || tagRelease) && isRelease(head) && !alreadyTagged(latest, head) {
			if err := r.CreateTag(head.Hash, latest, "", false, true); err != nil {
				errLogger.Printf("error: could not tag HEAD (%s): %s", head.Hash, err)
				return genericErrorExitCode
			}
			if pushTag {
				// TODO: add option to set name of remote
				if err := r.PushTag(latest, "origin"); err != nil {
					errLogger.Printf("error: could not push tag (%s): %s", latest, err)
					return genericErrorExitCode
				}
			}
		}
	}
	outLogger.Println(latest)
	return successExitCode
}

func boolEnv(env string) bool {
	env = "GOTAGGER_" + strings.ToUpper(env)
	val := os.Getenv(env)
	switch strings.ToLower(val) {
	case "", "false", "0", "no":
		return false
	default:
		return true
	}
}

func alreadyTagged(v *semver.Version, c git.Commit) bool {
	for _, t := range c.Tags {
		if v.Equal(t) {
			return true
		}
	}
	return false
}

func isRelease(c git.Commit) bool {
	m, _, _ := marker.Parse(c.Subject)
	return m == marker.Release
}

func getLatest(r git.Repo) (latest *semver.Version, hash string, err error) {
	taggedCommits, err := r.Tags()
	if err != nil {
		return latest, hash, err
	}
	latest = new(semver.Version)
	for _, commit := range taggedCommits {
		if len(commit.Tags) > 0 {
			if latest.LessThan(commit.Tags[0]) {
				latest = commit.Tags[0]
				hash = commit.Hash
			}
		}
	}
	return latest, hash, nil
}

func scanForMarkers(commits []git.Commit) (mark marker.Marker, isBreaking bool) {
	if len(commits) != 0 {
		for _, c := range commits {
			m, _, b := marker.Parse(c.Subject)
			switch m {
			case marker.Feature:
				mark = m
			case marker.Fix:
				if mark != marker.Feature {
					mark = m
				}
			}
			// if we already saw a breaking change, we can stop checking
			if !isBreaking && (b || marker.IsBreaking(c.Trailers)) {
				isBreaking = true
			}
		}
	}
	return mark, isBreaking
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

func setUsage(l *log.Logger, fs *flag.FlagSet) {
	fs.Usage = func() {
		l.Printf(usagePrefix, AppName)
		fs.PrintDefaults()
		l.Print(usageSuffix)
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
