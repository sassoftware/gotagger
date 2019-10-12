# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Fixed
- Always create annotated tags, otherwise we can't find our own tags.
- Call `git log` with the `--decorate=full` option, so that tags are properly prefixed
  with `refs/tags/`

## [0.1.0] - 2019/10/11
### Added
- git package for interacting with git repository
- marker package for parsing commmit markers
- basic cli capability: printing the new version and tagging a repo

[Unreleased]: https://github.com/sassoftware/gotagger/compare/0.1.0...master
[0.1.0]: https://github.com/sassoftware/gotagger/compare/e3ef062...0.1.0
