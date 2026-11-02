package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Balanov/shim/internal/config"
	"github.com/Balanov/shim/internal/i18n"
	"github.com/Balanov/shim/internal/tools"
)

func (a *App) mcpCommand(ctx context.Context, command string, args []string) error {
	if command == "mcps" {
		args = append([]string{"list"}, args...)
	}
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list":
		if len(a.Cfg.MCPServers) == 0 {
			fmt.Fprintln(a.Out, i18n.T(a.Lang, "no_mcp"))
			return nil
		}
		for name, server := range a.Cfg.MCPServers {
			target := server.Command
			if target != "" {
				target += " " + strings.Join(server.Args, " ")
			} else {
				target = server.URL
			}
			fmt.Fprintf(a.Out, "%-24s %s\n", name, target)
		}
		return nil
	case "add":
		if len(args) < 3 {
			return fmt.Errorf("usage: shim mcp add NAME COMMAND [ARGS...] | shim mcp add NAME URL")
		}
		name := args[1]
		if !validConfigName(name) {
			return fmt.Errorf("MCP name must contain only letters, digits, _ or -")
		}
		server := config.MCPServer{}
		if strings.HasPrefix(args[2], "http://") || strings.HasPrefix(args[2], "https://") {
			if len(args) != 3 {
				return fmt.Errorf("HTTP MCP syntax: shim mcp add NAME URL")
			}
			server.URL = args[2]
		} else {
			server.Command, server.Args = args[2], args[3:]
		}
		if a.Cfg.MCPServers == nil {
			a.Cfg.MCPServers = map[string]config.MCPServer{}
		}
		a.Cfg.MCPServers[name] = server
		if err := a.saveConfig(); err != nil {
			return err
		}
		a.reloadMCP()
		fmt.Fprintln(a.Out, "Added MCP server:", name)
		return nil
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: shim mcp remove NAME")
		}
		if _, ok := a.Cfg.MCPServers[args[1]]; !ok {
			return fmt.Errorf("unknown MCP server %q", args[1])
		}
		delete(a.Cfg.MCPServers, args[1])
		if err := a.saveConfig(); err != nil {
			return err
		}
		a.reloadMCP()
		fmt.Fprintln(a.Out, "Removed MCP server:", args[1])
		return nil
	case "tools":
		if len(args) != 2 {
			return fmt.Errorf("usage: shim mcp tools NAME")
		}
		registry := tools.NewRegistry()
		if err := a.MCP.Register(ctx, args[1], registry); err != nil {
			return err
		}
		for _, spec := range registry.Specs() {
			fmt.Fprintf(a.Out, "%-40s %s\n", spec.Name, spec.Description)
		}
		return nil
	case "call":
		if len(args) < 3 || len(args) > 4 {
			return fmt.Errorf("usage: shim mcp call SERVER TOOL [JSON_ARGS]")
		}
		raw := json.RawMessage(`{}`)
		if len(args) == 4 {
			raw = json.RawMessage(args[3])
			if !json.Valid(raw) {
				return fmt.Errorf("arguments must be valid JSON")
			}
		}
		result, err := a.MCP.Call(ctx, args[1], args[2], raw)
		if err != nil {
			return err
		}
		fmt.Fprintln(a.Out, result)
		return nil
	default:
		return fmt.Errorf("unknown mcp subcommand %q", args[0])
	}
}

func (a *App) toolsCommand(ctx context.Context) error {
	registry := tools.NewRegistry()
	files, err := tools.NewFileTools(a.Workspace)
	if err != nil {
		return err
	}
	files.Register(registry)
	tools.ShellTool{Root: a.Workspace}.Register(registry)
	a.skillManager().Register(registry)
	for _, err := range a.MCP.RegisterAll(ctx, registry) {
		fmt.Fprintln(a.Err, "mcp:", err)
	}
	for _, spec := range registry.Specs() {
		fmt.Fprintf(a.Out, "%-40s %s\n", spec.Name, spec.Description)
	}
	return nil
}

func validConfigName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}
