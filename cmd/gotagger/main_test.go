// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/sassoftware/gotagger/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionInfo(t *testing.T) {
	got := versionInfo("1", "abcdefg", "Today")
	want := fmt.Sprintf(`gotagger:
 version     : 1
 build date  : Today
 git hash    : abcdefg
 go version  : %s
 go compiler : %s
 platform    : %s/%s
`, runtime.Version(), runtime.Compiler, runtime.GOOS, runtime.GOARCH)
	if want != got {
		t.Errorf("WANT:\n%s\nGOT:\n%s", want, got)
	}
}

type setupFunc func(t *testing.T, repo *git.Repository, path string)
type testFunc func(t *testing.T, repo *git.Repository, path string, stdout *bytes.Buffer, stderr *bytes.Buffer)

func TestGoTagger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		title            string
		args             []string
		wantOut, wantErr string
		wantRc           int
		extraSetup       setupFunc
		extraTest        testFunc
	}{
		{
			title: "version flag",
			args:  []string{"-version"},
			wantOut: fmt.Sprintf(`gotagger:
 version     : dev
 build date  : none
 git hash    : unknown
 go version  : %s
 go compiler : %s
 platform    : %s/%s
`, runtime.Version(), runtime.Compiler, runtime.GOOS, runtime.GOARCH),
			wantErr: "",
		},
		{
			title:   "empty prefix",
			args:    []string{"-prefix", ""},
			wantOut: "0.1.0\n",
			wantErr: "",
		},
		{
			title:   "alt prefix",
			args:    []string{"-prefix", "prefix-"},
			wantOut: "prefix-0.1.0\n",
		},
		{
			title:   "no options",
			args:    []string{},
			wantOut: "v1.1.0\n",
		},
		{
			title:   "no modules",
			args:    []string{"-modules=false"},
			wantOut: "v1.1.0\n",
		},
		{
			title:     "no release commit",
			args:      []string{"-release"},
			wantOut:   "v1.1.0\n",
			extraTest: assertNoTag("v1.1.0"),
		},
		{
			title:      "release commit",
			args:       []string{"-release"},
			wantOut:    "v1.1.0\n",
			extraSetup: createReleaseCommit,
			extraTest:  assertTag("v1.1.0"),
		},
		{
			title:     "push no release commit",
			args:      []string{"-push"},
			wantOut:   "v1.1.0\n",
			extraTest: assertNoTag("v1.1.0"),
		},
		{
			title:      "push release commit",
			args:       []string{"-push"},
			wantErr:    "failed with exit code 128: fatal: 'origin' does not appear to be a git repository",
			wantRc:     1,
			extraSetup: createReleaseCommit,
			extraTest:  assertNoTag("v1.1.0"),
		},
		{
			title:      "push to upstream",
			args:       []string{"-push", "-remote", "upstream"},
			wantErr:    "failed with exit code 128: fatal: 'upstream' does not appear to be a git repository",
			wantRc:     1,
			extraSetup: createReleaseCommit,
			extraTest:  assertNoTag("v1.1.0"),
		},
		{
			title:   "invalid flag",
			args:    []string{"-foo"},
			wantErr: "flag provided but not defined: -foo\n",
			wantRc:  1,
		},
		{
			title:     "cpuprofile",
			args:      []string{"-cpuprofile=cpu.prof"},
			wantOut:   "v1.1.0\n",
			extraTest: assertFileExists("cpu.prof"),
		},
		{
			title:   "cpuprofile fail",
			args:    []string{"-cpuprofile=foo/cpu.prof"},
			wantErr: "error: could not create CPU profile: open ",
			wantRc:  1,
		},
		{
			title:     "memprofile",
			args:      []string{"-memprofile=mem.prof"},
			wantOut:   "v1.1.0\n",
			extraTest: assertFileExists("mem.prof"),
		},
		{
			title:   "memprofile fail",
			args:    []string{"-memprofile=foo/mem.prof"},
			wantErr: "error: could not create memory profile: open ",
			wantRc:  1,
		},
		{
			title:   "invalid dirty option",
			args:    []string{"-dirty=foo"},
			wantErr: "error: invalid version increment 'foo'",
			wantRc:  1,
		},
		{
			title:   "dirty minor",
			args:    []string{"-dirty=minor"},
			wantOut: "v1.4.0\n",
			extraSetup: func(t *testing.T, repo *git.Repository, path string) {
				testutils.CreateTag(t, repo, path, "v1.3.0")
				require.NoError(t, os.WriteFile(filepath.Join(path, "foo"), []byte("foo\n"), 0600))
			},
		},
		{
			title:   "dirty patch",
			args:    []string{"-dirty=patch"},
			wantOut: "v1.3.1\n",
			extraSetup: func(t *testing.T, repo *git.Repository, path string) {
				testutils.CreateTag(t, repo, path, "v1.3.0")
				require.NoError(t, os.WriteFile(filepath.Join(path, "foo"), []byte("foo\n"), 0600))
			},
		},
		{
			title:   "dirty major",
			args:    []string{"-dirty=major"},
			wantErr: "error: -dirty value must be minor, patch, or none",
			wantRc:  1,
		},
		{
			title:     "force flag",
			args:      []string{"-force"},
			wantOut:   "v1.1.0\n",
			extraTest: assertTag("v1.1.0"),
		},
		{
			title:   "filter to baz subdirectory",
			args:    []string{"-path", "baz"},
			wantOut: "v0.1.0\n",
			extraSetup: func(t *testing.T, repo *git.Repository, path string) {
				// need to be on the "other" branch
				w, err := repo.Worktree()
				if err != nil {
					t.Fatal(err)
				}

				if err := w.Checkout(&git.CheckoutOptions{
					Branch: plumbing.NewBranchReferenceName("other"),
				}); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			title:   "path filter does not exist",
			args:    []string{"-path", "missing"},
			wantErr: "error: invalid path filter",
			wantRc:  1,
			extraSetup: func(t *testing.T, repo *git.Repository, path string) {
				// need to be on the "other" branch
				w, err := repo.Worktree()
				if err != nil {
					t.Fatal(err)
				}

				if err := w.Checkout(&git.CheckoutOptions{
					Branch: plumbing.NewBranchReferenceName("other"),
				}); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			title:   "path filter is a file",
			args:    []string{"-path", "foo.go"},
			wantErr: "error: invalid path filter",
			wantRc:  1,
			extraSetup: func(t *testing.T, repo *git.Repository, path string) {
				// need to be on the "other" branch
				w, err := repo.Worktree()
				if err != nil {
					t.Fatal(err)
				}

				if err := w.Checkout(&git.CheckoutOptions{
					Branch: plumbing.NewBranchReferenceName("other"),
				}); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()

			repo, path := testutils.NewGitRepo(t)

			testutils.SimpleGitRepo(t, repo, path)

			if tt.extraSetup != nil {
				tt.extraSetup(t, repo, path)
			}

			wantErr := tt.wantErr
			if strings.Contains(wantErr, "%s") {
				wantErr = fmt.Sprintf(tt.wantErr, path)
			}

			g, stdout, stderr := newGotagger(t, path, tt.args)
			assert.Equal(t, tt.wantRc, g.Run())
			if wantErr != "" {
				assert.Contains(t, stderr.String(), wantErr)
			} else {
				assert.Empty(t, stderr.String())
			}
			assert.Equal(t, tt.wantOut, stdout.String())
			if tt.extraTest != nil {
				tt.extraTest(t, repo, path, stdout, stderr)
			}
		})
	}
}

func newGotagger(t *testing.T, dir string, args []string) (*GoTagger, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	g := &GoTagger{
		Args:       args,
		Stdout:     out,
		Stderr:     err,
		WorkingDir: dir,
	}

	return g, out, err
}

func assertFileExists(fn string) testFunc {
	return func(t *testing.T, repo *git.Repository, path string, stdout, stderr *bytes.Buffer) {
		t.Helper()

		assert.FileExists(t, filepath.Join(path, fn))
	}
}

func assertNoTag(tag string) testFunc {
	return func(t *testing.T, repo *git.Repository, path string, stdout *bytes.Buffer, stderr *bytes.Buffer) {
		t.Helper()

		_, err := repo.Tag(tag)
		assert.EqualError(t, err, "tag not found")
	}
}

func assertTag(tag string) testFunc {
	return func(t *testing.T, repo *git.Repository, path string, stdout *bytes.Buffer, stderr *bytes.Buffer) {
		t.Helper()

		_, err := repo.Tag(tag)
		assert.NoError(t, err)
	}
}

func createReleaseCommit(t *testing.T, repo *git.Repository, path string) {
	t.Helper()

	testutils.CommitFile(t, repo, path, "CHANGELOG.md", "release: cut the v1.1.0 release", []byte(`changelog`))
}
