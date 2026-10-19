# syntax=docker/dockerfile:1

# ---- stage 1: front-ends ----
FROM node:22-alpine AS web
WORKDIR /src
COPY web/widget/package*.json web/widget/
RUN cd web/widget && npm ci --no-audit --no-fund
COPY web/widget web/widget
RUN cd web/widget && npm run build

COPY web/dashboard/package*.json web/dashboard/
RUN cd web/dashboard && npm ci --no-audit --no-fund
COPY web/dashboard web/dashboard
RUN cd web/dashboard && npm run build

COPY web/setup/package*.json web/setup/
RUN cd web/setup && npm ci --no-audit --no-fund
COPY web/setup web/setup
RUN cd web/setup && npm run build

# ---- stage 2: static Go binary (cross-compiles via TARGETOS/TARGETARCH) ----
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/embed/dashboard ./embed/dashboard
COPY --from=web /src/embed/setup ./embed/setup
COPY --from=web /src/web/widget/dist/widget.js ./embed/widget/widget.js
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /out/evechat ./cmd/evechat

# ---- stage 3: runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/evechat /evechat
USER nonroot:nonroot
EXPOSE 3000
VOLUME ["/data"]
ENV EVECHAT_DATABASE__PATH=/data/evechat.db \
    EVECHAT_STORAGE__UPLOADS_DIR=/data/uploads \
    EVECHAT_SERVER__HOST=0.0.0.0 \
    EVECHAT_SERVER__PORT=3000
ENTRYPOINT ["/evechat"]
CMD ["--config", "/data/evechat.yaml"]
