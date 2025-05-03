package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/thoas/go-funk"
	"github.com/vanroy/microcli/impl/config"
	"golang.org/x/term"
)

// Color defines a custom color object which is defined by SGR parameters.
type color struct {
	params []Attribute
}

type Option struct {
	Id   string
	Name string
}

// Attribute defines a single SGR Code
type Attribute int

const escape = "\x1b"

// Base attributes
const (
	Reset Attribute = iota
	Bold
	Faint
	Italic
	Underline
	BlinkSlow
	BlinkRapid
	ReverseVideo
	Concealed
	CrossedOut
)

// Foreground text colors
const (
	FgBlack Attribute = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite
)

// Foreground Hi-Intensity text colors
const (
	FgHiBlack Attribute = iota + 90
	FgHiRed
	FgHiGreen
	FgHiYellow
	FgHiBlue
	FgHiMagenta
	FgHiCyan
	FgHiWhite
)

// Background text colors
const (
	BgBlack Attribute = iota + 40
	BgRed
	BgGreen
	BgYellow
	BgBlue
	BgMagenta
	BgCyan
	BgWhite
)

// Background Hi-Intensity text colors
const (
	BgHiBlack Attribute = iota + 100
	BgHiRed
	BgHiGreen
	BgHiYellow
	BgHiBlue
	BgHiMagenta
	BgHiCyan
	BgHiWhite
)

func PrintItem(text string) {
	if config.Options.Verbose {
		fmt.Printf("* %s \n", text)
	}
}

func PrintInfo(format string, a ...any) {
	if config.Options.Verbose {
		fmt.Printf("%s==> %s%s%s\n", Color(Bold, FgBlue), Color(FgWhite), fmt.Sprintf(format, a...), Color(Reset))
	}
}

func PrintWarn(format string, a ...any) {
	if config.Options.Verbose {
		fmt.Printf("%sWARNING: %s%s%s\n", Color(Bold, FgYellow), Color(FgWhite), fmt.Sprintf(format, a...), Color(Reset))
	}
}

func PrintError(message string) {
	if config.Options.Verbose {
		fmt.Printf("%sERROR: %s%s%s\n", Color(Bold, FgRed), Color(FgWhite), message, Color(Reset))
	}
}

func PrintErrorf(format string, a ...any) {
	if config.Options.Verbose {
		fmt.Printf("%sERROR: %s%s%s\n", Color(Bold, FgRed), Color(FgWhite), fmt.Sprintf(format, a...), Color(Reset))
	}
}

func Color(value ...Attribute) string {
	c := &color{params: value}
	return fmt.Sprintf("%s[%sm", escape, c.sequence())
}

func PrintReset() {
	fmt.Printf("%s[%dm", escape, Reset)
}

func PrintNewLine() {
	if config.Options.Verbose {
		fmt.Println()
	}
}

func Input(prompt string, defaultValue ...string) string {

	if len(defaultValue) == 1 && defaultValue[0] != "" {
		return defaultValue[0]
	}

	if !config.Options.Interactive {
		return ""
	}

	if prompt != "" {
		fmt.Print(prompt + " ")
	}

	text, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.Trim(text, "\n")
}

func RestrictedInput(prompt string, acceptedValues []string) string {
	for {
		input := Input(prompt + " ( " + strings.Join(acceptedValues, " / ") + " ) :")
		if funk.ContainsString(acceptedValues, input) {
			return input
		}
	}
}

func Choice(prompt string, options []Option, defaultValue ...string) string {

	if len(defaultValue) == 1 && defaultValue[0] != "" {
		return defaultValue[0]
	}

	if !config.Options.Interactive {
		return ""
	}

	if prompt != "" {
		fmt.Println(prompt)
	}

	for i := range options {
		fmt.Printf("%d) %s\n", i+1, options[i].Name)
	}

	for {
		str := Input("#? ")
		if str == "" {
			return ""
		}

		result, _ := strconv.Atoi(str)
		if result > 0 && result <= len(options) {
			return options[result-1].Id
		}
	}
}

func Password(prompt string) string {

	if !config.Options.Interactive {
		return ""
	}

	if prompt != "" {
		fmt.Print(prompt + " ")
	}

	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err == nil {
		return string(pass)
	}

	return ""
}

// sequence returns a formatted SGR sequence to be plugged into a "\x1b[...m"
// an example output might be: "1;36" -> bold cyan
func (c *color) sequence() string {
	format := make([]string, len(c.params))
	for i, v := range c.params {
		format[i] = strconv.Itoa(int(v))
	}

	return strings.Join(format, ";")
}
