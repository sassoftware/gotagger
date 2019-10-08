// Copyright (c) SAS Institute, Inc.

package main

import (
	"fmt"
	"runtime"
)

const (
	versionOutput = `gotagger:
 version     : %s
 build date  : %s
 git hash    : %s
 go version  : %s
 go compiler : %s
 platform    : %s/%s
`
)

var (
	AppVersion = "dev"
	Commit     = "unknown"
	BuildDate  = "none"
)

func main() {
	fmt.Println(versionInfo(AppVersion, Commit, BuildDate))
}

func versionInfo(version, commit, date string) string {
	return fmt.Sprintf(versionOutput, version, date, commit, runtime.Version(),
		runtime.Compiler, runtime.GOOS, runtime.GOARCH)
}
