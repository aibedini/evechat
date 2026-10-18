# syntax=docker/dockerfile:1

# ---- stage 1: front-ends ----
FROM node:26-alpine AS web
WORKDIR /src
COPY web/widget/package*.json web/widget/
RUN cd web/widget && npm ci --no-audit --no-fund
COPY web/widget web/widget
RUN cd web/widget && npm run build

COPY web/dashboard/package*.json web/dashboard/
RUN cd web/dashboard && npm ci --no-audit --no-fund
COPY web/dashboard web/dashboard
RUN cd web/dashboard && npm run build

# ---- stage 2: static Go binary ----
FROM golang:1.23-alpine AS build
RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/embed/dashboard ./embed/dashboard
COPY --from=web /src/web/widget/dist/widget.js ./embed/widget/widget.js
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION:-docker}" -o /out/evechat ./cmd/evechat

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
