.PHONY: build install clean test release-snapshot docker

VERSION ?= 0.1.0-dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -ldflags "-s -w -X github.com/ravensync/ravensync/internal/cli.Version=$(VERSION) -X github.com/ravensync/ravensync/internal/cli.Commit=$(COMMIT)"

build:
	go build $(LDFLAGS) -o bin/ravensync ./cmd/ravensync

install:
	go install $(LDFLAGS) ./cmd/ravensync

clean:
	rm -rf bin/ dist/

test:
	go test ./...

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
	@cd dist && for f in ravensync_linux_* ravensync_darwin_*; do tar czf "$$f.tar.gz" "$$f" && rm "$$f"; done
	@cd dist && for f in ravensync_windows_*.exe; do zip "$${f%.exe}.zip" "$$f" && rm "$$f"; done
	@echo "\nRelease artifacts in dist/:"
	@ls -lh dist/
