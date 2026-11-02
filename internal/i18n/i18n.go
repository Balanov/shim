package i18n

import "strings"

type Language string

const (
	English Language = "en"
	Russian Language = "ru"
)

func Normalize(s string) Language {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "ru") {
		return Russian
	}
	return English
}

func T(lang, key string) string {
	if Normalize(lang) == Russian {
		if s, ok := russian[key]; ok {
			return s
		}
	}
	if s, ok := english[key]; ok {
		return s
	}
	return key
}

var english = map[string]string{
	"welcome":     "shim — any model. Any tool. Just type.",
	"no_model":    "No model is configured. Run 'shim setup' or type /discover.",
	"prompt":      "you › ",
	"assistant":   "shim › ",
	"exiting":     "Bye.",
	"unknown":     "Unknown command. Type /help.",
	"model_set":   "Default model",
	"router_set":  "Local command router",
	"no_models":   "No models configured.",
	"no_skills":   "No skills found.",
	"no_mcp":      "No MCP servers configured.",
	"confirm":     "Run this command? [y/N] ",
	"cancelled":   "Cancelled.",
	"denied":      "Denied by permission policy.",
	"tool_allow":  "Allow this action? [y/N] ",
	"setup_done":  "Setup complete. Type a request, or /help for commands.",
	"local_found": "Local models found",
	"local_none":  "No running local model servers found.",
}

var russian = map[string]string{
	"welcome":     "shim — любые модели и инструменты. Просто напишите.",
	"no_model":    "Модель не настроена. Выполните 'shim setup' или введите /discover.",
	"prompt":      "вы › ",
	"assistant":   "shim › ",
	"exiting":     "Пока.",
	"unknown":     "Неизвестная команда. Введите /help.",
	"model_set":   "Модель по умолчанию",
	"router_set":  "Локальный роутер команд",
	"no_models":   "Модели не настроены.",
	"no_skills":   "Навыки не найдены.",
	"no_mcp":      "MCP-серверы не настроены.",
	"confirm":     "Выполнить эту команду? [y/N] ",
	"cancelled":   "Отменено.",
	"denied":      "Запрещено политикой разрешений.",
	"tool_allow":  "Разрешить это действие? [y/N] ",
	"setup_done":  "Настройка завершена. Напишите запрос или /help для списка команд.",
	"local_found": "Найдены локальные модели",
	"local_none":  "Запущенные локальные серверы моделей не найдены.",
}
