package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Balanov/shim/internal/config"
	"github.com/Balanov/shim/internal/i18n"
	"github.com/Balanov/shim/internal/provider"
)

func (a *App) setup(ctx context.Context) error {
	fmt.Fprintln(a.Out, "Shim setup — no files to edit, no UI, no telemetry.")
	language, err := a.ask(fmt.Sprintf("Language / Язык [%s]: ", a.Lang))
	if err != nil {
		return err
	}
	if language != "" {
		a.Lang = string(i18n.Normalize(language))
	}
	found := provider.DiscoverLocal(ctx)
	if len(found) > 0 {
		fmt.Fprintln(a.Out, "\n"+i18n.T(a.Lang, "local_found")+":")
		for i, model := range found {
			fmt.Fprintf(a.Out, "%2d. %s (%s)\n", i+1, model.Name, model.Provider)
		}
		choice, err := a.ask("Select a local model number (Enter to skip): ")
		if err != nil {
			return err
		}
		if choice != "" {
			index, err := strconv.Atoi(choice)
			if err != nil || index < 1 || index > len(found) {
				return fmt.Errorf("invalid selection %q", choice)
			}
			selected := found[index-1]
			model, _ := config.Preset(selected.Provider)
			model.Name, model.Model = selected.Name, selected.Model
			a.Cfg.UpsertModel(model)
			a.Cfg.DefaultModel = model.Name
			useRouter, err := a.ask("Use this local model for natural-language commands? [Y/n] ")
			if err != nil {
				return err
			}
			if !strings.EqualFold(strings.TrimSpace(useRouter), "n") && !strings.EqualFold(strings.TrimSpace(useRouter), "no") {
				a.Cfg.RouterModel = model.Name
			}
		}
	} else {
		fmt.Fprintln(a.Out, "\n"+i18n.T(a.Lang, "local_none"))
	}
	if a.Cfg.DefaultModel == "" {
		fmt.Fprintln(a.Out, "\nHosted presets: openai, anthropic, openrouter, gemini, github, huggingface, custom")
		presetName, err := a.ask("Provider to add (Enter to skip): ")
		if err != nil {
			return err
		}
		if presetName != "" {
			var model config.Model
			if presetName == "custom" {
				model = config.Model{Name: "custom", Provider: "custom", API: "openai"}
			} else {
				model, err = config.Preset(presetName)
				if err != nil {
					return err
				}
			}
			model.Model, err = a.ask("Model ID: ")
			if err != nil {
				return err
			}
			if model.Model == "" {
				return fmt.Errorf("model ID cannot be empty")
			}
			if presetName == "custom" {
				model.BaseURL, err = a.ask("OpenAI-compatible base URL: ")
				if err != nil || model.BaseURL == "" {
					return fmt.Errorf("base URL is required")
				}
				model.APIKeyEnv, err = a.ask("API key environment variable (optional): ")
				if err != nil {
					return err
				}
			}
			a.Cfg.UpsertModel(model)
			a.Cfg.DefaultModel = model.Name
			if model.APIKeyEnv != "" && os.Getenv(model.APIKeyEnv) == "" {
				fmt.Fprintf(a.Out, "Before use, set %s in your terminal. Shim never writes keys to config.\n", model.APIKeyEnv)
			}
		}
	}
	if err := a.saveConfig(); err != nil {
		return err
	}
	fmt.Fprintln(a.Out, "\n"+i18n.T(a.Lang, "setup_done"))
	fmt.Fprintln(a.Out, "Config:", a.Cfg.Path())
	return nil
}

func (a *App) ask(prompt string) (string, error) {
	fmt.Fprint(a.Out, prompt)
	answer, err := a.Reader.ReadString('\n')
	if err != nil && answer == "" {
		return "", err
	}
	return strings.TrimSpace(answer), nil
}
