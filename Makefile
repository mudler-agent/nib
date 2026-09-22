CURRENT_DIR := $(shell pwd)
MODEL?=gpt-4o-mini
API_KEY?=
BASE_URL?=https://api.openai.com/v1
LOG_LEVEL?=debug
GORELEASER?=
GO?=go

# check if goreleaser exists
ifeq (, $(shell which goreleaser))
	GORELEASER=curl -sfL https://goreleaser.com/static/run | bash -s --
else
	GORELEASER=$(shell which goreleaser)
endif

build: sync-readme
	go build -o nib .

# Sync the embedded README copy. selfdoc/README.md is a plain copy of the root
# README.md, compiled into the binary via go:embed so the agent can read its own
# documentation at runtime. Run before build, or via `go generate`.
.PHONY: sync-readme
sync-readme:
	cp README.md selfdoc/README.md

generate: sync-readme

run-docker:
	docker build -t nib .
	docker run -it -e LOG_LEVEL=$(LOG_LEVEL) -e MODEL=$(MODEL) -e API_KEY=$(API_KEY) -e BASE_URL=$(BASE_URL) --rm nib

install:
	bash install.sh --from-source

dev-dist:
	$(GORELEASER) build --snapshot --clean

dist:
	$(GORELEASER) build --clean

test:
	$(GO) test ./...

# Integration tests hit the live network (e.g. the web_search DuckDuckGo scrape)
# and are excluded from the default `test` target. They guard against upstream
# markup drift, so they run nightly in CI rather than on every change.
test-integration:
	$(GO) test -tags integration ./...