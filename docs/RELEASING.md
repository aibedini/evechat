# Releasing EveChat

## Versioning

SemVer tags: `vMAJOR.MINOR.PATCH` (e.g. `v1.1.0`). CI resolves the version from
the tag and injects it into the binary (`main.version`, `main.commit`,
`main.buildDate`).

## Checklist

```bash
# 0. clean tree, green tests
git status
make test && make test-widget

# 1. update the changelog: move Unreleased items under the new version
$EDITOR CHANGELOG.md

# 2. rebuild everything locally (enforces size budgets)
make build
evechat doctor --verbose

# 3. (optional) build the release matrix without publishing
make release-local VERSION=v1.1.0

# 4. tag and push
git add -A && git commit -m "chore(release): v1.1.0"
git tag -a v1.1.0 -m "v1.1.0"
git push origin main --tags
```

Pushing the tag triggers `.github/workflows/release.yml`, which:

1. builds widget (with the 16 KB gzip check), dashboard and setup wizard;
2. cross-compiles linux/amd64, linux/arm64, linux/armv7, darwin/amd64,
   darwin/arm64 and windows/amd64;
3. packages `evechat_<version>_<os>_<arch>.tar.gz|zip` with deploy files;
4. writes `checksums.txt` and extracts release notes from `CHANGELOG.md`;
5. publishes a GitHub Release and pushes a multi-arch image to
   `ghcr.io/aibedini/evechat` (`<version>`, `<major.minor>`, `latest`).

## Image visibility

For a public `docker pull` without authentication, set the package visibility to
public once (GitHub → Packages → evechat → Package settings → Change visibility).
Private packages require `docker login ghcr.io`.

## Version pinning contract

`install.sh` resolves `releases/latest` unless `--version` is given. Users who
need reproducible installs should always pass `--version` and verify
`checksums.txt`; the installer does the checksum check automatically.

## Post-release verification

```bash
curl -fsSL https://github.com/aibedini/evechat/releases/latest/download/checksums.txt
docker run --rm ghcr.io/aibedini/evechat:v1.1.0 version
# on a clean VM:
curl -fsSL .../install.sh | bash -s -- --version v1.1.0 --no-systemd
```

## Hotfix

Branch from the tag, add the smallest fix plus a regression test, run the
checklist with `vX.Y.Z+1`, and note it in the changelog under a patch heading.
