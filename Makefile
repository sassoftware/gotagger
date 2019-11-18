# commands
GO          = go
GOBUILD     = $(GO) build
GOCOV       = $(TOOLBIN)/gocov
GOCOVXML    = $(TOOLBIN)/gocov-xml
GOINSTALL  := GOOS= GOARCH= $(GO) install
LINTER      = $(TOOLBIN)/golangci-lint
TESTER      = $(TOOLBIN)/gotestsum

# variables
BUILDDATE := $(shell date +%Y-%m-%d)
COMMIT    := $(shell git rev-parse HEAD)
GOOS      := $(shell $(GO) env GOOS)
VERSION   := $(shell $(GO) run ./cmd/gotagger)

# directories
REPORTDIR = build/reports
TOOLBIN   = build/tools

# flags
BUILDFLAGS  = -v -ldflags '-X main.AppVersion=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILDDATE)'
COVERFLAGS  = -covermode $(COVERMODE) -coverprofile $(COVEROUT)
COVERMODE   = atomic
COVEROUT    = $(REPORTDIR)/coverage.out
COVERXML    = $(REPORTDIR)/coverage.xml
LINTFLAGS   =
REPORTFLAGS = --jsonfile $(REPORTJSON) --junitfile $(REPORTXML)
REPORTJSON  = $(REPORTDIR)/go-test.json
REPORTXML   = $(REPORTDIR)/go-test.xml
TESTFLAGS   = -- -timeout $(TIMEOUT) $(COVERFLAGS)
TIMEOUT     = 60s

TARGET = build/$(GOOS)/gotagger
TOOLREQS = tools/go.mod tools/go.sum

# recipes
.PHONY: all
all: lint build test

.PHONY: build
build: $(TARGET)

.PHONY: clean
clean:
	$(RM) -r $(TARGET) $(REPORTDIR)

.PHONY: distclean
distclean: clean
	$(RM) -r $(TOOLBIN)

.PHONY: format
format: LINTFLAGS += --fix
format: lint

.PHONY: lint
lint: | $(LINTER)
	$(LINTER) run $(LINTFLAGS)

.PHONY: reports
report: TESTFLAGS := $(REPORTFLAGS) $(TESTFLAGS)
report: test | $(GOCOV) $(GOCOVXML)
	$(GOCOV) convert $(COVEROUT) | $(GOCOVXML) > $(COVERXML)

.PHONY: test tests
test tests: | $(TESTER) $(REPORTDIR)
	$(TESTER) $(TESTFLAGS) ./...

$(TARGET):
	$(GOBUILD) $(BUILDFLAGS) -o $@ ./cmd/gotagger/main.go

$(REPORTDIR) $(TOOLBIN):
	@mkdir -p $@

tools/go.mod tools/go.sum: tools/tools.go
	cd tools/ && go mod tidy

define installtool
$1: tools/go.mod tools/go.sum | $$(TOOLBIN)
	cd tools/ && GOBIN=$$(CURDIR)/$$(TOOLBIN) $$(GOINSTALL) $2

endef

# tool targets
$(eval $(call installtool,$(GOCOV),github.com/axw/gocov/gocov))
$(eval $(call installtool,$(GOCOVXML),github.com/AlekSi/gocov-xml))
$(eval $(call installtool,$(LINTER),github.com/golangci/golangci-lint/cmd/golangci-lint))
$(eval $(call installtool,$(TESTER),gotest.tools/gotestsum))
