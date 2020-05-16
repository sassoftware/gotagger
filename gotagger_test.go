package gotagger

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	sgit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"sassoftware.io/clis/gotagger/internal/testutils"
)

func TestGotagger_SubmoduleVersion(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	simpleGitRepo(t, repo, path)

	v, err := g.SubmoduleVersion("sub/module")
	if err != nil {
		t.Fatalf("SubmoduleVersion() returned an error: %v", err)
	}
	if got, want := v, "sub/module/v0.1.1"; got != want {
		t.Errorf("SubmoduleVersion() returned %s, want %s", got, want)
	}
}

func TestGotagger_TagRepo(t *testing.T) {
	tests := []struct {
		title    string
		prefix   string
		repoFunc func(testutils.T, *sgit.Repository, string)
		message  string
		want     []string
	}{
		{
			title:    "v-prefix tags",
			prefix:   "v",
			repoFunc: mixedTagRepo,
			message:  "release: the foos",
			want:     []string{"v1.1.0"},
		},
		{
			title:    "unprefixed tags",
			prefix:   "",
			repoFunc: mixedTagRepo,
			message:  "release: the bars",
			want:     []string{"0.1.1"},
		},
		{
			title:    "release root v1 on master",
			prefix:   "v",
			repoFunc: masterV1GitRepo,
			message:  "release: the foos",
			want:     []string{"v1.0.1"},
		},
		{
			title:    "release bar v1 on master",
			prefix:   "v",
			repoFunc: masterV1GitRepo,
			message:  "release: the bars\n\nModules: bar",
			want:     []string{"bar/v1.0.0"},
		},
		{
			title:    "release all v1 on master",
			prefix:   "v",
			repoFunc: masterV1GitRepo,
			message:  "release: all the things\n\nModules: ., bar",
			want:     []string{"v1.0.1", "bar/v1.0.0"},
		},
		{
			title:    "release root v2 on master",
			prefix:   "v",
			repoFunc: masterV2GitRepo,
			message:  "release: the foos",
			want:     []string{"v2.0.1"},
		},
		{
			title:    "release bar v2 on master",
			prefix:   "v",
			repoFunc: masterV2GitRepo,
			message:  "release: the bars\n\nModules: bar",
			want:     []string{"bar/v2.0.0"},
		},
		{
			title:    "release all v2 on master",
			prefix:   "v",
			repoFunc: masterV2GitRepo,
			message:  "release: all the things\n\nModules: bar, .",
			want:     []string{"bar/v2.0.0", "v2.0.1"},
		},
		{
			title:    "release: foo v2 directory",
			prefix:   "v",
			repoFunc: v2DirGitRepo,
			message:  "release: the foos",
			want:     []string{"v2.0.1"},
		},
		{
			title:    "release: bar v2 directory",
			prefix:   "v",
			repoFunc: v2DirGitRepo,
			message:  "release: the bars\n\nModules: bar",
			want:     []string{"bar/v2.0.0"},
		},
		{
			title:    "release: all v2 directory",
			prefix:   "v",
			repoFunc: v2DirGitRepo,
			message:  "release: all the things\n\nModules: .,bar/v2",
			want:     []string{"v2.0.1", "bar/v2.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			if strings.HasSuffix(tt.title, " directory") {
				t.Skipf("skipping %s: directory versions are not supported", strings.ReplaceAll(tt.title, " ", "_"))
			}
			g, repo, path, teardown := newGotagger(t)
			defer teardown()

			tt.repoFunc(t, repo, path)

			// create a release commit
			testutils.CommitFile(t, repo, path, "CHANGELOG.md", tt.message, []byte(`changes`))

			g.Config.VersionPrefix = tt.prefix
			versions, err := g.TagRepo()
			if err != nil {
				t.Fatalf("TagRepo() returned an error: %v", err)
			}
			if got, want := versions, tt.want; !reflect.DeepEqual(got, want) {
				t.Errorf("TagRepo() returned\n%s\nwant\n%s", spew.Sdump(got), spew.Sdump(want))
			}
		})
	}
}

func TestGotagger_Version(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	simpleGitRepo(t, repo, path)

	v, err := g.Version()
	if err != nil {
		t.Fatalf("Version() returned an error: %v", err)
	}
	if got, want := v, "v1.1.0"; got != want {
		t.Errorf("Version() returned %s, want %s", got, want)
	}
}

func TestGotagger_Version_tag_head(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	simpleGitRepo(t, repo, path)

	// tag HEAD higher than what gotagger would return
	version := "v3.0.0"
	testutils.CreateTag(t, repo, path, version)

	v, err := g.Version()
	if err != nil {
		t.Fatalf("Version() returned an error: %v", err)
	}

	if got, want := v, version; got != want {
		t.Errorf("Version() returned %s, want %s", got, want)
	}
}

func TestGotagger_Version_breaking(t *testing.T) {
	g, repo, path, teardown := newGotagger(t)
	defer teardown()

	simpleGitRepo(t, repo, path)

	// make a breaking change
	testutils.CommitFile(t, repo, path, "new", "feat!: new is breaking", []byte("new data"))

	v, err := g.Version()
	if err != nil {
		t.Fatalf("Version() returned an error: %v", err)
	}

	if got, want := v, "v2.0.0"; got != want {
		t.Errorf("Version() returned %s, want %s", got, want)
	}
}

func Test_findAllModules(t *testing.T) {
	tests := []struct {
		title    string
		repoFunc func(testutils.T, *sgit.Repository, string)
		want     []module
	}{
		{
			"simple git repo",
			simpleGitRepo,
			[]module{
				{".", "foo"},
				{filepath.Join("sub", "module"), "foo/sub/module"},
			},
		},
		{
			"v1 on master branch",
			masterV1GitRepo,
			[]module{
				{".", "foo"},
				{"bar", "foo/bar"},
			},
		},
		{
			"v2 on master branch",
			masterV2GitRepo,
			[]module{
				{".", "foo/v2"},
				{"bar", "foo/bar/v2"},
			},
		},
		{
			"v2 directory",
			v2DirGitRepo,
			[]module{
				{".", "foo"},
				{"v2", "foo/v2"},
				{"bar", "foo/bar"},
				{filepath.Join("v2", "bar"), "foo/bar/v2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			repo, path, teardwon := testutils.NewGitRepo(t)
			defer teardwon()

			tt.repoFunc(t, repo, path)

			modules, err := findAllModules()
			if err != nil {
				t.Fatal(err)
			}

			if got, want := modules, tt.want; !reflect.DeepEqual(got, want) {
				t.Errorf("findAllModules() returned\n%swant\n%s", spew.Sdump(got), spew.Sdump(want))
			}
		})
	}
}

func Test_groupCommitsByModule(t *testing.T) {
	tests := []struct {
		title    string
		repoFunc func(testutils.T, *sgit.Repository, string)
		want     map[module][]string
	}{
		{
			"simple git repo",
			simpleGitRepo,
			map[module][]string{
				{".", "foo"}: {
					"feat: add go.mod",
					"feat: bar\n\nThis is a great bar.",
					"feat: more foo",
					"feat: foo",
				},
				{filepath.Join("sub", "module"), "foo/sub/module"}: {
					"fix: fix submodule",
					"feat: add a file to submodule",
					"feat: add a submodule",
				},
			},
		},
		{
			"v1 on master branch",
			masterV1GitRepo,
			map[module][]string{
				{".", "foo"}: {
					"feat: add foo.go",
					"feat: add go.mod",
				},
				{"bar", "foo/bar"}: {
					"feat: add bar/go.mod",
				},
			},
		},
		{
			"v2 on master branch",
			masterV2GitRepo,
			map[module][]string{
				{".", "foo/v2"}: {
					"feat!: add foo/v2 go.mod",
					"feat: add foo.go",
					"feat: add go.mod",
				},
				{"bar", "foo/bar/v2"}: {
					"feat!: add bar/v2 go.mod",
					"feat: add bar/go.mod",
				},
			},
		},
		{
			"v2 directory",
			v2DirGitRepo,
			map[module][]string{
				{".", "foo"}: {
					"feat: add foo.go",
					"feat: add go.mod",
				},
				{"v2", "foo/v2"}: {
					"feat: add v2/foo.go",
					"feat!: add v2/go.mod",
				},
				{"bar", "foo/bar"}: {
					"feat: add bar/go.mod",
				},
				{filepath.Join("v2", "bar"), "foo/bar/v2"}: {
					"feat!: add v2/bar/go.mod",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			g, repo, path, teardwon := newGotagger(t)
			defer teardwon()

			tt.repoFunc(t, repo, path)

			modules, err := findAllModules()
			if err != nil {
				t.Fatal(err)
			}

			commits, err := g.repo.RevList("HEAD", "")
			if err != nil {
				t.Fatal(err)
			}

			groups := groupCommitsByModule(commits, modules)

			// can't construct a commit, so convert groups into a map of modules to messages
			got := make(map[module][]string)
			for module, commits := range groups {
				messages := got[module]
				for _, commit := range commits {
					messages = append(messages, commit.Message)
				}
				got[module] = messages
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("groupCommitsByModule returned\n%s\nwant\n%s", spew.Sdump(got), spew.Sdump(tt.want))
			}
		})
	}
}

func newGotagger(t testutils.T) (g *Gotagger, repo *sgit.Repository, path string, teardown func()) {
	t.Helper()

	repo, path, teardown = testutils.NewGitRepo(t)

	g, err := New(path)
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
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

// create a repo with mixed tags
func mixedTagRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	setupV1Modules(t, repo, path)

	// commit and tag it 0.1.0 (no prefix)
	testutils.CommitFile(t, repo, path, "bar.go", "feat: add bar.go", []byte("bar\n"))
	testutils.CreateTag(t, repo, path, "0.1.0")
}

func v2DirGitRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	// create v1 modules
	setupV1Modules(t, repo, path)

	// create a v2 directory
	testutils.CommitFile(t, repo, path, filepath.Join("v2", "go.mod"), "feat!: add v2/go.mod", []byte("module foo/v2\n"))

	// create a file
	testutils.CommitFile(t, repo, path, filepath.Join("v2", "foo.go"), "feat: add v2/foo.go", []byte("foo\n"))

	// tag v2.0.0
	testutils.CreateTag(t, repo, path, "v2.0.0")

	// create v2 bar submodule
	testutils.CommitFile(t, repo, path, filepath.Join("v2", "bar", "go.mod"), "feat!: add v2/bar/go.mod", []byte("module foo/bar/v2\n"))
}

func setupV1Modules(t testutils.T, repo *sgit.Repository, path string) (head plumbing.Hash) {
	t.Helper()

	// create top-level go.mod
	testutils.CommitFile(t, repo, path, "go.mod", "feat: add go.mod", []byte("module foo\n"))

	// create a file
	testutils.CommitFile(t, repo, path, "foo.go", "feat: add foo.go", []byte("foo\n"))

	// tag v1.0.0
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
	return
}

func simpleGitRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	testutils.SimpleGitRepo(t, repo, path)
	testutils.CommitFile(t, repo, path, "go.mod", "feat: add go.mod", []byte("module foo\n"))
	testutils.CommitFile(t, repo, path, "sub/module/go.mod", "feat: add a submodule", []byte("module foo/sub/module\n"))
	testutils.CommitFile(t, repo, path, "sub/module/file", "feat: add a file to submodule", []byte("some data"))
	testutils.CreateTag(t, repo, path, "sub/module/v0.1.0")
	testutils.CommitFile(t, repo, path, "sub/module/file", "fix: fix submodule", []byte("some more data"))
}
