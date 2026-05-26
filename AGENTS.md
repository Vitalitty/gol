# AGENTS.md

## Project Shape

- This is the maintained `github.com/Vitalitty/gol` fork.
- The CLI entrypoint is `frontend/main.go`.
- The embeddable Go package is `gol.go` with implementation code in `pkg/`.
- The Astro frontend lives in `frontend/`; `frontend/dist/` is tracked because it is embedded into the Go binary.
- Docker runtime examples live in `docker/`; keep local runtime data out of Git.

## Build Workflow

- Expected local toolchain: Go 1.26.3 and Node.js 24 LTS.
- Regenerate frontend assets before Go embed builds:
  - `cd frontend`
  - `npm ci`
  - `npm run check`
  - `npm run build`
- Build the Go CLI from the repository root:
  - `go build -ldflags "-s -w -X main.version=dev" -o gol ./frontend`
- On Windows PowerShell, prefer `npm.cmd` over `npm` if script execution policy blocks `npm.ps1`.
- On this Windows checkout, use repo-local Go caches if the default cache paths are not writable:
  - `$env:GOCACHE=(Join-Path (Get-Location) '.gocache')`
  - `$env:GOPATH=(Join-Path (Get-Location) '.gopath')`

## Validation

- Run `go mod tidy` after dependency or module changes and keep `go.mod` / `go.sum` clean.
- Run `go build -buildvcs=false ./...`, `golangci-lint run ./...`, and `go test -race -v ./... -count=1`.
- For releases, run `goreleaser check` and a snapshot release before tagging:
  - `GOTOOLCHAIN=auto goreleaser release --snapshot --clean`
- For container changes, validate the multi-stage image when Docker is available:
  - `docker build -f docker/Dockerfile -t vitalitty/gol:local .`
- `npm audit --omit=dev` should be clean. A full `npm audit` can report a dev-only transitive advisory through `@astrojs/check`; keep the direct package current until upstream resolves it.

## Editing Notes

- Do not hand-edit `frontend/dist/index.html`; update Astro source files and rebuild.
- Keep release asset names compatible with `install.sh`: `gol-<os>-<arch>` plus ARM variants such as `gol-linux-armv6` and `gol-linux-armv7`.
- Preserve upstream credit in README while using `Vitalitty/gol` for current docs and imports.
