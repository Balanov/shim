package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/Balanov/shim/internal/i18n"
	"github.com/Balanov/shim/internal/skills"
)

func (a *App) skillManager() skills.Manager {
	return skills.Manager{GlobalDir: configSkillDir(a), Workspace: a.Workspace}
}

func (a *App) skillCommand(ctx context.Context, command string, args []string) error {
	if command == "skills" {
		args = append([]string{"list"}, args...)
	}
	if len(args) == 0 {
		args = []string{"list"}
	}
	manager := a.skillManager()
	switch args[0] {
	case "list":
		list, err := manager.List()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Fprintln(a.Out, i18n.T(a.Lang, "no_skills"))
			return nil
		}
		for _, skill := range list {
			fmt.Fprintf(a.Out, "%-24s %s\n", skill.Name, skill.Description)
		}
		return nil
	case "show":
		if len(args) != 2 {
			return fmt.Errorf("usage: shim skill show NAME")
		}
		content, err := manager.Load(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintln(a.Out, content)
		return nil
	case "add":
		if len(args) != 2 {
			return fmt.Errorf("usage: shim skill add SOURCE")
		}
		installed, err := manager.Install(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Fprintln(a.Out, "Installed skills:", strings.Join(installed, ", "))
		return nil
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: shim skill remove NAME")
		}
		if err := manager.Remove(args[1]); err != nil {
			return err
		}
		fmt.Fprintln(a.Out, "Removed skill:", args[1])
		return nil
	default:
		return fmt.Errorf("unknown skill subcommand %q", args[0])
	}
}
