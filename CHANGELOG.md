<!-- markdownlint-disable -->
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Changelog](#changelog)
  - [[v0.9.1] - 2023-09-16](#v091---2023-09-16)
    - [Fixed](#fixed)
  - [[v0.9.0] - 2022-07-20](#v090---2022-07-20)
    - [Removed](#removed)
    - [Added](#added)
    - [Fixed](#fixed-1)
    - [Chores](#chores)
    - [CI](#ci)
  - [[v0.8.0] - 2022-03-11](#v080---2022-03-11)
    - [Added](#added-1)
  - [[v0.7.0] - 2021-12-08](#v070---2021-12-08)
    - [Added](#added-2)
  - [[v0.6.3] - 2021-08-26](#v063---2021-08-26)
    - [Fixed](#fixed-2)
  - [[v0.6.2] - 2021-07-09](#v062---2021-07-09)
    - [Fixed](#fixed-3)
    - [CI](#ci-1)
  - [[v0.6.1] - 2021-02-16](#v061---2021-02-16)
    - [Fixed](#fixed-4)
  - [[v0.6.0] - 2020/10/12](#v060---20201012)
    - [Feature](#feature)
    - [Fix](#fix)
  - [[v0.5.2] - 2020/09/22](#v052---20200922)
    - [Fix](#fix-1)
  - [[v0.5.1] - 2020/09/17](#v051---20200917)
    - [Refactor](#refactor)
  - [[v0.5.0] - 2020/09/17](#v050---20200917)
    - [Feature](#feature-1)
    - [Fix](#fix-2)
    - [Refactor](#refactor-1)
  - [[v0.4.0] - 2019/07/10](#v040---20190710)
    - [Added](#added-3)
    - [Fixed](#fixed-5)
  - [[v0.3.1] - 2019/12/16](#v031---20191216)
    - [Fixed](#fixed-6)
  - [[v0.3.0] - 2019/11/18](#v030---20191118)
    - [Added](#added-4)
  - [[v0.2.0] - 2019/11/15](#v020---20191115)
    - [Added](#added-5)
    - [Changed](#changed)
  - [[v0.1.2] - 2019/10/14](#v012---20191014)
    - [Fixed](#fixed-7)
  - [[v0.1.1] - 2019/10/12](#v011---20191012)
    - [Fixed](#fixed-8)
  - [[v0.1.0] - 2019/10/11](#v010---20191011)
    - [Added](#added-6)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->
<!-- markdownlint-enable -->

<!-- markdownlint-disable MD013 MD024 -->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!-- stentor output starts -->
## [v0.9.1] - 2023-09-16

### Fixed

- `gotagger` now returns a minimum version based on the Go module version.

  For instance,
  if you bump a module from v1 to v2,
  `gotagger` will begin reporting the version as `v2.0.0`.
  [#211](https://github.com/sassoftware/gotagger/issues/211)


[v0.9.1]: https://github.com/sassoftware/gotagger/compare/v0.9.0...v0.9.1


----


## [v0.9.0] - 2022-07-20

### Removed

- This release of `gotagger` removes the deprecated *git* and *marker* packages,
  and the *TagRepo* public function that used them.

  `gotagger` is on it's 4th release
  since these were deprecated,
  and a v1.0.0 release is probably not far off.
  This is a good time to make this breaking change.
  [#105](https://github.com/sassoftware/gotagger/issues/105)


### Added

- Added a `Force` option to `gotagger.Config` and a corresponding `-force` flag to the CLI.
  Setting this to `true` forces the creation of a tag, even if the HEAD is not a "release" commit.
  [#11](https://github.com/sassoftware/gotagger/issues/11)
- `gotagger` now takes a `-path` flag
  that restricts version calculation
  to commits that affected files below that directory.
  [#93](https://github.com/sassoftware/gotagger/issues/93)


### Fixed

- Fixed an issue where `gotagger` could not version the first commit.
  [#103](https://github.com/sassoftware/gotagger/issues/103)
- Fixed an issue where `gotagger`
  would return an error
  versioning a go module that was not yet tagged.
  [#104](https://github.com/sassoftware/gotagger/issues/104)
- `gotagger` error output is now correctly written to stderr.

  The error output of the CLI was incorrectly sent to stdin.
  [#61](https://github.com/sassoftware/gotagger/issues/61)
- `gotagger` properly parses scopes with hyphens.

  The regex used by gotagger
  is now essentially the same
  as the default used by conventional-commit-parser.
  [#95](https://github.com/sassoftware/gotagger/issues/95)


### Chores

- The minimum version of go is now 1.17.
  Go 1.18 was released on 2022-03-15,
  which means go 1.16 is now EOL.

  The go version in the go.mod is updated to 1.17,
  so that gotagger can take advantage of the depdnency pruning changes
  introduced in that version of go.
  Generally,
  this is not desirable,
  but this time the benefits are worth the potential disruption.
  [#54](https://github.com/sassoftware/gotagger/issues/54)

### CI

- Set the git author name and email when generating tags.

[v0.9.0]: https://github.com/sassoftware/gotagger/compare/v0.8.0...v0.9.0


----


## [v0.8.0] - 2022-03-11

### Added

- Add a feature allowing users
  to configure semantic version increments.
  This is done by way of a config file,
  which is optionally passed to Gotagger.
  [#30](https://github.com/sassoftware/gotagger/issues/30)


[v0.8.0]: https://github.com/sassoftware/gotagger/compare/v0.7.0...v0.8.0


----


## [v0.7.0] - 2021-12-08

### Added

- Drop support for building `gotagger` with go 1.15
  now that it is no longer supported.
  [#20](https://github.com/sassoftware/gotagger/issues/20)
- Add a way to change how `gotagger` increments the version
  when there are no new commits,
  but the worktree is dirty.
  For the CLI, use the `-dirty` option.
  Go API users, set `Config.DirtyWorktreeIncrement`.
  [#24](https://github.com/sassoftware/gotagger/issues/24)


[v0.7.0]: https://github.com/sassoftware/gotagger/compare/v0.6.3...v0.7.0


----


## [v0.6.3] - 2021-08-26

### Fixed

- Fixed a bug where `gotagger` calculated the wrong major version for go modules.

  When finding the latest tag for a go module,
  `gotagger` was only considering tags that matched the major version of the module.
  This caused `gotagger` to essentially always calculate the version as `v1.0.0`.
  The filtering was changed to only filter out tags whose major version are greater than the module version.
  [#17](https://github.com/sassoftware/gotagger/issues/17)


[v0.6.3]: https://github.com/sassoftware/gotagger/compare/v0.6.2...v0.6.3


----


## [v0.6.2] - 2021-07-09

### Fixed

- Fix a bug in how `gotagger` calculates the version
  when a commit contains changes to more than one go module.
  [#12](https://github.com/sassoftware/gotagger/issues/12)


### CI

- Run `make release` as part of validating PR.
  This should catch invalid release commits,
  and other issues that could cause a release to fail.


[v0.6.2]: https://github.com/sassoftware/gotagger/compare/v0.6.1...v0.6.2


----


## [v0.6.1] - 2021-02-16

### Fixed

- When creating repositories during tests,
  configure the user name and email
  to avoid failures in CI.
  [#1](https://github.com/sassoftware/gotagger/issues/1)
- The way gotagger was determining the path of a module
  relative to the root of the repository
  did not work correctly for Windows paths.

  This fixes the problem by using the `filepath.Rel` call instead.
  [#3](https://github.com/sassoftware/gotagger/issues/3)
- `Gotagger.Version()` now reports the correct version
  when there are commits to multiple go modules.
  [#4](https://github.com/sassoftware/gotagger/issues/4)
- Change the module name to `github.com/sassoftware/gotagger`.

  We need to use this module name until the sassoftware.io URL is ready.
  [#7](https://github.com/sassoftware/gotagger/issues/7)


[v0.6.1]: https://github.com/sassoftware/gotagger/compare/v0.6.0...v0.6.1


----


## [v0.6.0] - 2020/10/12

### Feature

- Added an `IgnoreModules` option,
  and matching `-modules` flag,
  to control whether `gotagger` enforces go module versioning.
  This allows non-golang projects
  to use a go.mod for build tooling
  but opt-out of module versioning rules.

### Fix

- When running gotagger on windows the path to the root module was `\`.
- Ensure gotagger uses `/` characters in module prefixes and not `\`
  when deriving the module prefix from the module path.

## [v0.5.2] - 2020/09/22

### Fix

- ModuleVersions does not validate release commits.
  This behavior prevented using ModuleVersions
  to report the version of modules not in the release commit.

## [v0.5.1] - 2020/09/17

### Refactor

- Removed remaining use of github.com/go-git/go-git outside of the test suite.

## [v0.5.0] - 2020/09/17

### Feature

- Add an `ExcludeModules` option to `Config`.

  This is a list of module names
  or whole paths
  to ignore.

- Add a `PreMajor` option to `Config`.

  When `PreMajor` is true, `gotagger` will not rev the major version to 1,
  even if commits are flagged as breaking changes. This has no effect if the
  major version is 1 or higher.

- `TagRepo` and `ModuleVersions` validate
  that a release commit references only modules that are changed by the commit
  and that the commit references all of the changed modules.
- Add a `ModuleVersions` function that takes a variadic list of module names,
  and returns the versions of those modules,
  or all modules if called with no arguments.
- Add a `Version` function
  that returns the version of the project.

  In a multi-module repository,
  `Version` returns the version of the first module found.

- Add support for tagging any go module via release commits.

  A release commit may contain a `Modules` footer
  that is a comma-separated list of module names for gotagger to tag.

### Fix

- `Gotagger` no longer ignores all non-root go modules when given a relative path.
- Correctly set `CreateTag` option to `true` when `-push` flag is used.
- `gotagger` correctly ignores directories named `testdata`
  and directories that begin with `.` and `_`
  when looking for go modules.

### Refactor

- Rewrite git and conventional commit parsing.

  This is preparing for full go module support.
  The existing commit parsing
  and git repository interactions
  won't scale to solve the problem of tagging modules.
  These packages will remain until the v1.0.0 release of `gotagger`.

## [v0.4.0] - 2019/07/10

### Added

- The `gotagger` cli now takes
  `-remote`
  and `-prefix`
  options to set the name
  of the remote to push to
  and the version prefix,
  respectively.

### Fixed

- `gotagger` only considers tags that match the version prefix when determining
  the base version.

## [v0.3.1] - 2019/12/16

### Fixed

- `gotagger` no longer reports all git command failures as "not a git repository".

## [v0.3.0] - 2019/11/18

### Added

- The base package now exposes a `Config` struct and a `TagRepo` function that
  preforms the basic operations of `gotagger`.

## [v0.2.0] - 2019/11/15

### Added

- Add `-push` and `-release` flags to control when `gotagger` tags a release commit
  and pushes the commit.
- Source options from `GOTAGGER_`-prefixed environment variables.

### Changed

- When tagging a release commit, increment the patch version if there are no
  feat or fix commits since the last release.

## [v0.1.2] - 2019/10/14

### Fixed

- Use `--merged` argument to `git tag` so that we only generate tags that point to
  parents of HEAD.

## [v0.1.1] - 2019/10/12

### Fixed

- Always create annotated tags, otherwise we can't find our own tags.
- Call `git log` with the `--decorate=full` option, so that tags are properly prefixed
  with `refs/tags/`
- Remove unnecessary quotes from `git tag` format. These were being included in the
  formatted string.
- Address a bug in the cli where we tried to do a release when HEAD is already tagged.

## [v0.1.0] - 2019/10/11

### Added

- git package for interacting with git repository
- marker package for parsing commit markers
- basic cli capability: printing the new version and tagging a repo

[v0.5.1]: https://github.com/sassoftware/gotagger/compare/v0.5.0...v0.5.1
[v0.5.0]: https://github.com/sassoftware/gotagger/compare/v0.4.0...v0.5.0
[v0.4.0]: https://github.com/sassoftware/gotagger/compare/v0.3.1...v0.4.0
[v0.3.1]: https://github.com/sassoftware/gotagger/compare/v0.3.0...v0.3.1
[v0.3.0]: https://github.com/sassoftware/gotagger/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/sassoftware/gotagger/compare/v0.1.2...v0.2.0
[v0.1.2]: https://github.com/sassoftware/gotagger/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/sassoftware/gotagger/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/sassoftware/gotagger/compare/e3ef062...v0.1.0
