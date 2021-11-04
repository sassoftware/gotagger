// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// The git package contains functions for running git commands.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sassoftware/gotagger/internal/commit"
)

var (
	errEmptyStart = errors.New("Must specify a start")
)

// Commit represents a commit in a git repository.
type Commit struct {
	commit.Commit
	Hash    string
	Changes []Change
}

type Change struct {
	SourceName string
	DestName   string
	Action     string
	SourceMode string
	DestMode   string
	SourceSHA  string
	DestSHA    string
}

// Repository represents a git repository.
type Repository struct {
	GitDir string
	Path   string

	runner func([]string, string) (string, error)
}

// New returns a new git Repo. If path is not a git repo, then an error will be returned.
func New(path string) (*Repository, error) {
	gitDir, err := getGitDirectory(path)
	if err != nil {
		return nil, err
	}

	// if we got a relative path, then join it with path
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(path, gitDir)
		gitDir, err = filepath.Abs(gitDir)
		if err != nil {
			return nil, err
		}
	}

	repo := &Repository{
		GitDir: gitDir,
		Path:   path,
		runner: runGitCommand,
	}

	return repo, nil
}

// CreateTag tags a commit in a git repo.
//
// If prefix is a non-empty string, then the version will be prefixed with that string.
func (r *Repository) CreateTag(hash, name, message string, signed bool) error {
	if message == "" {
		message = "Release " + name
	}

	args := []string{"tag"}
	if signed {
		args = append(args, "-s")
	}

	args = append(args, "-m", message, name, hash)

	_, err := r.run(args)
	return err
}

func (r *Repository) DeleteTags(tags []string) error {
	var errorMsg string
	for _, tag := range tags {
		if _, terr := r.run([]string{"tag", "-d", tag}); terr != nil {
			if errorMsg == "" {
				errorMsg = "could not delete tags:"
			}
			errorMsg += "\n\t" + terr.Error()
		}
	}

	if errorMsg != "" {
		return errors.New(errorMsg)
	}

	return nil
}

// Head returns the commit at HEAD
func (r *Repository) Head() (Commit, error) {
	commits, err := r.RevList("HEAD", "HEAD^")
	if err != nil {
		return Commit{}, err
	}

	return commits[0], nil
}

// IsDirty returns a boolean indicating whether there are uncommited changes.
func (r *Repository) IsDirty() (bool, error) {
	out, err := r.run([]string{"status", "--porcelain"})
	return out != "", err
}

// PushTag pushes tag to remote.
func (r *Repository) PushTag(tag string, remote string) error {
	return r.PushTags([]string{tag}, remote)
}

// PushTags pushes tags to the remote repository remote.
func (r *Repository) PushTags(tags []string, remote string) error {
	refSpecs := make([]string, len(tags))
	for i, tag := range tags {
		refname := "refs/tags/" + tag
		refSpecs[i] = refname + ":" + refname
	}

	args := append([]string{"push", remote}, refSpecs...)
	_, err := r.run(args)
	return err
}

// RevList returns a slice of commits from start to end.
func (r *Repository) RevList(start, end string, paths ...string) ([]Commit, error) {
	if start == "" {
		return nil, errEmptyStart
	}

	args := []string{"log", "--format=raw", "--raw", "--no-abbrev", start}

	// add start and end refs
	if end != "" {
		args = append(args, "^"+end)
	}

	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}

	out, err := r.run(args)
	if err != nil {
		return nil, err
	}

	out = strings.TrimSpace(out)
	if len(out) == 0 {
		return []Commit{}, nil
	}

	return parseCommits(string(out)), nil
}

func (r *Repository) RevParse(rev string) (string, error) {
	out, err := r.run([]string{"rev-parse", rev})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

// Tags returns all tags that point to ancestors of rev.
//
// rev can be either a revision or a hash.
//
// prefix is a string prefix to filter tags with.
func (r *Repository) Tags(rev string, prefixes ...string) (tags []string, err error) {
	// list all tags that point to ancestors of rev
	args := []string{"tag", "--merged", rev}
	if len(prefixes) > 0 {
		args = append(args, "--list")
		for _, p := range prefixes {
			args = append(args, p+"*")
		}
	}

	out, err := r.run(args)
	if err != nil {
		return
	}

	out = strings.TrimSpace(out)
	tags = strings.Split(string(out), "\n")

	return
}

func (r *Repository) run(args []string) (string, error) {
	args = append([]string{"--git-dir", r.GitDir}, args...)
	return r.runner(args, r.Path)
}

func getGitDirectory(path string) (string, error) {
	out, err := runGitCommand([]string{"rev-parse", "--git-dir"}, path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// hasPrefix returns true if t has a prefix that matches any prefixes.
// The empty string matches if t has no prefix.
func hasPrefix(t string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if prefix == "" {
			// then t must have no prefix
			if _, err := strconv.Atoi(t[0:1]); err == nil {
				return true
			}
		} else if strings.HasPrefix(t, prefix) {
			return true
		}
	}

	return false
}

func parseChanges(lines []string) []Change {
	changes := make([]Change, len(lines))
	for i, line := range lines {
		parts := strings.Split(line, "\t")
		line, files := parts[0], parts[1:]

		parts = strings.Split(line, " ")
		c := Change{
			SourceMode: strings.TrimPrefix(parts[0], ":"),
			DestMode:   parts[1],
			SourceSHA:  parts[2],
			DestSHA:    parts[3],
			Action:     parts[4],
			SourceName: files[0],
		}

		if len(files) > 1 {
			c.DestName = files[1]
		}

		changes[i] = c
	}

	return changes
}

func parseCommits(data string) (commits []Commit) {
	// strip the first 'commit '
	data = strings.TrimPrefix(data, "commit ")

	// split on \n^commits to separate the raw output into raw commits
	rawCommits := strings.Split(data, "\ncommit ")
	for _, rawCommit := range rawCommits {
		// separate headers from message and changes
		parts := strings.Split(rawCommit, "\n\n")
		headers, message := parts[0], parts[1]

		var changes []Change
		if len(parts) > 2 {
			rawChanges := strings.TrimSpace(parts[2])
			if rawChanges != "" {
				changes = parseChanges(strings.Split(rawChanges, "\n"))
			}
		}

		// trim the leading four spaces from the commit message lines
		message = strings.TrimSpace(message)
		message = strings.ReplaceAll(message, "\n    ", "\n")

		// parse the commit message
		commit := Commit{
			Commit:  commit.Parse(message),
			Hash:    strings.Split(headers, "\n")[0],
			Changes: changes,
		}

		commits = append(commits, commit)
	}

	return
}

func runGitCommand(args []string, path string) (string, error) {
	c := exec.Command("git", args...)

	if path != "" {
		c.Dir = path
	}

	out, err := c.Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			code := err.ExitCode()
			switch code {
			case 127:
				return "", fmt.Errorf("git command not found. Make sure git is installed and on your path")
			default:
				command := "git"
				for _, arg := range args {
					if strings.Contains(arg, " ") {
						arg = "'" + arg + "'"
					}
					command += " " + arg
				}

				return "", fmt.Errorf("%s failed with exit code %d: %s", command, code, err.Stderr)
			}
		}

		return "", err
	}

	return string(out), err
}
