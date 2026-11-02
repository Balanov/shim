package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/Balanov/shim/internal/config"
	"github.com/Balanov/shim/internal/i18n"
	"github.com/Balanov/shim/internal/provider"
)

func (a *App) modelCommand(ctx context.Context, command string, args []string) error {
	if command == "models" {
		args = append([]string{"list"}, args...)
	} else if command == "use" {
		args = append([]string{"use"}, args...)
	} else if command == "discover" {
		args = append([]string{"discover"}, args...)
	}
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list":
		return a.listModels()
	case "discover":
		add := len(args) == 2 && args[1] == "--add"
		if len(args) > 1 && !add {
			return fmt.Errorf("usage: shim model discover [--add]")
		}
		return a.discoverModels(ctx, add)
	case "add":
		return a.addModel(args[1:])
	case "use":
		if len(args) != 2 {
			return fmt.Errorf("usage: shim model use NAME")
		}
		if _, ok := a.Cfg.ModelByName(args[1]); !ok {
			return fmt.Errorf("unknown model %q", args[1])
		}
		a.Cfg.DefaultModel = args[1]
		if err := a.saveConfig(); err != nil {
			return err
		}
		fmt.Fprintln(a.Out, i18n.T(a.Lang, "model_set")+":", args[1])
		return nil
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: shim model remove NAME")
		}
		if !a.Cfg.RemoveModel(args[1]) {
			return fmt.Errorf("unknown model %q", args[1])
		}
		if err := a.saveConfig(); err != nil {
			return err
		}
		fmt.Fprintln(a.Out, "Removed model:", args[1])
		return nil
	case "test":
		name := a.Cfg.DefaultModel
		if len(args) == 2 {
			name = args[1]
		} else if len(args) > 2 {
			return fmt.Errorf("usage: shim model test [NAME]")
		}
		return a.testModel(ctx, name)
	default:
		return fmt.Errorf("unknown model subcommand %q", args[0])
	}
}

func (a *App) listModels() error {
	if len(a.Cfg.Models) == 0 {
		fmt.Fprintln(a.Out, i18n.T(a.Lang, "no_models"))
		return nil
	}
	w := tabwriter.NewWriter(a.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "DEFAULT\tROUTER\tNAME\tPROVIDER\tMODEL")
	for _, m := range a.Cfg.Models {
		defaultMark, routerMark := "", ""
		if m.Name == a.Cfg.DefaultModel {
			defaultMark = "*"
		}
		if m.Name == a.Cfg.RouterModel {
			routerMark = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", defaultMark, routerMark, m.Name, m.Provider, m.Model)
	}
	return w.Flush()
}

func (a *App) discoverModels(ctx context.Context, add bool) error {
	found := provider.DiscoverLocal(ctx)
	if len(found) == 0 {
		fmt.Fprintln(a.Out, i18n.T(a.Lang, "local_none"))
		return nil
	}
	fmt.Fprintln(a.Out, i18n.T(a.Lang, "local_found")+":")
	for i, m := range found {
		fmt.Fprintf(a.Out, "%2d. %-28s %s\n", i+1, m.Name, m.BaseURL)
	}
	if !add {
		fmt.Fprintln(a.Out, "Run 'shim model discover --add' to add all of them.")
		return nil
	}
	for _, m := range found {
		preset, _ := config.Preset(m.Provider)
		preset.Name, preset.Model = m.Name, m.Model
		a.Cfg.UpsertModel(preset)
		if a.Cfg.DefaultModel == "" {
			a.Cfg.DefaultModel = m.Name
		}
	}
	if err := a.saveConfig(); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "Added %d local model(s). Default: %s\n", len(found), a.Cfg.DefaultModel)
	return nil
}

func (a *App) addModel(args []string) error {
	pos, opts, bools, err := parseOptions(args, map[string]bool{"name": true, "base-url": true, "api": true, "key-env": true}, map[string]bool{"use": true})
	if err != nil {
		return err
	}
	if len(pos) != 2 {
		return fmt.Errorf("usage: shim model add PRESET MODEL_ID [--name NAME] [--base-url URL] [--api API] [--key-env ENV] [--use]")
	}
	var model config.Model
	if pos[0] == "custom" {
		model = config.Model{Name: "custom", Provider: "custom", API: "openai"}
	} else {
		model, err = config.Preset(pos[0])
		if err != nil {
			return err
		}
	}
	model.Model = pos[1]
	if opts["name"] != "" {
		model.Name = opts["name"]
	}
	if opts["base-url"] != "" {
		model.BaseURL = opts["base-url"]
	}
	if opts["api"] != "" {
		model.API = opts["api"]
	}
	if opts["key-env"] != "" {
		model.APIKeyEnv = opts["key-env"]
	}
	if model.BaseURL == "" {
		return fmt.Errorf("custom models need --base-url")
	}
	a.Cfg.UpsertModel(model)
	if a.Cfg.DefaultModel == "" || bools["use"] {
		a.Cfg.DefaultModel = model.Name
	}
	if err := a.saveConfig(); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "Added %s (%s). Default: %s\n", model.Name, model.Model, a.Cfg.DefaultModel)
	if model.APIKeyEnv != "" && os.Getenv(model.APIKeyEnv) == "" {
		fmt.Fprintf(a.Out, "Set %s before use; Shim does not store API keys.\n", model.APIKeyEnv)
	}
	return nil
}

func (a *App) testModel(ctx context.Context, name string) error {
	model, ok := a.Cfg.ModelByName(name)
	if !ok {
		return fmt.Errorf("unknown model %q", name)
	}
	client, err := provider.New(model)
	if err != nil {
		return err
	}
	response, err := client.Complete(ctx, provider.Request{Model: model.Model, MaxTokens: 32, Messages: []provider.Message{{Role: "user", Content: "Reply with exactly: ok"}}})
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "%s: ok; response=%q tokens=%d+%d\n", name, response.Text, response.Usage.InputTokens, response.Usage.OutputTokens)
	return nil
}

func (a *App) routerCommand(args []string) error {
	if len(args) == 0 || args[0] == "show" {
		if a.Cfg.RouterModel == "" {
			fmt.Fprintln(a.Out, "Natural-language command router: off")
		} else {
			fmt.Fprintln(a.Out, "Natural-language command router:", a.Cfg.RouterModel)
		}
		return nil
	}
	if len(args) == 1 && args[0] == "off" {
		a.Cfg.RouterModel = ""
	} else if len(args) == 2 && args[0] == "use" {
		model, ok := a.Cfg.ModelByName(args[1])
		if !ok {
			return fmt.Errorf("unknown model %q", args[1])
		}
		a.Cfg.RouterModel = args[1]
		if !strings.Contains(model.BaseURL, "127.0.0.1") && !strings.Contains(model.BaseURL, "localhost") {
			fmt.Fprintln(a.Out, "Warning: this is not a loopback endpoint; non-command chat text may be sent to it for routing.")
		}
	} else {
		return fmt.Errorf("usage: shim router use NAME | shim router off")
	}
	if err := a.saveConfig(); err != nil {
		return err
	}
	fmt.Fprintln(a.Out, i18n.T(a.Lang, "router_set")+":", a.Cfg.RouterModel)
	return nil
}

func parseOptions(args []string, values, booleans map[string]bool) ([]string, map[string]string, map[string]bool, error) {
	var pos []string
	opts := map[string]string{}
	bools := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			pos = append(pos, arg)
			continue
		}
		name := strings.TrimPrefix(arg, "--")
		if key, value, ok := strings.Cut(name, "="); ok && values[key] {
			opts[key] = value
			continue
		}
		if booleans[name] {
			bools[name] = true
			continue
		}
		if values[name] {
			if i+1 >= len(args) {
				return nil, nil, nil, fmt.Errorf("--%s needs a value", name)
			}
			i++
			opts[name] = args[i]
			continue
		}
		return nil, nil, nil, fmt.Errorf("unknown option --%s", name)
	}
	return pos, opts, bools, nil
}
