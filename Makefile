# EveChat — build, test and packaging entrypoints.
SHELL := /bin/sh
VERSION ?= dev
GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all dev build widget dashboard go test test-widget lint migrate seed docker clean

all: build

## build: widget + dashboard + single static binary
build: widget dashboard go

widget:
	cd web/widget && npm ci && npm run build
	@node -e "const z=require('zlib'),fs=require('fs');const n=z.gzipSync(fs.readFileSync('web/widget/dist/widget.js')).length;if(n>=12288){console.error('widget too big: '+n);process.exit(1)};console.log('widget gzip bytes: '+n)"
	cp web/widget/dist/widget.js embed/widget/widget.js

dashboard:
	cd web/dashboard && npm ci && npm run build

go:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o evechat ./cmd/evechat

## dev: Go server + Vite dev server + widget rollup watch
dev:
	@echo "starting Go API (:3000), dashboard (:5173) and widget watcher"
	CGO_ENABLED=0 go run ./cmd/evechat --config evechat.yaml & \
	(cd web/dashboard && npm run dev) & \
	(cd web/widget && npm run dev) & \
	wait

test:
	CGO_ENABLED=0 go test ./... -count=1

test-widget:
	cd web/widget && npx vitest run

lint:
	go vet ./...

migrate:
	CGO_ENABLED=0 go run ./cmd/evechat --config evechat.yaml --migrate-only

seed:
	CGO_ENABLED=0 go run ./cmd/evechat --config evechat.yaml --seed

docker:
	docker build -t evechat:$(VERSION) .

clean:
	rm -f evechat
	rm -rf web/widget/dist web/dashboard/node_modules web/widget/node_modules
