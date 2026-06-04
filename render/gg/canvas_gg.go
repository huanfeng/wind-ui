package gg

import (
	"image"
	"image/color"
	"image/draw"

	foggg "github.com/gogpu/gg"

	"github.com/huanfeng/wind-ui/core"
)

// clipRect tracks the current clip rectangle in absolute (image) coordinates.
type clipRect struct {
	x1, y1, x2, y2 float64 // min/max corners
	active          bool
}

// GGCanvas implements core.Canvas using the fogleman/gg library
// for anti-aliased 2D drawing.
type GGCanvas struct {
	dc           *foggg.Context
	textRenderer core.TextRenderer
	width        int
	height       int

	// Manual translate tracking for operations that bypass gg's transform
	// (DrawText and DrawImage write directly to the raw *image.RGBA).
	txStack []point2
	tx, ty  float64 // accumulated translate offset

	// Manual clip tracking — mirrors gg's clip for fast early-out checks.
	// gg's Clip() provides pixel-perfect clipping; this rect lets us skip
	// draws that are entirely outside the clip without invoking gg at all.
	clip      clipRect
	clipStack []clipRect

	// Shared RGBA view over gg's internal Pixmap data — avoids copies.
	sharedTarget *image.RGBA
}

type point2 struct{ x, y float64 }

// NewGGCanvas creates a new GGCanvas with the given dimensions.
// textRenderer may be nil; text operations become no-ops in that case.
func NewGGCanvas(width, height int, textRenderer core.TextRenderer) *GGCanvas {
	dc := foggg.NewContext(width, height)
	return &GGCanvas{
		dc:           dc,
		textRenderer: textRenderer,
		width:        width,
		height:       height,
		clip:         clipRect{x1: 0, y1: 0, x2: float64(width), y2: float64(height), active: false},
	}
}

// NewGGCanvasForImage creates a GGCanvas backed by the same pixel buffer as img
// using zero-copy aliasing (NewPixmapFromBuffer). Writes through the canvas go
// directly into img.Pix without allocation or copy.
func NewGGCanvasForImage(img *image.RGBA, textRenderer core.TextRenderer) *GGCanvas {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	pm := foggg.NewPixmapFromBuffer(img.Pix, w, h)
	dc := foggg.NewContextForPixmap(pm)
	return &GGCanvas{
		dc:           dc,
		textRenderer: textRenderer,
		width:        w,
		height:       h,
		clip:         clipRect{x1: 0, y1: 0, x2: float64(w), y2: float64(h), active: false},
	}
}

// NewGGCanvasRetained creates a GGCanvas that reuses the pixel buffer from img
// (previous frame) via zero-copy aliasing. Dirty regions can be cleared and
// repainted while the rest of the frame is preserved in-place.
func NewGGCanvasRetained(img *image.RGBA, textRenderer core.TextRenderer) *GGCanvas {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	pm := foggg.NewPixmapFromBuffer(img.Pix, w, h)
	dc := foggg.NewContextForPixmap(pm)
	return &GGCanvas{
		dc:           dc,
		textRenderer: textRenderer,
		width:        w,
		height:       h,
		clip:         clipRect{x1: 0, y1: 0, x2: float64(w), y2: float64(h), active: false},
	}
}

// ClearRect zeroes the pixels within the given rectangle, preparing it
// for a fresh repaint while leaving the rest of the canvas untouched.
func (c *GGCanvas) ClearRect(rect core.Rect) {
	target := c.targetRGBA()
	if target == nil {
		return
	}
	bounds := target.Bounds()
	x0 := max(int(rect.X), bounds.Min.X)
	y0 := max(int(rect.Y), bounds.Min.Y)
	x1 := min(int(rect.X+rect.Width+0.5), bounds.Max.X)
	y1 := min(int(rect.Y+rect.Height+0.5), bounds.Max.Y)

	for y := y0; y < y1; y++ {
		off := (y-bounds.Min.Y)*target.Stride + (x0-bounds.Min.X)*4
		end := (y-bounds.Min.Y)*target.Stride + (x1-bounds.Min.X)*4
		clear(target.Pix[off:end])
	}
}

// ---------- Drawing primitives ----------

// DrawRect draws a filled or stroked rectangle.
func (c *GGCanvas) DrawRect(rect core.Rect, paint *core.Paint) {
	if paint == nil {
		return
	}
	if c.clip.active {
		if c.isOutsideClip(rect) {
			return
		}
		// Fill-only fast path: intersect rect with clip to avoid drawing
		// invisible portions. gg's clip handles the rest for correctness.
		if paint.DrawStyle == core.PaintFill {
			rect = c.clipLocalRect(rect)
			if rect.Width <= 0 || rect.Height <= 0 {
				return
			}
		}
	}
	c.dc.DrawRectangle(rect.X, rect.Y, rect.Width, rect.Height)
	c.applyPaint(paint)
}

// DrawRoundRect draws a rounded rectangle.
func (c *GGCanvas) DrawRoundRect(rect core.Rect, radius float64, paint *core.Paint) {
	if paint == nil {
		return
	}
	if radius <= 0 {
		c.DrawRect(rect, paint)
		return
	}
	if c.clip.active && c.isOutsideClip(rect) {
		return
	}
	c.dc.DrawRoundedRectangle(rect.X, rect.Y, rect.Width, rect.Height, radius)
	c.applyPaint(paint)
}

// DrawCircle draws a filled or stroked circle.
func (c *GGCanvas) DrawCircle(cx, cy, radius float64, paint *core.Paint) {
	if paint == nil {
		return
	}
	if c.clip.active {
		bounds := core.Rect{X: cx - radius, Y: cy - radius, Width: radius * 2, Height: radius * 2}
		if c.isOutsideClip(bounds) {
			return
		}
	}
	c.dc.DrawCircle(cx, cy, radius)
	c.applyPaint(paint)
}

// DrawLine draws a line between two points.
func (c *GGCanvas) DrawLine(x1, y1, x2, y2 float64, paint *core.Paint) {
	if paint == nil {
		return
	}
	sw := paint.StrokeWidth
	if sw <= 0 {
		sw = 1
	}
	if c.clip.active {
		bounds := core.Rect{
			X:      min(x1, x2) - sw,
			Y:      min(y1, y2) - sw,
			Width:  max(x1, x2) - min(x1, x2) + sw*2,
			Height: max(y1, y2) - min(y1, y2) + sw*2,
		}
		if c.isOutsideClip(bounds) {
			return
		}
	}
	c.dc.DrawLine(x1, y1, x2, y2)
	c.dc.SetColor(paint.Color)
	c.dc.SetLineWidth(sw)
	c.dc.Stroke()
}

// DrawImage draws an ImageResource into the destination rectangle.
func (c *GGCanvas) DrawImage(img *core.ImageResource, dst core.Rect) {
	if img == nil || img.Image == nil {
		return
	}
	srcBounds := img.Image.Bounds()

	// Simple nearest-neighbour scaling into a temp RGBA, then draw.
	tmp := image.NewRGBA(image.Rect(0, 0, int(dst.Width), int(dst.Height)))
	for py := 0; py < int(dst.Height); py++ {
		for px := 0; px < int(dst.Width); px++ {
			sx := srcBounds.Min.X + px*srcBounds.Dx()/int(dst.Width)
			sy := srcBounds.Min.Y + py*srcBounds.Dy()/int(dst.Height)
			if sx >= srcBounds.Max.X {
				sx = srcBounds.Max.X - 1
			}
			if sy >= srcBounds.Max.Y {
				sy = srcBounds.Max.Y - 1
			}
			tmp.Set(px, py, img.Image.At(sx, sy))
		}
	}

	// Apply accumulated translate offset since we write directly to raw pixels.
	dx := dst.X + c.tx
	dy := dst.Y + c.ty
	target := c.targetRGBA()

	// Clip the draw region to the current clip rect
	drawRect := image.Rect(int(dx), int(dy), int(dx+dst.Width), int(dy+dst.Height))
	if c.clip.active {
		cr := image.Rect(int(c.clip.x1), int(c.clip.y1), int(c.clip.x2), int(c.clip.y2))
		drawRect = drawRect.Intersect(cr)
	}

	srcPoint := image.Point{X: drawRect.Min.X - int(dx), Y: drawRect.Min.Y - int(dy)}
	draw.Draw(target, drawRect, tmp, srcPoint, draw.Over)
}

// DrawText delegates to the injected TextRenderer. No-op if nil.
// Applies accumulated translate offset since TextRenderer writes directly
// to the raw *image.RGBA, bypassing gg's internal transform.
func (c *GGCanvas) DrawText(text string, x, y float64, paint *core.Paint) {
	if c.textRenderer == nil || paint == nil {
		return
	}

	absX := x + c.tx
	absY := y + c.ty

	// Skip text entirely if it's outside the clip rect
	if c.clip.active {
		fontSize := paint.FontSize
		if fontSize <= 0 {
			fontSize = 14
		}
		// Estimate text height for clip check
		textH := fontSize * 1.5
		if absY+textH < c.clip.y1 || absY > c.clip.y2 {
			return // completely above or below clip
		}
		if absX > c.clip.x2 {
			return // completely to the right of clip
		}
	}

	c.textRenderer.SetFont(paint.FontFamily, paint.FontWeight, paint.FontSize)

	if c.clip.active {
		// Draw text to a temporary image, then copy only the clipped region
		c.drawTextClipped(text, absX, absY, paint)
	} else {
		c.textRenderer.DrawText(c, text, absX, absY, paint)
	}
}

// drawTextClipped renders text into a temporary buffer and copies only the
// portion that falls within the active clip rect to the canvas target.
func (c *GGCanvas) drawTextClipped(text string, absX, absY float64, paint *core.Paint) {
	target := c.targetRGBA()
	bounds := target.Bounds()

	// Save the region that will be drawn to, render text, then mask
	// pixels outside the clip rect by restoring the saved pixels.
	cr := image.Rect(int(c.clip.x1), int(c.clip.y1), int(c.clip.x2), int(c.clip.y2))
	cr = cr.Intersect(bounds)
	if cr.Empty() {
		return
	}

	// Estimate the text bounding box
	textSize := c.textRenderer.MeasureText(text)
	textRect := image.Rect(int(absX), int(absY), int(absX+textSize.Width+2), int(absY+textSize.Height+2))
	textRect = textRect.Intersect(bounds)
	if textRect.Empty() {
		return
	}

	// Find pixels that are inside textRect but OUTSIDE clip — save them
	type savedPixel struct {
		x, y int
		c    color.RGBA
	}
	var saved []savedPixel
	for py := textRect.Min.Y; py < textRect.Max.Y; py++ {
		for px := textRect.Min.X; px < textRect.Max.X; px++ {
			if !image.Pt(px, py).In(cr) {
				idx := target.PixOffset(px, py)
				if idx+3 < len(target.Pix) {
					saved = append(saved, savedPixel{
						x: px, y: py,
						c: color.RGBA{
							R: target.Pix[idx],
							G: target.Pix[idx+1],
							B: target.Pix[idx+2],
							A: target.Pix[idx+3],
						},
					})
				}
			}
		}
	}

	// Draw text normally
	c.textRenderer.DrawText(c, text, absX, absY, paint)

	// Restore pixels outside clip rect
	for _, sp := range saved {
		idx := target.PixOffset(sp.x, sp.y)
		if idx+3 < len(target.Pix) {
			target.Pix[idx] = sp.c.R
			target.Pix[idx+1] = sp.c.G
			target.Pix[idx+2] = sp.c.B
			target.Pix[idx+3] = sp.c.A
		}
	}
}

// MeasureText delegates to the injected TextRenderer. Returns zero Size if nil.
func (c *GGCanvas) MeasureText(text string, paint *core.Paint) core.Size {
	if c.textRenderer == nil || paint == nil {
		return core.Size{}
	}
	c.textRenderer.SetFont(paint.FontFamily, paint.FontWeight, paint.FontSize)
	return c.textRenderer.MeasureText(text)
}

// ---------- State management ----------

// Save pushes the current drawing state (transform, clip) onto the stack.
func (c *GGCanvas) Save() {
	c.dc.Push()
	c.txStack = append(c.txStack, point2{c.tx, c.ty})
	c.clipStack = append(c.clipStack, c.clip)
}

// Restore pops the most recent drawing state from the stack.
func (c *GGCanvas) Restore() {
	c.dc.Pop()
	if len(c.txStack) > 0 {
		top := c.txStack[len(c.txStack)-1]
		c.txStack = c.txStack[:len(c.txStack)-1]
		c.tx, c.ty = top.x, top.y
	}
	if len(c.clipStack) > 0 {
		c.clip = c.clipStack[len(c.clipStack)-1]
		c.clipStack = c.clipStack[:len(c.clipStack)-1]
	}
}

// Translate accumulates a translation offset.
func (c *GGCanvas) Translate(dx, dy float64) {
	c.dc.Translate(dx, dy)
	c.tx += dx
	c.ty += dy
}

// ClipRect intersects the current clip with the given rectangle.
// gg's Clip() provides pixel-perfect clipping (including antialiased edges).
// The manual clip rect is maintained in parallel for fast early-out checks
// in drawing methods (skip shapes entirely outside the clip).
func (c *GGCanvas) ClipRect(rect core.Rect) {
	c.dc.DrawRectangle(rect.X, rect.Y, rect.Width, rect.Height)
	c.dc.Clip()

	absX1 := rect.X + c.tx
	absY1 := rect.Y + c.ty
	absX2 := absX1 + rect.Width
	absY2 := absY1 + rect.Height

	if c.clip.active {
		absX1 = max(absX1, c.clip.x1)
		absY1 = max(absY1, c.clip.y1)
		absX2 = min(absX2, c.clip.x2)
		absY2 = min(absY2, c.clip.y2)
	}

	c.clip = clipRect{x1: absX1, y1: absY1, x2: absX2, y2: absY2, active: true}
}

// Target returns the underlying RGBA image.
func (c *GGCanvas) Target() *image.RGBA {
	return c.targetRGBA()
}

// ---------- Clip helpers ----------

// isOutsideClip checks if a local-coordinate rect is entirely outside the active clip.
func (c *GGCanvas) isOutsideClip(rect core.Rect) bool {
	if !c.clip.active {
		return false
	}
	absX := rect.X + c.tx
	absY := rect.Y + c.ty
	return absX+rect.Width <= c.clip.x1 || absX >= c.clip.x2 ||
		absY+rect.Height <= c.clip.y1 || absY >= c.clip.y2
}

// clipLocalRect intersects a local-coordinate rect with the active clip and
// returns the result in local coordinates.
func (c *GGCanvas) clipLocalRect(rect core.Rect) core.Rect {
	if !c.clip.active {
		return rect
	}
	absX1 := max(rect.X+c.tx, c.clip.x1)
	absY1 := max(rect.Y+c.ty, c.clip.y1)
	absX2 := min(rect.X+c.tx+rect.Width, c.clip.x2)
	absY2 := min(rect.Y+c.ty+rect.Height, c.clip.y2)
	if absX2 <= absX1 || absY2 <= absY1 {
		return core.Rect{}
	}
	return core.Rect{
		X:      absX1 - c.tx,
		Y:      absY1 - c.ty,
		Width:  absX2 - absX1,
		Height: absY2 - absY1,
	}
}

// ---------- Internal helpers ----------

// targetRGBA returns an *image.RGBA that shares the same underlying pixel
// memory as the gg context's internal Pixmap. This allows direct pixel
// manipulation (text rendering, image compositing) to coexist with gg's
// vector drawing on the same buffer without copies.
//
// FlushGPU is called to ensure any pending GPU-accelerated drawing
// operations are committed to the pixel buffer before access.
func (c *GGCanvas) targetRGBA() *image.RGBA {
	// Flush pending GPU operations so pixel data is up-to-date.
	c.dc.FlushGPU()

	if c.sharedTarget != nil {
		return c.sharedTarget
	}
	pm := c.dc.ResizeTarget()
	c.sharedTarget = pm.ImageView()
	return c.sharedTarget
}

// applyPaint applies fill/stroke based on the paint style.
func (c *GGCanvas) applyPaint(paint *core.Paint) {
	c.dc.SetColor(color.RGBA(paint.Color))
	switch paint.DrawStyle {
	case core.PaintFill:
		c.dc.Fill()
	case core.PaintStroke:
		sw := paint.StrokeWidth
		if sw <= 0 {
			sw = 1
		}
		c.dc.SetLineWidth(sw)
		c.dc.Stroke()
	case core.PaintFillAndStroke:
		c.dc.FillPreserve()
		sw := paint.StrokeWidth
		if sw <= 0 {
			sw = 1
		}
		c.dc.SetLineWidth(sw)
		c.dc.Stroke()
	}
}
