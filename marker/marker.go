// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package marker implements finding and working with markers in commit messages.
//
// This package is deprecated and will be removed before the v1.0.0 release of gotagger.
package marker

import (
	"regexp"
	"strings"
)

type Marker string

const (
	NoMarker Marker = ""
	Build    Marker = "build"
	Docs     Marker = "docs"
	Feature  Marker = "feat"
	Fix      Marker = "fix"
	Perf     Marker = "perf"
	Refactor Marker = "refactor"
	Release  Marker = "release"
	Style    Marker = "style"
	Test     Marker = "test"
)

var (
	typeRe = regexp.MustCompile(`^((?P<type>[a-z]+)(?P<scope>\([a-z]+\))?(?P<breaking>!)?:)?`)
)

// Parse returns the commit type, scopke and a boolean indicating if this is a breaking change.
func Parse(s string) (Marker, string, bool) {
	match := typeRe.FindStringSubmatch(s)
	if len(match) == 0 {
		return NoMarker, "", false
	}
	commitType, commitScope, breaking := match[2], match[3], match[4]
	// trim () from scope
	commitScope = strings.Trim(commitScope, "()")
	return Marker(commitType), commitScope, breaking == "!"
}

func IsBreaking(trailers []string) bool {
	for _, t := range trailers {
		if strings.HasPrefix(t, "Breaking-Change: ") {
			return true
		}
	}
	return false
}
