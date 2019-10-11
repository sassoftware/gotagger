// Copyright (c) SAS Institute, Inc.
//
// The git package contains functions for running git commands.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
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
}

func runGitCommand(args []string, path string) ([]byte, error) {
	c := exec.Command("git", args...)
	if path != "" {
		c.Dir = path
	}
	out, err := c.Output()
	if err != nil {
		if xerr, ok := err.(*exec.ExitError); ok {
			switch xerr.ExitCode() {
			case 127:
				err = fmt.Errorf("git command not found. Make sure git is installed and on your path")
			case 128:
				err = fmt.Errorf("not a git respository: %s", path)
			default:
				err = fmt.Errorf("git rev-prase failed: %s", xerr.Stderr)
			}
		}
		return []byte{}, err
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
	return Repo{dir: path, gitDir: gitDir}, nil
}

func (r Repo) run(args []string) ([]byte, error) {
	args = append([]string{"--git-dir", r.gitDir}, args...)
	return runGitCommand(args, r.dir)
}

// CreateTag tags a commit in a git repo.
//
// If signed is true, then the tag will be a signed tag. This requires that
// your git configuration is properly setup for signing.
//
// If prefix is true, the tag name will inclue a leading 'v'.
func (r Repo) CreateTag(commit string, v *semver.Version, message string, signed, prefix bool) error {
	vStr := v.String()
	if prefix {
		vStr = "v" + vStr
	}
	args := []string{"tag"}
	if signed {
		args = append(args, "-s")
		if message == "" {
			message = "Release " + vStr
		}
	}
	if message != "" {
		args = append(args, "-m", message)
	}
	args = append(args, vStr)
	_, err := r.run(args)
	return err
}

// Head returns the commit at HEAD
func (r Repo) Head() (Commit, error) {
	return Commit{}, nil
}

// RevList returns a slice of commits from start to end.
func (r Repo) RevList(start, end string) ([]Commit, error) {
	args := []string{"log", "--format=%H%x00%s%x00%b%x00%(trailers:only=yes,separator=%x2c,unfold=yes)%x00%d%x00%x00", start}
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

// Tags returns a slice of all tagged commits.
func (r Repo) Tags() ([]Commit, error) {
	args := []string{
		"tag",
		"--list",
		"--format='%(if)%(*objectname)%(then)%(*objectname)%(else)%(objectname)%(end)%00%00%00%00%(refname)'",
	}
	out, err := r.run(args)
	if err != nil {
		return nil, err
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return []Commit{}, nil
	}
	lines := bytes.Split(out, []byte("\n"))
	return parseCommits(lines), nil
}

func parseCommits(lines [][]byte) []Commit {
	commits := make([]Commit, len(lines))
	for i := 0; i < len(lines); i++ {
		line := bytes.Split(lines[i], gitLogSeparator)
		commits[i] = Commit{
			Hash:     string(line[0]),
			Subject:  string(line[1]),
			Body:     string(line[2]),
			Trailers: strings.Split(string(line[3]), ","),
			Tags:     parseTags(string(line[4])),
		}
	}
	return commits
}

func parseTags(refname string) []*semver.Version {
	tags := make([]*semver.Version, 0)
	if strings.Contains(refname, "refs/tags") {
		// remove (, ) and " "
		refname = strings.Trim(refname, "() ")
		// split on "refs/tags/"
		for _, tagString := range strings.Split(refname, "refs/tags/")[1:] {
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
