package main

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"

	"os/signal"
	"syscall"

	microcli "github.com/vanroy/microcli/impl"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/git"
	pmt "github.com/vanroy/microcli/impl/prompt"
)

func main() {

	handleSignals()

	conf, err := loadConf(context.Background())
	if err != nil {
		pmt.PrintErrorf("Command load config '%s'", err.Error())
		os.Exit(1)
	}

	app := &cli.Command{}
	app.Name = "mbx"
	app.Usage = "This script provides utilities to manage microservices git repositories."
	app.Version = "1.1.0"

	app.Before = initContext
	app.CommandNotFound = commandNotFound
	app.Commands = microcli.InitCommands(*conf)

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Disable verbose output",
		},
		&cli.BoolFlag{
			Name:    "non-interactive",
			Aliases: []string{"n"},
			Usage:   "Non interactive mode",
		},
	}

	app.Run(context.Background(), os.Args) //nolint:errcheck
}

// Load or init configuration
func loadConf(context context.Context) (*config.Config, error) {

	if exist, err := config.Exist(); !exist && err == nil {
		microcli.ShowBanner()
		pmt.PrintWarn("Settings not exist, starting initialization\n")
		git.Init(context, nil) //nolint:errcheck
		pmt.PrintInfo("Settings initialized, enjoy !")
		os.Exit(0)
	}

	return config.Load()
}

// Init context before commands
func initContext(ctx context.Context, c *cli.Command) (context.Context, error) {

	config.Options = config.GlobalOptions{
		Verbose:     !c.Bool("quiet"),
		Interactive: !c.Bool("non-interactive"),
	}

	microcli.ShowBanner()

	return ctx, nil
}

// Display command not found
func commandNotFound(ctx context.Context, c *cli.Command, cmd string) {
	pmt.PrintErrorf("Command not exist '%s'", cmd)
}

func handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-c
		os.Exit(0)
	}()
}
