package gotagger

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"sassoftware.io/clis/gotagger/git"
	"sassoftware.io/clis/gotagger/marker"
)

// Config represents how to tag a repo. If not default is mentioned, the option defaults
// to go's zero-value.
type Config struct {
	// RemoteName represents the name of the remote repository. Defaults to origin.
	RemoteName string

	// PushTag represents whether to push the tag to the remote git repository.
	PushTag bool

	// CreateTag represents whether to create the tag.
	CreateTag bool

	// VersionPrefix is a string that will be added to the front of the version. Defaults to 'v'.
	VersionPrefix string

	/* TODO
	// PreRelease is the string that will be used to generate pre-release versions. The
	// string may be a Golang text template. Valid arguments are:
	//
	//	- .CommitsSince
	//		The number of commits since the previous release.
	PreRelease string
	*/
}

// NewDefaultConfig returns a Config with default options set.
//
// If an option is not mentioned, then the default is the zero-value for its type.
//
//	- RemoteName
//		origin
//	- VersionPrefix
//		v
func NewDefaultConfig() *Config {
	return &Config{
		RemoteName:    "origin",
		VersionPrefix: "v",
	}
}

// TagRepo determines what the curent version of the repository is by parsing the commit
// history since previous release and returns that version. Depending on the state of
// the Config passed it, it may also create the tag and push it.
func TagRepo(cfg *Config, r git.Repo) (*semver.Version, error) {
	// Find the latest semver and the commit hash it references.
	latest, commitHash, err := getLatest(r)
	if err != nil {
		return nil, err
	}

	// Find the most significant marker between HEAD and the latest tagged commit.
	commits, err := r.RevList("HEAD", commitHash)
	if err != nil {
		return nil, fmt.Errorf("could not fetch commits HEAD..%s: %s", commitHash, err)
	}

	// If HEAD is already tagged, just display the latest version
	if len(commits) == 0 {
		return latest, nil
	}

	changeType, isBreaking := scanForMarkers(commits)
	switch {
	case isBreaking:
		*latest = latest.IncMajor()
	case changeType == marker.Feature:
		*latest = latest.IncMinor()
	default:
		*latest = latest.IncPatch()
	}
	if len(commits) > 0 {
		head := commits[0]
		if (cfg.CreateTag || cfg.PushTag) && isRelease(head) && !alreadyTagged(latest, head) {
			if err := r.CreateTag(head.Hash, latest, cfg.VersionPrefix, "", false); err != nil {
				return nil, fmt.Errorf("could not tag HEAD (%s): %s", head.Hash, err)
			}
			if cfg.PushTag {
				// TODO: add option to set name of remote
				if err := r.PushTag(latest, "origin"); err != nil {
					return nil, fmt.Errorf("could not push tag (%s): %s", latest, err)
				}
			}
		}
	}
	return latest, nil
}

func alreadyTagged(v *semver.Version, c git.Commit) bool {
	for _, t := range c.Tags {
		if v.Equal(t) {
			return true
		}
	}
	return false
}

func isRelease(c git.Commit) bool {
	m, _, _ := marker.Parse(c.Subject)
	return m == marker.Release
}

func getLatest(r git.Repo) (latest *semver.Version, hash string, err error) {
	taggedCommits, err := r.Tags()
	if err != nil {
		return latest, hash, err
	}
	latest = new(semver.Version)
	for _, commit := range taggedCommits {
		if len(commit.Tags) > 0 {
			if latest.LessThan(commit.Tags[0]) {
				latest = commit.Tags[0]
				hash = commit.Hash
			}
		}
	}
	return latest, hash, nil
}

func scanForMarkers(commits []git.Commit) (mark marker.Marker, isBreaking bool) {
	if len(commits) != 0 {
		for _, c := range commits {
			m, _, b := marker.Parse(c.Subject)
			switch m {
			case marker.Feature:
				mark = m
			case marker.Fix:
				if mark != marker.Feature {
					mark = m
				}
			}
			// if we already saw a breaking change, we can stop checking
			if !isBreaking && (b || marker.IsBreaking(c.Trailers)) {
				isBreaking = true
			}
		}
	}
	return mark, isBreaking
}
