// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// The git package contains functions for running git commands.
//
// This package is deprecated and will be removed before the v1.0.0 release of gotagger.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	gitLogLineTerminator = []byte("\x00\x00\n")
	gitLogSeparator      = []byte("\x00")
)

// Commit represents a commit in a git repository.
type Commit struct {
	Hash     string            // The commit hash
	Subject  string            // The commit subject, generally the first line of the commit message
	Body     string            // The commit body
	Tags     []*semver.Version // All tags that point to this commit.
	Trailers []string          // The commit trailers
}

// Repo represents a git repository.
type Repo struct {
	dir    string
	gitDir string
	runner func([]string, string) ([]byte, error)
}

func runGitCommand(args []string, path string) ([]byte, error) {
	c := exec.Command("git", args...)
	if path != "" {
		c.Dir = path
	}
	out, err := c.Output()
	if err != nil {
		switch err := err.(type) {
		case *exec.ExitError:
			code := err.ExitCode()
			switch code {
			case 127:
				return nil, fmt.Errorf("git command not found. Make sure git is installed and on your path")
			default:
				command := "git"
				for _, arg := range args {
					if strings.Contains(arg, " ") {
						arg = "'" + arg + "'"
					}
					command += " " + arg
				}
				return nil, fmt.Errorf("%s failed with exit code %d: %s", command, code, err.Stderr)
			}
		}
		return nil, err
	}
	return out, err
}

func getGitDirectory(path string) (string, error) {
	out, err := runGitCommand([]string{"rev-parse", "--git-dir"}, path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// New returns a new git Repo. If path is not a git repo, then an error will be returned.
func New(path string) (Repo, error) {
	gitDir, err := getGitDirectory(path)
	if err != nil {
		return Repo{}, err
	}
	return Repo{dir: path, gitDir: gitDir, runner: runGitCommand}, nil
}

func (r Repo) run(args []string) ([]byte, error) {
	args = append([]string{"--git-dir", r.gitDir}, args...)
	return r.runner(args, r.dir)
}

// CreateTag tags a commit in a git repo.
//
// If signed is true, then the tag will be a signed tag. This requires that
// your git configuration is properly setup for signing.
//
// If prefix is a non-empty string, then the version will be prefixed with that string.
func (r Repo) CreateTag(commit string, v *semver.Version, prefix, message string, signed bool) error {
	vStr := v.String()
	if prefix != "" {
		vStr = prefix + vStr
	}
	args := []string{"tag"}
	if signed {
		args = append(args, "-s")
	}
	if message == "" {
		message = "Release " + vStr
	}
	args = append(args, "-m", message, vStr, commit)
	_, err := r.run(args)
	return err
}

// Head returns the commit at HEAD
func (r Repo) Head() (Commit, error) {
	return Commit{}, nil
}

// PushTag pushes tag to the remote repository repo.
func (r Repo) PushTag(tag *semver.Version, repo string) error {
	refName := fmt.Sprintf("refs/tags/v%s", tag)
	args := []string{"push", repo, refName + ":" + refName}
	_, err := r.run(args)
	return err
}

// RevList returns a slice of commits from start to end.
func (r Repo) RevList(start, end string) ([]Commit, error) {
	return r.log(start, end, "--decorate=full")
}

// Tags returns a slice of all tagged commits.
func (r Repo) Tags(prefixes ...string) (commits []Commit, err error) {
	rawCommits, err := r.log("HEAD", "", "--simplify-by-decoration")
	if err != nil {
		return nil, err
	}

	for _, c := range rawCommits {
		if len(prefixes) > 0 {
			// filter tags by prefixes
			for _, t := range c.Tags {
				if hasPrefix(t, prefixes) {
					commits = append(commits, c)
					break
				}
			}
		} else if len(c.Tags) > 0 {
			commits = append(commits, c)
		}
	}

	return
}

// hasPrefix returns true if t has a prefix that matches any prefixes.
// The empty string matches if t has no prefix.
func hasPrefix(t *semver.Version, prefixes []string) bool {
	o := t.Original()
	for _, prefix := range prefixes {
		if prefix == "" {
			// then t must have no prefix
			if _, err := strconv.Atoi(o[0:1]); err == nil {
				return true
			}
		} else if strings.HasPrefix(o, prefix) {
			return true
		}
	}

	return false
}

func (r Repo) log(start, end string, extra ...string) ([]Commit, error) {
	args := []string{"log", "--format=%H%x00%s%x00%b%x00%(trailers:only,unfold,separator=|)%x00%d%x00%x00"}
	if extra != nil {
		args = append(args, extra...)
	}
	args = append(args, start)
	if end != "" {
		args = append(args, "^"+end)
	}
	out, err := r.run(args)
	if err != nil {
		return nil, err
	}
	out = bytes.TrimSuffix(out, gitLogLineTerminator)
	if len(out) == 0 {
		return []Commit{}, nil
	}
	lines := bytes.Split(out, gitLogLineTerminator)
	return parseCommits(lines), nil
}

func parseCommits(lines [][]byte) []Commit {
	commits := make([]Commit, len(lines))
	for i, line := range lines {
		parts := bytes.Split(line, gitLogSeparator)
		// sanity check
		if len(parts) != 5 {
			panic("unexpected commit entry format")
		}
		hash, subject, body, trailers, tags := parts[0], parts[1], parts[2], parts[3], parts[4]
		commits[i] = Commit{
			Hash:     string(hash),
			Subject:  string(subject),
			Body:     string(body),
			Trailers: strings.Split(string(trailers), "|"),
			Tags:     parseTags(string(tags)),
		}
	}
	return commits
}

func parseTags(refname string) []*semver.Version {
	tags := []*semver.Version{}
	if strings.Contains(refname, "tag:") {
		// remove (, ), and " "
		refname = strings.Trim(refname, "() ")
		// split on "tag: "
		for _, tagString := range strings.Split(refname, "tag: ")[1:] {
			// Strip refs/tags/
			tagString = strings.ReplaceAll(tagString, "refs/tags/", "")
			// git does not allow : or " " in tag names, so we can safely split on ", "
			candidate := strings.Split(tagString, ", ")[0]
			if tag, err := semver.NewVersion(candidate); err == nil {
				tags = append(tags, tag)
			}
		}
	}
	sort.Sort(sort.Reverse(semver.Collection(tags)))
	return tags
}
