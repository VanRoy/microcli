package initialzr

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/vanroy/microcli/impl/cmd"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/prompt"
)

type Initializr struct {
	config config.Config
}

func NewInitializr(config config.Config) *Initializr {
	return &Initializr{
		config: config,
	}
}

func (init *Initializr) Init(projectType string, projectName string, dependencies []string, destFolder string) error {

	// Download archive
	data := map[string]string{
		"type":         projectType,
		"name":         projectName,
		"dependencies": strings.Join(dependencies, ","),
	}

	archive := fmt.Sprintf("/tmp/spring-%s.tgz", projectName)

	url := "https://start.spring.io"
	if init.config.Initializr.Url != "" {
		url = init.config.Initializr.Url
	}

	resp, err := resty.New().R().
		SetFormData(data).
		SetOutput(archive).
		Post(fmt.Sprintf("%s/starter.tgz", url))

	if err != nil {
		prompt.PrintError(resp.String())
		return err
	}

	// Remove archive
	defer os.Remove(archive) //nolint:errcheck

	// Extract archive
	_, archiveErr := cmd.ExecCmdDir("tar", []string{"xvzf", archive}, destFolder)
	if archiveErr != nil {
		prompt.PrintError(archiveErr.Error())
		return archiveErr
	}

	return nil
}
