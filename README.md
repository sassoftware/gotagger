<!-- markdownlint-disable -->
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [gotagger](#gotagger)
    - [Usage](#usage)
        - [Go Submodule Support](#go-submodule-support)
    - [Using gotagger as a library](#using-gotagger-as-a-library)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->
<!-- markdownlint-enable -->
<!-- markdownlint-disable MD013 -->

# gotagger

## Overview

`gotagger` is a library and CLI
for tagging releases in git repositories
as part of a CI process.

`gotagger` looks through the git history of the current commit
for the latest semantic version.
This becomes the "base" version.
Then `gotagger` examines all of the commit messages
between the current commit and that tag,
to determine if the most significant change was a
feature,
bug fix,
or breaking change
per the [Conventional Commits] format.
`gotagger` then increments the base version accordingly
and print the new version.

If the current commit type is `release`
and the `-release` flag
or `GOTAGGER_RELEASE` environment variable is set,
then gotagger will tag the current commit with the new version.
If there are no commits explicitly marked as a feature or a bug fix,
then the patch version is incremented.


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

Run `gotagger` inside of a git repository to see what the current version is.

```bash
git clone https://github.com/sassoftware/gotagger.git
cd gotagger
make build
build/$(go env GOOS)/gotagger
v0.4.0
```

**Note**: The version reported may be different,
depending on what unreleased changes exist.

To tag a release, make any changes need to prepare your project for releasing
(ie. update the change log, merge any feature branches).
Then create a "release" commit and run gotagger again:

```bash
VERSION="$(gotagger)"
git commit -m "release: $VERSION"
gotagger -release
```

You can now perform any release builds,
push the tag to your central git repository,
or any other post-release tasks.


### Go Submodule Support

`gotagger` can also tag go submodules.
To tag one ore more submodules,
include a `Modules` footer in your commit message:

```text
release: my/submodule and my/other-module

Modules: my/submodule, my/other-module
```

You can also use multiple `Modules` footers if you prefer:

```text
release: my/submodule and my/other-module

Modules: my/submodule
Modules: my/other-module
```

You can release the "main" module by using the "." character in the Modules list:

```text
release: foo and submodule bar

Modules: bar
Modules: .

# "Modules: bar, ." also works
```

`gotagger` will print out all of the versions it tagged
in the order they are specified in the `Modules` footer.


## Using gotagger as a library

```go
import sassoftware.io/clis/gotagger
```

Create a Gotagger instance

```go
g, err := gotagger.New("path/to/repo")
if err != nil {
    return err
}

// get the current version of the main module
version, err := g.Version()
if err != nil {
    return err
}
fmt.Println("version:", version)

// get the version of submodule foo
fooVersion, err := g.SubmoduleVersion("foo")
if err != nil {
    return err
}
fmt.Println("foo version:", fooVersion)

// Check what versions will be tagged.
// If HEAD is not a release commit,
// then only the the main module version is returned.
versions, err := g.TagRepo()
if err != nil {
    return err
}

for _, v := range versions {
    fmt.Println(v)
}

// Create the tags
g.Config.CreateTag = true

// uncomment to push tags as well
// g.Config.PushTag = true

_, err := g.TagRepo()
if err != nil {
    return err
}
```


## Contributing

> We welcome your contributions!
  Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details
  on how to submit contributions to this project.


## License

> This project is licensed under the [Apache 2.0 License](LICENSE).

[Conventional Commits]: https://www.conventionalcommits.org/en/v1.0.0/
