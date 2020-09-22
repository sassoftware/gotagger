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
	"unicode"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/mod/modfile"
	ggit "sassoftware.io/clis/gotagger/git"
	"sassoftware.io/clis/gotagger/internal/commit"
	igit "sassoftware.io/clis/gotagger/internal/git"
	"sassoftware.io/clis/gotagger/marker"
)

const (
	goMod          = "go.mod"
	head           = "HEAD"
	rootModulePath = "."
)

var (
	ErrNoSubmodule = errors.New("no submodule found")
	ErrNotRelease  = errors.New("HEAD is not a release commit")
)

// Config represents how to tag a repo. If not default is mentioned, the option defaults
// to go's zero-value.
type Config struct {
	// CreateTag represents whether to create the tag.
	CreateTag bool

	// ExcludeModules is a list of module names or paths to exclude.
	ExcludeModules []string

	// RemoteName represents the name of the remote repository. Defaults to origin.
	RemoteName string

	// PreMajor controls whether gotagger will increase the major version from 0
	// to 1 for breaking changes.
	PreMajor bool

	// PushTag represents whether to push the tag to the remote git repository.
	PushTag bool

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

	return &Gotagger{Config: NewDefaultConfig(), repo: r}, nil
}

// ModuleVersions returns the current version for all go modules in the repository
// in the order they were found by a depth-first, lexicographically sorted search.
//
// For example, in a repository with a root go.mod and a submodule foo/bar, the
// slice returned would be: []string{"v0.1.0", "bar/v0.1.0"}
//
// If module names are passed in, then only the versions for those modules are
// returned.
func (g *Gotagger) ModuleVersions(names ...string) ([]string, error) {
	modules, err := g.findAllModules(names)
	if err != nil {
		return nil, err
	}

	return g.versions(modules)
}

// TagRepo determines the curent version of the repository by parsing the commit
// history since the previous release and returns that version. Depending
// on the CreateTag and PushTag configuration options tags may be created and
// pushed.
//
// If the current commit contains one or more Modules footers, then tags are
// created for each module listed. In this case if the root module is not
// explicitly included in a Modules footer then it will not be included.
func (g *Gotagger) TagRepo() ([]string, error) {
	// get all modules, if any
	modules, err := g.findAllModules(nil)
	if err != nil {
		return nil, err
	}

	// get the current HEAD commit
	c, err := g.repo.Head()
	if err != nil {
		return nil, err
	}

	// NOTE: we only validate the commit if this repository contains go modules
	var versions []string
	if len(modules) > 0 {
		// there are go modules, so we need to do a module aware version calculation
		// validate that if this is a release commit it is correct
		var commitModules []module
		commitModules, err = g.validateCommit(c, modules)
		if err != nil {
			return nil, err
		}

		// collect versions we need to create
		versions, err = g.versionsModules(modules, commitModules)
		if err != nil {
			return nil, err
		}
	} else {
		// this is the no module case, so just do a simple version calculation
		versions, err = g.versionsSimple()
		if err != nil {
			return nil, err
		}
	}

	// determine if we should create and push a tag or not
	if c.Type == commit.TypeRelease && g.Config.CreateTag {
		// create tag
		tags := make([]string, 0, len(versions))
		for _, ver := range versions {
			if err := g.repo.CreateTag(c.Hash, ver, "", false); err != nil {
				// clean up tags we already created
				if terr := g.repo.DeleteTags(tags); terr != nil {
					err = fmt.Errorf("%w\n%s", err, terr)
				}
				return nil, err
			}
			tags = append(tags, ver)
		}

		// push tags
		if g.Config.PushTag {
			if err := g.repo.PushTags(tags, g.Config.RemoteName); err != nil {
				// currently pushes are not atomic so some of the tags may be
				// pushed while others fail. we delete all of the local tags to
				// be safe
				if terr := g.repo.DeleteTags(tags); terr != nil {
					err = fmt.Errorf("%w\n%s", err, terr)
				}
				return nil, err
			}
		}
	}

	return versions, nil
}

// Version returns the current version for the repository.
//
// In a repository that contains multiple go modules, this returns the version
// of the first module found by a depth-first, lexicographically sorted search.
// Usually this is the root module, but possibly not if the repo is a monorepo
// with no root module.
func (g *Gotagger) Version() (string, error) {
	modules, err := g.findAllModules(nil)
	if err != nil {
		return "", err
	}

	// only calculate the version of the first module found
	if len(modules) > 0 {
		modules = modules[:1]
	}

	versions, err := g.versions(modules)
	if err != nil {
		return "", err
	}

	return versions[0], nil
}

func (g *Gotagger) findAllModules(include []string) (modules []module, err error) {
	// either return all modules, or only explicitly included modules
	modinclude := map[string]bool{}
	for _, name := range include {
		modinclude[name] = true
	}

	// ignore these modules
	modexclude := map[string]bool{}
	for _, name := range g.Config.ExcludeModules {
		modexclude[name] = true
	}

	pathexclude := make([]string, len(g.Config.ExcludeModules))
	for i, exclude := range g.Config.ExcludeModules {
		pathexclude[i] = normalizePath(exclude)
	}

	// walk root and find all modules
	err = filepath.Walk(g.repo.Path, func(pth string, info os.FileInfo, err error) error {
		// bail on errors
		if err != nil {
			return err
		}

		// ignore directories
		if info.IsDir() {
			// don't recurse into directories that start with '.', '_', or are named 'testdata'
			dirname := info.Name()
			if dirname != "." && (strings.HasPrefix(dirname, ".") || strings.HasPrefix(dirname, "_") || dirname == "testdata") {
				return filepath.SkipDir
			}

			return nil
		}

		// add the directory leading up to any valid go.mod
		relPath := strings.TrimPrefix(pth, g.repo.Path)
		relPath = strings.TrimPrefix(relPath, "/")

		if strings.HasSuffix(relPath, string(filepath.Separator)+goMod) || relPath == goMod {
			data, err := ioutil.ReadFile(pth)
			if err != nil {
				return err
			}

			// ignore go.mods that don't parse a module path
			if modName := modfile.ModulePath(data); modName != "" {
				modPath := filepath.Dir(relPath)

				// ignore module if it is not an included one
				if _, include := modinclude[modName]; !include && len(modinclude) > 0 {
					return nil
				}

				// ingore module if it is excluded by name
				if _, excludeName := modexclude[modName]; excludeName {
					// ignore this module
					return nil
				}

				// normalize module path to ease comparisons
				normPath := normalizePath(modPath)

				// see if an exclude is a prefix of normPath
				for _, exclude := range pathexclude {
					if strings.HasPrefix(normPath, exclude) {
						return nil
					}
				}

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

func (g *Gotagger) incrementVersion(v *semver.Version, commits []igit.Commit) (version string) {
	// If this is the latest tagged commit, then return
	if len(commits) > 0 {
		change, breaking := g.parseCommits(commits)
		switch {
		// ignore breaking if this is a 0.x.y version and PreMajor is set
		case breaking && !(g.Config.PreMajor && v.Major() == 0):
			version = v.IncMajor().String()
		case change == commit.TypeFeature:
			version = v.IncMinor().String()
		default:
			version = v.IncPatch().String()
		}
	} else {
		version = v.String()
	}

	return
}

func (g *Gotagger) latest(tags []string) (latest *semver.Version, hash string, err error) {
	latest = &semver.Version{}
	for _, tag := range tags {
		if tver, err := semver.NewVersion(tag); err == nil && latest.LessThan(tver) {
			hash, err = g.repo.RevParse(tag + "^{commit}")
			if err != nil {
				return nil, "", err
			}
			latest = tver
		}
	}

	return
}

func (g *Gotagger) latestModule(m module, tags []string) (*semver.Version, string, error) {
	var hash string
	latest := &semver.Version{}
	for _, tag := range tags {
		tagName := strings.TrimPrefix(tag, m.prefix)
		if tver, err := semver.NewVersion(tagName); err == nil && latest.LessThan(tver) {
			hash, err = g.repo.RevParse(tag + "^{commit}")
			if err != nil {
				return nil, "", err
			}
			latest = tver
		}
	}

	return latest, hash, nil
}

func (g *Gotagger) parseCommits(cs []igit.Commit) (ctype commit.Type, breaking bool) {
	for _, c := range cs {
		switch c.Type {
		case commit.TypeFeature:
			ctype = c.Type
		case commit.TypeBugFix:
			if ctype != commit.TypeFeature {
				ctype = c.Type
			}
		}
		if c.Breaking {
			breaking = true
		}
	}

	return ctype, breaking
}

func (g *Gotagger) validateCommit(c igit.Commit, modules []module) ([]module, error) {
	// if no modules were found, then skip validation
	if len(modules) == 0 {
		return nil, nil
	}

	// map module name to module
	moduleNameMap := map[string]module{}
	for _, m := range modules {
		moduleNameMap[m.name] = m
	}

	// extract modules from Modules footers
	var commitModules []module
	for _, footer := range c.Footers {
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

	// default to the root module, or the first module found
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

	if c.Type == commit.TypeRelease {
		// generate a list of modules changed by this commit
		var changedModules []module
		for _, change := range c.Changes {
			if mod, ok := isModuleFile(change.SourceName, modules); ok {
				changedModules = append(changedModules, mod)
			} else if mod, ok := isModuleFile(change.DestName, modules); ok {
				changedModules = append(changedModules, mod)
			}
		}

		if err := validateCommitModules(commitModules, changedModules); err != nil {
			return nil, err
		}
	}

	return commitModules, nil
}

func (g *Gotagger) versions(modules []module) (versions []string, err error) {
	if len(modules) != 0 {
		versions, err = g.versionsModules(modules, nil)
	} else {
		versions, err = g.versionsSimple()
	}

	return
}

var versionRegex = regexp.MustCompile(`/v\d+$`)

func (g *Gotagger) versionsModules(modules []module, commitModules []module) ([]string, error) {
	// if no commit modules, then get versions for all modules
	if len(commitModules) == 0 {
		commitModules = modules
	}

	versions := make([]string, len(commitModules))
	for i, mod := range commitModules {
		// we determine the tag prefix by concatinating the module prefix, the
		// version prefix, and the major version of this module.
		// the major version is the version part of the module name
		// (foo/v2, foo/v3) normalized to 'X.'
		var prefixes []string
		if major := strings.TrimPrefix(versionRegex.FindString(mod.name), "/"); major == "" {
			// no major version in module name, so v0.x and v1.x are allowed
			prefixes = []string{mod.prefix + "v0.*", mod.prefix + "v1.*"}
		} else {
			prefixes = []string{mod.prefix + major + ".*"}
		}

		// get tags that match the prefixes
		tags, err := g.repo.Tags(head, prefixes...)
		if err != nil {
			return nil, err
		}

		// get latest commit for this module
		latest, hash, err := g.latestModule(mod, tags)
		if err != nil {
			return nil, err
		}

		// find the commits between HEAD and latest that touched the module
		commits, err := g.repo.RevList(head, hash, mod.path)
		if err != nil {
			return nil, fmt.Errorf("could not fetch commits HEAD..%s: %w", hash, err)
		}

		// filter out commits that do not touch this module
		commits = filterCommitsByModule(mod, commits, modules)

		version := g.incrementVersion(latest, commits)
		versions[i] = mod.prefix + g.Config.VersionPrefix + version
	}

	return versions, nil
}

func (g *Gotagger) versionsSimple() ([]string, error) {
	// simple version calculation where we consider all tags that match the
	// configured prefix
	tags, err := g.repo.Tags(head, g.Config.VersionPrefix)
	if err != nil {
		return nil, err
	}

	// if the tag prefix is an empty string, then we need to filter out
	// any tags that *have* a prefix
	if g.Config.VersionPrefix == "" {
		filtered := make([]string, 0, len(tags))
		for _, tag := range tags {
			if unicode.IsDigit(rune(tag[0])) {
				filtered = append(filtered, tag)
			}
		}
		tags = filtered[:]
	}

	// find the latest tag and its hash
	latest, hash, err := g.latest(tags)
	if err != nil {
		return nil, err
	}

	// find all commits between HEAD and the latest tag
	commits, err := g.repo.RevList(head, hash)
	if err != nil {
		return nil, fmt.Errorf("could not fetch commits HEAD..%s: %w", hash, err)
	}

	// increment the version
	version := g.incrementVersion(latest, commits)
	return []string{g.Config.VersionPrefix + version}, nil
}

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

func filterCommitsByModule(mod module, commits []igit.Commit, modules []module) []igit.Commit {
	grouped := make(map[module][]igit.Commit)
	for _, commit := range commits {
		for _, change := range commit.Changes {
			if mod, ok := isModuleFile(change.SourceName, modules); ok {
				grouped[mod] = append(grouped[mod], commit)
				break
			}
			// check if the dest name touched this module
			if change.DestName != "" {
				if mod, ok := isModuleFile(change.DestName, modules); ok {
					grouped[mod] = append(grouped[mod], commit)
					break
				}
			}
		}
	}

	return grouped[mod]
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

func normalizePath(p string) string {
	// normalize to /
	p = filepath.ToSlash(p)

	// ensure leading "./"
	if !strings.HasPrefix(p, "./") && p != "." {
		p = "./" + p
	}

	// ensure trailing /
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}

	return p
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

	return
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
	commits, err := r.RevList(head, commitHash)
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
		c := commits[0]
		if (cfg.CreateTag || cfg.PushTag) && isRelease(c) && !alreadyTagged(latest, c) {
			if err := r.CreateTag(c.Hash, latest, cfg.VersionPrefix, "", false); err != nil {
				return nil, fmt.Errorf("could not tag HEAD (%s): %s", c.Hash, err)
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

func getLatest(r ggit.Repo, prefix string) (latest *semver.Version, hash string, err error) {
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
