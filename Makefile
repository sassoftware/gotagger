# commands
GO = go
GOBUILD = $(GO) build
GOCOV   = $(TOOLBIN)/gocov
GOCOVXML = $(TOOLBIN)/gocov-xml
LINTER  = $(TOOLBIN)/golangci-lint
TESTER  = $(TOOLBIN)/gotestsum

# variables
BUILDDATE := $(shell date +%Y-%m-%d)
COMMIT    := $(shell git rev-parse HEAD)
GOOS      := $(shell $(GO) env GOOS)
VERSION    = 0.1.0

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

# real targets
$(GOCOV): $(TOOLREQS) | $(TOOLBIN)
	$(call installtool,github.com/axw/gocov/gocov)

$(GOCOVXML): | $(TOOLBIN)
	$(call installtool,github.com/AlekSi/gocov-xml)

$(LINTER): | $(TOOLBIN)
	$(call installtool,github.com/golangci/golangci-lint/cmd/golangci-lint)

$(TARGET):
	$(GOBUILD) $(BUILDFLAGS) -o $@ ./cmd/gotagger/main.go

$(TESTER): | $(TOOLBIN)
	$(call installtool,gotest.tools/gotestsum)

$(REPORTDIR) $(TOOLBIN):
	@mkdir -p $@

define installtool
	cd tools/ && GOBIN=$(CURDIR)/$(TOOLBIN) go install $1
endef
