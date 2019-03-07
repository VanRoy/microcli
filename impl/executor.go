package impl

import (
	"flag"
	"fmt"
	"github.com/urfave/cli"
	"os"
	"strings"
)

type Executor struct {
	context *cli.Context
}

func NexExecutor(context *cli.Context) *Executor {
	return &Executor{
		context: context,
	}
}

// executor executes command and print the output.
func (e *Executor) Execute(s string) {

	s = strings.TrimSpace(s)
	if s == "" {
		return
	} else if s == "quit" || s == "exit" {
		fmt.Println("Bye!")
		os.Exit(0)
		return
	}

	args := strings.Split(s, " ")
	cmd := args[0]

	cli.HandleAction(e.context.App.Command(cmd).Action, e.makeContext(args[1:]))
}

func (e *Executor) makeContext(args []string) *cli.Context {

	flagSet := flag.NewFlagSet("cmd", flag.ContinueOnError)
	flagSet.Parse(args)

	return cli.NewContext(e.context.App, flagSet, e.context)
}
