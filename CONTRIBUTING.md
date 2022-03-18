# How to Contribute

## Table of Contents

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Types of Contributions](#types-of-contributions)
  - [Bug reports](#bug-reports)
- [Fix Bugs](#fix-bugs)
  - [Implement Features](#implement-features)
  - [Write Documentation](#write-documentation)
  - [Submit Feedback](#submit-feedback)
- [Development](#development)
  - [VS Code Dev Container](#vs-code-dev-container)
  - [Local Development](#local-development)
  - [Get Started](#get-started)
  - [Pull Request Guidelines](#pull-request-guidelines)
- [Changelog Entries](#changelog-entries)
- [Contributor License Agreement](#contributor-license-agreement)
- [Code reviews](#code-reviews)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

Contributions are welcome,
and they are greatly appreciated!
There's no such thing as a contribution that is "too small".
Grammar and spelling corrections
are just as important as fixing bugs or adding features.
Every little bit helps,
and credit will always be given.

You can contribute in many ways:


## Types of Contributions

### Bug reports

Report bugs in the [GitHub Issues tracker](https://github.com/sassoftware/gotagger/issues).

If you are reporting a bug, please include:

- Your operating system name and version.
- Any details about your local setup that might be helpful in troubleshooting.
- Detailed steps to reproduce the bug.


## Fix Bugs

Look through the issues for [bugs](https://github.com/sassoftware/gotagger/labels/bug).
Anything in that list is open to whoever wants to fix it.


### Implement Features

Look through the issues for [features](https://github.com/sassoftware/gotagger/labels/feature).
Anything in that list is open to whoever wants to implement it.


### Write Documentation

`gotagger` can always use more documentation,
whether as part of the official docs or in godocs.

`gotagger` uses [semantic newlines] in all documentation files and comments:

```text
This is a sentence.
This is another sentance,
with a clause.
```

### Submit Feedback

The best way to send feedback
is to file an [issue](https://github.com/sassoftware/gotagger/issues/new/choose).

If you are proposing a feature:

- Explain in detail how it would work.
- Keep the scope as narrow as possible, to make it easier to implement.


## Development

Ready to contribute? Here's how to set up `gotagger` for local development.


### VS Code Dev Container

The `gotagger` project provides a devcontainer setup for VS Code,
and a set of recommended extensions.

To use the devcontainer with VS Code,
first install the [Remote - Container] extension.

You can either follow the [Local Development](#local-development) instructions
and mount your local clone into the devcontainer,
or clone the repository into a docker volume.

Read the official [documentation](https://code.visualstudio.com/docs/remote/containers)
for details.


### Local Development

First,
make sure you have a supported version of [go](https://golang.org/dl/) installed.
While `gotagger` supports the two most recent releases,
development should be done with the lowest supported version.

You will also want [GNU make] >= 3.81 and [pre-commit]
to ensure that your changes will pass our CI checks.

```bash
pre-commit install -t commit-msg -t pre-commit
```


### Get Started

1. Make a fork of the `gotagger` repository
   by going to <https://github.com/sassoftware/gotagger>
   and clicking on the **Fork** button near the top of the page.
1. Clone your fork of the `gotagger` repository:

   ```bash
   git clone git@github.com:<username>/gotagger.git
   ```

1. Create a branch to track your changes.

   ```bash
   git checkout -b my-branch-name
   ```

1. Write your changes,
   making sure to run the tests and linters
   as you work.
1. When your changes are ready,
   commit them to your branch.

   ```bash
   git add .
   git commit
   ```

1. Push your changes to GitHub.

   ```bash
   git push -u origin my-branch-name
   ```


### Pull Request Guidelines

Before you submit a pull request,
first open an issue.
All pull requests must reference an issue
before they will be accepted.
Additionally,
check that it meets these guidelines:

- Pull requests that do not pass our [CI] checks will not receive feedback.
  If you need help passing the CI checks,
  add the `CI Triage` label to your pull request.
- Tests are required for all changes.
  If you fix a bug,
  add a test to ensure that bug stays fixed.
  If you add a feature,
  add a test to show how that feature works.
- New features require new documentation.
- `gotagger`'s API is still considered in flux,
  but API breaking changes need to clear a higher bar than new features.
- Commits must use the [conventional commits] standard.
  You also may want to read [How to Write a Git Commit Message].
  The conventional commit standard takes precedence
  when it and *How to Write a Git Commit Message* disagree.
- All changes should include a changelog entry.
  Add a single file to the `.stentor.d` directory
  as part of your pull request named
  `<pull request #>.(security|deprecate|remove|change|feature|fix).<short-description>.md`.
  See [below](#changelog-entries) for details.


## Changelog Entries

`gotagger` uses a news file management tool called [stentor]
to update the CHANGELOG.md file.

Changelog entries should follow these rules:

- Use [semantic newlines],
  just like other documentation.
- Wrap the names of things in backticks: `like this`.
- Wrap arguments with asterisks: *these* or *attributes*.
- Names of functions or other callables should be followed by parentheses:
  `my_cool_function()`.
- Use the active voice and either present tense or simple past tense.

  ```markdown
  - Added `my_cool_function()` to do cool things.
  - Creating `Foo` objects with the _many_ argument no longer raises a `RuntimeError`.
  ```

- For changes that address multiple pull requests,
  create one fragment for the primary pull request
  and reference the others in the body.


## Contributor License Agreement

Contributions to this project must be accompanied by a signed
[Contributor Agreement](https://github.com/sassoftware/gotagger/ContributorAgreement.txt).
You (or your employer) retain the copyright to your contribution,
this simply gives us permission to use
and redistribute your contributions as part of the project.


## Code reviews

All submissions,
including submissions by project members,
require review.
We use GitHub pull requests for this purpose.
Consult [GitHub Help] for more information on using pull requests.


[GNU make]: https://www.gnu.org/software/make/
[How to Write a Git Commit Message]: https://chris.beams.io/posts/git-commit/
[Remote - Container]: https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers
[ci]: https://github.com/sassoftware/gotagger/actions?query=workflow%3ACI
[pre-commit]: https://pre-commit.com/
[rapid]: https://github.com/flyingmutant/rapid
[semantic newlines]: https://rhodesmill.org/brandon/2012/one-sentence-per-line/
[stentor]: https://github.com/wfscheper/stentor
[table-driven tests]: https://github.com/golang/go/wiki/TableDrivenTests
