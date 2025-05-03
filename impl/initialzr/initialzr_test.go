package initialzr

import (
	"testing"

	"github.com/vanroy/microcli/impl/config"
)

func SkipTestDownload(t *testing.T) {

	initializr := NewInitializr(config.Config{})

	err := initializr.Init("gradle-project", "test", []string{}, "/tmp/microcli-init")
	if err != nil {
		t.Error("Error during init")
	}
}
