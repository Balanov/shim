package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Balanov/shim/internal/provider"
	"github.com/Balanov/shim/internal/session"
	"github.com/Balanov/shim/internal/tools"
)

func (a *App) status() error {
	skills, _ := a.skillManager().List()
	sessions, _ := a.Store.List()
	fmt.Fprintln(a.Out, "shim", Version)
	fmt.Fprintln(a.Out, "Config:    ", a.Cfg.Path())
	fmt.Fprintln(a.Out, "Workspace: ", a.Workspace)
	fmt.Fprintln(a.Out, "Language:  ", a.Lang)
	fmt.Fprintln(a.Out, "Model:     ", valueOr(a.Cfg.DefaultModel, "not configured"))
	fmt.Fprintln(a.Out, "Router:    ", valueOr(a.Cfg.RouterModel, "off"))
	fmt.Fprintln(a.Out, "Permissions:", a.Cfg.Permissions)
	fmt.Fprintf(a.Out, "Models: %d  Skills: %d  MCP: %d  Sessions: %d\n", len(a.Cfg.Models), len(skills), len(a.Cfg.MCPServers), len(sessions))
	return nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (a *App) doctor(ctx context.Context) error {
	problems := 0
	check := func(ok bool, name, detail string) {
		mark := "ok"
		if !ok {
			mark = "FAIL"
			problems++
		}
		fmt.Fprintf(a.Out, "[%-4s] %-28s %s\n", mark, name, detail)
	}
	check(a.Cfg.Validate() == nil, "config", a.Cfg.Path())
	_, err := os.Stat(a.Workspace)
	check(err == nil, "workspace", a.Workspace)
	for _, model := range a.Cfg.Models {
		if model.APIKeyEnv != "" && model.Provider != "ollama" && model.Provider != "lmstudio" && model.Provider != "llamacpp" {
			check(os.Getenv(model.APIKeyEnv) != "", "key/"+model.Name, model.APIKeyEnv)
		}
	}
	local := provider.DiscoverLocal(ctx)
	if len(local) == 0 {
		fmt.Fprintln(a.Out, "[info] local model discovery      optional; no local servers found")
	} else {
		check(true, "local model discovery", fmt.Sprintf("%d model(s)", len(local)))
	}
	for name := range a.Cfg.MCPServers {
		registry := tools.NewRegistry()
		err := a.MCP.Register(ctx, name, registry)
		detail := fmt.Sprintf("%d tool(s)", len(registry.Specs()))
		if err != nil {
			detail = err.Error()
		}
		check(err == nil, "mcp/"+name, detail)
	}
	if problems > 0 {
		return fmt.Errorf("doctor found %d problem(s)", problems)
	}
	return nil
}

func (a *App) sessionCommand(command string, args []string) error {
	if command == "clear" {
		args = []string{"new"}
	}
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list":
		list, err := a.Store.List()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(a.Out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tMESSAGES\tUPDATED\tBYTES")
		for _, info := range list {
			fmt.Fprintf(w, "%s\t%d\t%s\t%d\n", info.ID, info.Messages, info.Updated.Local().Format(time.DateTime), info.Bytes)
		}
		return w.Flush()
	case "new":
		a.Store = session.New(a.Store.Dir)
		fmt.Fprintln(a.Out, "Started a new session. It will be saved on the first message.")
		return nil
	default:
		return fmt.Errorf("usage: shim session list|new")
	}
}

func (a *App) configCommand(args []string) error {
	if len(args) != 1 || args[0] != "path" && args[0] != "show" {
		return fmt.Errorf("usage: shim config path|show")
	}
	if args[0] == "path" {
		fmt.Fprintln(a.Out, a.Cfg.Path())
		return nil
	}
	data, err := json.MarshalIndent(a.Cfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(a.Out, string(data))
	return nil
}

func (a *App) permissionsCommand(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(a.Out, a.Cfg.Permissions)
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: shim permissions ask|read-only|auto")
	}
	switch args[0] {
	case "ask", "read-only", "auto":
		a.Cfg.Permissions = args[0]
	default:
		return fmt.Errorf("permissions must be ask, read-only, or auto")
	}
	if err := a.saveConfig(); err != nil {
		return err
	}
	fmt.Fprintln(a.Out, "Permissions:", args[0])
	return nil
}

func (a *App) completion(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: shim completion bash|zsh|fish|powershell")
	}
	commands := "chat run setup model router skill mcp tools status doctor session config permissions completion help version"
	switch args[0] {
	case "bash":
		fmt.Fprintf(a.Out, "complete -W '%s' shim\n", commands)
	case "zsh":
		fmt.Fprintf(a.Out, "#compdef shim\n_arguments '1:command:(%s)'\n", commands)
	case "fish":
		for _, command := range strings.Fields(commands) {
			fmt.Fprintf(a.Out, "complete -c shim -f -a %s\n", command)
		}
	case "powershell":
		fmt.Fprintf(a.Out, "Register-ArgumentCompleter -Native -CommandName shim -ScriptBlock { param($wordToComplete) '%s'.Split(' ') | Where-Object { $_ -like \"$wordToComplete*\" } }\n", commands)
	default:
		return fmt.Errorf("unsupported shell %q", args[0])
	}
	return nil
}
