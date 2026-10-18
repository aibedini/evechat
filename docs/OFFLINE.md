# Offline / air-gapped installation

EveChat has **no runtime network dependency**: the binary embeds the dashboard,
widget, setup wizard, fonts, OpenAPI description and all assets. The only
optional outbound calls are the integrations you enable (Telegram, EVE, GMweb,
SMTP, Anthropic).

## Build the bundle (on a connected machine)

```bash
make bundle VERSION=v1.1.0
# -> dist/evechat-offline-v1.1.0-linux-amd64.tar.gz
```

Contents: static `linux/amd64` binary, `checksums.txt`, `install-offline.sh`,
`evechat.example.yaml`, systemd/Caddy/nginx examples, Vazirmatn fonts, LICENSE,
NOTICE and the Persian README.

Cross-arch: `GOARCH=arm64 make bundle` (adjust the Makefile target or run
`scripts/offline-bundle.sh` with `GOARCH` exported).

## Install

```bash
tar xzf evechat-offline-v1.1.0-linux-amd64.tar.gz
cd evechat-offline-v1.1.0-linux-amd64
sudo ./install-offline.sh --port 3000 --public-url http://10.0.0.5:3000
```

The script creates the service user, generates secrets, writes the config,
installs the systemd unit (unless `--no-systemd`), runs migrations and the
first-run wizard, then prints credentials. It performs **zero** network calls.

## Docker without a registry

```bash
make image-bundle VERSION=v1.1.0     # docker save | gzip
scp evechat-v1.1.0.image.tar.gz host:
ssh host 'docker load < evechat-v1.1.0.image.tar.gz'
```

## Mirroring an update channel

Copy each release's `*.tar.gz`, `*.zip` and `checksums.txt` to
`https://mirror.internal/evechat/<tag>/` and pass
`--base-url https://mirror.internal/evechat` to `install.sh` (or set
`EVECHAT_UPDATE_BASE_URL` for `evechat update`).

## Verifying the bundle

```bash
sha256sum -c checksums.txt
./evechat version
./evechat doctor --verbose
```

`scripts/offline-audit.sh` (CI job `offline-bundle`) installs the bundle on a
clean runner with networking disabled to prove nothing is fetched at runtime.
