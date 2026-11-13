# Packaging

EnvDoctor ships as a single static binary per platform via GoReleaser.

- No Docker image for end users: the tool inspects the *host* environment, so running it in a
  container would report the container.
- Release matrix at M6: windows/amd64, windows/arm64, linux/amd64, linux/arm64, darwin/amd64,
  darwin/arm64.
