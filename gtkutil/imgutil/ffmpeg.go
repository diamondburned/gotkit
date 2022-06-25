package imgutil

import (
	"context"
	"log"
	"net/url"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/utils/cachegc"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

// FFmpegOpts is the options for FFmpeg.
type FFmpegOpts struct {
	Format    string // default "jpeg"
	AllowFile bool   // default false
}

// FFmpegProvider implements imgutil.Provider and uses FFmpeg to render the
// image using a given HTTP(S) URL. It supports images, videos and more.
var FFmpegProvider = FFmpegOpts{
	Format:    "jpeg",
	AllowFile: false,
}

// Schemes implements Provider.
func (p FFmpegOpts) Schemes() []string {
	if !p.AllowFile {
		return []string{"http", "https"}
	}
	return []string{"http", "https", "file"}
}

// Do implements Provider.
func (p FFmpegOpts) Do(ctx context.Context, url *url.URL, img ImageSetter) {
	go func() {
		o := OptsFromContext(ctx)

		var urlStr string
		if url.Scheme == "file" {
			urlStr = url.Host + url.Path // path is parsed weirdly
		} else {
			urlStr = url.String()
		}

		path, err := FFmpegThumbnail(ctx, p.Format, urlStr)
		if err != nil {
			o.Error(err)
			return
		}

		p, err := gdkpixbuf.NewPixbufFromFile(path)
		if err != nil {
			o.Error(errors.Wrap(err, "cannot create pixbuf"))
			return
		}

		glib.IdleAdd(func() {
			select {
			case <-ctx.Done():
				o.Error(ctx.Err())
			default:
			}

			switch {
			case img.SetFromPixbuf != nil:
				img.SetFromPixbuf(p)
			case img.SetFromPaintable != nil:
				img.SetFromPaintable(gdk.NewTextureForPixbuf(p))
			}
		})
	}()
}

var (
	hasFFmpeg  bool
	ffmpegOnce sync.Once
)

// FFmpegThumbnail fetches the thumbnail of the given URL and returns the path
// to the file. If format is empty, then jpeg is used.
func FFmpegThumbnail(ctx context.Context, format, url string) (string, error) {
	ffmpegOnce.Do(func() {
		ffmpeg, _ := exec.LookPath("ffmpeg")
		hasFFmpeg = ffmpeg != ""
	})

	if !hasFFmpeg {
		return "", nil
	}

	if format == "" {
		format = "jpeg"
	}

	app := app.FromContext(ctx)
	thumbDir := app.CachePath("thumbnails")
	thumbDst := urlPath(thumbDir, url)

	if cachegc.IsFile(thumbDst) {
		return thumbDst, nil
	}

	if err := ffmpegSema.Acquire(ctx, 1); err != nil {
		return thumbDst, err
	}
	defer ffmpegSema.Release(1)

	err := cachegc.WithTmp(thumbDst, "*."+format, func(out string) error {
		return doFFmpeg(ctx, url, out, "-frames:v", "1", "-f", "image2")
	})

	cachegc.Do(thumbDir, CacheAge)
	return thumbDst, err
}

var ffmpegSema = semaphore.NewWeighted(int64(runtime.GOMAXPROCS(-1)))

func doFFmpeg(ctx context.Context, src, dst string, opts ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := make([]string, 0, len(opts)+10)
	args = append(args, "-y", "-loglevel", "warning", "-i", src)
	args = append(args, opts...)
	args = append(args, dst)

	if err := exec.CommandContext(ctx, "ffmpeg", args...).Run(); err != nil {
		var exitErr *exec.ExitError

		if errors.As(err, &exitErr) {
			log.Printf(
				"ffmpeg exited with status %d, error: %s",
				exitErr.ExitCode(), string(exitErr.Stderr),
			)
		}

		return err
	}

	return nil
}
