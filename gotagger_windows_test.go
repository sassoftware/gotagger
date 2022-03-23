package gotagger

import (
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/sassoftware/gotagger/internal/git"
	"github.com/sassoftware/gotagger/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowsPaths(t *testing.T) {
	repo, path, teardown := testutils.NewGitRepo(t)
	defer teardown()

	// ensure / in path
	path = filepath.ToSlash(path)

	simpleGoRepo(t, repo, path)

	r, err := git.New(path)
	require.NoError(t, err)

	g := &Gotagger{
		Config: NewDefaultConfig(),
		logger: logr.Discard(),
		repo:   r,
	}

	if versions, err := g.TagRepo(); assert.NoError(t, err) {
		assert.Equal(t, []string{"v1.1.0"}, versions)
	}
}
