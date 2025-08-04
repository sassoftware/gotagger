// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gotagger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"github.com/sassoftware/gotagger/internal/git"
	"github.com/sassoftware/gotagger/mapper"
	"golang.org/x/mod/modfile"
)

const (
	filepathSep    = string(filepath.Separator)
	goMod          = "go.mod"
	goModSep       = "/"
	head           = "HEAD"
	rootModulePath = "."
)

var (
	ErrNoSubmodule = errors.New("no submodule found")
	ErrNotRelease  = errors.New("HEAD is not a release commit")
)

type Gotagger struct {
	Config Config

	repo   *git.Repository
	logger logr.Logger
}

func New(path string) (*Gotagger, error) {
	r, err := git.New(path)
	if err != nil {
		return nil, err
	}

	return &Gotagger{
		Config: NewDefaultConfig(),
		logger: logr.Discard(),
		repo:   r,
	}, nil
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

	return g.versions(modules, nil)
}

func (g *Gotagger) SetLogger(l logr.Logger) {
	// we only really log debug messages,
	// so set the default V-level to 1
	l = l.V(1)
	l.Info("updating logger")
	g.logger = l.WithName("gotagger")
	g.repo.SetLogger(g.logger.WithName("git"))
}

// TagRepo determines the current version of the repository by parsing the commit
// history since the previous release and returns that version. Depending
// on the CreateTag and PushTag configuration options tags may be created and
// pushed.
//
// If the current commit contains one or more Modules footers, then tags are
// created for each module listed. In this case if the root module is not
// explicitly included in a Modules footer then it will not be included.
func (g *Gotagger) TagRepo() ([]string, error) {
	// get all modules, if any, unless we're explicitly ignoring them
	var modules []module
	if !g.Config.IgnoreModules {
		m, err := g.findAllModules(nil)
		if err != nil {
			return nil, err
		}
		modules = m
	}

	// get the current HEAD commit
	c, err := g.repo.Head()
	if err != nil {
		return nil, err
	}

	var commitModules []module
	if len(modules) > 0 {
		// there are go modules, so validate that if this is a release commit it is correct
		commitModules, err = extractCommitModules(c, modules)
		if err != nil {
			return nil, err
		}

		if err := g.validateCommit(c, modules, commitModules); err != nil {
			return nil, err
		}
	}

	versions, err := g.versions(modules, commitModules)
	if err != nil {
		return nil, err
	}

	// determine if we should create and push a tag or not
	if (g.Config.Force || c.Type == mapper.TypeRelease) && g.Config.CreateTag {
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
	// find modules unless we're explicitly ignoring them
	var modules []module
	if !g.Config.IgnoreModules {
		m, err := g.findAllModules(nil)
		if err != nil {
			return "", err
		}
		modules = m
	}

	versions, err := g.versions(modules, nil)
	if err != nil {
		return "", err
	}

	// only return the first version
	return versions[0], nil
}

func (g *Gotagger) findAllModules(include []string) (modules []module, err error) {
	g.logger.Info("finding modules")

	// either return all modules, or only explicitly included modules
	modinclude := map[string]struct{}{}
	for _, name := range include {
		g.logger.Info("explicitly including module", "module", name)
		modinclude[name] = struct{}{}
	}

	// ignore these modules
	modexclude := map[string]struct{}{}
	pathexclude := make([]string, len(g.Config.ExcludeModules))
	for i, name := range g.Config.ExcludeModules {
		g.logger.Info("excluding module", "module", name)
		modexclude[name] = struct{}{}
		pathexclude[i] = normalizePath(name)
	}

	// walk root and find all modules
	err = filepath.Walk(g.repo.Path, func(pth string, info os.FileInfo, err error) error {
		// bail on errors
		if err != nil {
			return err
		}

		logger := g.logger.WithValues("path", pth)

		// ignore directories
		if info.IsDir() {
			// don't recurse into directories that start with '.', '_', or are named 'testdata'
			dirname := info.Name()
			if dirname != "." && (strings.HasPrefix(dirname, ".") || strings.HasPrefix(dirname, "_") || dirname == "testdata") {
				logger.Info("not recursing into directory: ignored by default")
				return filepath.SkipDir
			}

			return nil
		}

		// add the directory leading up to any valid go.mod
		relPath, err := filepath.Rel(g.repo.Path, pth)
		if err != nil {
			return err
		}

		if strings.HasSuffix(relPath, filepathSep+goMod) || relPath == goMod {
			logger.Info("found go module")
			data, err := os.ReadFile(pth)
			if err != nil {
				return err
			}

			// ignore go.mods that don't parse a module path
			if modName := modfile.ModulePath(data); modName != "" {
				modPath := filepath.Dir(relPath)
				logger := logger.WithValues("module", modName, "modulePath", modPath)

				// ignore module if it is not an included one
				if _, include := modinclude[modName]; !include && len(modinclude) > 0 {
					logger.Info("ignoring module that is not explicitly included")
					return nil
				}

				// ignore module if it is excluded by name
				if _, excludeName := modexclude[modName]; excludeName {
					logger.Info("ignoring excluded module")
					// ignore this module
					return nil
				}

				// normalize module path to ease comparisons
				normPath := normalizePath(modPath)
				for _, exclude := range pathexclude {
					// see if an exclude is a prefix of normPath
					if strings.HasPrefix(normPath, exclude) {
						logger.Info("ignoring excluded module path")
						return nil
					}
				}

				// derive modPrefix from modPath
				modPrefix := filepath.ToSlash(modPath)
				if modPrefix == rootModulePath {
					modPrefix = ""
				} else {
					// determine the major version prefix for this module
					major := strings.TrimPrefix(versionRegex.FindString(modName), goModSep)

					// strip trailing major version directory from prefix
					modPrefix = strings.TrimSuffix(modPrefix, major)
					if modPrefix != "" && !strings.HasSuffix(modPrefix, goModSep) {
						modPrefix += goModSep
					}
				}

				logger.Info("adding moddule", "modulePrefix", modPrefix)
				modules = append(modules, module{modPath, modName, modPrefix})
			}
		}

		return nil
	})

	if len(modules) > 0 && len(g.Config.Paths) > 0 {
		err = errors.New("cannot use path filtering with go modules")
	}

	sortByPath(modules).Sort()
	return
}

func (g *Gotagger) incrementVersion(v *semver.Version, commits []git.Commit) (string, error) {

	// If this is the latest tagged commit, then return
	if len(commits) > 0 {
		change := g.parseCommits(commits, v)
		switch change {
		case mapper.IncrementMajor:
			g.logger.Info("incrementing major version")
			return v.IncMajor().String(), nil
		case mapper.IncrementMinor:
			g.logger.Info("incrementing minor version")
			return v.IncMinor().String(), nil
		case mapper.IncrementPatch:
			g.logger.Info("incrementing patch version")
			return v.IncPatch().String(), nil
		default:
			g.logger.Info("not incrementing version")
			return v.String(), nil
		}
	} else {
		isDirty, err := g.repo.IsDirty()
		if err != nil {
			return "", err
		}

		switch {
		case isDirty && g.Config.DirtyWorktreeIncrement == mapper.IncrementMinor:
			g.logger.Info("incrementing minor version due to dirty worktree")
			return v.IncMinor().String(), nil
		case isDirty && g.Config.DirtyWorktreeIncrement == mapper.IncrementPatch:
			g.logger.Info("incrementing patch version due to dirty worktree")
			return v.IncPatch().String(), nil
		default:
			return v.String(), nil
		}
	}
}

func (g *Gotagger) latest(tags []string, prefix string) (latest *semver.Version, hash string, err error) {
	logger := g.logger.WithValues("prefix", prefix)
	logger.Info("finding latest tag")

	latest = &semver.Version{}
	for _, tag := range tags {
		tagName := strings.TrimPrefix(tag, prefix)
		if tver, err := semver.NewVersion(tagName); err == nil && latest.LessThan(tver) {
			g.logger.Info("found newer tag", "tag", tver)
			hash, err = g.repo.RevParse(tag + "^{commit}")
			if err != nil {
				return nil, "", err
			}
			latest = tver
		}
	}

	return
}

// latestModule returns the latest version of m and the hash of the commit
// tagged with that version.
func (g *Gotagger) latestModule(tags []string, m module) (*semver.Version, string, error) {
	logger := g.logger.WithValues("module", m.name, "module_prefix", m.prefix, "module_path", m.path)
	logger.Info("finding latest tag for module")

	majorVersion := strings.TrimPrefix(versionRegex.FindString(m.name), goModSep)
	if majorVersion == "" {
		majorVersion = "v0"
	}

	moduleVersion, err := semver.NewVersion(majorVersion + ".0.0")
	if err != nil {
		return nil, "", err
	}

	_maximumVersion := moduleVersion.IncMajor()
	if majorVersion == "v0" {
		_maximumVersion = _maximumVersion.IncMajor()
	}
	maximumVersion := &_maximumVersion
	logger.Info("ignoring modules greater than " + g.Config.VersionPrefix + maximumVersion.String())

	var latestVersion *semver.Version
	var latestTag string
	for _, tag := range tags {
		// strip the module prefix from the tag so we can parse it as a semver
		tagName := strings.TrimPrefix(tag, m.prefix)
		// we want the highest version that is less than the next major version
		tver, err := semver.NewVersion(tagName)
		if err != nil {
			continue
		}
		if tver.Compare(maximumVersion) < 0 && tver.Compare(moduleVersion) >= 0 {
			if latestVersion == nil || latestVersion.LessThan(tver) {
				logger.Info("found newer tag", "tag", tag)
				latestVersion = tver
				latestTag = tag
			}
		}
	}

	// if there were no tags, then return the base module version
	if latestVersion == nil {
		return moduleVersion, "", nil
	}

	hash, err := g.repo.RevParse(latestTag + "^{commit}")
	if err != nil {
		return nil, "", err
	}

	logger.Info("found latest tag", "tag", latestVersion, "commit", hash)
	return latestVersion, hash, nil
}

func (g *Gotagger) parseCommits(cs []git.Commit, v *semver.Version) (vinc mapper.Increment) {
	g.logger.Info("determining version increment from commits")

	for _, c := range cs {
		logger := g.logger.WithValues("commit", c.Hash)
		inc := g.Config.CommitTypeTable.Get(c.Type)
		if c.Breaking {
			// ignore breaking if this is a 0.x.y version and PreMajor is set
			logger.Info("breaking change found")
			if !g.Config.PreMajor || v.Major() != 0 {
				return mapper.IncrementMajor
			}
			logger.Info("ignoring due to pre-release version")
		}

		switch inc {
		case mapper.IncrementMinor:
			logger.Info("minor increment")
			if vinc < mapper.IncrementMajor {
				vinc = inc
			}
		case mapper.IncrementPatch:
			logger.Info("patch increment")
			if vinc < mapper.IncrementMinor {
				vinc = inc
			}
		case mapper.IncrementNone:
			logger.Info("no increment")
			if vinc < mapper.IncrementPatch {
				vinc = inc
			}
		}
	}

	return vinc
}

func (g *Gotagger) validateCommit(c git.Commit, modules []module, commitModules []module) error {
	logger := g.logger.WithValues("commit", c.Hash)

	// if no modules were found, then skip validation
	if len(modules) == 0 {
		return nil
	}

	// map modules by path for faster lookup
	modulesByPath := mapModulesByPath(modules)

	if c.Type == mapper.TypeRelease {
		// generate a list of modules changed by this commit
		var changedModules []module
		for _, change := range c.Changes {
			if mod, ok := isModuleFile(change.SourceName, modulesByPath); ok {
				logger.Info("module affected by commit", "module", mod.name, "path", change.SourceName)
				changedModules = append(changedModules, mod)
			} else if mod, ok := isModuleFile(change.DestName, modulesByPath); ok {
				logger.Info("module affected by commit", "module", mod.name, "path", change.DestName)
				changedModules = append(changedModules, mod)
			}
		}

		if err := validateCommitModules(commitModules, changedModules); err != nil {
			return err
		}
	}

	return nil
}

func (g *Gotagger) versions(modules, commitModules []module) (versions []string, err error) {
	if len(modules) != 0 {
		g.logger.Info("enforcing module versioning")
		versions, err = g.versionsModules(modules, commitModules)
	} else {
		versions, err = g.versionsSimple()
	}

	return
}

var versionRegex = regexp.MustCompile(`/v\d+$`)

func (g *Gotagger) versionsModules(modules []module, commitModules []module) ([]string, error) {
	g.logger.Info("versioning modules")

	// if no commit modules, then get versions for all modules
	if len(commitModules) == 0 {
		commitModules = modules
	}

	versions := make([]string, len(commitModules))
	for i, mod := range commitModules {
		logger := g.logger.WithValues("module", mod.name)

		// we determine the tag prefix by concatenating the module prefix, the
		// version prefix, and the major version of this module.
		// the major version is the version part of the module name
		// (foo/v2, foo/v3) normalized to 'X.'
		prefix := g.Config.VersionPrefix
		if mod.prefix != "" {
			prefix = mod.prefix + prefix
		}

		// get tags that match the prefixes
		tags, err := g.repo.Tags(head, prefix)
		if err != nil {
			return nil, err
		}
		logger.Info("found tags", "tags", tags)

		// get latest commit for this module
		latest, hash, err := g.latestModule(tags, mod)
		if err != nil {
			return nil, err
		}

		// empty hash means this is a newly incremented module
		if hash == "" {
			versions[i] = prefix + latest.String()
			continue
		}

		// Find the commits between HEAD and latest
		// that touched any path under the module.
		// This list will need further filtering to deal with modules
		// that are sub-directories of this module.
		commits, err := g.repo.RevList(head, hash, mod.path)
		if err != nil {
			return nil, fmt.Errorf("could not fetch commits HEAD..%s: %w", hash, err)
		}

		// group the commits by the modules they affected
		commitsByModule := g.groupCommitsByModule(commits, modules)

		version, err := g.incrementVersion(latest, commitsByModule[mod])
		if err != nil {
			return nil, fmt.Errorf("could not increment version: %w", err)
		}

		versions[i] = prefix + version
	}

	return versions, nil
}

func (g *Gotagger) versionsSimple() ([]string, error) {
	// simple version calculation where we consider all tags that match the
	// configured prefix

	// need to ensure we default to the root path, "."
	if len(g.Config.Paths) == 0 {
		g.Config.Paths = []string{"."}
	}

	var versions []string
	for _, pth := range g.Config.Paths {
		version, err := g.versionPath(pth)
		if err != nil {
			return nil, err
		}

		versions = append(versions, version)
	}

	return versions, nil
}

func (g *Gotagger) versionPath(p string) (string, error) {
	prefix := g.Config.VersionPrefix

	tags, err := g.repo.Tags(head, prefix)
	if err != nil {
		return "", err
	}

	// if the tag prefix is an empty string, then we need to filter out
	// any tags that *have* a prefix
	if prefix == "" {
		filtered := make([]string, 0, len(tags))
		for _, tag := range tags {
			if unicode.IsDigit(rune(tag[0])) {
				filtered = append(filtered, tag)
			}
		}
		tags = filtered
	}

	// find the latest tag and its hash
	latest, hash, err := g.latest(tags, prefix)
	if err != nil {
		return "", err
	}

	// find all commits between HEAD and the latest tag that touch files under
	// directory p
	commits, err := g.repo.RevList(head, hash, p)
	if err != nil {
		return "", fmt.Errorf("could not fetch commits HEAD..%s: %w", hash, err)
	}

	// group the commits by the configured paths
	// this eliminates commits that only touched files that are
	// beneath subpaths of p
	commitsByPath := g.groupCommitsByPath(commits)

	// increment the version
	version, err := g.incrementVersion(latest, commitsByPath[p])
	if err != nil {
		return "", fmt.Errorf("could not increment version: %w", err)
	}

	return prefix + version, nil
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

// extractCommitModules returns the modules referenced in the commit Footer(s).
// If there are no modules referenced, then this returns the root module.
func extractCommitModules(c git.Commit, modules []module) ([]module, error) {
	// map module name to module for faster lookup
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
				if m, ok := moduleNameMap[moduleName]; ok {
					commitModules = append(commitModules, m)
				} else {
					return nil, fmt.Errorf("no module %s found", moduleName)
				}
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

	return commitModules, nil
}

func (g *Gotagger) groupCommitsByModule(commits []git.Commit, modules []module) map[module][]git.Commit {
	g.logger.Info("group commits by module")

	// map modules by path for faster lookup
	modulesByPath := mapModulesByPath(modules)

	grouped := map[module][]git.Commit{}
	for _, commit := range commits {
		logger := g.logger.WithValues("commit", commit.Hash)
		mappedModules := map[module]struct{}{}
		for _, change := range commit.Changes {
			if m, ok := isModuleFile(change.SourceName, modulesByPath); ok {
				logger.Info("module affected by commit", "module", m.name, "path", change.SourceName)
				if _, mapped := mappedModules[m]; !mapped {
					grouped[m] = append(grouped[m], commit)
					mappedModules[m] = struct{}{}
				}
				continue
			}
			// check if the dest name touched this module
			if change.DestName != "" {
				if m, ok := isModuleFile(change.DestName, modulesByPath); ok {
					logger.Info("module affected by commit", "module", m.name, "path", change.DestName)
					if _, mapped := mappedModules[m]; !mapped {
						grouped[m] = append(grouped[m], commit)
						mappedModules[m] = struct{}{}
					}
					continue
				}
			}
		}
	}

	return grouped
}

func (g *Gotagger) groupCommitsByPath(commits []git.Commit) map[string][]git.Commit {
	g.logger.Info("group commits by path")

	// make a map of paths for faster lookup
	pathsMap := map[string]string{}
	for _, p := range g.Config.Paths {
		pathsMap[p] = p
	}

	grouped := map[string][]git.Commit{}
	for _, commit := range commits {
		logger := g.logger.WithValues("commit", commit.Hash)
		for _, change := range commit.Changes {
			if p, ok := isPathFile(change.SourceName, pathsMap); ok {
				logger.Info("path affected by commit", "path", change.SourceName, "selectedPath", p)
				grouped[p] = append(grouped[p], commit)
			}

			if p, ok := isPathFile(change.DestName, pathsMap); ok {
				logger.Info("path affected by commit", "path", change.DestName, "selectedPath", p)
				grouped[p] = append(grouped[p], commit)
			}
		}
	}

	return grouped
}

func isModuleFile(filename string, moduleMap map[string]module) (mod module, ok bool) {
	for dir := filepath.Dir(filename); ; dir = filepath.Dir(dir) {
		mod, ok = moduleMap[dir]
		// break out of the loop if we found a module or hit the root path
		if ok || dir == rootModulePath {
			break
		}
	}

	return
}

func isPathFile(filename string, pathMap map[string]string) (p string, ok bool) {
	for dir := filepath.Dir(filename); ; dir = filepath.Dir(dir) {
		p, ok = pathMap[dir]
		// break out of the loop if we found a module or hit the root path
		if ok || dir == rootModulePath {
			break
		}
	}

	return
}

func mapModulesByPath(modules []module) map[string]module {
	// make map of module path to module for quicker lookup
	moduleMap := map[string]module{}
	for _, m := range modules {
		moduleMap[m.path] = m
	}

	return moduleMap
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
	commitMap := make(map[string]struct{})
	for _, m := range commitModules {
		commitMap[m.name] = struct{}{}
	}

	// create a set of changed modules
	changedMap := make(map[string]struct{})
	for _, m := range changedModules {
		changedMap[m.name] = struct{}{}
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
