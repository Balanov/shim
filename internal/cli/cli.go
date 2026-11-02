package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/Balanov/shim/internal/config"
	"github.com/Balanov/shim/internal/i18n"
	"github.com/Balanov/shim/internal/mcp"
	"github.com/Balanov/shim/internal/session"
)

const Version = "0.1.0"

type App struct {
	In        io.Reader
	Out       io.Writer
	Err       io.Writer
	Reader    *bufio.Reader
	Cfg       config.Config
	Workspace string
	Lang      string
	Store     *session.Store
	MCP       *mcp.Manager
	Verbose   bool
}

func Run(args []string, in io.Reader, out, errOut io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	app := &App{In: in, Out: out, Err: errOut, Reader: bufio.NewReader(in)}
	var configPath string
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				fmt.Fprintln(errOut, "--config needs a path")
				return 2
			}
			i++
			configPath = args[i]
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintln(errOut, "--workspace needs a path")
				return 2
			}
			i++
			app.Workspace = args[i]
		case "--lang":
			if i+1 >= len(args) {
				fmt.Fprintln(errOut, "--lang needs en or ru")
				return 2
			}
			i++
			app.Lang = args[i]
		case "--version", "-v":
			fmt.Fprintln(out, "shim "+Version)
			return 0
		case "--help", "-h":
			printHelp(out)
			return 0
		default:
			positional = append(positional, args[i:]...)
			i = len(args)
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(errOut, "shim:", err)
		return 1
	}
	app.Cfg = cfg
	if app.Lang == "" {
		app.Lang = cfg.Language
	}
	if app.Lang == "" {
		app.Lang = "en"
	}
	if app.Workspace == "" {
		app.Workspace, err = os.Getwd()
		if err != nil {
			fmt.Fprintln(errOut, "shim:", err)
			return 1
		}
	}
	app.Store = session.New(filepath.Join(cfg.Dir(), "sessions"))
	app.MCP = mcp.NewManager(cfg.MCPServers)
	defer app.MCP.Close()
	if len(positional) == 0 {
		positional = []string{"chat"}
	}
	if err := app.dispatch(ctx, positional); err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(errOut, "\nshim: interrupted")
			return 130
		}
		fmt.Fprintln(errOut, "shim:", err)
		return 1
	}
	return 0
}

func (a *App) dispatch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return a.chat(ctx)
	}
	command := args[0]
	if strings.HasPrefix(command, "/") {
		command = command[1:]
	}
	switch command {
	case "help":
		printHelp(a.Out)
		return nil
	case "version":
		fmt.Fprintln(a.Out, "shim "+Version)
		return nil
	case "chat":
		return a.chat(ctx)
	case "run":
		return a.run(ctx, args[1:])
	case "setup":
		return a.setup(ctx)
	case "model", "models", "use", "discover":
		return a.modelCommand(ctx, command, args[1:])
	case "router":
		return a.routerCommand(args[1:])
	case "skill", "skills":
		return a.skillCommand(ctx, command, args[1:])
	case "mcp", "mcps":
		return a.mcpCommand(ctx, command, args[1:])
	case "tools":
		return a.toolsCommand(ctx)
	case "status":
		return a.status()
	case "doctor":
		return a.doctor(ctx)
	case "session", "clear":
		return a.sessionCommand(command, args[1:])
	case "config":
		return a.configCommand(args[1:])
	case "permissions":
		return a.permissionsCommand(args[1:])
	case "completion":
		return a.completion(args[1:])
	case "exit", "quit":
		return nil
	default:
		return fmt.Errorf("unknown command %q; run 'shim help'", args[0])
	}
}

func (a *App) reloadMCP() {
	a.MCP.Close()
	a.MCP = mcp.NewManager(a.Cfg.MCPServers)
}

func (a *App) saveConfig() error {
	a.Cfg.Language = string(i18n.Normalize(a.Lang))
	return a.Cfg.Save()
}
