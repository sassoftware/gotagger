# gotagger

## Table of Contents

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Overview](#overview)
  - [Installation](#installation)
- [Getting started](#getting-started)
  - [Running](#running)
  - [Go Module Support](#go-module-support)
- [Using gotagger as a library](#using-gotagger-as-a-library)
- [Contributing](#contributing)
- [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->
<!-- markdownlint-disable MD012 MD013 -->

## Overview

`gotagger` is a library and CLI
for tagging releases in git repositories
as part of a CI process.

`gotagger` looks through the git history of the current commit
for the latest semantic version.
This becomes the "base" version.
Then `gotagger` examines all of the commit messages
between the current commit and the latest tag,
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
then `gotagger` will tag the current commit with the new version.
If there are no commits explicitly marked as a feature or a bug fix,
then the patch version is incremented.


### Installation

You can install `gotagger`
by downloading a pre-built binary for your OS and architecture
from our [releases](https://github.com/sassoftware/gotagger/releases) page.

Alternatively, you can install `gotagger` directly with `go get`.
If you go this route,
we recommend that you create a "fake" module,
so you can ensure you build a supported release:

```bash
mkdir tmp
cd tmp
go mod init fake
go get github.com/sassoftware/gotagger
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

To tag a release,
make any changes needed to prepare your project for releasing
(ie. update the change log,
merge any feature branches).
Then create a "release" commit and run gotagger again:

```bash
VERSION="$(gotagger)"
git commit -m "release: $VERSION"
gotagger -release
```

You can now perform any release builds,
push the tag to your central git repository,
or any other post-release tasks.

`gotagger` can also push any tags it creates,
by using the `-push` flag.

```bash
gotagger -release -push
```


### Go Module Support

By default `gotagger` will enforce
[semantic import versioning](https://github.com/golang/go/wiki/Modules#semantic-import-versioning)
on any project that has one or more `go.mod` files.
This means `gotagger` will ignore tags whose major version
does not match the major version of the module,
as well as tags whose prefix does not match the
path to the module's `go.mod` file.

For projects that are not written in go
but do have a `go.mod` for build tooling,
the `-modules` flag
and `GOTAGGER_MODULES` environment variable
can be used to disable this behavior.

`gotagger` can also tag go multi-module repositories.
To tag one ore more modules,
include a `Modules` footer in your commit message
containing a comma-separated list of modules to tag:

```text
release: the bar and baz modules

Modules: foo/bar, foo/baz
```

You can also use multiple `Modules` footers if you prefer:

```text
release: the bar and baz modules in separate footers

Modules: foo/bar
Modules: foo/baz
```

To release the "root" module explicitly list it in the `Modules` footer:

```text
release: foo and bar

Modules: foo, foo/bar

# "Modules: foo/bar, foo" also works
```

`gotagger` will print out all of the versions it tagged
in the order they are specified in the `Modules` footer.


## Using gotagger as a library

```go
import github.com/sassoftware/gotagger
```

Create a Gotagger instance

```go
g, err := gotagger.New("path/to/repo")
if err != nil {
    return err
}

// get the current version of a repository
version, err := g.Version()
if err != nil {
    return err
}
fmt.Println("version:", version)

// Uncomment this to ignore the module example.com/bar or any modules under some/path
// g.Config.ExcludeModules = []string{"example.com/bar", "some/path"}

// get the version of module foo
fooVersion, err := g.ModuleVersion("foo")
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
