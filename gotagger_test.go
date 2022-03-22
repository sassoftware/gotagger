// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gotagger

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	sgit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sassoftware/gotagger/internal/commit"
	"github.com/sassoftware/gotagger/internal/git"
	"github.com/sassoftware/gotagger/internal/testutils"
	"github.com/sassoftware/gotagger/mapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type setupRepoFunc func(testutils.T, *sgit.Repository, string)

func TestGotagger_latestModule(t *testing.T) {
	tests := []struct {
		disabled bool
		title    string
		module   module
		repoFunc setupRepoFunc
		want     string
	}{
		{
			title:    "no latest",
			module:   module{".", "foo", ""},
			repoFunc: simpleGoRepo,
			want:     "v1.0.0",
		},
		{
			title:    "sub module",
			module:   module{filepath.Join("sub", "module"), "foo/sub/module", "sub/module/"},
			repoFunc: simpleGoRepo,
			want:     "v0.1.0",
		},
		{
			title:    "latest foo v1 directory",
			module:   module{".", "foo", ""},
			repoFunc: v2DirGitRepo,
			want:     "v1.0.0",
		},
		{
			title:    "latest bar v1 directory",
			module:   module{"bar", "foo/bar", "bar/"},
			repoFunc: v2DirGitRepo,
			want:     "v1.0.0",
		},
		{
			title:    "latest foo v2 directory",
			module:   module{"v2", "foo/v2", ""},
			repoFunc: v2DirGitRepo,
			want:     "v2.0.0",
		},
		{
			title:    "latest foo/bar v2 directory",
			module:   module{filepath.Join("bar", "v2"), "foo/bar/v2", "bar/"},
			repoFunc: v2DirGitRepo,
			want:     "v2.0.0",
		},
		{
			title:    "breaking change in v1 module",
			module:   module{".", "foo/v2", ""},
			repoFunc: untaggedV2Repo,
			want:     "v1.0.0",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()

			if tt.disabled {
				t.Skip("disabled in test code")
			}

			g, repo, path, teardown := newGotagger(t)
			defer teardown()

			tt.repoFunc(t, repo, path)

			tags, err := g.repo.Tags("HEAD", tt.module.prefix+"v")
			require.NoError(t, err)

			if got, _, err := g.latestModule(tt.module, tags); assert.NoError(t, err) {
				assert.Equal(t, tt.want, got.Original())
			}
		})
	}
}

func TestGotagger_ModuleVersion(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	simpleGoRepo(t, repo, path)

	if v, err := g.ModuleVersions("foo/sub/module"); assert.NoError(t, err) {
		assert.Equal(t, []string{"sub/module/v0.1.1"}, v)
	}
}

func TestGotagger_ModuleVersions_PreMajor(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	// set PreMajor
	g.Config.PreMajor = true

	simpleGoRepo(t, repo, path)

	// make a breaking change to foo
	testutils.CommitFile(t, repo, path, "foo.go", "feat!: breaking change", []byte(`contents`))

	// major version should rev
	if v, err := g.ModuleVersions("foo"); assert.NoError(t, err) {
		assert.Equal(t, []string{"v2.0.0"}, v)
	}

	// make a breaking change to sub/module
	testutils.CommitFile(t, repo, path, filepath.Join("sub", "module", "file"), "feat!: breaking change", []byte(`contents`))

	// version should not rev major
	if v, err := g.ModuleVersions("foo/sub/module"); assert.NoError(t, err) {
		assert.Equal(t, []string{"sub/module/v0.2.0"}, v)
	}
}

func TestGotagger_versioning(t *testing.T) {
	tests := []struct {
		disabled bool
		title    string
		prefix   string
		repoFunc setupRepoFunc
		message  string
		files    []testutils.FileCommit
		checks   map[string]gotaggerCheckFunc
	}{
		{
			title:    "v-prefix tags",
			prefix:   "v",
			repoFunc: mixedTagRepo,
			message:  "release: the foos\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.0.1"}),
				"Version": checkVersion("v1.0.1"),
			},
		},
		{
			title:    "empty prefix tags",
			prefix:   "",
			repoFunc: mixedTagRepo,
			message:  "release: the bars\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"0.2.0"}),
				"Version": checkVersion("0.2.0"),
			},
		},
		{
			title:    "v-prefix tags go mod",
			prefix:   "v",
			repoFunc: mixedTagGoRepo,
			message:  "release: the foos\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.1.0"}),
				"Version": checkVersion("v1.1.0"),
			},
		},
		{
			title:    "empty prefix tags go mod",
			prefix:   "",
			repoFunc: mixedTagGoRepo,
			message:  "release: the bars\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"1.1.0"}),
				"Version": checkVersion("1.1.0"),
			},
		},
		{
			title:  "release root v1 on master implicit",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV1GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.1.0"}),
				"Version": checkVersion("v1.1.0"),
			},
		},
		{
			title:  "release root v1 on master explicit",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV1GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n\nModules: foo\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.1.0"}),
				"Version": checkVersion("v1.1.0"),
			},
		},
		{
			title:  "release bar v1 on master",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV1GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: the bars\n\nModules: foo/bar",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"bar/v1.1.0"}),
				"Version": checkVersion("v1.0.0"),
			},
		},
		{
			title:  "release all v1 on master",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV1GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: all the things\n\nModules: foo, foo/bar",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.1.0", "bar/v1.1.0"}),
				"Version": checkVersion("v1.1.0"),
			},
		},
		{
			title:  "release root v2 on master implicit",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV2GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v2.1.0"}),
				"Version": checkVersion("v2.1.0"),
			},
		},
		{
			title:  "release root v2 on master explicit",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV2GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n\nModules: foo/v2\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v2.1.0"}),
				"Version": checkVersion("v2.1.0"),
			},
		},
		{
			title:  "release bar v2 on master",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV2GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: the bars\n\nModules: foo/bar/v2",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"bar/v2.1.0"}),
				"Version": checkVersion("v2.0.0"),
			},
		},
		{
			title:  "release all v2 on master",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV2GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: all the things\n\nModules: foo/bar/v2, foo/v2",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"bar/v2.1.0", "v2.1.0"}),
				"Version": checkVersion("v2.1.0"),
			},
		},
		{
			title:  "release foo v1 implicit directory",
			prefix: "v",
			repoFunc: func(t testutils.T, repo *sgit.Repository, path string) {
				v2DirGitRepo(t, repo, path)

				// update foo
				testutils.CommitFile(t, repo, path, "foo.go", "feat: add foo.go\n", []byte("foo\n"))
			},
			message: "release: the foos\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.1.0"}),
				"Version": checkVersion("v1.1.0"),
			},
		},
		{
			title:  "release foo v1 explicit directory",
			prefix: "v",
			repoFunc: func(t testutils.T, repo *sgit.Repository, path string) {
				v2DirGitRepo(t, repo, path)

				// update foo
				testutils.CommitFile(t, repo, path, "foo.go", "feat: add foo.go\n", []byte("foo\n"))
			},
			message: "release: the foos\n\nModules: foo\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.1.0"}),
				"Version": checkVersion("v1.1.0"),
			},
		},
		{
			title:  "release foo v2 explicit directory",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("v2", "foo.go"), "feat: add v2/foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n\nModules: foo/v2\n",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("v2", "CHANGELOG.md"),
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v2.1.0"}),
				"Version": checkVersion("v1.0.0"),
			},
		},
		{
			title:  "release bar v1 directory",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: the bars\n\nModules: foo/bar\n",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"bar/v1.1.0"}),
				"Version": checkVersion("v1.0.0"),
			},
		},
		{
			title:  "release bar v2 directory",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("bar", "v2", "bar.go"), "feat: add bar/v2/bar.go", []byte("bar\n"))
			},
			message: "release: the bars\n\nModules: foo/bar/v2\n",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("bar", "v2", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"bar/v2.1.0"}),
				"Version": checkVersion("v1.0.0"),
			},
		},
		{
			title:  "release all v1 directory",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: all the v1 things\n\nModules: foo, foo/bar",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.1.0", "bar/v1.1.0"}),
				"Version": checkVersion("v1.1.0"),
			},
		},
		{
			title:  "release all v2 directory",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("v2", "foo.go"), "feat: add v2/foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "v2", "bar.go"), "feat: add bar/v2/bar.go", []byte("bar\n"))
			},
			message: "release: all the v2 things\n\nModules: foo/v2, foo/bar/v2",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("v2", "CHANGELOG.md"),
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "v2", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v2.1.0", "bar/v2.1.0"}),
				"Version": checkVersion("v1.0.0"),
			},
		},
		{
			title:  "release all directory",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))

				testutils.CommitFile(t, r, p, filepath.Join("v2", "foo.go"), "feat: add v2/foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "v2", "bar.go"), "feat: add bar/v2/bar.go", []byte("bar\n"))
			},
			message: "release: all the things\n\nModules: foo, foo/bar, foo/v2, foo/bar/v2\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
				{
					Path:     filepath.Join("v2", "CHANGELOG.md"),
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "v2", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.1.0", "bar/v1.1.0", "v2.1.0", "bar/v2.1.0"}),
				"Version": checkVersion("v1.1.0"),
			},
		},
		{
			title:  "release main module when submodules have feats",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				simpleGoRepo(t, r, p)
				testutils.CreateTag(t, r, p, "v1.1.0")
				testutils.CommitFile(t, r, p, "sub/module/other", "feat: add other submodule file", []byte("contents"))
				testutils.CommitFile(t, r, p, "foo.go", "fix: add file to foo", []byte("foo"))
			},
			message: "release: foo v1.1.1\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.1.1"}),
				"Version": checkVersion("v1.1.1"),
			},
		},
		{
			title:  "multi-module commit",
			prefix: "v",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				simpleGoRepo(t, r, p)
				testutils.CreateTag(t, r, p, "v1.1.0")
				testutils.CommitFile(t, r, p, "fix: bar", "bar", []byte(`fix bar\n`))
				testutils.CommitFiles(t, r, p, "feat: change both modules", []testutils.FileCommit{
					{
						Path:     "sub/module/file",
						Contents: []byte(`changed contents\n`),
					},
					{
						Path:     "zed",
						Contents: []byte(`zed\n`),
					},
				})
			},
			message: "release: foo v1.2.0",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			checks: map[string]gotaggerCheckFunc{
				"TagRepo": checkTagRepo([]string{"v1.2.0"}),
				"Version": checkVersion("v1.2.0"),
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()

			if tt.disabled {
				t.Skip("disabled in test code")
			}

			g, repo, path, teardown := newGotagger(t)
			defer teardown()

			tt.repoFunc(t, repo, path)

			// create a release commit
			testutils.CommitFiles(t, repo, path, tt.message, tt.files)

			g.Config.VersionPrefix = tt.prefix
			for name, check := range tt.checks {
				t.Run(name, func(t *testing.T) {
					check(t, g)
				})
			}
		})
	}
}

type gotaggerCheckFunc func(*testing.T, *Gotagger)

func checkTagRepo(want []string) gotaggerCheckFunc {
	return func(t *testing.T, g *Gotagger) {
		if versions, err := g.TagRepo(); assert.NoError(t, err) {
			assert.Equal(t, want, versions)
		}
	}
}

// checkVersion only works for default version prefix.
func checkVersion(want string) gotaggerCheckFunc {
	return func(t *testing.T, g *Gotagger) {
		if version, err := g.Version(); assert.NoError(t, err) {
			assert.Equal(t, want, version)
		}
	}
}

func TestGotagger_TagRepo_ignore_modules(t *testing.T) {
	tests := []struct {
		title    string
		repoFunc setupRepoFunc
		message  string
		files    []testutils.FileCommit
		want     []string
	}{
		{
			title: "release root v1 on master implicit",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV1GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			want: []string{"v1.1.0"},
		},
		{
			title: "release root v1 on master explicit",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV1GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n\nModules: foo\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			want: []string{"v1.1.0"},
		},
		{
			title: "release bar v1 on master",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV1GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: the bars\n\nModules: foo/bar",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			want: []string{"v1.1.0"},
		},
		{
			title: "release all v1 on master",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV1GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: all the things\n\nModules: foo, foo/bar",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			want: []string{"v1.1.0"},
		},
		{
			title: "release root v2 on master implicit",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV2GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release root v2 on master explicit",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV2GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n\nModules: foo/v2\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release bar v2 on master",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV2GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: the bars\n\nModules: foo/bar/v2",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release all v2 on master",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				masterV2GitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: all the things\n\nModules: foo/bar/v2, foo/v2",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release foo v1 implicit directory",
			repoFunc: func(t testutils.T, repo *sgit.Repository, path string) {
				v2DirGitRepo(t, repo, path)

				// update foo
				testutils.CommitFile(t, repo, path, "foo.go", "feat: add foo.go\n", []byte("foo\n"))
			},
			message: "release: the foos\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release foo v1 explicit directory",
			repoFunc: func(t testutils.T, repo *sgit.Repository, path string) {
				v2DirGitRepo(t, repo, path)

				// update foo
				testutils.CommitFile(t, repo, path, "foo.go", "feat: add foo.go\n", []byte("foo\n"))
			},
			message: "release: the foos\n\nModules: foo\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release foo v2 explicit directory",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("v2", "foo.go"), "feat: add v2/foo.go", []byte("foo\n"))
			},
			message: "release: the foos\n\nModules: foo/v2\n",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("v2", "CHANGELOG.md"),
					Contents: []byte("# Foo Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release bar v1 directory",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: the bars\n\nModules: foo/bar\n",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release bar v2 directory",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("bar", "v2", "bar.go"), "feat: add bar/v2/bar.go", []byte("bar\n"))
			},
			message: "release: the bars\n\nModules: foo/bar/v2\n",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("bar", "v2", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release all v1 directory",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))
			},
			message: "release: all the v1 things\n\nModules: foo, foo/bar",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release all v2 directory",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, filepath.Join("v2", "foo.go"), "feat: add v2/foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "v2", "bar.go"), "feat: add bar/v2/bar.go", []byte("bar\n"))
			},
			message: "release: all the v2 things\n\nModules: foo/v2, foo/bar/v2",
			files: []testutils.FileCommit{
				{
					Path:     filepath.Join("v2", "CHANGELOG.md"),
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "v2", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
		{
			title: "release all directory",
			repoFunc: func(t testutils.T, r *sgit.Repository, p string) {
				v2DirGitRepo(t, r, p)

				testutils.CommitFile(t, r, p, "foo.go", "feat: add foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "bar.go"), "feat: add bar/bar.go", []byte("bar\n"))

				testutils.CommitFile(t, r, p, filepath.Join("v2", "foo.go"), "feat: add v2/foo.go", []byte("foo\n"))
				testutils.CommitFile(t, r, p, filepath.Join("bar", "v2", "bar.go"), "feat: add bar/v2/bar.go", []byte("bar\n"))
			},
			message: "release: all the things\n\nModules: foo, foo/bar, foo/v2, foo/bar/v2\n",
			files: []testutils.FileCommit{
				{
					Path:     "CHANGELOG.md",
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
				{
					Path:     filepath.Join("v2", "CHANGELOG.md"),
					Contents: []byte("# Foo Change Log\n"),
				},
				{
					Path:     filepath.Join("bar", "v2", "CHANGELOG.md"),
					Contents: []byte("# Bar Change Log\n"),
				},
			},
			want: []string{"v3.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()

			g, repo, path, teardown := newGotagger(t)
			defer teardown()

			tt.repoFunc(t, repo, path)

			// create a release commit
			testutils.CommitFiles(t, repo, path, tt.message, tt.files)

			g.Config.IgnoreModules = true
			if versions, err := g.TagRepo(); assert.NoError(t, err) {
				assert.Equal(t, tt.want, versions)
			}
		})
	}
}

func TestGotagger_TagRepo_force(t *testing.T) {
	t.Parallel()

	t.Run("force-false", func(t *testing.T) {
		t.Parallel()
		g, repo, path, teardown := newGotagger(t)
		defer teardown()

		simpleGoRepo(t, repo, path)

		// gotagger should not create a tag, because HEAD is not a "release" commit
		g.Config.CreateTag = true
		if versions, err := g.TagRepo(); assert.NoError(t, err) {
			assert.Equal(t, []string{"v1.1.0"}, versions)
			_, gerr := repo.Tag("v1.1.0")
			assert.Error(t, gerr)
		}
	})

	t.Run("force-true", func(t *testing.T) {
		t.Parallel()
		g, repo, path, teardown := newGotagger(t)
		defer teardown()

		simpleGoRepo(t, repo, path)

		// gotagger should create a tag, even though HEAD is not a "release" commit
		g.Config.CreateTag = true
		g.Config.Force = true
		if versions, err := g.TagRepo(); assert.NoError(t, err) {
			assert.Equal(t, []string{"v1.1.0"}, versions)
			_, gerr := repo.Tag("v1.1.0")
			assert.NoError(t, gerr)
		}
	})
}

func TestGotagger_TagRepo_validation_extra(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	masterV1GitRepo(t, repo, path)

	commitMsg := `release: extra module

Modules: foo/bar, foo
`
	testutils.CommitFile(t, repo, path, "CHANGELOG.md", commitMsg, []byte(`changes`))

	g.Config.CreateTag = true
	_, err := g.TagRepo()
	assert.EqualError(t, err, "module validation failed:\nmodules not changed by commit: foo/bar")
}

func TestGotagger_TagRepo_validation_missing(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	masterV1GitRepo(t, repo, path)

	if err := ioutil.WriteFile(filepath.Join(path, "CHANGELOG.md"), []byte(`contents`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(path, "bar", "CHANGELOG.md"), []byte(`contents`), 0o600); err != nil {
		t.Fatal(err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := wt.Add("CHANGELOG.md"); err != nil {
		t.Fatal(err)
	}

	if _, err := wt.Add(filepath.Join("bar", "CHANGELOG.md")); err != nil {
		t.Fatal(err)
	}

	if _, err := wt.Commit("release: missing module\n", &sgit.CommitOptions{
		Author: &object.Signature{
			Email: testutils.GotaggerEmail,
			Name:  testutils.GotaggerName,
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatal(err)
	}

	g.Config.CreateTag = true
	_, err = g.TagRepo()
	assert.EqualError(t, err, "module validation failed:\nchanged modules not released by commit: foo/bar")
}

func TestGotagger_Version(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	simpleGoRepo(t, repo, path)

	if v, err := g.Version(); assert.NoError(t, err) {
		assert.Equal(t, "v1.1.0", v)
	}
}

func TestGotagger_Version_no_module(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	testutils.SimpleGitRepo(t, repo, path)

	if v, err := g.Version(); assert.NoError(t, err) {
		assert.Equal(t, "v1.1.0", v)
	}
}

func TestGotagger_Version_tag_head(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	simpleGoRepo(t, repo, path)

	// tag HEAD higher than what gotagger would return
	version := "v1.10.0"
	testutils.CreateTag(t, repo, path, version)

	if got, err := g.Version(); assert.NoError(t, err) {
		assert.Equal(t, version, got)
	}
}

func TestGotagger_Version_IgnoreModules(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	// set PreMajor
	g.Config.IgnoreModules = true

	simpleGoRepo(t, repo, path)

	// create a v2 tag
	testutils.CreateTag(t, repo, path, "v2.0.0")

	// make a feature commit
	testutils.CommitFile(t, repo, path, "foo.go", "feat: update foo", []byte("foo contents\n"))

	if got, err := g.Version(); assert.NoError(t, err) {
		assert.Equal(t, "v2.1.0", got)
	}
}

func TestGotagger_Version_breaking(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	simpleGoRepo(t, repo, path)

	// make a breaking change
	testutils.CommitFile(t, repo, path, "new", "feat!: new is breaking", []byte("new data"))

	if v, err := g.Version(); assert.NoError(t, err) {
		assert.Equal(t, "v2.0.0", v)
	}
}

func TestNew(t *testing.T) {
	_, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	// invalid path should return an error
	_, err := New(filepath.FromSlash("/does/not/exist"))
	assert.Error(t, err)

	if g, err := New(path); assert.NoError(t, err) && assert.NotNil(t, g) {
		assert.Equal(t, NewDefaultConfig(), g.Config)
	}
}

func TestGotagger_findAllModules(t *testing.T) {
	tests := []struct {
		title    string
		repoFunc func(testutils.T, *sgit.Repository, string)
		include  []string
		exclude  []string
		want     []module
	}{
		{
			title:    "simple git repo",
			repoFunc: simpleGoRepo,
			want: []module{
				{".", "foo", ""},
				{filepath.Join("sub", "module"), "foo/sub/module", "sub/module/"},
			},
		},
		{
			title:    "v1 on master branch",
			repoFunc: masterV1GitRepo,
			want: []module{
				{".", "foo", ""},
				{"bar", "foo/bar", "bar/"},
			},
		},
		{
			title:    "v1 on master branch, exclude foo",
			repoFunc: masterV1GitRepo,
			exclude:  []string{"foo"},
			want: []module{
				{"bar", "foo/bar", "bar/"},
			},
		},
		{
			title:    "v1 on master branch, exclude all by path",
			repoFunc: masterV1GitRepo,
			exclude:  []string{"."},
		},
		{
			title:    "v1 on master branch, exclude foo/bar",
			repoFunc: masterV1GitRepo,
			exclude:  []string{"foo/bar"},
			want: []module{
				{".", "foo", ""},
			},
		},
		{
			title:    "v1 on master branch, exclude foo/bar by path",
			repoFunc: masterV1GitRepo,
			exclude:  []string{"bar"},
			want: []module{
				{".", "foo", ""},
			},
		},
		{
			title:    "v1 on master branch, include foo",
			repoFunc: masterV1GitRepo,
			include:  []string{"foo"},
			want: []module{
				{".", "foo", ""},
			},
		},
		{
			title:    "v1 on master branch, include foo/bar",
			repoFunc: masterV1GitRepo,
			include:  []string{"foo/bar"},
			want: []module{
				{"bar", "foo/bar", "bar/"},
			},
		},
		{
			title:    "v1 on master branch, explicitly include all",
			repoFunc: masterV1GitRepo,
			include:  []string{"foo", "foo/bar"},
			want: []module{
				{".", "foo", ""},
				{"bar", "foo/bar", "bar/"},
			},
		},
		{
			title:    "v1 on master branch, include none",
			repoFunc: masterV1GitRepo,
			include:  []string{"foz"},
		},
		{
			title:    "v2 on master branch",
			repoFunc: masterV2GitRepo,
			want: []module{
				{".", "foo/v2", ""},
				{"bar", "foo/bar/v2", "bar/"},
			},
		},
		{
			title:    "v2 directory",
			repoFunc: v2DirGitRepo,
			want: []module{
				{".", "foo", ""},
				{"v2", "foo/v2", ""},
				{"bar", "foo/bar", "bar/"},
				{filepath.Join("bar", "v2"), "foo/bar/v2", "bar/"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()

			g, repo, path, teardown := newGotagger(t)
			defer teardown()

			tt.repoFunc(t, repo, path)

			g.Config.ExcludeModules = tt.exclude
			if modules, err := g.findAllModules(tt.include); assert.NoError(t, err) {
				assert.Equal(t, tt.want, modules)
			}
		})
	}
}

func TestGotagger_incrementVersion(t *testing.T) {
	tests := []struct {
		title          string
		repoFunc       func(testutils.T, *sgit.Repository, string)
		dirtyIncrement mapper.Increment
		preMajor       bool
		commits        []git.Commit
		want           string
	}{
		{
			title: "breaking feat",
			commits: []git.Commit{
				{Commit: commit.Commit{Type: mapper.TypeFeature, Breaking: true}},
			},
			want: "1.0.0",
		},
		{
			title: "breaking fix",
			commits: []git.Commit{
				{Commit: commit.Commit{Type: mapper.TypeBugFix, Breaking: true}},
			},
			want: "1.0.0",
		},
		{
			title: "breaking unknown",
			commits: []git.Commit{
				{Commit: commit.Commit{Type: "unknown", Breaking: true}},
			},
			want: "1.0.0",
		},
		{
			title:    "breaking feat pre-major",
			preMajor: true,
			commits: []git.Commit{
				{Commit: commit.Commit{Type: mapper.TypeFeature, Breaking: true}},
			},
			want: "0.2.0",
		},
		{
			title:    "breaking fix pre-major",
			preMajor: true,
			commits: []git.Commit{
				{Commit: commit.Commit{Type: mapper.TypeBugFix, Breaking: true}},
			},
			want: "0.1.1",
		},
		{
			title:    "breaking unknown pre-major",
			preMajor: true,
			commits: []git.Commit{
				{Commit: commit.Commit{Type: "unknown", Breaking: true}},
			},
			want: "0.1.1",
		},
		{
			title:          "dirty minor",
			dirtyIncrement: mapper.IncrementMinor,
			want:           "0.2.0",
		},
		{
			title:          "dirty patch",
			dirtyIncrement: mapper.IncrementPatch,
			want:           "0.1.1",
		},
		{
			title:          "dirty unknown",
			dirtyIncrement: mapper.Increment(23),
			want:           "0.1.0",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()

			g, repo, path, teardown := newGotagger(t)
			defer teardown()

			if tt.repoFunc != nil {
				tt.repoFunc(t, repo, path)
			} else {
				testutils.SimpleGitRepo(t, repo, path)
			}

			g.Config.DirtyWorktreeIncrement = tt.dirtyIncrement
			g.Config.PreMajor = tt.preMajor

			// add untracked file for dirty tests
			require.NoError(t, ioutil.WriteFile(filepath.Join(path, "untracked"), []byte("untracked\n"), 0600))

			if got, err := g.incrementVersion(semver.MustParse("0.1.0"), tt.commits); assert.NoError(t, err) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_filterCommitsByModule(t *testing.T) {
	tests := []struct {
		title    string
		repoFunc func(testutils.T, *sgit.Repository, string)
		mod      module
		want     []string
	}{
		{
			title:    "simple git repo foo module",
			repoFunc: simpleGoRepo,
			mod:      module{".", "foo", ""},
			want: []string{
				"feat: add go.mod",
				"feat: bar\n\nThis is a great bar.",
				"feat: more foo",
				"feat: foo",
			},
		},
		{
			title:    "simple git repo sub/module",
			repoFunc: simpleGoRepo,
			mod:      module{filepath.Join("sub", "module"), "sub/module", "sub/module/"},
			want: []string{
				"fix: fix submodule",
				"feat: add a file to submodule",
				"feat: add a submodule",
			},
		},
		{
			title:    "v1 on master branch foo module",
			repoFunc: masterV1GitRepo,
			mod:      module{".", "foo", ""},
			want: []string{
				"feat: add go.mod",
			},
		},
		{
			title:    "v1 on master branch bar module",
			repoFunc: masterV1GitRepo,
			mod:      module{"bar", "foo/bar", "bar/"},
			want: []string{
				"feat: add bar/go.mod",
			},
		},
		{
			title:    "v2 on master branch foo module",
			repoFunc: masterV2GitRepo,
			mod:      module{".", "foo/v2", ""},
			want: []string{
				"feat!: add foo/v2 go.mod",
				"feat: add go.mod",
			},
		},
		{
			title:    "v2 on master branch bar module",
			repoFunc: masterV2GitRepo,
			mod:      module{"bar", "foo/bar/v2", "bar/"},
			want: []string{
				"feat!: add bar/v2 go.mod",
				"feat: add bar/go.mod",
			},
		},
		{
			title:    "v2 directory foo module",
			repoFunc: v2DirGitRepo,
			mod:      module{".", "foo", ""},
			want: []string{
				"feat: add go.mod",
			},
		},
		{
			title:    "v2 directory foo/v2 module",
			repoFunc: v2DirGitRepo,
			mod:      module{".", "foo", ""},
			want: []string{
				"feat!: add v2/go.mod",
			},
		},
		{
			title:    "v2 directory bar module",
			repoFunc: v2DirGitRepo,
			mod:      module{"bar", "foo/bar", "bar/"},
			want: []string{
				"feat: add bar/go.mod",
			},
		},
		{
			title:    "v2 directory",
			repoFunc: v2DirGitRepo,
			mod:      module{filepath.Join("bar", "v2"), "foo/bar/v2", "bar/"},
			want: []string{
				"feat!: add bar/v2/go.mod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()

			g, repo, path, teardown := newGotagger(t)
			defer teardown()

			tt.repoFunc(t, repo, path)

			modules, err := g.findAllModules(nil)
			require.NoError(t, err)

			commits, err := g.repo.RevList("HEAD", "")
			require.NoError(t, err)

			commits = filterCommitsByModule(tt.mod, commits, modules)

			// convert to a map of modules to commit subject
			messages := make([]string, len(commits))
			for i, commit := range commits {
				messages[i] = commit.Message()
			}

			assert.Equal(t, tt.want, messages)
		})
	}
}

func TestGotagger_validateModules(t *testing.T) {
	tests := []struct {
		title   string
		commit  []module
		changed []module
		want    string
	}{
		{
			title:   "all match",
			commit:  []module{{".", "foo", ""}},
			changed: []module{{".", "foo", ""}},
			want:    "",
		},
		{
			title:   "extra bar",
			commit:  []module{{".", "foo", ""}, {"bar", "bar", "bar/"}},
			changed: []module{{".", "foo", ""}},
			want:    "module validation failed:\nmodules not changed by commit: bar",
		},
		{
			title:   "missing bar",
			commit:  []module{{".", "foo", ""}},
			changed: []module{{".", "foo", ""}, {"bar", "bar", "bar/"}},
			want:    "module validation failed:\nchanged modules not released by commit: bar",
		},
		{
			title:   "extra bar, baz",
			commit:  []module{{".", "foo", ""}, {"bar", "bar", "bar/"}, {"baz", "baz", "baz/"}},
			changed: []module{{".", "foo", ""}},
			want:    "module validation failed:\nmodules not changed by commit: bar, baz",
		},
		{
			title:   "missing bar, baz",
			commit:  []module{{".", "foo", ""}},
			changed: []module{{".", "foo", ""}, {"bar", "bar", "bar/"}, {"baz", "baz", "baz/"}},
			want:    "module validation failed:\nchanged modules not released by commit: bar, baz",
		},
		{
			title:   "extra bar, missing baz",
			commit:  []module{{".", "foo", ""}, {"bar", "bar", "bar/"}},
			changed: []module{{".", "foo", ""}, {"baz", "baz", "baz/"}},
			want:    "module validation failed:\nmodules not changed by commit: bar\nchanged modules not released by commit: baz",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()

			err := validateCommitModules(tt.commit, tt.changed)
			if tt.want == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.want)
			}
		})
	}
}

func newGotagger(t testutils.T) (g *Gotagger, repo *sgit.Repository, path string, teardown func()) {
	t.Helper()

	repo, path, teardown = testutils.NewGitRepo(t)

	r, err := git.New(path)
	if err != nil {
		t.Fatal(err)
	}

	g = &Gotagger{
		Config: NewDefaultConfig(),
		repo:   r,
	}

	return
}

// create a repo that has foo and foo/bar in master, and foo/v2 and foo/bar/v2 in v2.
func masterV1GitRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	// setup v1 modules
	h := setupV1Modules(t, repo, path)

	// create a v2 branch
	b := plumbing.NewBranchReferenceName("v2")
	ref := plumbing.NewHashReference(b, h)
	if err := repo.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	// v2 commits go into v2
	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Checkout(&sgit.CheckoutOptions{
		Branch: b,
	}); err != nil {
		t.Fatal(err)
	}

	setupV2Modules(t, repo, path)

	// checkout master
	if err := w.Checkout(&sgit.CheckoutOptions{
		Branch: plumbing.Master,
	}); err != nil {
		t.Fatal(err)
	}
}

// create a repo that has foo and foo/bar in v1, and foo/v2 and foo/bar/v2 in master.
func masterV2GitRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	// create v1 modules
	h := setupV1Modules(t, repo, path)

	// create a v1 branch
	b := plumbing.NewBranchReferenceName("v1")
	ref := plumbing.NewHashReference(b, h)
	if err := repo.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	// v2 commits go into master
	setupV2Modules(t, repo, path)
}

// create a repo with mixed tags.
func mixedTagRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	// create bar.go and tag it 0.1.0 (no prefix)
	testutils.CommitFile(t, repo, path, "bar.go", "feat: add bar.go", []byte("bar\n"))
	testutils.CreateTag(t, repo, path, "0.1.0")

	// create foo.go and tag it v1.0.0
	testutils.CommitFile(t, repo, path, "foo.go", "feat: add foo.go", []byte("foo\n"))
	testutils.CreateTag(t, repo, path, "v1.0.0")
}

func mixedTagGoRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	mixedTagRepo(t, repo, path)

	// create a go.mod
	testutils.CommitFile(t, repo, path, "go.mod", "feat: add go.mod", []byte("module foo\n"))
}

func v2DirGitRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	// create top-level go.mod and tag it v1.0.0
	testutils.CommitFile(t, repo, path, "go.mod", "feat: add go.mod", []byte("module foo\n"))
	testutils.CreateTag(t, repo, path, "v1.0.0")

	// create sub module and tag it v1.0.0
	testutils.CommitFile(t, repo, path, filepath.Join("bar", "go.mod"), "feat: add bar/go.mod", []byte("module foo/bar\n"))
	testutils.CreateTag(t, repo, path, "bar/v1.0.0")

	// create a v2 directory and tag v2.0.0
	testutils.CommitFile(t, repo, path, filepath.Join("v2", "go.mod"), "feat!: add v2/go.mod", []byte("module foo/v2\n"))
	testutils.CreateTag(t, repo, path, "v2.0.0")

	// create bar/v2 directory and tag bar/v2.0.0
	testutils.CommitFile(t, repo, path, filepath.Join("bar", "v2", "go.mod"), "feat!: add bar/v2/go.mod", []byte("module foo/bar/v2\n"))
	testutils.CreateTag(t, repo, path, "bar/v2.0.0")
}

func setupV1Modules(t testutils.T, repo *sgit.Repository, path string) (head plumbing.Hash) {
	t.Helper()

	// create top-level go.mod and tag it v1.0.0
	testutils.CommitFile(t, repo, path, "go.mod", "feat: add go.mod", []byte("module foo\n"))
	testutils.CreateTag(t, repo, path, "v1.0.0")

	// create sub module and tag it v1.0.0
	head = testutils.CommitFile(t, repo, path, filepath.Join("bar", "go.mod"), "feat: add bar/go.mod", []byte("module foo/bar\n"))
	testutils.CreateTag(t, repo, path, "bar/v1.0.0")

	return
}

func setupV2Modules(t testutils.T, repo *sgit.Repository, path string) (head plumbing.Hash) {
	t.Helper()

	testutils.CommitFile(t, repo, path, "go.mod", "feat!: add foo/v2 go.mod", []byte("module foo/v2\n"))
	testutils.CreateTag(t, repo, path, "v2.0.0")

	// update bar module to v2
	head = testutils.CommitFile(t, repo, path, filepath.Join("bar", "go.mod"), "feat!: add bar/v2 go.mod", []byte("module foo/bar/v2\n"))
	testutils.CreateTag(t, repo, path, "bar/v2.0.0")

	return
}

func simpleGoRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	testutils.SimpleGitRepo(t, repo, path)
	testutils.CommitFile(t, repo, path, "go.mod", "feat: add go.mod", []byte("module foo\n"))
	testutils.CommitFile(t, repo, path, "sub/module/go.mod", "feat: add a submodule", []byte("module foo/sub/module\n"))
	testutils.CommitFile(t, repo, path, "sub/module/file", "feat: add a file to submodule", []byte("some data"))
	testutils.CreateTag(t, repo, path, "sub/module/v0.1.0")
	testutils.CommitFile(t, repo, path, "sub/module/file", "fix: fix submodule", []byte("some more data"))
}

func untaggedV2Repo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	simpleGoRepo(t, repo, path)
	testutils.CommitFile(t, repo, path, "go.mod", "feat!: now v2", []byte("module foo/v2\n"))
}
