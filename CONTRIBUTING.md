# Contributing

Forks and focused pull requests are welcome.

## Principles

- Keep the runtime dependency-free and the core readable.
- Do not add a TUI, daemon, telemetry, account requirement, or mandatory cloud service.
- New providers and transports need offline protocol tests.
- New consequential tools must pass through the permission executor.
- Preserve PowerShell, macOS, and Linux support.
- Document limitations honestly; do not claim unmeasured speed, benchmark, or compatibility wins.

## Workflow

1. Open an issue for a substantial behavior or schema change.
2. Create a small branch from `main`.
3. Run `go fmt ./...`, `go test ./...`, and `go vet ./...`.
4. Include a test that fails without your fix and update user-facing docs when needed.
5. Keep commits scoped; do not commit binaries, local config, credentials, or `.tools/`.

By contributing, you agree that your contribution is licensed under the project's MIT License.
