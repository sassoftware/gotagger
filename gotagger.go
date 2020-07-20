package gotagger

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5/plumbing/object"
	"golang.org/x/mod/modfile"
	"sassoftware.io/clis/gotagger/git"
	ggit "sassoftware.io/clis/gotagger/git"
	"sassoftware.io/clis/gotagger/internal/commit"
	igit "sassoftware.io/clis/gotagger/internal/git"
	"sassoftware.io/clis/gotagger/marker"
)

const (
	goMod          = "go.mod"
	rootModulePath = "."
)

var (
	ErrNoSubmodule = errors.New("no submodule found")
	ErrNotRelease  = errors.New("HEAD is not a releaes commit")
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
func NewDefaultConfig() Config {
	return Config{
		RemoteName:    "origin",
		VersionPrefix: "v",
	}
}

type Gotagger struct {
	Config Config

	repo *igit.Repository
}

func New(path string) (*Gotagger, error) {
	r, err := igit.New(path)
	if err != nil {
		return nil, err
	}

	g := &Gotagger{
		Config: Config{
			RemoteName:    "origin",
			VersionPrefix: "v",
		},
		repo: r,
	}
	return g, nil
}

// SubmoduleVersion returns the current release version for submodule s.
func (g *Gotagger) SubmoduleVersion(s string) (string, error) {
	modules, err := findAllModules(g.repo.Path)
	if err != nil {
		return "", err
	}

	var submodule module
	for _, m := range modules {
		if s == m.name {
			submodule = m
			break
		}
	}

	return g.version(submodule, modules)
}

// TagRepo determines what the curent version of the repository is by parsing
// the commit history since previous release and returns that version. Depending
// on the state of the Config passed it, it may also create the tag and push it.
//
// If the current commit contains one or more Modules footers, then tags are
// created for each module listed. In this case, unless the root module is
// explicitly included via the special module name "." then it will not be
// tagged.
func (g *Gotagger) TagRepo() ([]string, error) {
	// ensure HEAD is a release commit
	head, err := g.repo.Head()
	if err != nil {
		return nil, err
	}

	// get all modules
	modules, err := findAllModules(g.repo.Path)
	if err != nil {
		return nil, err
	}

	// map module name to module
	moduleNameMap := map[string]module{}
	for _, m := range modules {
		moduleNameMap[m.name] = m
	}

	cc := commit.Parse(head.Message)

	var commitModules []module
	for _, footer := range cc.Footers {
		if footer.Title == "Modules" {
			for _, moduleName := range strings.Split(footer.Text, ",") {
				moduleName = strings.TrimSpace(moduleName)
				m, ok := moduleNameMap[moduleName]
				if !ok {
					return nil, fmt.Errorf("no module %s found", moduleName)
				}
				commitModules = append(commitModules, m)
			}
		}
	}

	// default to the root module
	if len(commitModules) == 0 {
		// find the root module, defaulting to the first module found
		rootModule := modules[0]
		for _, m := range modules {
			if m.path == rootModulePath {
				rootModule = m
				break
			}
		}
		commitModules = []module{rootModule}
	}

	// collect versions we need to create
	var versions []string
	for _, module := range commitModules {
		v, err := g.version(module, modules)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}

	// if this is a release commit, then validate that modules are correct
	if cc.Type == commit.TypeRelease {
		// validate that the commit touches every module listed in the footer,
		// and that no changes are to unlisted modules
		stats, err := head.Stats()
		if err != nil {
			return nil, err
		}

		// generate a list of modules changed by this commit
		var changedModules []module
		for _, stat := range stats {
			if mod, ok := isModuleFile(stat.Name, modules); ok {
				changedModules = append(changedModules, mod)
			}
		}

		// validate that the two lists are the same
		if err := validateCommitModules(commitModules, changedModules); err != nil {
			return nil, err
		}

		if g.Config.CreateTag {
			// create tag
			tags := make([]*object.Tag, len(versions))
			for i, ver := range versions {
				tag, err := g.repo.CreateTag(head.Hash, ver, "")
				if err != nil {
					// clean up tags we already created
					if terr := g.repo.DeleteTags(tags); terr != nil {
						err = fmt.Errorf("%w\n%s", err, terr)
					}
					return nil, err
				}
				tags[i] = tag
			}

			// push tags
			if g.Config.PushTag {
				if err := g.repo.PushTags(tags, g.Config.RemoteName); err != nil {
					return versions, err
				}
			}
		}
	}

	return versions, nil
}

// Version returns the current release version for the main module.
func (g *Gotagger) Version() (string, error) {
	modules, err := findAllModules(g.repo.Path)
	if err != nil {
		return "", err
	}

	// find root module, defaulting to first module in list
	root := modules[0]
	for _, m := range modules {
		if m.path == rootModulePath {
			root = m
			break
		}
	}

	return g.version(root, modules)
}

func (g *Gotagger) getLatest(m module) (latest *semver.Version, hash string, err error) {
	// determine the major version prefix for this module by normalizing
	// the version part of the name to 'X.'
	major := strings.TrimPrefix(versionRegex.FindString(m.name), "/")
	if major != "" {
		major = strings.TrimPrefix(major, g.Config.VersionPrefix) + "."
	}

	tags, err := g.repo.Tags("HEAD", m.prefix+g.Config.VersionPrefix)
	if err == nil {
		latest = &semver.Version{}
		for _, t := range tags {
			// filter out major versions greater than this module
			tname := strings.TrimPrefix(t.Name().Short(), m.prefix)
			if (major == "" && (strings.HasPrefix(tname, g.Config.VersionPrefix+"0.") || strings.HasPrefix(tname, g.Config.VersionPrefix+"1."))) ||
				(major != "" && strings.HasPrefix(tname, g.Config.VersionPrefix+major)) {
				if tver, err := semver.NewVersion(tname); err == nil && latest.LessThan(tver) {
					hash = t.Hash().String()
					latest = tver
				}
			}
		}
	}

	return
}

func (g *Gotagger) parseCommits(cs []*object.Commit) (ctype commit.Type, breaking bool) {
	for _, c := range cs {
		cc := commit.Parse(c.Message)
		if cc.Type != "" {
			switch cc.Type {
			case commit.TypeFeature:
				ctype = cc.Type
			case commit.TypeBugFix:
				if ctype != commit.TypeFeature {
					ctype = cc.Type
				}
			}
			breaking = breaking || cc.Breaking
		}
	}
	return ctype, breaking
}

func (g *Gotagger) version(submodule module, modules []module) (string, error) {
	var mod module
	for _, m := range modules {
		if submodule.name == m.name {
			mod = m
			break
		}
	}

	// if we didn't match, exit
	if mod.name == "" {
		return "", ErrNoSubmodule
	}

	// get latest commit for this submodule
	latest, hash, err := g.getLatest(mod)
	if err != nil {
		return "", err
	}

	// Find the most significant change between HEAD and latest
	commits, err := g.repo.RevList("HEAD", hash)
	if err != nil {
		return "", fmt.Errorf("could not fetch commits HEAD..%s: %w", hash, err)
	}

	// group commits by modules and only keep the ones that match submodule
	groups := groupCommitsByModule(commits, modules)
	commits = groups[mod]

	// If this is the latest tagged commit, then return
	var version string
	if len(commits) > 0 {
		change, breaking := g.parseCommits(commits)
		switch {
		case breaking:
			version = latest.IncMajor().String()
		case change == commit.TypeFeature:
			version = latest.IncMinor().String()
		default:
			version = latest.IncPatch().String()
		}
	} else {
		version = latest.String()
	}

	return mod.prefix + g.Config.VersionPrefix + version, nil
}

var versionRegex = regexp.MustCompile(`/v\d+$`)

type module struct {
	path   string
	name   string
	prefix string
}

type sortByPath []module

func (s sortByPath) Len() int      { return len(s) }
func (s sortByPath) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortByPath) Sort()         { sort.Sort(s) }
func (s sortByPath) Less(i, j int) bool {
	si, sj := s[i], s[j]
	if len(si.path) < len(sj.path) {
		return true
	}
	return si.path < sj.path
}

func findAllModules(root string) (modules []module, err error) {
	err = filepath.Walk(root, func(pth string, info os.FileInfo, err error) error {
		// bail on errors
		if err != nil {
			return err
		}

		// ignore directories
		if info.IsDir() {
			// don't recurse into the .git directory
			if strings.HasSuffix(info.Name(), ".git") {
				return filepath.SkipDir
			}
			return nil
		}

		// add the directory leading up to any valid go.mod
		relPath := strings.TrimPrefix(pth, root+string(filepath.Separator))
		if strings.HasSuffix(relPath, string(filepath.Separator)+goMod) || relPath == goMod {
			data, err := ioutil.ReadFile(pth)
			if err != nil {
				return err
			}

			// ignore go.mods that don't parse a module path
			if modName := modfile.ModulePath(data); modName != "" {
				modPath := filepath.Dir(relPath)

				// convert rootModule to empty string, otherwise add a trailing slash
				modPrefix := modPath
				if modPrefix == rootModulePath {
					modPrefix = ""
				} else {
					// determine the major version prefix for this module
					major := strings.TrimPrefix(versionRegex.FindString(modName), "/")

					// strip trailing major version directory from prefix
					modPrefix = strings.TrimSuffix(modPrefix, major)
					if modPrefix != "" && !strings.HasSuffix(modPrefix, "/") {
						modPrefix += "/"
					}
				}

				modules = append(modules, module{modPath, modName, modPrefix})
			}
		}

		return nil
	})

	sortByPath(modules).Sort()
	return
}

func groupCommitsByModule(commits []*object.Commit, modules []module) (grouped map[module][]*object.Commit) {
	grouped = make(map[module][]*object.Commit)
	for _, commit := range commits {
		stats, err := commit.Stats()
		if err != nil {
			continue
		}

		for _, stat := range stats {
			if mod, ok := isModuleFile(stat.Name, modules); ok {
				grouped[mod] = append(grouped[mod], commit)
				break
			}
		}
	}

	return
}

func isModuleFile(filename string, modules []module) (mod module, ok bool) {
	// make map of module path to module for quicker lookup below
	moduleMap := map[string]module{}
	for _, m := range modules {
		moduleMap[m.path] = m
	}

	for dir := filepath.Dir(filename); ; dir = filepath.Dir(dir) {
		mod, ok = moduleMap[dir]
		// break out of the loop if we found a module or hit the root path
		if ok || dir == rootModulePath {
			break
		}
	}

	return
}

func validateCommitModules(commitModules, changedModules []module) (err error) {
	// create a set of commit modules
	commitMap := make(map[string]bool)
	for _, m := range commitModules {
		commitMap[m.name] = true
	}

	// create a set of changed modules
	changedMap := make(map[string]bool)
	for _, m := range changedModules {
		changedMap[m.name] = true
	}

	var extra []string
	for modName := range commitMap {
		if _, ok := changedMap[modName]; !ok {
			// this is extra
			extra = append(extra, modName)
		}
	}
	sort.StringSlice(extra).Sort()

	var missing []string
	for modName := range changedMap {
		if _, ok := commitMap[modName]; !ok {
			// this is missing
			missing = append(missing, modName)
		}
	}
	sort.StringSlice(missing).Sort()

	var msg string
	if len(extra) > 0 {
		msg += "\nmodules not changed by commit: " + strings.Join(extra, ", ")
	}
	if len(missing) > 0 {
		msg += "\nchanged modules not released by commit: " + strings.Join(missing, ", ")
	}

	if msg != "" {
		err = errors.New("module validation failed:" + msg)
	}

	return err
}

// TagRepo determines what the curent version of the repository is by parsing the commit
// history since previous release and returns that version. Depending on the state of
// the Config passed it, it may also create the tag and push it.
//
// This function is deprecated and will be removed before the v1.0.0 release of gotagger.
func TagRepo(cfg *Config, r ggit.Repo) (*semver.Version, error) {
	// Find the latest semver and the commit hash it references.
	latest, commitHash, err := getLatest(r, cfg.VersionPrefix)
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

func alreadyTagged(v *semver.Version, c ggit.Commit) bool {
	for _, t := range c.Tags {
		if v.Equal(t) {
			return true
		}
	}
	return false
}

func isRelease(c ggit.Commit) bool {
	m, _, _ := marker.Parse(c.Subject)
	return m == marker.Release
}

func getLatest(r git.Repo, prefix string) (latest *semver.Version, hash string, err error) {
	taggedCommits, err := r.Tags(prefix)
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

func scanForMarkers(commits []ggit.Commit) (mark marker.Marker, isBreaking bool) {
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
