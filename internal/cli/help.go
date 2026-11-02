package cli

import "io"

func printHelp(w io.Writer) {
	io.WriteString(w, `shim — any model. Any tool. Just type.

USAGE
  shim                         Interactive chat + commands
  shim run [flags] <prompt>    One-shot, scriptable agent run
  shim setup                   Guided terminal setup

MODELS
  shim model list                         List configured models
  shim model discover [--add]             Find Ollama/LM Studio/llama.cpp
  shim model add <preset> <id> [flags]    Add a model
  shim model use <name>                   Set default model
  shim model test [name]                  Send a tiny test request
  shim model remove <name>                Remove a model
  shim router use <name>|off              Set natural-language command router

  Presets: ollama, lmstudio, llamacpp, openai, anthropic,
           openrouter, gemini, github, huggingface, custom
  Add flags: --name ALIAS --base-url URL --api openai|anthropic
             --key-env ENV_VAR --use

SKILLS & MCP
  shim skill list|show <name>
  shim skill add <github-owner/repo|url|path>
  shim skill remove <name>
  shim mcp list
  shim mcp add <name> <command|url> [args...]
  shim mcp tools <name>
  shim mcp call <name> <tool> [json]
  shim mcp remove <name>
  shim tools                             List tools visible to the agent

SESSIONS & SAFETY
  shim session list|new
  shim run --resume <session-id> <prompt>
  shim permissions ask|read-only|auto
  shim status | doctor
  shim config path|show

RUN FLAGS (before the prompt)
  --model NAME       Override the default model
  --resume ID        Resume a local JSONL session
  --max-steps N      Agent loop limit (default 16)
  --yes              Allow writes, shell, and MCP without prompts
  --read-only        Disable writes, shell, and MCP
  --verbose          Print tool names

INSIDE CHAT
  /help /models /use NAME /discover [--add]
  /skills /mcps /tools /status /doctor
  /session new|list /permissions MODE /exit

Ordinary text goes to the agent. With 'router use LOCAL_MODEL', ordinary
language can also manage Shim. Proposed commands are allowlisted; changes
require confirmation. No API keys are stored in config.json.

GLOBAL FLAGS
  --config PATH --workspace PATH --lang en|ru --version
`)
}
