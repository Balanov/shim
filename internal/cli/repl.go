package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Balanov/shim/internal/i18n"
	"github.com/Balanov/shim/internal/provider"
	"github.com/Balanov/shim/internal/router"
)

func (a *App) chat(ctx context.Context) error {
	fmt.Fprintln(a.Out, i18n.T(a.Lang, "welcome"))
	fmt.Fprintln(a.Out, "Type /help for commands. Ordinary text talks to the agent.")
	if a.Cfg.DefaultModel == "" && a.In == os.Stdin {
		if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			if err := a.setup(ctx); err != nil {
				return err
			}
		}
	}
	if a.Cfg.DefaultModel == "" {
		fmt.Fprintln(a.Out, i18n.T(a.Lang, "no_model"))
	}
	for {
		var routerClient provider.Client
		var routerModel string
		if a.Cfg.RouterModel != "" {
			if m, ok := a.Cfg.ModelByName(a.Cfg.RouterModel); ok {
				client, err := provider.New(m)
				if err != nil {
					fmt.Fprintln(a.Err, "router:", err)
				} else {
					routerClient, routerModel = client, m.Model
				}
			}
		}
		fmt.Fprint(a.Out, "\n"+i18n.T(a.Lang, "prompt"))
		line, err := a.Reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if errors.Is(err, io.EOF) {
				fmt.Fprintln(a.Out, i18n.T(a.Lang, "exiting"))
				return nil
			}
			continue
		}
		if line == "/exit" || line == "/quit" {
			fmt.Fprintln(a.Out, i18n.T(a.Lang, "exiting"))
			return nil
		}
		if strings.HasPrefix(line, "/") {
			if err := a.dispatchSlash(ctx, line); err != nil {
				fmt.Fprintln(a.Err, "shim:", err)
			}
		} else {
			command, handled := router.Deterministic(line)
			if !handled && routerClient != nil {
				compiled, err := router.Compile(ctx, routerClient, routerModel, line)
				if err != nil {
					fmt.Fprintln(a.Err, "router:", err)
				} else if compiled.Text != "" {
					command, handled = compiled, true
				}
			}
			if handled {
				fmt.Fprintln(a.Out, "↳", command.Text)
				if command.Mutating && !a.confirm(i18n.T(a.Lang, "confirm")) {
					fmt.Fprintln(a.Out, i18n.T(a.Lang, "cancelled"))
					continue
				}
				if err := a.dispatchSlash(ctx, command.Text); err != nil {
					fmt.Fprintln(a.Err, "shim:", err)
				}
			} else {
				if a.Cfg.DefaultModel == "" {
					fmt.Fprintln(a.Out, i18n.T(a.Lang, "no_model"))
				} else if _, err := a.runPrompt(ctx, line, "", "", 16, a.Cfg.Permissions); err != nil {
					fmt.Fprintln(a.Err, "shim:", err)
				}
			}
		}
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(a.Out, i18n.T(a.Lang, "exiting"))
			return nil
		}
	}
}

func (a *App) dispatchSlash(ctx context.Context, line string) error {
	args, err := router.Split(strings.TrimPrefix(line, "/"))
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return nil
	}
	aliases := map[string]string{
		"models": "model list", "use": "model use", "discover": "model discover",
		"skills": "skill list", "mcps": "mcp list", "clear": "session new",
	}
	if replacement, ok := aliases[args[0]]; ok {
		base, _ := router.Split(replacement)
		args = append(base, args[1:]...)
	}
	return a.dispatch(ctx, args)
}

func (a *App) confirm(prompt string) bool {
	fmt.Fprint(a.Out, prompt)
	answer, err := a.Reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes", "д", "да":
		return true
	default:
		return false
	}
}
