package main

import (
	"os"

	"github.com/c-bata/go-prompt"
	"github.com/urfave/cli"

	microcli "github.com/vanroy/microcli/impl"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/git"
	pmt "github.com/vanroy/microcli/impl/prompt"
	"os/signal"
	"syscall"
)

func main() {

	handleSignals()

	conf, err := loadConf()
	if err != nil {
		pmt.PrintError("Command load config '%s'", err.Error())
		os.Exit(1)
	}

	app := cli.NewApp()
	app.Name = "Microbox"
	app.Usage = "This script provides utilities to manage microservices git repositories."
	app.Version = "1.0.0"

	app.Before = initContext
	app.Action = displayPrompt
	app.CommandNotFound = commandNotFound
	app.Commands = microcli.InitCommands(*conf)

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Disable verbose output",
		},
		cli.BoolFlag{
			Name:  "non-interactive, n",
			Usage: "Non interactive mode",
		},
	}

	app.Run(os.Args)
}

// Load or init configuration
func loadConf() (*config.Config, error) {

	if exist, err := config.Exist(); exist == false && err == nil {
		microcli.ShowBanner()
		pmt.PrintWarn("Settings not exist, starting initialization\n")
		git.Init(nil)
		pmt.PrintInfo("Settings initialized, enjoy !")
		os.Exit(0)
	}

	return config.Load()
}

// Init context before commands
func initContext(c *cli.Context) error {

	config.Options = config.GlobalOptions{
		Verbose:     !c.Bool("quiet"),
		Interactive: !c.Bool("non-interactive"),
	}

	microcli.ShowBanner()

	return nil
}

// Display interactive prompt
func displayPrompt(c *cli.Context) error {

	p := prompt.New(
		microcli.NexExecutor(c).Execute,
		microcli.Completer,
		prompt.OptionTitle("Microbox CLI"),
		prompt.OptionPrefix("=> "),
		prompt.OptionPrefixTextColor(prompt.DefaultColor),
		prompt.OptionSuggestionBGColor(prompt.LightGray),
		prompt.OptionSuggestionTextColor(prompt.White),
		prompt.OptionDescriptionBGColor(prompt.DarkGray),
		prompt.OptionDescriptionTextColor(prompt.White),
		prompt.OptionPreviewSuggestionBGColor(prompt.DarkGray),
		prompt.OptionPreviewSuggestionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSelectedSuggestionTextColor(prompt.White),
		prompt.OptionSelectedDescriptionBGColor(prompt.LightGray),
		prompt.OptionSelectedDescriptionTextColor(prompt.White),
		prompt.OptionAddKeyBind(prompt.KeyBind{Key: prompt.ControlC, Fn: func(buffer *prompt.Buffer) {
			os.Exit(1)
		}}),
	)

	p.Run()

	return nil
}

// Display command not found
func commandNotFound(c *cli.Context, cmd string) {
	pmt.PrintError("Command not exit '%s'", cmd)
	os.Exit(1)
}

func handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-c
		os.Exit(0)
	}()
}
