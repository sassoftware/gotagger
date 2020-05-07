// Copyright (c) SAS Institute, Inc.
//
// The git package contains functions for running git commands.
package git

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	errEOC        = errors.New("end of commits")
	errEmptyStart = errors.New("Must specify a start")
)

// Commit represents a commit in a git repository.
type Commit struct {
	Hash     string            // The commit hash
	Subject  string            // The commit subject, generally the first line of the commit message
	Body     string            // The commit body
	Tags     []*semver.Version // All tags that point to this commit.
	Trailers []string          // The commit trailers
}

// Repository represents a git repository.
type Repository struct {
	Path string

	repo *git.Repository
}

// New returns a new git Repo. If path is not a git repo, then an error will be returned.
func New(path string) (*Repository, error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return &Repository{Path: path, repo: r}, nil
}

// CreateTag tags a commit in a git repo.
//
// If prefix is a non-empty string, then the version will be prefixed with that string.
func (r *Repository) CreateTag(hash plumbing.Hash, name, message string) (*object.Tag, error) {
	if message == "" {
		message = "Release " + name
	}
	cfg, err := r.repo.Config()
	if err != nil {
		return nil, err
	}
	ref, err := r.repo.CreateTag(name, hash, &git.CreateTagOptions{
		Message: message,
		Tagger: &object.Signature{
			Email: cfg.Raw.Section("user").Option("email"),
			Name:  cfg.Raw.Section("user").Option("name"),
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, err
	}
	return r.repo.TagObject(ref.Hash())
}

func (r *Repository) DeleteTags(tags []*object.Tag) error {
	var errorMsg string
	for _, tag := range tags {
		if terr := r.repo.DeleteTag(tag.Name); terr != nil {
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
func (r *Repository) Head() (*object.Commit, error) {
	ref, err := r.repo.Head()
	if err != nil {
		return nil, err
	}
	return r.repo.CommitObject(ref.Hash())
}

// PushTag pushes tag to remote.
func (r *Repository) PushTag(tag *object.Tag, remote string) error {
	return r.PushTags([]*object.Tag{tag}, remote)
}

// PushTags pushes tags to the remote repository remote.
func (r *Repository) PushTags(tags []*object.Tag, remote string) error {
	refSpecs := make([]config.RefSpec, len(tags))
	for i, tag := range tags {
		refname := "refs/tags/" + tag.Name
		refSpecs[i] = config.RefSpec(refname + ":" + refname)
	}
	return r.repo.Push(&git.PushOptions{
		RemoteName: remote,
		RefSpecs:   refSpecs,
	})
}

// RevList returns a slice of commits from start to end.
func (r *Repository) RevList(s, e string, paths ...string) ([]*object.Commit, error) {
	if s == "" {
		return nil, errEmptyStart
	}
	start, err := r.parseRevOrHash(s)
	if err != nil {
		return nil, err
	}
	end, err := r.parseRevOrHash(e)
	if err != nil {
		return nil, err
	}
	cIter, err := r.repo.Log(&git.LogOptions{
		From:       start,
		PathFilter: matchPaths(paths),
	})
	if err != nil {
		return nil, err
	}
	commits := []*object.Commit{}
	if err := cIter.ForEach(func(c *object.Commit) error {
		if c.Hash == end {
			return errEOC
		}

		commits = append(commits, c)

		return nil
	}); err != nil && err != errEOC {
		return nil, err
	}

	return commits, nil
}

// Tags returns all tags that point to ancestors of rev.
//
// rev can be either a revision or a hash.
//
// prefix is a string prefix to filter tags with.
func (r *Repository) Tags(rev string, prefixes ...string) (tags []*plumbing.Reference, err error) {
	h, err := r.parseRevOrHash(rev)
	if err != nil {
		return nil, err
	}

	c, err := r.repo.CommitObject(h)
	if err != nil {
		return nil, err
	}

	tIter, err := r.repo.Tags()
	if err != nil {
		return nil, err
	}

	if err := tIter.ForEach(func(ref *plumbing.Reference) error {
		// resolve tag to commit
		var tc *object.Commit
		t, err := r.repo.TagObject(ref.Hash())
		switch err {
		case nil:
			// annotated tag
			tc, err = t.Commit()
			if err != nil {
				return err
			}
		case plumbing.ErrObjectNotFound:
			// light weight tag
			tc, err = r.repo.CommitObject(ref.Hash())
			if err != nil {
				return err
			}
		default:
			// some other error
			return err
		}

		// check if this tag matches one of our prefixes
		// "" prefix means no prefix
		if len(prefixes) > 0 {
			// strip refs/tags/ from name
			tagName := ref.Name().Short()
			if !hasPrefix(tagName, prefixes) {
				return nil
			}
		}

		// if the tagged commit is an ancestor of rev, then add it to tags
		ok, err := tc.IsAncestor(c)
		if err != nil {
			return err
		}
		if ok {
			tags = append(tags, ref)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return tags, nil
}

func (r *Repository) parseRevOrHash(s string) (plumbing.Hash, error) {
	if s != "" {
		if i, err := r.repo.ResolveRevision(plumbing.Revision(s)); err == nil {
			return *i, err
		}
	}
	return plumbing.NewHash(s), nil
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

func matchPaths(paths []string) func(string) bool {
	// return true if there are no paths to match against
	if len(paths) == 0 {
		return func(_ string) bool { return true }
	}

	return func(s string) (ok bool) {
		var matcher func(a, b string) bool
		for _, p := range paths {
			if strings.HasSuffix(p, "/") {
				// path is a directory, so do prefix matching
				matcher = func(a, b string) bool { return strings.HasPrefix(b, a) }
			} else {
				// path is a file so do strict matching
				matcher = func(a, b string) bool { return a == b }
			}
			ok = ok || matcher(p, s)
		}
		return
	}
}
