# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.3.1] -  2019/12/16

### Fixed

- gotagger no longer reports all git command failures as "not a git repository".

## [v0.3.0] - 2019/11/18

### Added

- The base package now exposes a Config struct and a TagRepo function that preforms the
  basic operations of gotagger.

## [v0.2.0] - 2019/11/15

### Added

- Add -push and -release flags to control when gotagger tags a releasea commit
  and pushes the commit.
- Source options from `GOTAGGER_`-prefixed environment variables.

### Changed

- When tagging a release commit, increment the patch version if there are no feat or fix
  commits since the last release.

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
- marker package for parsing commmit markers
- basic cli capability: printing the new version and tagging a repo

[v0.3.1]: https://github.com/sassoftware/gotagger/compare/v0.3.0...v0.3.1
[v0.3.0]: https://github.com/sassoftware/gotagger/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/sassoftware/gotagger/compare/v0.1.2...v0.2.0
[v0.1.2]: https://github.com/sassoftware/gotagger/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/sassoftware/gotagger/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/sassoftware/gotagger/compare/e3ef062...v0.1.0
