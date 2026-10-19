# EveChat — build, test, release and packaging entrypoints.
SHELL := /bin/sh
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: all build widget dashboard setup fonts go test test-widget lint fmt migrate seed dev docker \
        release-local checksums bundle image-bundle clean fresh

all: build

## build: widget + dashboard + setup wizard + fonts check + static binary
build: widget dashboard setup fonts go
	@node scripts/url-audit.mjs

widget:
	cd web/widget && npm ci --no-audit --no-fund && npm run build
	@node -e "const z=require('zlib'),fs=require('fs');const n=z.gzipSync(fs.readFileSync('web/widget/dist/widget.js')).length;if(n>=16384){console.error('widget too big: '+n);process.exit(1)};console.log('widget gzip bytes: '+n)"
	cp web/widget/dist/widget.js embed/widget/widget.js

dashboard:
	cd web/dashboard && npm ci --no-audit --no-fund && npm run build

setup:
	cd web/setup && npm ci --no-audit --no-fund && npm run build

## fonts: verify the vendored Vazirmatn files
fonts:
	@test -f embed/assets/fonts/vazirmatn-variable.woff2 || (echo "run scripts/fetch-fonts.sh first"; exit 1)
	@node -e "const fs=require('fs');const d='embed/assets/fonts';let t=0;for(const f of fs.readdirSync(d))t+=fs.statSync(d+'/'+f).size;if(t>400000){console.error('fonts too big: '+t);process.exit(1)};console.log('fonts bytes: '+t)"

go:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o evechat ./cmd/evechat
	@node -e "const s=require('fs').statSync('evechat').size;if(s>45*1024*1024){console.error('binary too big: '+s);process.exit(1)};console.log('binary bytes: '+s)"

## dev: Go server + Vite dashboard + widget watcher
dev:
	@echo "starting Go API (:3000), dashboard (:5173) and widget watcher"
	CGO_ENABLED=0 go run ./cmd/evechat --config evechat.yaml serve & \
	(cd web/dashboard && npm run dev) & \
	(cd web/widget && npm run dev) & \
	wait

## fresh: throwaway instance for the setup wizard (PowerShell; never touches ./data)
fresh:
ifeq ($(OS),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/fresh.ps1
else
	pwsh -NoProfile -File scripts/fresh.ps1
endif

test:
	CGO_ENABLED=0 go test ./... -count=1

test-widget:
	cd web/widget && npx vitest run

lint:
	go vet ./...

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './web/*')

migrate:
	CGO_ENABLED=0 go run ./cmd/evechat --config evechat.yaml migrate

seed:
	CGO_ENABLED=0 go run ./cmd/evechat --config evechat.yaml setup --non-interactive --force --print-credentials

doctor:
	CGO_ENABLED=0 go run ./cmd/evechat --config evechat.yaml doctor --verbose

docker:
	docker build --build-arg VERSION=$(VERSION) -t evechat:$(VERSION) .

## release-local: build every release artifact into dist/
release-local:
	@bash scripts/release-local.sh $(VERSION)

checksums:
	@cd dist && sha256sum * > checksums.txt && cat checksums.txt

## bundle: offline tarball with binary, deploy files, fonts and docs
bundle: go fonts
	@bash scripts/offline-bundle.sh $(VERSION)

image-bundle:
	docker pull ghcr.io/aibedini/evechat:$(VERSION) || docker build -t ghcr.io/aibedini/evechat:$(VERSION) .
	docker save ghcr.io/aibedini/evechat:$(VERSION) | gzip > evechat-$(VERSION).image.tar.gz
	@echo "load with: docker load < evechat-$(VERSION).image.tar.gz"

clean:
	rm -f evechat evechat.exe
	rm -rf web/widget/dist web/dashboard/dist web/setup/dist dist stage
