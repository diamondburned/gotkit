package po

import (
	"embed"

	"github.com/diamondburned/gotkit/app/locale"
)

//go:embed *
var po embed.FS

func init() {
	locale.RegisterLocaleDomain("gotkit", po)
}
