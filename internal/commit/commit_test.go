package commit

import (
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"pgregory.net/rapid"
	"sassoftware.io/clis/gotagger/internal/testutils"
)

func TestParse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ctype := rapid.StringMatching(`^\w*$`).Draw(t, "type").(string)
		scope := rapid.StringMatching(`^\w*$`).Draw(t, "scope").(string)
		isBreaking := rapid.Bool().Draw(t, "breaking").(bool)
		subject := rapid.StringMatching(`^.*$`).Draw(t, "subject").(string)
		body := rapid.SliceOf(rapid.String()).Map(func(s []string) string {
			return strings.Join(s, "\n")
		}).Draw(t, "body").(string)

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
				Type:     Type(ctype),
				Scope:    scope,
				Subject:  strings.TrimSpace(subject),
				Body:     strings.TrimSpace(body),
				Breaking: isBreaking,
				Header:   header,
			}
		}
		if got, want := Parse(input), c; !reflect.DeepEqual(got, want) {
			t.Errorf("Parse(%s) returned\n%s\nwant\n%s", input, spew.Sdump(got), spew.Sdump(want))
		}
	})
}

func TestParse_empty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := rapid.StringMatching(`^\s*`).Draw(t, "input").(string)
		if got, want := Parse(input), (Commit{}); !reflect.DeepEqual(got, want) {
			t.Errorf("Parse(%q) returned %+v, want %+v", input, got, want)
		}
	})
}

func TestParse_merge(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ctype := rapid.StringMatching(`^\w+$`).Draw(t, "type").(string)
		scope := rapid.StringMatching(`^\w*$`).Draw(t, "scope").(string)
		isBreaking := rapid.Bool().Draw(t, "breaking").(bool)
		subject := rapid.StringMatching(`^.+$`).Draw(t, "subject").(string)
		body := rapid.SliceOf(rapid.String()).Map(func(s []string) string {
			return strings.Join(s, "\n")
		}).Draw(t, "body").(string)

		header := ctype
		if scope != "" {
			header += "(" + scope + ")"
		}
		if isBreaking {
			header += "!"
		}
		header += ": " + subject

		input := "Merge \"" + header + "\"" + "\n\n" + body
		if got, want := Parse(input), (Commit{
			Type:     Type(ctype),
			Scope:    scope,
			Subject:  strings.TrimSpace(subject),
			Body:     strings.TrimSpace(body),
			Breaking: isBreaking,
			Header:   header,
			Merge:    true,
		}); !reflect.DeepEqual(got, want) {
			t.Errorf("Parse(%s) returned\n%#v\nwant %#v", input, got, want)
		}
	})
}

func TestParse_revert(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ctype := rapid.StringMatching(`^\w+$`).Draw(t, "type").(string)
		scope := rapid.StringMatching(`^\w*$`).Draw(t, "scope").(string)
		isBreaking := rapid.Bool().Draw(t, "breaking").(bool)
		subject := rapid.StringMatching(`^.+$`).Draw(t, "subject").(string)
		hash := rapid.StringMatching(`^\w*$`).Draw(t, "hash").(string)

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
				Type:     Type(ctype),
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
		if got, want := Parse(input), c; !reflect.DeepEqual(got, want) {
			testutils.DiffErrorf(t, "Parse(%s) returned\n%s\nwant%s\ndiff%s", got, want, input)
		}
	})
}

func TestParse_arbitrary(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := rapid.String().Draw(t, "input").(string)
		if got, want := Parse(input), (Commit{}); !reflect.DeepEqual(got, want) {
			testutils.DiffErrorf(t, "Parse(%s) returned:\n%s\nwant:\n%s\ndiff:%s\n", got, want, input)
		}
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
		).(string)
		bFooterText := rapid.
			SliceOf(rapid.String().Filter(func(s string) bool { return s != "" })).
			Map(func(s []string) string { return strings.Join(s, "\n") }).
			Draw(t, "bFooterText").(string)
		footerTitle := rapid.StringMatching(`^([[:alnum:]][-\w ]*)?`).Draw(t, "footerTitle").(string)
		footerText := rapid.
			SliceOf(rapid.String().Filter(func(s string) bool { return s != "" })).
			Map(func(s []string) string { return strings.Join(s, "\n") }).
			Draw(t, "footerText").(string)
		input := header + "\n\n" + body + "\n\n" + bFooterTitle + ": " + bFooterText
		if footerTitle != "" {
			input += "\n" + footerTitle + ": " + footerText
		}
		isBreaking := strings.EqualFold(bFooterTitle, "breaking change") || strings.EqualFold(bFooterTitle, "breaking-change")

		c := Parse(input)

		// check that we got the number of headers expected
		var want int
		if bFooterTitle != "" && footerTitle != "" {
			want = 2
		} else if (bFooterTitle != "" && footerTitle == "") || (bFooterTitle == "" && footerTitle != "") {
			want = 1
		}
		if got, want := len(c.Footers), want; got != want {
			t.Fatalf("Parse(%q) returned an unexected footer: %+v", input, c.Footers)
		}

		// check that we parsed a breaking change
		if got, want := c, isBreaking; got.Breaking != want {
			t.Errorf("Parse(%q) returned Breaking %v", input, got.Breaking)
		}

		// validate that footer structs are correct
		errorF := func(got, want interface{}) {
			t.Errorf("Parse(%q) returned %+v, want %+v", input, got, want)
		}
		if isBreaking {
			if got, want := c.Footers[0], (Footer{bFooterTitle, bFooterText}); !reflect.DeepEqual(got, want) {
				errorF(got, want)
			}
		}
		if footerTitle != "" {
			var i int
			if isBreaking {
				i = 1
			}
			if got, want := c.Footers[i], (Footer{footerTitle, footerText}); !reflect.DeepEqual(got, want) {
				errorF(got, want)
			}
		}
	})
}

func Test_parseMessageBody(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		footerTitle := rapid.StringMatching(
			`^[bB][rR][eE][aA][kK][iI][nN][gG]-[cC][hH][aA][nN][gG][eE]`,
		).Draw(t, "footerTitle").(string)
		footerText := rapid.String().Draw(t, "footerText").(string)
		inputBody := "Some text"
		input := inputBody + "\n\n" + footerTitle + ": " + footerText

		body, footers, breaking := parseMessageBody(strings.Split(input, "\n"))
		if got, want := body, inputBody; got != want {
			t.Errorf("parseMessageBody returned body %q, want %q", got, want)
		}

		if got, want := footers, []Footer{{Title: footerTitle, Text: footerText}}; !reflect.DeepEqual(got, want) {
			testutils.DiffErrorf(t, "parseMessageBody returned:\n%s\nwant:\n%s\ndiff:\n%s", got, want)
		}

		if !breaking {
			t.Errorf("parseMessageBody returned breaking %v", breaking)
		}
	})
}
