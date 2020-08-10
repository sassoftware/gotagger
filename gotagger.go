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
//
// If HEAD is a release commit, then every module referenced by the commit
// message must contain at least one file with changes in the commit.
func (g *Gotagger) ModuleVersions(names ...string) ([]string, error) {
	modules, err := g.findAllModules(names)
	if err != nil {
		return nil, err
	}

	head, err := g.repo.Head()
	if err != nil {
		return nil, err
	}

	if _, _, err := g.validateCommit(head); err != nil {
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
	head, err := g.repo.Head()
	if err != nil {
		return nil, err
	}

	// validate that if this is a release commit it is correct
	cc, modules, err := g.validateCommit(head)
	if err != nil {
		return nil, err
	}

	// collect versions we need to create
	versions, err := g.versions(modules)
	if err != nil {
		return nil, err
	}

	// if this is a release commit, then validate that modules are correct
	if cc.Type == commit.TypeRelease && g.Config.CreateTag {
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
// of the first module found by a depth-first, lexicographical search.
func (g *Gotagger) Version() (string, error) {
	modules, err := g.findAllModules(nil)
	if err != nil {
		return "", err
	}

	// only calculate the version of the first module found
	if len(modules) > 0 {
		modules = modules[:0]
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
			if strings.HasPrefix(dirname, ".") || strings.HasPrefix(dirname, "_") || dirname == "testdata" {
				return filepath.SkipDir
			}

			return nil
		}

		// add the directory leading up to any valid go.mod
		relPath := strings.TrimPrefix(pth, g.repo.Path+string(filepath.Separator))
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

func (g *Gotagger) validateCommit(head *object.Commit) (commit.Commit, []module, error) {
	// parse HEAD's commit message
	cc := commit.Parse(head.Message)

	// get all modules
	modules, err := g.findAllModules(nil)
	if err != nil {
		return commit.Commit{}, nil, err
	}

	// if no modules were found, then skip validation
	if len(modules) == 0 {
		return cc, nil, nil
	}

	// map module name to module
	moduleNameMap := map[string]module{}
	for _, m := range modules {
		moduleNameMap[m.name] = m
	}

	// extract modules from Modules footers
	var commitModules []module
	for _, footer := range cc.Footers {
		if footer.Title == "Modules" {
			for _, moduleName := range strings.Split(footer.Text, ",") {
				moduleName = strings.TrimSpace(moduleName)
				m, ok := moduleNameMap[moduleName]
				if !ok {
					return commit.Commit{}, nil, fmt.Errorf("no module %s found", moduleName)
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

	if cc.Type == commit.TypeRelease {
		stats, err := head.Stats()
		if err != nil {
			return commit.Commit{}, nil, err
		}

		// generate a list of modules changed by this commit
		var changedModules []module
		for _, stat := range stats {
			if mod, ok := isModuleFile(stat.Name, modules); ok {
				changedModules = append(changedModules, mod)
			}
		}

		if err := validateCommitModules(commitModules, changedModules); err != nil {
			return commit.Commit{}, nil, err
		}
	}

	return cc, commitModules, nil
}

func (g *Gotagger) versions(modules []module) ([]string, error) {
	if len(modules) == 0 {
		// no modules, so fake a "root" module with no name or prefix
		modules = []module{{path: "."}}
	}

	versions := make([]string, len(modules))
	for i, mod := range modules {
		// get latest commit for this submodule
		latest, hash, err := g.getLatest(mod)
		if err != nil {
			return nil, err
		}

		// Find the most significant change between HEAD and latest
		commits, err := g.repo.RevList("HEAD", hash)
		if err != nil {
			return nil, fmt.Errorf("could not fetch commits HEAD..%s: %w", hash, err)
		}

		// group commits by modules and only keep the ones that match submodule
		groups := groupCommitsByModule(commits, modules)
		commits = groups[mod]

		// If this is the latest tagged commit, then return
		var version string
		if len(commits) > 0 {
			change, breaking := g.parseCommits(commits)
			// set breaking false if this is a 0.x.y version and PreMajor is set
			if g.Config.PreMajor && latest.Major() == 0 {
				breaking = false
			}
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

		versions[i] = mod.prefix + g.Config.VersionPrefix + version
	}

	return versions, nil
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
