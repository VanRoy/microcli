package impl

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

type Executor struct {
	command *cli.Command
}

func NexExecutor(command *cli.Command) *Executor {
	return &Executor{
		command: command,
	}
}

// executor executes command and print the output.
func (e *Executor) Execute(s string) {

	s = strings.TrimSpace(s)
	switch s {
	case "":
		return
	case "quit", "exit":
		fmt.Println("Bye!")
		os.Exit(0)
		return
	}

	args := strings.Split(s, " ")
	cmd := args[0]

	command := e.command.Command(cmd)
	if command != nil {
		command.Action(nil, e.command) //nolint:errcheck
		return
	}

	e.command.CommandNotFound(nil, e.command, cmd)
}
