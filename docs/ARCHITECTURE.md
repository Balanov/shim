# Architecture

Shim is a standard-library-only Go module. The public command is `cmd/shim`; implementation packages live under `internal/` so forks can change the core without inheriting an accidental API compatibility burden.

## Request flow

```text
terminal
  └─ cli: argument parsing, setup, REPL, confirmations
      ├─ router: deterministic phrase → allowlisted slash command
      │          optional local model → JSON command → allowlist validation
      └─ agent: system prompt + messages + tool schemas
             ├─ provider: OpenAI-compatible / Anthropic HTTP adapters
             ├─ tools: workspace files, shell, permission gate
             ├─ skills: metadata in prompt, full SKILL.md on demand
             ├─ mcp: stdio / Streamable HTTP JSON-RPC clients
             └─ session: append-only JSONL transcript
```

## Package boundaries

- `internal/config`: versioned JSON schema, provider presets, atomic save. No credentials.
- `internal/provider`: normalized messages, tools, responses, usage, local discovery.
- `internal/agent`: bounded model → tool → model loop. Providers do not execute tools.
- `internal/tools`: tool registry and the single permission enforcement point.
- `internal/skills`: standard `SKILL.md` discovery, lazy loading, local/Git installation.
- `internal/mcp`: JSON-RPC transports and namespaced remote tools.
- `internal/router`: management-only natural-language compiler. No arbitrary shell generation.
- `internal/session`: crash-friendly append-only message history.
- `internal/cli`: terminal composition root.

## Invariants

1. **No implicit credential persistence.** Model keys come from environment variables.
2. **Every consequential tool crosses `tools.Executor`.** A provider or MCP adapter cannot bypass permission checks.
3. **Workspace file access is confined.** Both lexical and resolved symlink paths are checked.
4. **Router output is data, not authority.** It is parsed, allowlisted, and confirmed when mutating.
5. **No background runtime.** MCP subprocesses live only for the current Shim process and are closed on exit.
6. **No mandatory network.** Commands, config, skills, sessions, and deterministic routing work offline; model calls use only configured endpoints.
7. **Lazy context.** Skill bodies and MCP results enter the transcript only when requested or called.

## Adding a provider

Implement `provider.Client`, add an API kind to config validation and `provider.New`, then add request/response tests using `httptest`. Do not put tool execution or terminal I/O in an adapter.

## Adding a tool

Register `tools.Tool` with a JSON Schema, a permission `Level`, and a bounded handler. File tools must use the existing workspace resolver. Mutating tools must not be marked `Read` to bypass approval.

## Compatibility policy during 0.x

The JSON config has an explicit version. Preserve unknown-provider forward compatibility where practical, never silently drop user config, and add migrations before incrementing `config.CurrentVersion`.
