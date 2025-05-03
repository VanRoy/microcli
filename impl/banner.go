package impl

import (
	"fmt"

	"github.com/vanroy/microcli/impl/config"
)

var banner = `
    __  ___ _                     __
   /  |/  /(_)_____ _____ ____   / /_   ____   _  __
  / /|_/ // // ___// ___// __ \ / __ \ / __ \ | |/_/
 / /  / // // /__ / /   / /_/ // /_/ // /_/ /_>  <
/_/  /_//_/ \___//_/    \____//_____/ \____//_/|_|
`

var displayed = false

func ShowBanner() {
	if displayed || !config.Options.Interactive {
		return
	}

	fmt.Println(banner)
	displayed = true
}
