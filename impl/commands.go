package impl

import (
	"fmt"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/git"
	"os"
	"sort"
)

type CliCommands map[string]cli.Command

var Commands CliCommands

func InitCommands(config config.Config) []cli.Command {

	gitCommands := git.NewCommands(config)

	labels := gitCommands.GetLabels()

	Commands = funk.ToMap([]cli.Command{
		{Name: "init", Usage: "init workspace in current folder", Action: git.Init},
		{Name: "list", Usage: "list projects on workspace", Action: gitCommands.ListLocal},

		{Name: "glist", Usage: "list all remote " + labels.RepositoriesLabel + " from " + labels.GroupsLabel, Action: gitCommands.ListRemote},
		{Name: "gclone", Usage: "clone all remote " + labels.RepositoriesLabel + " from " + labels.GroupsLabel, ArgsUsage: "[glob]", Action: gitCommands.Clone},
		{Name: "gup", Usage: "git pull + rebase all local " + labels.RepositoriesLabel, ArgsUsage: "[glob]", Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "stash, s",
				Usage: "Enable autostash before pull",
			},
		}, Action: gitCommands.Up},
		{Name: "gst", Usage: "show git status for all local " + labels.RepositoriesLabel, ArgsUsage: "[glob]", Action: gitCommands.St},
		{Name: "ggadd", Usage: "create new " + labels.GroupLabel, ArgsUsage: labels.CreateGroupUsage, Action: gitCommands.AddGroup},
		{Name: "gadd", Usage: "create new " + labels.RepositoryLabel, ArgsUsage: labels.CreateRepositoryUsage, Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "init, i",
				Usage: "Init repository with Initializr",
			},
			cli.StringFlag{
				Name:  "type, t",
				Usage: "Project initializr type",
			},
			cli.StringFlag{
				Name:  "name, n",
				Usage: "Project initializr name",
			},
			cli.StringFlag{
				Name:  "dependencies, d",
				Usage: "Project dependencies ( separated with comma )",
			},
		}, Action: gitCommands.Add},
		{Name: "ginit", Usage: "initialize " + labels.RepositoryLabel + " with Initializr", ArgsUsage: "repo type name dependencies", Action: gitCommands.Init},

		{Name: "exec", Usage: "execute script / action on " + labels.RepositoryLabel + "", ArgsUsage: "glob action", Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "interactive, i",
				Usage: "Wait manual approval after each steps",
			},
			cli.StringFlag{
				Name:  "branch, b",
				Usage: "Name of branch to create for execution",
			},
			cli.StringFlag{
				Name:  "commitMessage, cm",
				Usage: "Message of commit",
			},
			cli.BoolFlag{
				Name:  "review, r",
				Usage: "Enable creation of " + labels.CodeReviewRequest,
			},
			cli.StringFlag{
				Name:  "reviewTitle, rt",
				Usage: "Title of review use on " + labels.CodeReviewRequest,
			},
			cli.StringFlag{
				Name:  "reviewMessage, rm",
				Usage: "Message of review use on " + labels.CodeReviewRequest,
			},
		}, Action: gitCommands.Exec},

		{Name: "exit", Usage: "exit the prompt", Action: exitCommand},
		{Name: "clear", Usage: "clear the screen", Action: clearCommand},
	}, "Name").(map[string]cli.Command)

	return Commands.GetCliCmdArray()
}

func (c *CliCommands) GetCliCmdArray() []cli.Command {

	commands := funk.Values(c).([]cli.Command)

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	return commands
}

func exitCommand(c *cli.Context) error {
	os.Exit(0)
	return nil
}

func clearCommand(c *cli.Context) error {
	fmt.Print("\033[2J")
	fmt.Printf("\033[%d;%dH", 1, 1)
	return nil
}
