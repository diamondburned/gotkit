package sounds

import (
	"embed"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/pkg/errors"
)

//go:embed *.opus
var sounds embed.FS

// Sound IDs.
const (
	Bell    = "bell"
	Message = "message"
)

var (
	fileExistsCache  sync.Map
	soundsLastPlayed sync.Map
)

const soundDebounce = 200 * time.Millisecond

// Play plays the given sound ID. It first uses Canberra, falling back to
// ~/.cache/gotktrix/{id}.opus, then the embedded audio (if any), then
// display.Beep() otherwise.
//
// Play is asynchronous; it returning does not mean the audio has successfully
// been played to the user.
func Play(app *app.Application, id string) {
	go play(app, id)
}

func play(app *app.Application, id string) {
	now := time.Now()

	if t, ok := soundsLastPlayed.Load(id); ok {
		t := t.(time.Time)
		if now.Sub(t) < soundDebounce {
			return
		}
	}

	soundsLastPlayed.Store(id, now)

	canberra := exec.Command("canberra-gtk-play", "--id", id)
	if err := canberra.Run(); err == nil {
		return
	} else {
		log.Println("canberra error:", err)
	}

	name := id
	if filepath.Ext(name) == "" {
		name += ".opus"
	}

	dst := app.CachePath("sounds", name)

	var fileExists bool
	if b, ok := fileExistsCache.Load(dst); ok && b.(bool) {
		fileExists = true
	} else {
		_, err := os.Stat(dst)
		if err != nil {
			if err := copyToFS(dst, name); err != nil {
				log.Printf("cannot copy sound %q: %v", id, err)
				glib.IdleAdd(beep)
				return
			}
		}
		fileExists = true
		fileExistsCache.Store(id, fileExists)
	}

	glib.IdleAdd(func() {
		media := gtk.NewMediaFileForFilename(dst)

		mediaWeak := glib.NewWeakRef(media)
		media.NotifyProperty("error", func() {
			media := mediaWeak.Get()
			fileExistsCache.Delete(id)
			playEmbedError(id, media.Error())
		})

		media.Play()
	})
}

func playEmbedError(name string, err error) {
	log.Printf("error playing embedded %s.opus: %v", name, err)
	beep()
}

func beep() {
	log.Println("using beep() instead")
	disp := gdk.DisplayGetDefault()
	disp.Beep()
}

func copyToFS(dst string, name string) error {
	src, err := sounds.Open(name)
	if err != nil {
		return err
	}

	defer src.Close()

	dir := filepath.Dir(dst)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Wrap(err, "cannot mkdir sounds/")
	}

	f, err := os.CreateTemp(dir, ".tmp.*")
	if err != nil {
		return errors.Wrap(err, "cannot mktemp in cache dir")
	}

	defer os.Remove(f.Name())
	defer f.Close()

	if _, err := io.Copy(f, src); err != nil {
		return errors.Wrap(err, "cannot write audio to disk")
	}

	if err := f.Close(); err != nil {
		return errors.Wrap(err, "cannot close written audio")
	}

	if err := os.Rename(f.Name(), dst); err != nil {
		return errors.Wrap(err, "cannot commit written audio")
	}

	return nil
}
