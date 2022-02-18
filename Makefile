# commands
GO          = go
GOBUILD     = $(GO) build
GOCOV       = $(TOOLBIN)/gocov
GOCOVXML    = $(TOOLBIN)/gocov-xml
GOINSTALL  := GOOS= GOARCH= $(GO) install
GORELEASER  = $(TOOLBIN)/goreleaser
LINTER      = $(TOOLBIN)/golangci-lint
STENTOR     = $(TOOLBIN)/stentor
TESTER      = $(TOOLBIN)/gotestsum

# variables
BUILDDATE := $(shell date +%Y-%m-%d)
COMMIT    := $(shell git rev-parse HEAD)
GOOS      := $(shell $(GO) env GOOS)
VERSION   := $(shell $(GO) run ./cmd/gotagger)
$(if $(VERSION),,$(error failed to determine version))

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
TESTFLAGS   = --format=$(TESTFORMAT) -- -timeout $(TIMEOUT) $(COVERFLAGS)
TESTFORMAT  = short
TIMEOUT     = 60s

# conditional flags
ifeq ($(RELEASE_DRY_RUN),false)
TAGFLAGS     = -release -push
RELEASEFLAGS =
STENTORFLAGS = -release
else
TAGFLAGS     =
RELEASEFLAGS = --snapshot --skip-publish --rm-dist
STENTORFLAGS =
endif

TARGET = build/$(GOOS)/gotagger
TOOLREQS = tools/go.mod tools/go.sum

# recipes
.PHONY: all
all: lint build test

.PHONY: build
build: $(TARGET)

$(TARGET):
	$(GOBUILD) $(BUILDFLAGS) -o $@ ./cmd/gotagger/main.go


.PHONY: changelog
changelog: | $(STENTOR)
	$(STENTOR) $(STENTORFLAGS) $(VERSION) "$$(git tag --list --sort=version:refname | tail -n1)"

.PHONY: clean
clean:
	$(RM) $(TARGET)
	$(RM) -r $(REPORTDIR)/ dist/

.PHONY: distclean
distclean: clean
	$(RM) -r $(TOOLBIN)/

.PHONY: format
format: LINTFLAGS += --fix
format: lint

.PHONY: lint
lint: | $(LINTER)
	$(LINTER) run $(LINTFLAGS)

.PHONY: release
release: build | $(GORELEASER)
	$(TARGET) $(TAGFLAGS)
	BUILDDATE=$(BUILDDATE) \
	COMMIT=$(COMMIT) \
	VERSION=$(VERSION) \
	$(GORELEASER) $(RELEASEFLAGS)

.PHONY: report
report: TESTFLAGS := $(REPORTFLAGS) $(TESTFLAGS)
report: test | $(GOCOV) $(GOCOVXML)
	$(GOCOV) convert $(COVEROUT) | $(GOCOVXML) > $(COVERXML)

.PHONY: test tests
test tests: | $(TESTER) $(REPORTDIR)
	$(TESTER) $(TESTFLAGS) ./...

.PHONY: version
version:
	@echo $(VERSION)

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
$(eval $(call installtool,$(GORELEASER),github.com/goreleaser/goreleaser))
$(eval $(call installtool,$(LINTER),github.com/golangci/golangci-lint/cmd/golangci-lint))
$(eval $(call installtool,$(STENTOR),github.com/wfscheper/stentor/cmd/stentor))
$(eval $(call installtool,$(TESTER),gotest.tools/gotestsum))

.PHONY: help
help:
	@printf "Available targets:\
	\n  all         lint, build, and test code\
	\n  build       builds gotagger exectuable\
	\n  changelog   run stentor to show changelog entry\
	\n  clean       removes generated files\
	\n  distclean   reset's workspace to original state\
	\n  format      format source code\
	\n  lint        run linters on source code\
	\n  report      generate test and coverage reports\
	\n  test        run tests\
	"
