package sounds

import (
	"embed"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/pkg/errors"
)

//go:embed *.opus
var sounds embed.FS

// Sound IDs.
const (
	Bell    = "bell.opus"
	Message = "message.opus"
)

type mediaFile struct {
	*gtk.MediaFile
	ID    string
	Error error
}

var mediaFiles = map[string]mediaFile{}

// Play plays the given sound ID. It first uses Canberra, falling back to
// ~/.cache/gotktrix/{id}.opus, then the embedded audio (if any), then
// display.Beep() otherwise.
//
// Play is asynchronous; it returning does not mean the audio has successfully
// been played to the user.
func Play(app *app.Application, id string) {
	glib.IdleAdd(func() {
		media, ok := mediaFiles[id]
		if ok {
			if media.Error != nil {
				playEmbedError(media.ID, media.Error)
			} else {
				media.Play()
			}
			return
		}

		go play(app, id)
	})
}

func play(app *app.Application, id string) {
	name := id
	if filepath.Ext(name) == "" {
		name += ".opus"
	}

	// app := app.FromContext(ctx)
	dst := app.CachePath("sounds", name)

	_, err := os.Stat(dst)
	if err != nil {
		canberra := exec.Command("canberra-gtk-play", "--id", id)
		if err := canberra.Run(); err == nil {
			return
		} else {
			log.Println("canberra error:", err)
		}

		if err := copyToFS(dst, name); err != nil {
			log.Printf("cannot copy sound %q: %v", id, err)
			glib.IdleAdd(beep)
			return
		}
	}

	glib.IdleAdd(func() {
		media, ok := mediaFiles[id]
		if !ok {
			media = mediaFile{
				MediaFile: gtk.NewMediaFileForFilename(dst),
				ID:        id,
				Error:     nil,
			}
			mediaFiles[id] = media
		}

		media.NotifyProperty("error", func() {
			f := mediaFiles[id]
			f.Error = media.MediaFile.Error()
			mediaFiles[id] = f

			playEmbedError(id, f.Error)
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
