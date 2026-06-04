package widget

import (
	"image/color"

	"github.com/huanfeng/wind-ui/core"
	"github.com/huanfeng/wind-ui/theme"
)

// CheckBox is a checkable box with a text label.
type CheckBox struct {
	BaseView
	checked   bool
	onChanged func(checked bool)
}

// NewCheckBox creates a new CheckBox with the given label text.
func NewCheckBox(text string) *CheckBox {
	cb := &CheckBox{}
	cb.node = initNode("CheckBox", cb)
	cb.node.SetPainter(&checkBoxPainter{cb: cb})
	cb.node.SetHandler(&checkBoxHandler{cb: cb})
	cb.node.SetStyle(&core.Style{
		FontSize:  14,
		TextColor: theme.CurrentColors().TextPrimary,
	})
	cb.node.SetData("text", text)
	cb.node.SetData("checked", false)
	return cb
}

// IsChecked reports whether the checkbox is currently checked.
func (cb *CheckBox) IsChecked() bool {
	return cb.checked
}

// SetChecked sets the checked state.
func (cb *CheckBox) SetChecked(checked bool) {
	cb.checked = checked
	cb.node.SetData("checked", checked)
	cb.node.MarkDirty()
}

// SetOnCheckedChanged sets the callback invoked when the checked state changes.
func (cb *CheckBox) SetOnCheckedChanged(fn func(checked bool)) {
	cb.onChanged = fn
}

// SetText updates the label text.
func (cb *CheckBox) SetText(text string) {
	cb.node.SetData("text", text)
	cb.node.MarkDirty()
}

// GetText returns the current label text.
func (cb *CheckBox) GetText() string {
	return cb.node.GetDataString("text")
}

// checkBoxPainter handles measurement and painting of the checkbox.
type checkBoxPainter struct {
	cb *CheckBox
}

const (
	checkBoxSize = 20.0 // box size in dp
	checkBoxGap  = 8.0  // gap between box and text
)

func (p *checkBoxPainter) Measure(node *core.Node, ws, hs core.MeasureSpec) core.Size {
	scale := getDPIScale(node)
	boxSize := checkBoxSize * scale
	gap := checkBoxGap * scale

	text := node.GetDataString("text")
	s := node.GetStyle()
	fontSize := 14.0
	if s != nil && s.FontSize > 0 {
		fontSize = s.FontSize
	}

	paint := &core.Paint{FontSize: fontSize}
	if s != nil {
		paint.FontFamily = s.FontFamily
		paint.FontWeight = s.FontWeight
	}
	textSize := core.NodeMeasureText(node, text, paint)

	w := boxSize + gap + textSize.Width
	h := max(boxSize, textSize.Height)

	if ws.Mode == core.MeasureModeExact {
		w = ws.Size
	} else if ws.Mode == core.MeasureModeAtMost && w > ws.Size {
		w = ws.Size
	}
	if hs.Mode == core.MeasureModeExact {
		h = hs.Size
	} else if hs.Mode == core.MeasureModeAtMost && h > hs.Size {
		h = hs.Size
	}

	return core.Size{Width: w, Height: h}
}

func (p *checkBoxPainter) Paint(node *core.Node, canvas core.Canvas) {
	s := node.GetStyle()
	if s == nil {
		return
	}
	b := node.Bounds()
	scale := getDPIScale(node)
	boxSize := checkBoxSize * scale
	gap := checkBoxGap * scale

	// Position box vertically centered
	boxX := 0.0
	boxY := (b.Height - boxSize) / 2
	boxRect := core.Rect{X: boxX, Y: boxY, Width: boxSize, Height: boxSize}

	// Clear the box area first to prevent residual pixels from previous state.
	// Use the parent's background color if available, otherwise transparent.
	bgColor := color.RGBA{A: 0}
	if s.BackgroundColor.A > 0 {
		bgColor = s.BackgroundColor
	} else if parent := node.Parent(); parent != nil {
		if ps := parent.GetStyle(); ps != nil && ps.BackgroundColor.A > 0 {
			bgColor = ps.BackgroundColor
		}
	}
	canvas.DrawRoundRect(boxRect, 3*scale, &core.Paint{Color: bgColor, DrawStyle: core.PaintFill})

	primaryColor := theme.CurrentColors().Primary

	if p.cb.checked {
		// Filled box with primary color
		fillPaint := &core.Paint{Color: primaryColor, DrawStyle: core.PaintFill}
		boxRect := core.Rect{X: boxX, Y: boxY, Width: boxSize, Height: boxSize}
		canvas.DrawRoundRect(boxRect, 3*scale, fillPaint)

		// Draw checkmark (white)
		checkPaint := &core.Paint{
			Color:       color.RGBA{R: 255, G: 255, B: 255, A: 255},
			DrawStyle:   core.PaintStroke,
			StrokeWidth: 2 * scale,
		}
		// Short leg: bottom-left to bottom-center
		canvas.DrawLine(boxX+4*scale, boxY+10*scale, boxX+8*scale, boxY+14*scale, checkPaint)
		// Long leg: bottom-center to top-right
		canvas.DrawLine(boxX+8*scale, boxY+14*scale, boxX+16*scale, boxY+5*scale, checkPaint)
	} else {
		// Empty box outline
		borderPaint := &core.Paint{
			Color:       color.RGBA{R: 117, G: 117, B: 117, A: 255},
			DrawStyle:   core.PaintStroke,
			StrokeWidth: 2 * scale,
		}
		boxRect := core.Rect{X: boxX, Y: boxY, Width: boxSize, Height: boxSize}
		canvas.DrawRoundRect(boxRect, 3*scale, borderPaint)
	}

	// Draw label text to the right of the box
	text := node.GetDataString("text")
	if text != "" {
		fontSize := 14.0
		if s.FontSize > 0 {
			fontSize = s.FontSize
		}
		textPaint := &core.Paint{
			Color:      s.TextColor,
			FontSize:   fontSize,
			FontFamily: s.FontFamily,
			FontWeight: s.FontWeight,
		}
		textX := boxSize + gap
		// Vertically center text
		textSize := canvas.MeasureText(text, textPaint)
		textY := (b.Height - textSize.Height) / 2
		canvas.DrawText(text, textX, textY, textPaint)
	}
}

// checkBoxHandler handles click events to toggle the checkbox state.
type checkBoxHandler struct {
	core.DefaultHandler
	cb      *CheckBox
	pressed bool
}

func (h *checkBoxHandler) OnEvent(node *core.Node, event core.Event) bool {
	me, ok := event.(*core.MotionEvent)
	if !ok {
		return false
	}

	switch me.Action {
	case core.ActionDown:
		h.pressed = true
		node.MarkDirty()
		return true
	case core.ActionUp:
		if h.pressed && node.IsEnabled() {
			h.pressed = false
			newChecked := !h.cb.checked
			h.cb.SetChecked(newChecked)
			if h.cb.onChanged != nil {
				h.cb.onChanged(newChecked)
			}
		}
		return true
	case core.ActionCancel:
		h.pressed = false
		node.MarkDirty()
		return true
	}
	return false
}
