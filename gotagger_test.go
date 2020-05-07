package gotagger

import (
	"testing"

	sgit "github.com/go-git/go-git/v5"
	"sassoftware.io/clis/gotagger/git"
	"sassoftware.io/clis/gotagger/internal/testutils"
)

func TestGotagger_TagRepo(t *testing.T) {
	tests := []struct {
		title    string
		prefix   string
		repoFunc func(testutils.T, *sgit.Repository, string)
		message  string
		want     string
	}{
		{
			"v-prefix tags",
			"v",
			mixedTagRepo,
			"release: the foos",
			"v1.1.0",
		},
		{
			"unprefixed tags",
			"",
			mixedTagRepo,
			"release: the bars",
			"0.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			g, repo, path, teardown := newRepo(t)
			defer teardown()

			tt.repoFunc(t, repo, path)

			// create a release commit
			testutils.CommitFile(t, repo, path, "CHANGELOG.md", tt.message, []byte(`changes`))

			cfg := NewDefaultConfig()
			cfg.VersionPrefix = tt.prefix

			version, err := TagRepo(cfg, g)
			if err != nil {
				t.Fatalf("TagRepo() returned an error: %v", err)
			}
			if got, want := version.Original(), tt.want; got != want {
				t.Errorf("TagRepo() returned %s, want %s", got, want)
			}
		})
	}
}

func newRepo(t testutils.T) (g git.Repo, repo *sgit.Repository, path string, teardown func()) {
	t.Helper()

	repo, path, teardown = testutils.NewGitRepo(t)

	g, err := git.New(path)
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}

	return
}

func mixedTagRepo(t testutils.T, repo *sgit.Repository, path string) {
	t.Helper()

	// create top-level go.mod
	testutils.CommitFile(t, repo, path, "go.mod", "feat: add go.mod", []byte("module foo\n"))

	// create a file
	testutils.CommitFile(t, repo, path, "foo.go", "feat: add foo.go", []byte("foo\n"))

	// tag v1.0.0
	testutils.CreateTag(t, repo, path, "v1.0.0")

	// commit and tag it 0.1.0 (no prefix)
	testutils.CommitFile(t, repo, path, "bar.go", "feat: add bar.go", []byte("bar\n"))
	testutils.CreateTag(t, repo, path, "0.1.0")
}
