<!-- markdownlint-disable -->
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [gotagger](#gotagger)
    - [Usage](#usage)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->
<!-- markdownlint-enable -->

# gotagger

## Overview

`gotagger` is a CLI for tagging releases in git repositories as part of a CI process.


### Installation

You can install `gotagger`
by downloading a pre-built binary for your OS and architecture
from our [releases](./releases) page.

Alternatively, you can install `gotagger` directly with `go get`.
If you go this route,
we recommend that you create a "fake" module,
so you can ensure you build a supported release:

```bash
mkdir tmp
cd tmp
go mod init fake
go get sassoftware.io/clis/gotagger
```


## Getting started

### Running

Just run `gotagger` inside of a git repository.
`gotagger` will look back through the git history for the current commit
in order to determine what the last tagged version was.
This becomes the "base" version.
`gotagger` then looks through all of the commit messages
between the current commit and that tag,
and checks if any commit is a
feature,
bug fix,
or breaking change
per the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.
`gotagger` will then increment the base version accordingly
and print the new version.

Additionally,
if the current commit contains the `release` type,
and the `-release` flag
or `GOTAGGER_RELEASE` environment variable is set,
then gotagger will tag the current commit with the new version.
If there are no commits explicitly marked as a feature or a bug fix,
then the patch version is incremented.

## Contributing

> We welcome your contributions!
  Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details
  on how to submit contributions to this project.


## License

> This project is licensed under the [Apache 2.0 License](LICENSE).
