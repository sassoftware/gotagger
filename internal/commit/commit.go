// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package commit

import (
	"regexp"
	"strings"
)

var (
	typeRe   = regexp.MustCompile(`^(?P<type>\w+)(?P<scope>\(\w+\))?(?P<breaking>!)?: (?P<subject>.+)`)
	mergeRe  = regexp.MustCompile(`^Merge "(.*)"$`)
	revertRe = regexp.MustCompile(`^Revert\s"([\s\S]+)"\s*This reverts commit (\w+)\.`)
	footerRe = regexp.MustCompile(`^(?P<title>[-\w ]+): (?P<text>.*)`)
)

// Commit represents the parsed data from a conventional commit message.
type Commit struct {
	Type     string
	Scope    string
	Subject  string
	Body     string
	Breaking bool
	Header   string
	Footers  []Footer
	Merge    bool
	Revert   Revert
}

func (c Commit) Message() string {
	message := c.Header
	if c.Body != "" {
		message += "\n\n" + c.Body
	}
	var footer string
	for _, f := range c.Footers {
		footer += "\n" + f.String()
	}

	if footer != "" {
		message += "\n" + footer
	}

	return message
}

// Footer represents a conventional commit footer, which roughly corresponds to a
// git trailer: Foo-bar: some text.
type Footer struct {
	Title string
	Text  string
}

func (f Footer) String() string {
	return f.Title + ": " + f.Text
}

// Revert represents what this commmit reverts.
type Revert struct {
	Header string
	Hash   string
}

// Parse parses a commit message and returns a conventional commit.
//
// If the message does not follow the format, then nil is returned.
func Parse(s string) (c Commit) {
	if s == "" {
		return
	}

	lines := strings.Split(s, "\n")
	header, lines := lines[0], lines[1:]

	// Is this a merge commit
	var merge bool
	if m := mergeRe.FindStringSubmatch(header); len(m) > 0 {
		merge = true
		header = m[1]
	}

	// is this a revert commit
	var revert Revert
	if m := revertRe.FindStringSubmatch(s); len(m) > 0 {
		revert.Header = m[1]
		revert.Hash = m[2]
		header = m[1]
	}

	m := typeRe.FindStringSubmatch(header)
	if len(m) == 0 {
		return
	}

	typ, scope, subject := m[1], strings.Trim(m[2], "()"), strings.TrimSpace(m[4])
	body, footers, breaking := parseMessageBody(lines)
	breaking = breaking || m[3] == "!"
	c = Commit{
		Type:     typ,
		Scope:    scope,
		Subject:  subject,
		Breaking: breaking,
		Body:     body,
		Header:   header,
		Footers:  footers,
		Merge:    merge,
		Revert:   revert,
	}
	return
}

func parseMessageBody(lines []string) (body string, footers []Footer, breaking bool) {
	var f Footer
	var inFooter bool
	for _, line := range lines {
		if m := footerRe.FindStringSubmatch(line); len(m) > 0 {
			if inFooter {
				// add the current footer to footers
				footers = append(footers, f)
			}
			// start a new footer
			f = Footer{
				Title: m[1],
				Text:  m[2],
			}
			breaking = breaking ||
				strings.EqualFold(f.Title, "BREAKING CHANGE") ||
				strings.EqualFold(f.Title, "Breaking-Change")
			inFooter = true
			continue
		}

		if inFooter {
			f.Text += "\n" + line
		} else {
			body += "\n" + line
		}
	}

	// check if we need to add the last footer
	if f.Title != "" {
		footers = append(footers, f)
	}

	// trim body
	body = strings.TrimSpace(body)

	return
}
