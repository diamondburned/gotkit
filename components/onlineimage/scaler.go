package onlineimage

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
)

// maxScale is the maximum supported scale that we can supply a properly scaled
// pixbuf for.
const maxScale = 3

// 1x, 2x and 3x
type pixbufScales [maxScale]*gdkpixbuf.Pixbuf

type pixbufScaler struct {
	parent *baseImage
	// parentSz keeps track of the parent widget's sizes in case it has been
	// changed, which would force us to invalidate all scaled pixbufs.
	parentSz [2]int
	// scales stores scaled pixbufs.
	scales pixbufScales
	// src is the source pixbuf.
	src *gdkpixbuf.Pixbuf
}

// SetFromPixbuf invalidates and sets the internal scaler's pixbuf. The
// SetFromPixbuf call might be bubbled up to the parent widget.
func (p *pixbufScaler) SetFromPixbuf(pixbuf *gdkpixbuf.Pixbuf) {
	p.src = pixbuf
	p.scales = pixbufScales{}
	p.invalidate()
}

// Invalidate prompts the scaler to rescale.
func (p *pixbufScaler) Invalidate() {
	p.invalidate()
}

// ParentSize gets the cached parent widget's size request.
func (p *pixbufScaler) ParentSize() (w, h int) {
	return p.parentSz[0], p.parentSz[1]
}

func (p *pixbufScaler) init(parent *baseImage) {
	if parent.set().SetFromPixbuf == nil {
		log.Panicf("pixbufScaler: baseImage %T missing SetFromPixbuf", parent.imageParent)
	}

	p.parent = parent

	base := gtk.BaseWidget(parent)
	base.ConnectMap(func() {
		p.Invalidate()
	})
	base.NotifyProperty("scale-factor", func() {
		gtkutil.SetScaleFactor(base.ScaleFactor())
		p.Invalidate()
	})
}

func (p *pixbufScaler) setParentPixbuf(pixbuf *gdkpixbuf.Pixbuf) {
	setter := p.parent.set()
	setter.SetFromPixbuf(pixbuf)
}

// invalidate invalidates the scaled pixbuf and optionally refetches one if
// needed. The user should use this method instead of calling on the parent
// widget's Refetch method.
func (p *pixbufScaler) invalidate() {
	if p.src == nil {
		p.setParentPixbuf(nil)
		return
	}

	parent := gtk.BaseWidget(p.parent)

	scale := parent.ScaleFactor()
	if scale == 0 {
		// No allocations yet.
		return
	}

	dstW, dstH := p.parent.sizeRequest()
	if dstW < 1 || dstH < 1 {
		// No exact size requested, so we can't really scale relatively to that
		// size. Use the original pixbuf.
		p.setParentPixbuf(p.src)
		return
	}

	if p.parentSz != [2]int{dstW, dstH} {
		// Size changed, so invalidate all known pixbufs.
		p.scales = pixbufScales{}
		p.parentSz = [2]int{dstW, dstH}
	}

	// Scale the width and height up.
	dstW *= scale
	dstH *= scale

	srcW := p.src.Width()
	srcH := p.src.Height()

	if dstW >= srcW || dstH >= srcH {
		p.parent.set().SetFromPixbuf(p.src)
		return
	}

	if scale > maxScale {
		// We don't have these scales, so just use the source. User gets jagged
		// image, but on a 3x HiDPI display, it doesn't matter, unless the user
		// has both 3x and 1x displays.
		p.setParentPixbuf(p.src)
		return
	}

	pixbuf := p.scales[scale-1]

	if pixbuf == nil {
		if dstW == srcW && dstH == srcH {
			// Scaling is the same either way, so just use this for the current
			// scaling. This saves memory on most machines with only 1 scaling.
			pixbuf = p.src
		} else {
			// InterpTiles is apparently as good as bilinear when downscaling,
			// which is what we want.
			pixbuf = p.src.ScaleSimple(dstW, dstH, gdkpixbuf.InterpTiles)
		}

		p.scales[scale-1] = pixbuf
	}

	p.setParentPixbuf(pixbuf)
}
