package gotagger

import (
	"encoding/json"
	"fmt"

	"github.com/sassoftware/gotagger/mapper"
)

type config struct {
	DefaultIncrement         string            `json:"defaultIncrement"`
	IncrementDirtyWorktree   string            `json:"incrementDirtyWorktree"`
	ExcludeModules           []string          `json:"excludeModules"`
	IgnoreModules            bool              `json:"ignoreModules"`
	IncrementMappings        map[string]string `json:"incrementMappings"`
	IncrementPreReleaseMinor bool              `json:"incrementPreReleaseMinor"`
	VersionPrefix            *string           `json:"versionPrefix"`
}

// Config represents how to tag a repo.
//
// If no default is mentioned, the option defaults to go's zero-value.
type Config struct {
	// CreateTag represents whether to create the tag.
	CreateTag bool

	// ExcludeModules is a list of module names or paths to exclude.
	ExcludeModules []string

	// IgnoreModules controls whether gotagger will ignore the existence of
	// go.mod files when determining how to version a project.
	IgnoreModules bool

	// RemoteName represents the name of the remote repository. Defaults to origin.
	RemoteName string

	// PreMajor controls whether gotagger will increase the major version from 0
	// to 1 for breaking changes.
	PreMajor bool

	// PushTag represents whether to push the tag to the remote git repository.
	PushTag bool

	// VersionPrefix is a string that will be added to the front of the version. Defaults to 'v'.
	VersionPrefix string

	// DirtyWorktreeIncrement is a string that sets how to increment the version
	// if there are no new commits, but the worktree is "dirty".
	DirtyWorktreeIncrement mapper.Increment

	// CommitTypeTable used for looking up version increments based on the commit type.
	CommitTypeTable mapper.Table

	// Force controls whether gotagger will create a tag even if HEAD is not a "release" commit.
	Force bool

	// Paths is a list of sub-paths within the repo to restrict the git
	// history used to calculate a version. The versions returned will be
	// prefixed with their path.
	Paths []string

	/* TODO
	// PreRelease is the string that will be used to generate pre-release versions. The
	// string may be a Golang text template. Valid arguments are:
	//
	//	- .CommitsSince
	//		The number of commits since the previous release.
	PreRelease string
	*/
}

// ParseJSON unmarshals a byte slice containing mappings of commit type to semver increment. Mappings determine
// how much to increment the semver based on the commit type. The 'release' commit type has special meaning to gotagger
// and cannot be overridden in the config file. Unknown commit types will fall back to the config default.
// Invalid increments will throw an error. Duplicate type definitions will take the last entry.
func (c *Config) ParseJSON(data []byte) error {
	// unmarshal our private struct
	cfg := config{
		IncrementMappings: make(map[string]string),
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// validate dirty worktree increment
	inc, err := mapper.Convert(cfg.IncrementDirtyWorktree)
	switch {
	case err != nil:
		return fmt.Errorf("invalid dirty worktree increment: %s", cfg.IncrementDirtyWorktree)
	case inc == mapper.IncrementMajor:
		return fmt.Errorf("major version increments are not allowed for dirty worktrees")
	default:
		c.DirtyWorktreeIncrement = inc
	}

	// version prefix is a pointer
	// so the config file can set it to ""
	// and we can preserve the default of "v"
	if cfg.VersionPrefix != nil {
		c.VersionPrefix = *cfg.VersionPrefix
	}

	// we do not allow configuring the release type,
	// as it means something particular to gotagger
	if _, ok := cfg.IncrementMappings["release"]; ok {
		return fmt.Errorf("release mapping is not allowed")
	}

	// generate the commit type table from the parsed mappings
	var table mapper.Mapper
	for typ, inc := range cfg.IncrementMappings {
		conversion, err := mapper.Convert(inc)
		if err != nil {
			return err
		}

		if conversion == mapper.IncrementMajor {
			return fmt.Errorf("major version increments cannot be mapped to commit types. use the commit spec directives for this")
		}

		if table == nil {
			table = make(mapper.Mapper)
		}

		table[typ] = conversion
		continue
	}

	// default increment to patch
	if cfg.DefaultIncrement == "" {
		cfg.DefaultIncrement = "patch"
	}
	def, err := mapper.Convert(cfg.DefaultIncrement)
	if err != nil {
		return err
	}

	c.CommitTypeTable = mapper.NewTable(table, def)

	// copy over static values
	c.ExcludeModules = cfg.ExcludeModules
	c.IgnoreModules = cfg.IgnoreModules
	c.PreMajor = cfg.IncrementPreReleaseMinor

	return nil
}

// NewDefaultConfig returns a Config with default options set.
//
// If an option is not mentioned, then the default is the zero-value for its type.
//
//	- RemoteName
//		origin
//	- VersionPrefix
//		v
func NewDefaultConfig() Config {
	return Config{
		CommitTypeTable: mapper.NewTable(nil, mapper.IncrementPatch),
		RemoteName:      "origin",
		VersionPrefix:   "v",
	}
}
