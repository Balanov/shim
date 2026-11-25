## 0.1.2 — 2026-11-25

### Fixed

- The MCP HTTP client decodes a complete SSE event instead of every `data:` line. A valid multi-line event, including one that carries the initialize reply, is accepted; the request-id filtering and the transport limits are unchanged.

## 0.1.1 - 2026-11-15

### Fixed

- A resumed session sends the system prompt again. It is built fresh on every run and the stored messages follow it, so `--resume` no longer runs without the workspace path, AGENTS.md and the skills list. The session file stays append-only.
# Changelog

## 0.1.0 — 2026-11-02

Initial public release.

- Static, dependency-free Go binary for Windows, macOS, and Linux.
- Interactive REPL, guided setup, and scriptable one-shot agent runs.
- OpenAI-compatible and Anthropic adapters with hosted/local presets.
- Ollama, LM Studio, and llama.cpp discovery.
- English/Russian deterministic management phrases and optional local-model command router.
- MCP stdio and Streamable HTTP clients.
- Lazy Agent Skills discovery and Git/local installation.
- Workspace file and shell tools with ask/read-only/auto permissions.
- JSON configuration and JSONL sessions.
- Offline unit/integration tests and MIT-licensed source.
