package cli

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Balanov/shim/internal/agent"
	"github.com/Balanov/shim/internal/provider"
	"github.com/Balanov/shim/internal/skills"
	"github.com/Balanov/shim/internal/tools"
)

func (a *App) run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	model := fs.String("model", "", "model name")
	resume := fs.String("resume", "", "session id")
	maxSteps := fs.Int("max-steps", 16, "agent step limit")
	yes := fs.Bool("yes", false, "allow all tools")
	readOnly := fs.Bool("read-only", false, "read only")
	verbose := fs.Bool("verbose", false, "print tool names")
	if err := fs.Parse(args); err != nil {
		return err
	}
	prompt := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if prompt == "" {
		return fmt.Errorf("run needs a prompt")
	}
	if *yes && *readOnly {
		return fmt.Errorf("--yes and --read-only are mutually exclusive")
	}
	mode := a.Cfg.Permissions
	if *yes {
		mode = "auto"
	}
	if *readOnly {
		mode = "read-only"
	}
	a.Verbose = *verbose
	_, err := a.runPrompt(ctx, prompt, *model, *resume, *maxSteps, mode)
	return err
}

func (a *App) runPrompt(ctx context.Context, prompt, modelName, resume string, maxSteps int, mode string) (string, error) {
	if modelName == "" {
		modelName = a.Cfg.DefaultModel
	}
	if modelName == "" {
		return "", fmt.Errorf("no default model; run 'shim setup' or 'shim model add'")
	}
	model, ok := a.Cfg.ModelByName(modelName)
	if !ok {
		return "", fmt.Errorf("unknown model %q", modelName)
	}
	client, err := provider.New(model)
	if err != nil {
		return "", err
	}
	registry := tools.NewRegistry()
	files, err := tools.NewFileTools(a.Workspace)
	if err != nil {
		return "", err
	}
	files.Register(registry)
	tools.ShellTool{Root: a.Workspace}.Register(registry)
	skillManager := skills.Manager{GlobalDir: configSkillDir(a), Workspace: a.Workspace}
	skillManager.Register(registry)
	if mode != "read-only" {
		for _, err := range a.MCP.RegisterAll(ctx, registry) {
			fmt.Fprintln(a.Err, "mcp:", err)
		}
	}
	messages := []provider.Message{{Role: "system", Content: agent.SystemPrompt(a.Workspace, a.Lang, skillManager)}}
	if resume != "" {
		messages, err = a.Store.Resume(resume)
		if err != nil {
			return "", err
		}
		if len(messages) > 0 && messages[0].Role == "system" {
			messages[0].Content = agent.SystemPrompt(a.Workspace, a.Lang, skillManager)
		}
	}
	executor := &tools.Executor{Registry: registry, Mode: mode, Language: a.Lang, Input: a.Reader, Output: a.Out}
	runner := &agent.Agent{
		Client: client, Model: model.Model, Messages: messages, Registry: registry,
		Executor: executor, Store: a.Store, MaxSteps: maxSteps, Output: a.Out, Verbose: a.Verbose,
	}
	return runner.Run(ctx, prompt)
}

func configSkillDir(a *App) string {
	return filepath.Join(a.Cfg.Dir(), "skills")
}
