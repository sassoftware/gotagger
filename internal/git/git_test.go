// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/sassoftware/gotagger/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	repo, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	testutils.SimpleGitRepo(t, repo, path)

	if _, err := New(path); err != nil {
		t.Errorf("New(%q) returned an error: %v", path, err)
	}
}

func TestNew_no_repo(t *testing.T) {
	dir, err := ioutil.TempDir("", "gotagger")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.RemoveAll(dir) }()

	if _, err := New(dir); err == nil {
		t.Errorf("New(%q) did not return an error.", dir)
	}
}

func TestCreateTag(t *testing.T) {
	tests := []struct {
		message string
		signed  bool
		want    []string
	}{
		{
			want: []string{"--git-dir", ".git", "tag", "-m", "Release v1.0.0", "v1.0.0", "hash"},
		},
		{
			message: "message",
			want:    []string{"--git-dir", ".git", "tag", "-m", "message", "v1.0.0", "hash"},
		},
		{
			message: "message",
			signed:  true,
			want:    []string{"--git-dir", ".git", "tag", "-s", "-m", "message", "v1.0.0", "hash"},
		},
		{
			signed: true,
			want:   []string{"--git-dir", ".git", "tag", "-s", "-m", "Release v1.0.0", "v1.0.0", "hash"},
		},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.want), func(t *testing.T) {
			tt := tt

			r := &Repository{GitDir: ".git", Path: "path", runner: mockRunGitCommand(t, tt.want, "path"), logger: logr.Discard()}
			_ = r.CreateTag("hash", "v1.0.0", tt.message, tt.signed)
		})
	}
}

func TestHead(t *testing.T) {
	repo, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	testutils.SimpleGitRepo(t, repo, path)

	r, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	c, err := r.Head()
	if err != nil {
		t.Errorf("Head() returned an error: %v", err)
	}

	if got, want := c.Message(), "feat: bar\n\nThis is a great bar."; got != want {
		t.Errorf("Head() returned %q, want %q", got, want)
	}
}

func TestIsDirty(t *testing.T) {
	t.Parallel()

	t.Run("clean chekcout", func(t *testing.T) {
		repo, path, teardown := testutils.NewGitRepo(t)
		defer teardown()

		testutils.SimpleGitRepo(t, repo, path)

		r, err := New(path)
		require.NoError(t, err)

		if got, err := r.IsDirty(); assert.NoError(t, err) {
			assert.False(t, got)
		}
	})

	t.Run("untracked file", func(t *testing.T) {
		repo, path, teardown := testutils.NewGitRepo(t)
		defer teardown()

		testutils.SimpleGitRepo(t, repo, path)

		r, err := New(path)
		require.NoError(t, err)
		require.NoError(t, ioutil.WriteFile(filepath.Join(path, "untracked"), []byte("foo\n"), 0600))

		if got, err := r.IsDirty(); assert.NoError(t, err) {
			assert.True(t, got)
		}

		// stage it
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("foo")
		require.NoError(t, err)

		if got, err := r.IsDirty(); assert.NoError(t, err) {
			assert.True(t, got)
		}
	})

	t.Run("changed file", func(t *testing.T) {
		repo, path, teardown := testutils.NewGitRepo(t)
		defer teardown()

		testutils.SimpleGitRepo(t, repo, path)

		r, err := New(path)
		require.NoError(t, err)
		require.NoError(t, ioutil.WriteFile(filepath.Join(path, "foo"), []byte("some new content\n"), 0600))

		if got, err := r.IsDirty(); assert.NoError(t, err) {
			assert.True(t, got)
		}
	})

	t.Run("staged file", func(t *testing.T) {
		repo, path, teardown := testutils.NewGitRepo(t)
		defer teardown()

		testutils.SimpleGitRepo(t, repo, path)

		r, err := New(path)
		require.NoError(t, err)
		require.NoError(t, ioutil.WriteFile(filepath.Join(path, "foo"), []byte("some new content\n"), 0600))

		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("foo")
		require.NoError(t, err)

		if got, err := r.IsDirty(); assert.NoError(t, err) {
			assert.True(t, got)
		}
	})
}

func TestPushTags(t *testing.T) {
	wantArgs := []string{"--git-dir", ".git", "push", "origin", "refs/tags/v1.0.0:refs/tags/v1.0.0"}
	wantPath := "path"
	r := &Repository{GitDir: ".git", Path: "path", runner: mockRunGitCommand(t, wantArgs, wantPath), logger: logr.Discard()}
	_ = r.PushTags([]string{"v1.0.0"}, "origin")
}

func TestPushTag_no_remote(t *testing.T) {
	repo, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	testutils.SimpleGitRepo(t, repo, path)

	r, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}

	if err := r.CreateTag(head.Hash, "tag", "", false); err != nil {
		t.Fatal(err)
	}

	// we don't expect this to work, since no remote is configured
	if err := r.PushTags([]string{"tag"}, "remote"); assert.Error(t, err) {
		assert.Contains(t, err.Error(), "fatal: 'remote' does not appear to be a git repository")
	}

}

func TestRevList(t *testing.T) {
	tests := []struct {
		start, end string
		paths      []string
		want       int
	}{
		{
			start: "HEAD",
			want:  3,
		},
		{
			start: "HEAD",
			end:   "HEAD~2",
			want:  2,
		},
		{
			start: "HEAD",
			end:   "HEAD~1",
			want:  1,
		},
		{
			start: "HEAD",
			end:   "HEAD",
			want:  0,
		},
		{
			start: "HEAD",
			paths: []string{"foo"},
			want:  2,
		},
		{
			start: "HEAD",
			paths: []string{"bar"},
			want:  1,
		},
		{
			start: "HEAD",
			paths: []string{"bar", "foo"},
			want:  3,
		},
		{
			start: "other",
			paths: []string{"baz"},
			want:  1,
		},
		{
			start: "other",
			paths: []string{"baz/"},
			want:  1,
		},
	}

	repo, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	testutils.SimpleGitRepo(t, repo, path)

	r, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d:%v", i, tt), func(t *testing.T) {
			if commits, err := r.RevList(tt.start, tt.end, tt.paths...); assert.NoError(t, err) {
				assert.Equal(t, tt.want, len(commits))
			}
		})
	}
}

func TestRevList_one_commit(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	repo, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	testutils.CommitFile(t, repo, path, "foo", "add foo", []byte("contents"))

	r, err := New(path)
	require.NoError(err)

	if commits, err := r.RevList("HEAD", ""); assert.NoError(err) {
		assert.Equal(1, len(commits))
	}

	if _, err := r.RevList("HEAD", "HEAD~1"); assert.Error(err) {
		assert.Contains(err.Error(), "bad revision '^HEAD~1")
	}
}

func TestRevList_empty_repo(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	_, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	r, err := New(path)
	require.NoError(err)

	if _, err := r.RevList("HEAD", ""); assert.Error(err) {
		assert.Contains(err.Error(), "unknown revision")
	}

	if _, err := r.RevList("HEAD", "HEAD^"); assert.Error(err) {
		assert.Contains(err.Error(), "unknown revision")
	}
}

func TestRevList_empty_start(t *testing.T) {
	repo, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	testutils.SimpleGitRepo(t, repo, path)

	r, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.RevList("", "")
	if got, want := err, errEmptyStart; got != want {
		t.Errorf("RevList(\"\", \"\") returned an error %v, want %v", got, want)
	}
}

func TestTags(t *testing.T) {
	repo, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	testutils.SimpleGitRepo(t, repo, path)

	r, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	tags, err := r.Tags("master")
	if err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			t.Fatal(string(eerr.Stderr))
		}
		t.Fatal(err)
	}

	if got, want := len(tags), 1; got != want {
		t.Errorf("Tags returned %d tags, want %d", got, want)
	} else if got, want := tags[0], "v1.0.0"; got != want {
		t.Errorf("Tags returned %s, want %s", got, want)
	}
}

func TestTags_prefixes(t *testing.T) {
	repo, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	testutils.SimpleGitRepo(t, repo, path)

	// add a submodule tag
	submodule := "sub/module"
	testutils.CommitFile(t, repo, path, filepath.Join("sub", "module", "file"), "feat: add submodule", []byte("data"))
	testutils.CreateTag(t, repo, path, submodule+"/v0.1.0")

	r, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	tags, err := r.Tags("master", submodule+"/")
	if err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			t.Fatal(string(eerr.Stderr))
		}
		t.Fatal(err)
	}

	if got, want := len(tags), 1; got != want {
		t.Errorf("Tags returned %d tags, want %d", got, want)
	} else if got, want := tags[0], submodule+"/v0.1.0"; got != want {
		t.Errorf("Tags returned %s, want %s", got, want)
	}
}

func Test_hasPrefix(t *testing.T) {
	tests := []struct {
		title    string
		version  string
		prefixes []string
		want     bool
	}{
		{
			"match v1.0.0",
			"v1.0.0",
			[]string{"v"},
			true,
		},
		{
			"match 1.0.0",
			"1.0.0",
			[]string{""},
			true,
		},
		{
			"do not match v1.0.0",
			"v1.0.0",
			[]string{"foo"},
			false,
		},
		{
			"do not match 1.0.0",
			"1.0.0",
			[]string{"v"},
			false,
		},
		{
			"multiple prefixes",
			"v1.0.0",
			[]string{"", "v"},
			true,
		},
		{
			"multiple prefixes",
			"1.0.0",
			[]string{"", "v"},
			true,
		},
	}

	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.title, func(t *testing.T) {
			if got, want := hasPrefix(tt.version, tt.prefixes), tt.want; got != want {
				t.Errorf("hasPrefix returned %v, want %v", got, want)
			}
		})
	}
}

// tests that inject a mock runner function
func mockRunGitCommand(t *testing.T, wantArgs []string, wantPath string) func([]string, string) (string, error) {
	return func(args []string, path string) (string, error) {
		assert.Equal(t, wantArgs, args)
		assert.Equal(t, wantPath, path)
		return "", nil
	}
}
