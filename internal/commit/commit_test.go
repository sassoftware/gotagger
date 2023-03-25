// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package commit

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestCommit_Message(t *testing.T) {
	tests := []struct {
		commit Commit
		want   string
	}{
		{
			commit: Commit{Header: "header"},
			want:   "header",
		},
		{
			commit: Commit{Header: "header", Body: "body"},
			want:   "header\n\nbody",
		},
		{
			commit: Commit{Header: "header", Footers: []Footer{{"title", "text"}}},
			want:   "header\n\ntitle: text",
		},
		{
			commit: Commit{Header: "header", Body: "body", Footers: []Footer{{"title", "text"}}},
			want:   "header\n\nbody\n\ntitle: text",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			message := tt.commit.Message()
			assert.Equal(t, tt.want, message)
		})
	}
}

func TestParse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ctype := rapid.StringMatching(`^\w*$`).Draw(t, "type")
		scope := rapid.StringMatching(`[\w$.\-*/ ]*`).Draw(t, "scope")
		isBreaking := rapid.Bool().Draw(t, "breaking")
		subject := rapid.StringMatching(`^.*$`).Draw(t, "subject")
		body := rapid.Map(rapid.SliceOf(
			rapid.String().Filter(func(s string) bool { return !strings.Contains(s, ": ") }),
		), func(s []string) string {
			return strings.Join(s, "\n")
		}).Draw(t, "body")

		header := ctype
		if scope != "" {
			header += "(" + scope + ")"
		}
		if isBreaking {
			header += "!"
		}
		if subject != "" {
			header += ": " + subject
		}
		input := header
		if body != "" {
			input += "\n\n" + body
		}
		var c Commit
		if ctype != "" && subject != "" {
			c = Commit{
				Type:     ctype,
				Scope:    scope,
				Subject:  strings.TrimSpace(subject),
				Body:     strings.TrimSpace(body),
				Breaking: isBreaking,
				Header:   header,
			}
		}
		got := Parse(input)
		assert.Equal(t, c, got)
	})
}

func TestParse_empty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := rapid.StringMatching(`^\s*`).Draw(t, "input")
		got := Parse(input)
		assert.Equal(t, Commit{}, got)
	})
}

func TestParse_merge(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ctype := rapid.StringMatching(`^\w+$`).Draw(t, "type")
		scope := rapid.StringMatching(`^\w*$`).Draw(t, "scope")
		isBreaking := rapid.Bool().Draw(t, "breaking")
		subject := rapid.StringMatching(`^.+$`).Draw(t, "subject")
		body := rapid.Map(
			rapid.SliceOf(
				rapid.String().Filter(func(s string) bool { return !strings.Contains(s, ": ") }),
			),
			func(s []string) string {
				return strings.Join(s, "\n")
			},
		).Draw(t, "body")

		header := ctype
		if scope != "" {
			header += "(" + scope + ")"
		}
		if isBreaking {
			header += "!"
		}
		header += ": " + subject

		want := Commit{
			Type:     ctype,
			Scope:    scope,
			Subject:  strings.TrimSpace(subject),
			Body:     strings.TrimSpace(body),
			Breaking: isBreaking,
			Header:   header,
			Merge:    true,
		}

		input := "Merge \"" + header + "\"" + "\n\n" + body
		got := Parse(input)
		assert.Equal(t, want, got)
	})
}

func TestParse_revert(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ctype := rapid.StringMatching(`^\w+$`).Draw(t, "type")
		scope := rapid.StringMatching(`^\w*$`).Draw(t, "scope")
		isBreaking := rapid.Bool().Draw(t, "breaking")
		subject := rapid.StringMatching(`^.+$`).Draw(t, "subject")
		hash := rapid.StringMatching(`^\w*$`).Draw(t, "hash")

		header := ctype
		if scope != "" {
			header += "(" + scope + ")"
		}
		if isBreaking {
			header += "!"
		}
		header += ": " + subject

		input := "Revert \"" + header + "\"\n\nThis reverts commit " + hash + "."
		var c Commit
		if hash != "" {
			c = Commit{
				Type:     ctype,
				Scope:    scope,
				Subject:  strings.TrimSpace(subject),
				Body:     strings.TrimSpace("This reverts commit " + hash + "."),
				Breaking: isBreaking,
				Header:   header,
				Revert: Revert{
					Header: header,
					Hash:   hash,
				},
			}
		}
		got := Parse(input)
		assert.Equal(t, c, got)
	})
}

func TestParse_arbitrary(t *testing.T) {
	want := Commit{}
	rapid.Check(t, func(t *rapid.T) {
		input := rapid.String().Draw(t, "input")
		got := Parse(input)
		assert.Equal(t, want, got)
	})
}

func TestParse_footer(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		header := "feat: some feature"
		body := "some text\nthat people wrote"
		bFooterTitle := rapid.StringMatching(
			`^([bB][rR][eE][aA][kK][iI][nN][gG](-| )[cC][hH][aA][nN][gG][eE])?$`,
		).Draw(
			t, "bFooterTitle",
		)
		bFooterText := rapid.Map(
			rapid.SliceOf(
				rapid.
					String().
					Filter(func(s string) bool {
						if bFooterTitle != "" {
							return s != "" && !strings.Contains(s, ": ")
						}

						return false
					})),
			func(s []string) string { return strings.Join(s, "\n") },
		).Draw(t, "bFooterText")
		footerTitle := rapid.StringMatching(`^([[:alnum:]][-\w ]*)?`).Draw(t, "footerTitle")
		footerText := rapid.Map(
			rapid.SliceOf(
				rapid.
					String().
					Filter(func(s string) bool {
						if footerTitle != "" {
							return s != "" && !strings.Contains(s, ": ")
						}

						return false
					})),
			func(s []string) string { return strings.Join(s, "\n") },
		).Draw(t, "footerText")
		input := header + "\n\n" + body + "\n\n" + bFooterTitle + ": " + bFooterText
		if footerTitle != "" {
			input += "\n" + footerTitle + ": " + footerText
		}
		isBreaking := strings.EqualFold(bFooterTitle, "breaking change") || strings.EqualFold(bFooterTitle, "breaking-change")

		c := Parse(input)

		// check that we got the number of headers expected
		var want int
		if bFooterTitle != "" {
			want++
		}
		if footerTitle != "" {
			want++
		}
		if got := len(c.Footers); want != got {
			t.Errorf("wrong number of footers: want %d, got %d", want, got)
		}

		// check that we parsed a breaking change
		if want, got := isBreaking, c.Breaking; want != got {
			t.Errorf("want c.Breaking == %v, got %v", want, got)
		}

		// validate that footer structs are correct
		if isBreaking {
			if got, want := c.Footers[0], (Footer{bFooterTitle, bFooterText}); !reflect.DeepEqual(want, got) {
				t.Errorf("expected footer %#v, got %#v", want, got)
			}
		}

		if footerTitle != "" {
			var i int
			if isBreaking {
				i = 1
			}
			if got, want := c.Footers[i], (Footer{footerTitle, footerText}); !reflect.DeepEqual(got, want) {
				t.Errorf("want footer %#v, got %#v", want, got)
			}
		}
	})
}

func Test_parseMessageBody(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		footerTitle := rapid.StringMatching(
			`^[bB][rR][eE][aA][kK][iI][nN][gG]-[cC][hH][aA][nN][gG][eE]`,
		).Draw(t, "footerTitle")
		footerText := rapid.String().Draw(t, "footerText")
		inputBody := "Some text"
		input := inputBody + "\n\n" + footerTitle + ": " + footerText

		body, footers, breaking := parseMessageBody(strings.Split(input, "\n"))
		if got, want := body, inputBody; got != want {
			t.Errorf("want body %q, got %q", want, got)
		}
		if got, want := footers, []Footer{{Title: footerTitle, Text: footerText}}; !reflect.DeepEqual(got, want) {
			t.Errorf("wanted footers %#v, got %#v", want, got)
		}
		if !breaking {
			t.Errorf("wanted breaking change")
		}
	})
}
