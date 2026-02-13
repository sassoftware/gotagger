# commands
GO          = go
GOBUILD     = $(GO) build
GOBIN       = $(CURDIR)/build/tools

# variables
BUILDDATE := $(shell date +%Y-%m-%d)
COMMIT    := $(shell git rev-parse HEAD)
GOOS      := $(shell $(GO) env GOOS)
VERSION   := $(shell $(GO) run ./cmd/gotagger)
$(if $(VERSION),,$(error failed to determine version))

# directories
REPORTDIR = build/reports

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
ifeq ($(DRY_RUN),false)
STENTORFLAGS = -release
else
STENTORFLAGS =
endif

TARGET = build/$(GOOS)/gotagger

# recipes
.PHONY: all
all: lint build test

include .bingo/Variables.mk

.PHONY: build
build:
	$(GOBUILD) $(BUILDFLAGS) -o $(TARGET) ./cmd/gotagger/main.go


.PHONY: changelog
changelog: | $(STENTOR)
	$(STENTOR) $(STENTORFLAGS) $(VERSION) "$$(git tag --list --sort=version:refname | tail -n1)"

.PHONY: clean
clean:
	$(RM) $(TARGET)
	$(RM) -r $(REPORTDIR)/ dist/

.PHONY: distclean
distclean: clean
	$(RM) -r build/

.PHONY: format
format: LINTFLAGS += --fix
format: lint

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run $(LINTFLAGS)

.PHONY: report
report: TESTFLAGS := $(REPORTFLAGS) $(TESTFLAGS)
report: test | $(GOCOVER_COBERTURA)
	$(GOCOVER_COBERTURA) <$(COVEROUT) >$(COVERXML)

.PHONY: test tests
test tests: | $(GOTESTSUM) $(REPORTDIR)
	$(GOTESTSUM) $(TESTFLAGS) ./...

.PHONY: version
version:
	@echo $(VERSION)

$(REPORTDIR):
	@mkdir -p $@

.PHONY: help
help:
	@printf "Available targets:\
	\n  all         lint, build, and test code\
	\n  build       builds gotagger exectuable\
	\n  changelog   run stentor to show changelog entry\
	\n  clean       removes generated files\
	\n  distclean   reset's workspace to original state\
	\n  format      format source code\
	\n  lint        run GOLANGCI_LINTs on source code\
	\n  report      generate test and coverage reports\
	\n  test        run tests\
	"
