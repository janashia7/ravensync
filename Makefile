.PHONY: build install clean test lint release-snapshot docker

VERSION ?= 0.1.0-dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Keep in sync with .github/workflows/ci.yml and .pre-commit-config.yaml
GOLANGCI_LINT_VERSION ?= v2.11.4
LDFLAGS  = -ldflags "-s -w -X github.com/ravensync/ravensync/internal/cli.Version=$(VERSION) -X github.com/ravensync/ravensync/internal/cli.Commit=$(COMMIT)"

build:
	go build $(LDFLAGS) -o bin/ravensync ./cmd/ravensync

install:
	go install $(LDFLAGS) ./cmd/ravensync

clean:
	rm -rf bin/ dist/

test:
	go test ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

docker:
	docker build -t ravensync:$(VERSION) .

release-snapshot: clean
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/ravensync_linux_amd64   ./cmd/ravensync
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o dist/ravensync_linux_arm64   ./cmd/ravensync
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/ravensync_darwin_amd64  ./cmd/ravensync
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/ravensync_darwin_arm64  ./cmd/ravensync
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/ravensync_windows_amd64.exe ./cmd/ravensync
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/ravensync_windows_arm64.exe ./cmd/ravensync
	@cd dist && for f in ravensync_linux_* ravensync_darwin_*; do mv "$$f" ravensync && tar czf "$$f.tar.gz" ravensync && rm ravensync; done
	@cd dist && for f in ravensync_windows_*.exe; do mv "$$f" ravensync.exe && zip "$${f%.exe}.zip" ravensync.exe && rm ravensync.exe; done
	@echo "\nRelease artifacts in dist/:"
	@ls -lh dist/
