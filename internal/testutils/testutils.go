package testutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	GotaggerEmail = "Gotagger.Test@nowhere.com"
	GotaggerName  = "Gotagger Test"
)

type T interface {
	Errorf(string, ...interface{})
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Helper()
	Log(args ...interface{})
}

type FileCommit struct {
	Path     string
	Contents []byte
}

func CommitFile(t T, repo *git.Repository, path, filename, message string, data []byte) plumbing.Hash {
	t.Helper()

	return CommitFiles(t, repo, path, message, []FileCommit{{Path: filename, Contents: data}})
}

func CommitFiles(t T, repo *git.Repository, path, message string, files []FileCommit) plumbing.Hash {
	t.Helper()

	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	// create files
	for _, file := range files {
		fname := filepath.Join(path, file.Path)

		if err := os.MkdirAll(filepath.Dir(fname), 0700); err != nil {
			t.Fatal(err)
		}

		if err := ioutil.WriteFile(fname, file.Contents, 0600); err != nil {
			t.Fatal(err)
		}

		if _, err := w.Add(file.Path); err != nil {
			t.Fatal(err)
		}
	}

	h, err := w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Email: GotaggerEmail,
			Name:  GotaggerName,
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func CreateTag(t T, r *git.Repository, path, name string) {
	t.Helper()

	rev, err := r.ResolveRevision(plumbing.Revision("HEAD"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.CreateTag(name, *rev, &git.CreateTagOptions{
		Tagger: &object.Signature{
			Email: GotaggerEmail,
			Name:  GotaggerName,
			When:  time.Now(),
		},
		Message: name,
	}); err != nil {
		t.Fatal(err)
	}
}

func NewGitRepo(t T) (repo *git.Repository, path string, teardown func()) {
	t.Helper()

	path, teardown = TempDir(t)

	// init git repo
	var err error
	repo, err = git.PlainInit(path, false)
	if err != nil {
		t.Fatal(err)
	}

	return
}

func SimpleGitRepo(t T, repo *git.Repository, path string) {
	t.Helper()

	// commit a file
	h := CommitFile(t, repo, path, "foo", "feat: foo", []byte("foo"))
	// commit a change to the file
	CommitFile(t, repo, path, "foo", "feat: more foo", []byte("foo more"))

	// tag commit
	CreateTag(t, repo, path, "v1.0.0")

	// commit another change
	CommitFile(t, repo, path, "bar", "feat: bar\n\nThis is a great bar.", []byte("some bars too"))

	// create a new branch
	b := plumbing.NewBranchReferenceName("other")
	ref := plumbing.NewHashReference(b, h)
	if err := repo.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Checkout(&git.CheckoutOptions{
		Branch: b,
	}); err != nil {
		t.Fatal(err)
	}

	// commit to that branch and tag it
	CommitFile(t, repo, path, filepath.Join("baz", "foo"), "feat: commit a baz", []byte("baz"))
	CreateTag(t, repo, path, "v0.1.0")

	// back to master
	if err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.Master,
	}); err != nil {
		t.Fatal(err)
	}
}

func TempDir(t T) (tmpdir string, teardown func()) {
	t.Helper()

	var err error
	tmpdir, err = ioutil.TempDir("", "gotagger-")
	if err != nil {
		t.Fatal(err)
	}

	teardown = func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			t.Log(err)
		}
	}

	return
}
