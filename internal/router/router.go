package router

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/Balanov/shim/internal/provider"
)

type Command struct {
	Text     string `json:"command"`
	Mutating bool   `json:"mutating"`
}

var usePattern = regexp.MustCompile(`(?i)^(?:use|switch to|используй|переключись(?: на)?| utilize| usa| utilise)\s+(?:model\s+|модель\s+|modelo\s+)?([\w./:@-]+)$`)

func Deterministic(input string) (Command, bool) {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)
	readOnly := map[string]string{
		"help": "/help", "помощь": "/help", "что ты умеешь": "/help", "что ты умеешь?": "/help",
		"status": "/status", "статус": "/status",
		"list models": "/models", "show models": "/models", "models": "/models",
		"список моделей": "/models", "покажи модели": "/models", "какие модели": "/models", "какие модели?": "/models",
		"list skills": "/skills", "show skills": "/skills", "список навыков": "/skills", "покажи навыки": "/skills",
		"list tools": "/tools", "show tools": "/tools", "список инструментов": "/tools", "покажи инструменты": "/tools",
		"list mcp": "/mcps", "show mcp": "/mcps", "список mcp": "/mcps", "покажи mcp": "/mcps",
		"doctor": "/doctor", "диагностика": "/doctor", "проверь настройку": "/doctor", "проверь настройки": "/doctor",
		"list sessions": "/session list", "список сессий": "/session list",
	}
	if command, ok := readOnly[lower]; ok {
		return Command{Text: command}, true
	}
	mutating := map[string]string{
		"find local models": "/discover --add", "scan local models": "/discover --add", "discover local models": "/discover --add",
		"connect ollama": "/discover --add", "connect local model": "/discover --add",
		"найди локальные модели": "/discover --add", "подключи локальную модель": "/discover --add", "подключи ollama": "/discover --add",
		"new session": "/session new", "clear session": "/session new", "новая сессия": "/session new", "очисти сессию": "/session new",
	}
	if command, ok := mutating[lower]; ok {
		return Command{Text: command, Mutating: true}, true
	}
	if match := usePattern.FindStringSubmatch(trimmed); len(match) == 2 {
		return Command{Text: "/model use " + match[1], Mutating: true}, true
	}
	return Command{}, false
}

const compilerPrompt = `You compile natural-language harness management requests into one safe CLI command.
Return ONLY JSON: {"command":"/models","mutating":false}. If the user is asking for ordinary chat, coding, research, or an action in the workspace rather than Shim configuration, return {"command":"","mutating":false}.
Allowed commands:
/help
/status
/doctor
/models
/model use NAME
/model add PRESET MODEL_ID [--name NAME]
/discover [--add]
/model test [NAME]
/skills
/skill show NAME
/skill add SOURCE
/mcps
/mcp list
/mcp tools NAME
/mcp add NAME COMMAND [ARGS...]
/tools
/session list
/session new
/router use NAME
/router off
/permissions ask|read-only|auto
Use PRESET only from: openai, anthropic, openrouter, gemini, github, huggingface, ollama, lmstudio, llamacpp, custom.
Never invent /shell, file-editing, package-install, or configuration commands outside this list. Preserve user-provided model IDs and sources. Quote arguments containing spaces. mutating=true for every command that adds, removes, changes, enables, or connects something.`

func Compile(ctx context.Context, client provider.Client, model, input string) (Command, error) {
	response, err := client.Complete(ctx, provider.Request{Model: model, MaxTokens: 300, Messages: []provider.Message{
		{Role: "system", Content: compilerPrompt},
		{Role: "user", Content: input},
	}})
	if err != nil {
		return Command{}, err
	}
	text := strings.TrimSpace(response.Text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	var command Command
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &command); err != nil {
		return Command{}, fmt.Errorf("router returned invalid JSON: %w", err)
	}
	if command.Text == "" {
		return Command{}, nil
	}
	mutating, err := Validate(command.Text)
	if err != nil {
		return Command{}, err
	}
	command.Mutating = mutating
	return command, nil
}

func Validate(command string) (bool, error) {
	args, err := Split(command)
	if err != nil {
		return false, err
	}
	if len(args) == 0 || !strings.HasPrefix(args[0], "/") {
		return false, fmt.Errorf("router command must start with /")
	}
	readOnly := map[string]bool{
		"/help": true, "/status": true, "/doctor": true, "/models": true, "/skills": true,
		"/mcps": true, "/tools": true, "/discover": true,
	}
	if readOnly[args[0]] {
		if args[0] == "/discover" && len(args) == 2 && args[1] == "--add" {
			return true, nil
		}
		if len(args) != 1 {
			return false, fmt.Errorf("unexpected arguments for %s", args[0])
		}
		return false, nil
	}
	switch args[0] {
	case "/model":
		if len(args) >= 2 {
			switch args[1] {
			case "test":
				return false, nil
			case "use", "add", "remove":
				return true, nil
			}
		}
	case "/skill":
		if len(args) == 3 && args[1] == "show" {
			return false, nil
		}
		if len(args) >= 2 && (args[1] == "add" || args[1] == "remove") {
			return true, nil
		}
	case "/mcp":
		if len(args) == 2 && args[1] == "list" {
			return false, nil
		}
		if len(args) == 3 && args[1] == "tools" {
			return false, nil
		}
		if len(args) >= 2 && (args[1] == "add" || args[1] == "remove") {
			return true, nil
		}
	case "/session":
		if len(args) == 2 && args[1] == "list" {
			return false, nil
		}
		if len(args) == 2 && args[1] == "new" {
			return true, nil
		}
	case "/router":
		if len(args) == 2 && (args[1] == "off" || args[1] == "show") {
			return args[1] == "off", nil
		}
		if len(args) == 3 && args[1] == "use" {
			return true, nil
		}
	case "/permissions":
		if len(args) == 2 {
			return true, nil
		}
	}
	return false, fmt.Errorf("router produced a disallowed command: %s", command)
}

func Split(input string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped, started := false, false
	flush := func() {
		if started {
			args = append(args, current.String())
			current.Reset()
			started = false
		}
	}
	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped, started = false, true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped, started = true, true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			started = true
			continue
		}
		if r == '\'' || r == '"' || r == '`' {
			quote, started = r, true
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		current.WriteRune(r)
		started = true
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	flush()
	return args, nil
}
