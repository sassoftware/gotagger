// Copyright (c) SAS Institute, Inc.

package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Masterminds/semver/v3"
)

// tests that inject a mock runner function
func mockRunGitCommand(t *testing.T, wantArgs []string, wantPath string) func([]string, string) ([]byte, error) {
	return func(args []string, path string) ([]byte, error) {
		if !reflect.DeepEqual(wantArgs, args) {
			t.Errorf("want %q, got %q", wantArgs, args)
		}
		if wantPath != path {
			t.Errorf("want '%s', got '%s'", wantPath, path)
		}
		return []byte{}, nil
	}
}

func TestCreateTag(t *testing.T) {
	tests := []struct {
		message        string
		signed, prefix bool
		version        *semver.Version
		want           []string
	}{
		{
			want: []string{"--git-dir", ".git", "tag", "-m", "Release 1.0.0", "1.0.0", "commit"},
		},
		{
			message: "message",
			want:    []string{"--git-dir", ".git", "tag", "-m", "message", "1.0.0", "commit"},
		},
		{
			message: "message",
			prefix:  true,
			want:    []string{"--git-dir", ".git", "tag", "-m", "message", "v1.0.0", "commit"},
		},
		{
			message: "message",
			signed:  true,
			want:    []string{"--git-dir", ".git", "tag", "-s", "-m", "message", "1.0.0", "commit"},
		},
		{
			message: "message",
			prefix:  true,
			signed:  true,
			want:    []string{"--git-dir", ".git", "tag", "-s", "-m", "message", "v1.0.0", "commit"},
		},
		{
			prefix: true,
			want:   []string{"--git-dir", ".git", "tag", "-m", "Release v1.0.0", "v1.0.0", "commit"},
		},
		{
			prefix: true,
			signed: true,
			want:   []string{"--git-dir", ".git", "tag", "-s", "-m", "Release v1.0.0", "v1.0.0", "commit"},
		},
		{
			signed: true,
			want:   []string{"--git-dir", ".git", "tag", "-s", "-m", "Release 1.0.0", "1.0.0", "commit"},
		},
	}

	t.Parallel()
	v := semver.MustParse("1.0.0")
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := Repo{"path", ".git", mockRunGitCommand(t, tt.want, "path")}
			_ = r.CreateTag("commit", v, tt.message, tt.signed, tt.prefix)
		})
	}
}

func TestPushTags(t *testing.T) {
	v := semver.MustParse("1.0.0")
	wantArgs := []string{"--git-dir", ".git", "push", "origin", "refs/tags/v1.0.0:refs/tags/v1.0.0"}
	wantPath := "path"
	repo := Repo{"path", ".git", mockRunGitCommand(t, wantArgs, wantPath)}
	_ = repo.PushTag(v, "origin")
}

// tests that use a real git repo
func runTestGitCommand(t *testing.T, path string, args ...string) {
	_, err := runGitCommand(args, path)
	if err != nil {
		t.Fatal(err)
	}
}

func commitFile(t *testing.T, path, filename, message string, data []byte) {
	if err := ioutil.WriteFile(filepath.Join(path, filename), data, 0600); err != nil {
		t.Fatal(err)
	}
	runTestGitCommand(t, path, "add", ".")
	runTestGitCommand(t, path, "commit", "-m", message)
}

func makeGitRepo(t *testing.T) string {
	path, err := ioutil.TempDir("", "gotagger-")
	if err != nil {
		t.Fatal(err)
	}
	// init git repo
	runTestGitCommand(t, path, "init")
	// commit a file
	commitFile(t, path, "foo", "Commit foo", []byte("foo"))
	// commit a change to the file
	commitFile(t, path, "foo", "Commit more foo", []byte("foo more"))
	// tag commit
	runTestGitCommand(t, path, "tag", "-m", "v1.0.0", "v1.0.0")
	// commit another change
	commitFile(t, path, "bar", "Commit bar\n\nThis is a great bar.", []byte("some bars too"))
	return path
}

func Test_getGitDirectory(t *testing.T) {
	dir := makeGitRepo(t)
	defer func() { os.RemoveAll(dir) }()
	got, err := getGitDirectory(dir)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if ".git" != got {
		t.Errorf("Want '.git', got %s", got)
	}
}

func Test_getGitDirectoryNoGit(t *testing.T) {
	dir, err := ioutil.TempDir("", "gotagger")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.RemoveAll(dir)
	}()
	if _, err := getGitDirectory(dir); err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func Test_parseCommits(t *testing.T) {
	commit := []byte("hash\x00This is a subject\x00And this is a body\nthat is across several\nlines\x00Trailer-One: value one|TrailerTwo: value two\x00")
	want := Commit{
		Hash:     "hash",
		Subject:  "This is a subject",
		Body:     "And this is a body\nthat is across several\nlines",
		Trailers: []string{"Trailer-One: value one", "TrailerTwo: value two"},
		Tags:     []*semver.Version{},
	}
	got := parseCommits([][]byte{commit})[0]
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %+v, got %+v", want, got)
	}
}

func Test_parseTags(t *testing.T) {
	tests := []struct {
		refname string
		want    []*semver.Version
	}{
		{refname: "(HEAD -> master)", want: []*semver.Version{}},
		{refname: "(master, origin/master)", want: []*semver.Version{}},
		{refname: "(master, 1.2)", want: []*semver.Version{}},
		{refname: "(master, 1.2.2)", want: []*semver.Version{}},
		{refname: "(master, refs/tags/1.2)", want: []*semver.Version{semver.MustParse("1.2")}},
		{refname: "(master, refs/tags/1.2.2)", want: []*semver.Version{semver.MustParse("1.2.2")}},
		{refname: "(master, refs/tags/v1.2.2)", want: []*semver.Version{semver.MustParse("1.2.2")}},
		{refname: "(refs/tags/1.2, master)", want: []*semver.Version{semver.MustParse("1.2")}},
		{refname: "(refs/tags/1.2.2, master)", want: []*semver.Version{semver.MustParse("1.2.2")}},
		{refname: "(refs/tags/v1.2.2, master)", want: []*semver.Version{semver.MustParse("1.2.2")}},
		{
			refname: "(refs/tags/1.2.3, refs/tags/v1.2.2, master)",
			want:    []*semver.Version{semver.MustParse("1.2.3"), semver.MustParse("1.2.2")},
		},
		{
			refname: "(refs/tags/1.2.2, refs/tags/v1.2.3, master)",
			want:    []*semver.Version{semver.MustParse("1.2.3"), semver.MustParse("1.2.2")},
		},
		{
			refname: "(refs/tags/v1.2.2, refs/tags/v1.2.3, master)",
			want:    []*semver.Version{semver.MustParse("1.2.3"), semver.MustParse("1.2.2")},
		},
	}
	t.Parallel()
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d:%s-%q", i, tt.refname, tt.want), func(t *testing.T) {
			got := parseTags(tt.refname)
			if len(got) != len(tt.want) {
				t.Errorf("want %d tags, got %d tags", len(tt.want), len(got))
			}
			for i, v := range tt.want {
				if !v.Equal(got[i]) {
					t.Errorf("want %s at index %d, got %s", v, i, got[i])
				}
			}
		})
	}
}

func TestRevList(t *testing.T) {
	tests := []struct {
		start, end string
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
	}
	dir := makeGitRepo(t)
	defer func() { os.RemoveAll(dir) }()
	r, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Parallel()
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d:%s-%s-%d", i, tt.start, tt.end, tt.want), func(t *testing.T) {
			commits, err := r.RevList(tt.start, tt.end)
			if err != nil {
				t.Fatal(err)
			}
			if tt.want != len(commits) {
				t.Errorf("want %d commits, got %d", tt.want, len(commits))
			}
		})
	}
}

func TestTags(t *testing.T) {
	dir := makeGitRepo(t)
	defer func() { os.RemoveAll(dir) }()
	r, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := r.Tags()
	if err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			t.Fatal(string(eerr.Stderr))
		}
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Errorf("want %d commits, got %d", 1, len(commits))
	}
	commit := commits[0]
	if commit.Body != "" {
		t.Errorf("tag commit has body")
	}
	if len(commit.Tags) != 1 {
		t.Errorf("want 1 tag, got %d", len(commit.Tags))
	}
	want := semver.MustParse("v1.0.0")
	got := commit.Tags[0]
	if !want.Equal(got) {
		t.Errorf("want %s, got %s", want, got)
	}
}
