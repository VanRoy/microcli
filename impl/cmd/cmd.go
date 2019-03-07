package cmd

import (
	"os"
	"os/exec"
	"strings"
)

func ExecCmd(cmdName string, cmdArgs []string) (string, error) {
	return ExecCmdDir(cmdName, cmdArgs, "")
}

func ExecCmdDir(cmdName string, cmdArgs []string, dir string) (string, error) {

	var (
		cmdOut []byte
		err    error
	)

	cmd := exec.Command(cmdName, cmdArgs...)
	if dir != "" {
		cmd.Dir = dir
	}
	if cmdOut, err = cmd.CombinedOutput(); err != nil {
		return string(cmdOut), err
	}
	return strings.TrimSuffix(string(cmdOut), "\n"), nil
}

func ExecAndOutCmd(cmdName string, cmdArgs []string) error {
	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func ErrorString(err string) string {
	return strings.Replace(strings.TrimSuffix(err, "\n"), "\n", " ", -1)
}
