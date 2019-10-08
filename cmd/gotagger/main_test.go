// Copyright (c) SAS Institute, Inc.

package main

import (
	"fmt"
	"runtime"
	"testing"
)

func TestVersionInfo(t *testing.T) {
	got := versionInfo("1", "abcdefg", "Today")
	want := fmt.Sprintf(`gotagger:
 version     : 1
 build date  : Today
 git hash    : abcdefg
 go version  : %s
 go compiler : %s
 platform    : %s/%s
`, runtime.Version(), runtime.Compiler, runtime.GOOS, runtime.GOARCH)
	if want != got {
		t.Errorf("WANT:\n%s\nGOT:\n%s", want, got)
	}
}
