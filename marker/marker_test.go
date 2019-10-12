package marker

import (
	"testing"
)

func TestParse(t *testing.T) {
	markers := []Marker{Build, Docs, Feature, Fix, Perf, Refactor, Release, Style, Test}
	scope := []string{"", "scope"}
	breaking := []bool{false, true}

	t.Parallel()
	for _, m := range markers {
		for _, s := range scope {
			for _, b := range breaking {
				commit := string(m)
				if s != "" {
					commit += "(" + s + ")"
				}
				if b {
					commit += "!"
				}
				t.Run(commit, func(t *testing.T) {
					commit += ": commit subject"
					gotType, gotScope, gotBreak := Parse(commit)
					if gotType != m {
						t.Errorf("want type '%s', got '%s'", m, gotType)
					}
					if gotScope != s {
						t.Errorf("want scope '%s', got '%s'", s, gotScope)
					}
					if gotBreak != b {
						t.Errorf("want breaking '%v', got '%v'", b, gotBreak)
					}
				})
			}
		}
	}
}

func TestParseNonStandard(t *testing.T) {
	gotType, _, _ := Parse("random: commit")
	if gotType != Marker("random") {
		t.Errorf("want 'random', got '%s'", gotType)
	}
}

func TestParseNoMarker(t *testing.T) {
	tests := []string{
		"a commmit",
		"bad type: commit",
		"type[scope]: commit",
		"!type: commit",
		"(scope): commit",
		"(scope)!: commit",
		"[scope]: commit",
		"[scope]!: commit",
		"type(): commit",
		"type()!: commit",
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			gotType, gotScope, gotBreak := Parse(tt)
			if gotType != NoMarker {
				t.Errorf("want type '', got %s", gotType)
			}
			if gotScope != "" {
				t.Errorf("want scope '', got '%s'", gotScope)
			}
			if gotBreak {
				t.Errorf("want breaking 'false', got '%v'", gotBreak)
			}
		})
	}
}

func TestIsBreaking(t *testing.T) {
	isBreaking := []string{"Breaking-Change: desc"}
	notBreaking := []string{"Change-Id: xxxx"}
	if !IsBreaking(isBreaking) {
		t.Errorf("want is breaking")
	}
	if IsBreaking(notBreaking) {
		t.Errorf("want not breaking")
	}
}
