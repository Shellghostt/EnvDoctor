# Packaging

EnvDoctor ships as a single static binary per platform via GoReleaser.

- No Docker image for end users: the tool inspects the *host* environment, so running it in a
  container would report the container.
- Target matrix: windows/amd64, windows/arm64, linux/amd64, linux/arm64, darwin/amd64,
  darwin/arm64.

## Configuration

[`.goreleaser.yaml`](../.goreleaser.yaml) is the single source of truth. It builds all six targets
with `CGO_ENABLED=0`, names archives `<project>_<version>_<os>_<arch>` (zip on Windows, tar.gz
elsewhere), writes `checksums.txt` with SHA-256, and drafts releases rather than publishing them.

Verified locally (M1, 2026-11-13):

```bash
goreleaser build --snapshot --clean
```

This produced all six binaries; the Windows build reported `envdoctor.exe` at roughly 3.7 MB.

`goreleaser check` additionally validates the release configuration against the git remote. Before
publication it failed for that reason alone; the repository now has a remote and `goreleaser check`
passes.

## Release workflow

Not implemented yet (M6). The intended shape:

1. A `vX.Y.Z` tag pushed by a maintainer triggers `.github/workflows/release.yml`.
2. CI re-runs `gofmt`, `go vet`, `go test -race ./...` and `go build ./...` before releasing.
3. GoReleaser drafts the release; a human reviews the notes and publishes.
4. Checksums are published alongside the archives. Signing is M6 and will be documented here only
   once it exists.

Do not publish a binary whose `version` was not injected through `-X main.version`; the default is
`dev`.
