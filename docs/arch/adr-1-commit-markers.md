# Commit markers used by gotagger

- Status: accepted
- Deciders: Walter Scheper, Mike Camp, Bailey Hayes, Casey Hadden
- Date: 2019/10/11

## Table of Contents
<!-- markdownlint-disable -->
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Context and Problem Statement](#context-and-problem-statement)
- [Considered Options](#considered-options)
- [Decision Outcome](#decision-outcome)
- [Pros and Cons of the Options](#pros-and-cons-of-the-options)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->
<!-- markdownlint-enable -->

## Context and Problem Statement

We need to decided on a convention for commit message markers that gotagger will
use to determine how to modify the base version, and when to tag a release
commit. These markers should be clear in their meaning and not something what
would be part of a normal commit message.

## Considered Options

- Use hashtag markers: #{text}
- Use Sem-Ver: {text}
- Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)

## Decision Outcome

Decided to use conventional commits with the extended set of types plus a
release type for deciding when to tag the repo.

## Pros and Cons of the Options

Hashtags are very readable and simple, but the # conflicts with git's default
comment marker. This makes it hard to use hashtags at the beginning of lines
without adjusting git config settings. Conventional commits allows for more
descriptive change indicators without losing parsing or readability. The main
downside of conventional commits is that the type/scope prefix takes up some of
the limited characters generally allowed in a commit subject, but the trade off
seems worth it.
