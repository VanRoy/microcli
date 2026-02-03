package impl

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/c-bata/go-prompt"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli/v3"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/git"
)

type CliCommands map[string]cli.Command

var Commands CliCommands

func InitCommands(config config.Config) []*cli.Command {

	gitCommands := git.NewCommands(config)

	labels := gitCommands.GetLabels()

	Commands = funk.ToMap([]cli.Command{
		{Name: "init", Usage: "init workspace in current folder", Action: git.Init},
		{Name: "authenticate", Usage: "refresh authentication token", Action: git.Auth},
		{Name: "list", Usage: "list projects on workspace", Action: gitCommands.ListLocal},

		{Name: "glist", Usage: "list all remote " + labels.RepositoriesLabel + " from " + labels.GroupsLabel, Action: gitCommands.ListRemote},
		{Name: "gclone", Usage: "clone all remote " + labels.RepositoriesLabel + " from " + labels.GroupsLabel, ArgsUsage: "[glob]", Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "exclude",
				Aliases: []string{"e"},
				Usage:   "Pattern to exclude",
			},
		}, Action: gitCommands.Clone},
		{Name: "gup", Usage: "git pull + rebase all local " + labels.RepositoriesLabel, ArgsUsage: "[glob]", Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "stash",
				Aliases: []string{"s"},
				Usage:   "Enable autostash before pull",
			},
		}, Action: gitCommands.Up},
		{Name: "gst", Usage: "show git status for all local " + labels.RepositoriesLabel, ArgsUsage: "[glob]", Action: gitCommands.St},
		{Name: "ggadd", Usage: "create new " + labels.GroupLabel, ArgsUsage: labels.CreateGroupUsage, Action: gitCommands.AddGroup},
		{Name: "gadd", Usage: "create new " + labels.RepositoryLabel, ArgsUsage: labels.CreateRepositoryUsage, Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "init",
				Aliases: []string{"i"},
				Usage:   "Init repository with Initializr",
			},
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "Project initializr type",
			},
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Project initializr name",
			},
			&cli.StringFlag{
				Name:    "dependencies",
				Aliases: []string{"d"},
				Usage:   "Project dependencies ( separated with comma )",
			},
		}, Action: gitCommands.Add},
		{Name: "ginit", Usage: "initialize " + labels.RepositoryLabel + " with Initializr", ArgsUsage: "repo type name dependencies", Action: gitCommands.Init},

		{Name: "exec", Usage: "execute script / action on " + labels.RepositoryLabel + "", ArgsUsage: "[glob] [action]", Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "interactive",
				Aliases: []string{"i"},
				Usage:   "Wait manual approval after each steps",
			},
			&cli.StringFlag{
				Name:    "branch",
				Aliases: []string{"b"},
				Usage:   "Name of branch to create for execution",
			},
			&cli.StringFlag{
				Name:    "commit-message",
				Aliases: []string{"cm"},
				Usage:   "Message of commit",
			},
			&cli.BoolFlag{
				Name:    "review",
				Aliases: []string{"r"},
				Usage:   "Enable creation of " + labels.CodeReviewRequest,
			},
			&cli.StringFlag{
				Name:    "review-title",
				Aliases: []string{"rt"},
				Usage:   "Title of review use on " + labels.CodeReviewRequest,
			},
			&cli.StringFlag{
				Name:    "review-message",
				Aliases: []string{"rm"},
				Usage:   "Message of review use on " + labels.CodeReviewRequest,
			},
			&cli.BoolFlag{
				Name:    "review-draft",
				Aliases: []string{"rd"},
				Usage:   "Submit " + labels.CodeReviewRequest + " as draft",
			},
		}, Action: gitCommands.Exec},

		{Name: "shell", Usage: "Enter in interactive shell mode", Action: displayPrompt},
		{Name: "exit", Usage: "exit the prompt", Action: exitCommand},
		{Name: "clear", Usage: "clear the screen", Action: clearCommand},
	}, "Name").(map[string]cli.Command)

	return Commands.GetCliCmdArray()
}

func (c *CliCommands) GetCliCmdArray() []*cli.Command {

	commands := funk.Values(c).([]cli.Command)

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	//	var cmds []*cli.Command
	return funk.Map(commands, func(c cli.Command) *cli.Command { return &c }).([]*cli.Command)
}

func exitCommand(_ context.Context, c *cli.Command) error {
	os.Exit(0)
	return nil
}

func clearCommand(_ context.Context, c *cli.Command) error {
	fmt.Print("\033[2J")
	fmt.Printf("\033[%d;%dH", 1, 1)
	return nil
}

// Display interactive prompt
func displayPrompt(_ context.Context, c *cli.Command) error {

	p := prompt.New(
		NexExecutor(c.Root()).Execute,
		Completer,
		prompt.OptionTitle("Microbox CLI"),
		prompt.OptionPrefix("=> "),
		prompt.OptionAddKeyBind(prompt.KeyBind{Key: prompt.ControlC, Fn: func(buffer *prompt.Buffer) {
			os.Exit(1)
		}}),
	)

	p.Run()

	return nil
}
