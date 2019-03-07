package impl

import (
	"github.com/c-bata/go-prompt"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli"
)

// completer returns the completion items from user input.
func Completer(d prompt.Document) []prompt.Suggest {

	s := funk.Map(Commands, func(k string, v cli.Command) prompt.Suggest {
		return prompt.Suggest{Text: v.Name, Description: v.Usage}
	}).([]prompt.Suggest)

	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}
