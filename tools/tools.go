// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// +build tools

package main

import (
	_ "github.com/AlekSi/gocov-xml"
	_ "github.com/axw/gocov/gocov"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/goreleaser/goreleaser"
	_ "github.com/wfscheper/stentor"
	_ "gotest.tools/gotestsum"
)
