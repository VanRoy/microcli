package initialzr

import (
	"github.com/vanroy/microcli/impl/config"
	"testing"
)

func SkipTestDownload(t *testing.T) {

	initializr := NewInitializr(config.Config{})

	initializr.Init("gradle-project", "test", []string{}, "/tmp/microcli-init")
}
