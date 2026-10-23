VERSION ?= $(shell (git describe --tags --always --dirty 2>/dev/null || echo dev) | sed 's/^v//')
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

VERSION_PKG := github.com/EdrenaVosh/squadup/internal/version
LDFLAGS := -s -w -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).Date=$(DATE)

SANDBOX := $(CURDIR)/.sandbox
SANDBOX_ENV := CLAUDE_CONFIG_DIR=$(SANDBOX)/claude SQUADUP_HOME=$(SANDBOX)/squadup

.PHONY: build test lint sandbox sandbox-presets sandbox-reset try try-claude snapshot install clean

build:
	go build -ldflags '$(LDFLAGS)' -o bin/squad ./cmd/squad

lint:
	golangci-lint run ./...

test: lint
	go vet ./...
	go test -race ./...

sandbox:
	./scripts/sandbox.sh "$(SANDBOX)"

# Add demo presets (solo-indie, bug-hunt, extra-helper) to the sandbox.
sandbox-presets: sandbox
	./scripts/sandbox-presets.sh "$(SANDBOX)"

sandbox-reset:
	rm -rf "$(SANDBOX)"
	./scripts/sandbox.sh "$(SANDBOX)"

# Run squad against the sandbox only: make try ARGS="use solo-indie"
try: build sandbox
	$(SANDBOX_ENV) ./bin/squad $(ARGS)

# Open the real claude on the sandbox config. Login is stored per config
# directory, so the first run asks you to sign in again.
try-claude: sandbox
	$(SANDBOX_ENV) claude

snapshot:
	goreleaser release --snapshot --clean

# Installs into ~/.local/bin for testing on your real config. The first run
# captures Initial; `squad restore` brings everything back.
install: build
	mkdir -p $(HOME)/.local/bin
	cp bin/squad $(HOME)/.local/bin/squad

clean:
	rm -rf bin dist
