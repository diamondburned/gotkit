package sounds

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/utils/osutil"
)

//go:embed *.opus
var embeddedSoundsFS embed.FS

// SoundsFS is a list of filesystems to search for sounds.
var SoundsFS = []fs.FS{embeddedSoundsFS}

// Sound IDs.
const (
	Bell    = "bell"
	Message = "message"
)

var (
	loadedSounds   = map[string]loadedSound{}
	loadedSoundsMu sync.RWMutex
)

type loadedSound struct {
	playing bool
	file    *gtk.MediaFile
}

func getLoadedSound(id string) (loadedSound, bool) {
	loadedSoundsMu.RLock()
	defer loadedSoundsMu.RUnlock()

	sound, ok := loadedSounds[id]
	return sound, ok
}

func setLoadedSound(id string, sound loadedSound) {
	loadedSoundsMu.Lock()
	defer loadedSoundsMu.Unlock()

	loadedSounds[id] = sound
}

func setLoadedSoundPlaying(id string, playing bool) {
	loadedSoundsMu.Lock()
	defer loadedSoundsMu.Unlock()

	if sound, ok := loadedSounds[id]; ok {
		sound.playing = playing
		loadedSounds[id] = sound
	}
}

func unloadSound(id string) {
	loadedSoundsMu.Lock()
	defer loadedSoundsMu.Unlock()

	delete(loadedSounds, id)
}

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
	sound, ok := getLoadedSound(id)
	if !ok {
		// If we can play with Canberra, we don't need to load the sound.
		// Mark the sound as loaded to prevent future loading.
		if playWithCanberra(id) {
			setLoadedSound(id, loadedSound{})
			return
		}

		soundFilename := id
		if filepath.Ext(soundFilename) == "" {
			soundFilename += ".opus"
		}

		soundFilepath := app.CachePath("sounds", soundFilename)

		if _, err := os.Stat(soundFilepath); err != nil {
			if !os.IsNotExist(err) {
				slog.Error(
					"cannot stat sound file, playing fallback beep",
					"module", "sounds",
					"err", err,
					"id", id,
					"path", soundFilepath)
				beep()
				return
			}

			if err := copyToFS(soundFilepath, soundFilename); err != nil {
				slog.Error(
					"cannot copy sound file to disk, playing fallback beep",
					"module", "sounds",
					"err", err,
					"id", id,
					"path", soundFilepath)
				beep()
				return
			}
		}

		glib.IdleAdd(func() {
			var soundFile *gtk.MediaFile

			if sound, ok := getLoadedSound(id); ok {
				soundFile = sound.file
				setLoadedSoundPlaying(id, true)
			} else {
				slog.Debug(
					"creating new media file for sound",
					"module", "sounds",
					"id", id,
					"path", soundFilepath)

				soundFile = gtk.NewMediaFileForFilename(soundFilepath)
				soundFile.NotifyProperty("error", func() {
					slog.Error(
						"could not load sound file, playing fallback beep",
						"module", "sounds",
						"err", soundFile.Error(),
						"id", id,
						"path", soundFilepath)
					beep()
					unloadSound(id)
				})
				soundFile.NotifyProperty("playing", func() {
					if soundFile.Playing() {
						slog.Debug(
							"playing sound with loaded media file",
							"module", "sounds",
							"id", id,
							"path", soundFilepath)
					} else {
						slog.Debug(
							"sound file stopped playing",
							"module", "sounds",
							"id", id,
							"path", soundFilepath)
						setLoadedSoundPlaying(id, false)
					}
				})

				setLoadedSound(id, loadedSound{
					playing: true,
					file:    soundFile,
				})
			}

			soundFile.Play()
		})

		return
	}

	if sound.playing {
		slog.Debug(
			"not playing sound, already playing",
			"module", "sounds",
			"id", id)
		return
	}

	slog.Debug(
		"sound loaded from cache, playing",
		"module", "sounds",
		"id", id,
		"is_media", sound.file != nil)

	if sound.file != nil {
		glib.IdleAdd(func() {
			setLoadedSoundPlaying(id, true)
			sound.file.Play()
		})
		return
	}

	if playWithCanberra(id) {
		return
	}

	// If Canberra fails after a successful play, we'll wipe the cache
	// and play the sound again.
	unloadSound(id)
	play(app, id)
}

var enableCanberra = true

func playWithCanberra(id string) bool {
	if !enableCanberra {
		return false
	}

	slog.Debug(
		"playing sound with canberra",
		"module", "sounds",
		"id", id)

	cmd := exec.Command("canberra-gtk-play", "--id", id)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		slog.Error(
			"failed to play sound with canberra",
			"module", "sounds",
			"id", id,
			"err", err)
		return false
	}

	return true
}

func beep() {
	glib.IdleAdd(func() {
		disp := gdk.DisplayGetDefault()
		disp.Beep()
	})
}

func copyToFS(dst string, name string) error {
	var soundData []byte

	for _, sounds := range SoundsFS {
		d, err := fs.ReadFile(sounds, name)
		if err == nil {
			soundData = d
			break
		}
	}

	if soundData == nil {
		return fmt.Errorf("embedded sound not found: %s", name)
	}

	return osutil.WriteFile(dst, soundData)
}
