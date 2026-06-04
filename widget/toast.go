package widget

import (
	"image/color"
	"time"

	"github.com/huanfeng/wind-ui/core"
)

// ToastDuration defines how long a toast is displayed.
type ToastDuration int

const (
	ToastShort ToastDuration = iota // ~2 seconds
	ToastLong                       // ~4 seconds
)

const (
	toastHeight   = 48.0
	toastMargin   = 24.0
	toastPaddingH = 24.0
	toastRadius   = 24.0
)

// Toast displays a brief message at the bottom of the screen.
// It auto-dismisses after the specified duration.
// Modeled after Android's Toast / Snackbar.
type Toast struct {
	BaseView
	message     string
	actionText  string
	actionClick func()
	duration    ToastDuration
	showing     bool
	overlayNode *core.Node
	dismissCh   chan struct{}

	hoveredAction bool
}

// ShowToast creates and shows a toast message on the given root node.
func ShowToast(root *core.Node, message string, duration ToastDuration) *Toast {
	t := &Toast{
		message:   message,
		duration:  duration,
		dismissCh: make(chan struct{}),
	}
	t.node = initNode("Toast", t)
	t.node.SetPainter(&toastPainter{t: t})
	t.node.SetHandler(&toastHandler{t: t})
	t.node.SetStyle(&core.Style{
		BackgroundColor: color.RGBA{R: 50, G: 50, B: 50, A: 230},
		TextColor:       color.RGBA{R: 255, G: 255, B: 255, A: 255},
		FontSize:        14,
	})

	t.show(root)
	return t
}

// NewSnackbar creates a toast with an action button (Snackbar style).
func NewSnackbar(root *core.Node, message, actionText string, action func()) *Toast {
	t := ShowToast(root, message, ToastLong)
	t.actionText = actionText
	t.actionClick = action
	return t
}

// GetMessage returns the toast message.
func (t *Toast) GetMessage() string {
	return t.message
}

// IsShowing returns whether the toast is currently displayed.
func (t *Toast) IsShowing() bool {
	return t.showing
}

// Dismiss immediately hides the toast.
func (t *Toast) Dismiss() {
	if !t.showing {
		return
	}
	t.showing = false
	// Signal the auto-dismiss goroutine to stop
	select {
	case t.dismissCh <- struct{}{}:
	default:
	}
	// Hide the overlay instead of removing it from the tree.
	// Removing mid-paint leaves stale canvas pixels because PaintNodeDirty
	// won't re-visit a removed node. Setting Gone lets the overlay paint a
	// transparent background to clear its region, then subsequent frames
	// skip it entirely via the visibility check in PaintNodeRecursive.
	if t.overlayNode != nil {
		t.overlayNode.SetVisibility(core.Gone)
		// Trigger a repaint so the overlay's transparent background is drawn
		// and the stale canvas pixels are cleared.
		if inv := t.overlayNode.GetInvalidator(); inv != nil {
			inv.Invalidate()
		}
	}
}

func (t *Toast) show(root *core.Node) {
	// Navigate to actual root
	for root.Parent() != nil {
		root = root.Parent()
	}
	t.showing = true

	// Toast uses a minimal overlay that only covers the toast area (non-modal).
	// We still use isOverlay so it renders on top and skips layout,
	// but the overlay node has NO handler — events pass through to content below.
	t.overlayNode = core.NewNode("ToastOverlay")
	t.overlayNode.SetPainter(&toastOverlayPainter{t: t})
	t.overlayNode.SetData("paintsChildren", true)
	t.overlayNode.SetData("isOverlay", true)
	t.overlayNode.SetData("nonModal", true) // events pass through
	t.overlayNode.AddChild(t.node)

	// Set overlay bounds to just the toast area at the bottom (non-modal).
	// This way events outside the toast reach the content behind it.
	rootSize := root.MeasuredSize()
	rootBounds := root.Bounds()
	w := rootSize.Width
	if w <= 0 {
		w = rootBounds.Width
	}
	h := rootSize.Height
	if h <= 0 {
		h = rootBounds.Height
	}
	dpi := getDPIScale(root)
	toastH := (toastHeight + toastMargin*2) * dpi
	t.overlayNode.SetBounds(core.Rect{X: 0, Y: h - toastH, Width: w, Height: toastH})
	t.overlayNode.SetMeasuredSize(core.Size{Width: w, Height: toastH})

	root.AddChild(t.overlayNode)

	// Auto-dismiss after duration
	dur := 2 * time.Second
	if t.duration == ToastLong {
		dur = 4 * time.Second
	}
	go func() {
		select {
		case <-time.After(dur):
			// Post dismiss to main thread: set showing=false, mark dirty,
			// and trigger a repaint via the window's Invalidate().
			t.showing = false
			if t.overlayNode != nil {
				t.overlayNode.MarkDirty()
				if inv := t.overlayNode.GetInvalidator(); inv != nil {
					inv.Invalidate()
				}
			}
		case <-t.dismissCh:
			// Already dismissed
		}
	}()
}

// ---------- toastOverlayPainter ----------

type toastOverlayPainter struct {
	t *Toast
}

func (p *toastOverlayPainter) Measure(node *core.Node, ws, hs core.MeasureSpec) core.Size {
	w, h := 0.0, 0.0
	if ws.Mode == core.MeasureModeExact {
		w = ws.Size
	}
	if hs.Mode == core.MeasureModeExact {
		h = hs.Size
	}
	return core.Size{Width: w, Height: h}
}

func (p *toastOverlayPainter) Paint(node *core.Node, canvas core.Canvas) {
	t := p.t
	if !t.showing {
		// Auto-dismiss triggered from goroutine — paint transparent background
		// to clear stale canvas pixels, then set overlay to Gone so subsequent
		// frames skip it via PaintNodeRecursive's visibility check.
		b := node.Bounds()
		canvas.DrawRect(core.Rect{Width: b.Width, Height: b.Height},
			&core.Paint{Color: color.RGBA{A: 0}, DrawStyle: core.PaintFill})
		t.Dismiss()
		return
	}

	b := node.Bounds()
	dpi := getDPIScale(node)
	margin := toastMargin * dpi
	height := toastHeight * dpi
	padH := toastPaddingH * dpi
	radius := toastRadius * dpi

	// Calculate toast width based on content
	s := t.node.GetStyle()
	fontSize := 14.0 * dpi // fallback
	if s != nil && s.FontSize > 0 {
		fontSize = s.FontSize // already DPI-scaled
	}
	toastPaint := &core.Paint{FontSize: fontSize}
	msgSize := core.NodeMeasureText(t.node, t.message, toastPaint)
	toastW := msgSize.Width + padH*2
	if t.actionText != "" {
		actionSize := core.NodeMeasureText(t.node, t.actionText, toastPaint)
		toastW += actionSize.Width + padH
	}
	maxW := b.Width - margin*2
	if toastW > maxW {
		toastW = maxW
	}
	if toastW < 200*dpi {
		toastW = 200 * dpi
	}

	// Position within the overlay's local bounds (overlay is at the bottom of screen)
	toastX := (b.Width - toastW) / 2
	toastY := b.Height - margin - height
	if toastY < 0 {
		toastY = 0
	}

	t.node.SetBounds(core.Rect{X: toastX, Y: toastY, Width: toastW, Height: height})
	t.node.SetMeasuredSize(core.Size{Width: toastW, Height: height})
	t.node.SetData("dpiScale", dpi)

	// Draw shadow
	shadowPaint := &core.Paint{Color: color.RGBA{A: 30}, DrawStyle: core.PaintFill}
	canvas.DrawRoundRect(core.Rect{X: toastX + 2*dpi, Y: toastY + 2*dpi, Width: toastW, Height: height}, radius, shadowPaint)

	core.PaintNodeRecursive(t.node, canvas)
}

// ---------- toastPainter ----------

type toastPainter struct {
	t *Toast
}

func (p *toastPainter) Measure(node *core.Node, ws, hs core.MeasureSpec) core.Size {
	return core.Size{Width: 200, Height: toastHeight}
}

func (p *toastPainter) Paint(node *core.Node, canvas core.Canvas) {
	t := p.t
	s := node.GetStyle()
	if s == nil {
		return
	}
	b := node.Bounds()
	dpi := getDPIScale(node)
	radius := toastRadius * dpi
	padH := toastPaddingH * dpi

	// Background
	bgPaint := &core.Paint{Color: s.BackgroundColor, DrawStyle: core.PaintFill}
	canvas.DrawRoundRect(core.Rect{Width: b.Width, Height: b.Height}, radius, bgPaint)

	fontSize := 14.0 * dpi // fallback
	if s.FontSize > 0 {
		fontSize = s.FontSize // already DPI-scaled
	}

	// Message text
	textPaint := &core.Paint{Color: s.TextColor, FontSize: fontSize}
	textSize := canvas.MeasureText(t.message, textPaint)
	textY := (b.Height - textSize.Height) / 2
	canvas.DrawText(t.message, padH, textY, textPaint)

	// Action button (Snackbar style)
	if t.actionText != "" {
		actionColor := color.RGBA{R: 130, G: 200, B: 255, A: 255}
		actionPaint := &core.Paint{Color: actionColor, FontSize: fontSize, FontWeight: 700}

		if t.hoveredAction {
			actionPaint.Color = color.RGBA{R: 180, G: 220, B: 255, A: 255}
		}

		actionSize := canvas.MeasureText(t.actionText, actionPaint)
		actionX := b.Width - padH - actionSize.Width
		canvas.DrawText(t.actionText, actionX, textY, actionPaint)
	}
}

// ---------- toastHandler ----------

type toastHandler struct {
	core.DefaultHandler
	t *Toast
}

func (h *toastHandler) OnEvent(node *core.Node, event core.Event) bool {
	t := h.t
	me, ok := event.(*core.MotionEvent)
	if !ok {
		return false
	}

	// Only handle action button clicks if present
	if t.actionText == "" {
		return false
	}

	switch me.Action {
	case core.ActionUp:
		if t.actionClick != nil && node.IsEnabled() {
			t.actionClick()
			t.Dismiss()
		}
		return true

	case core.ActionHoverEnter:
		t.hoveredAction = true
		node.MarkDirty()
		return true

	case core.ActionHoverExit:
		t.hoveredAction = false
		node.MarkDirty()
		return true
	}
	return false
}
