// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package mapper

import "fmt"

func Convert(inc string) (Increment, error) {
	switch inc {
	case "major":
		return IncrementMajor, nil
	case "minor":
		return IncrementMinor, nil
	case "patch":
		return IncrementPatch, nil
	case "none", "":
		return IncrementNone, nil
	}
	return IncrementNone, fmt.Errorf("invalid version increment '%s'", inc)
}

type Increment int

const (
	IncrementNone  = iota
	IncrementPatch = iota
	IncrementMinor = iota
	IncrementMajor = iota
)

const (
	TypeFeature     = "feat"
	TypeBugFix      = "fix"
	TypeRelease     = "release"
	TypeRefactor    = "refactor"
	TypePerformance = "perf"
	TypeTest        = "test"
	TypeStyle       = "style"
	TypeBuild       = "build"
	TypeChore       = "chore"
	TypeCI          = "ci"
	TypeDocs        = "docs"
	TypeRevert      = "revert"
)

// All other commit types are patch by default.
var defaultCommitTypeMapper = map[string]Increment{
	TypeFeature: IncrementMinor,
}

type Mapper map[string]Increment

type Table struct {
	Mapper
	defaultInc Increment
}

func NewTable(tm Mapper, defInc Increment) Table {
	mapper := tm
	if mapper == nil {
		mapper = defaultCommitTypeMapper
	}

	return Table{
		Mapper:     mapper,
		defaultInc: defInc,
	}
}

// Get returns the configured increment for the provided commit type. Returns the default increment if no mapping for
// the input type is found.
func (t Table) Get(typ string) Increment {
	// release type is always a patch increment
	if typ == TypeRelease {
		return IncrementPatch
	}

	inc, ok := t.Mapper[typ]
	if !ok {
		return t.defaultInc
	}

	return inc
}
