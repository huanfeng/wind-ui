package windows

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/huanfeng/wind-ui/core"
	"github.com/huanfeng/wind-ui/layout"
	"github.com/huanfeng/wind-ui/platform"
)

// GDI font procs used by native EDIT controls.
var (
	procCreateFontW  *syscall.LazyProc
	procSetTextColor *syscall.LazyProc
	procSetBkMode    *syscall.LazyProc
	procExtTextOutW  *syscall.LazyProc
	gdiTextProcsOnce sync.Once
)

func initGDITextProcs() {
	gdiTextProcsOnce.Do(func() {
		gdi := syscall.NewLazyDLL("gdi32.dll")
		procCreateFontW = gdi.NewProc("CreateFontW")
		procSetTextColor = gdi.NewProc("SetTextColor")
		procSetBkMode = gdi.NewProc("SetBkMode")
		procExtTextOutW = gdi.NewProc("ExtTextOutW")
	})
}

// Win32 EDIT control styles and messages.
const (
	wsChild       = 0x40000000
	wsBorder      = 0x00800000
	wsTabStop     = 0x00010000
	esLeft        = 0x0000
	esCenter      = 0x0001
	esMultiLine   = 0x0004
	esPassword    = 0x0020
	esAutoHScroll = 0x0080
	esAutoVScroll = 0x0040
	esNumber      = 0x2000

	wmSetText       = 0x000C
	wmGetText       = 0x000D
	wmGetTextLength = 0x000E
	wmSetFont       = 0x0030
	wmCommand       = 0x0111
	enChange        = 0x0300

	emSetSel       = 0x00B1
	emSetCueBanner = 0x1501
	emSetLimitText = 0x00C5
)

// Win32 constants for subclassing and keyboard handling.
const (
	gwlpWndProc    = -4 // GWLP_WNDPROC
	wmKeyDown      = 0x0100
	wmMouseWheel   = 0x020A
	vkA            = 0x41
	emGetLineCount = 0x00BA
	emGetFirstVisibleLine = 0x00CE
	sbVert         = 1
)

// procSendMessageW and procSetFocus are loaded lazily so that the
// native_edit module does not duplicate declarations from other files.
var (
	nativeEditProcsOnce sync.Once
	procSendMessageW    *syscall.LazyProc
	procSetFocus        *syscall.LazyProc
	procSetWindowLongPtrW *syscall.LazyProc
	procCallWindowProcW   *syscall.LazyProc
	procGetKeyState       *syscall.LazyProc
)

func initNativeEditProcs() {
	nativeEditProcsOnce.Do(func() {
		u32 := syscall.NewLazyDLL("user32.dll")
		procSendMessageW = u32.NewProc("SendMessageW")
		procSetFocus = u32.NewProc("SetFocus")
		procSetWindowLongPtrW = u32.NewProc("SetWindowLongPtrW")
		procCallWindowProcW = u32.NewProc("CallWindowProcW")
		procGetKeyState = u32.NewProc("GetKeyState")
	})
}

// editSubclassMap stores hwnd → *win32NativeEdit for the subclass wndProc.
var editSubclassMap sync.Map

// editSubclassProc is the subclassed wndProc for EDIT controls.
// It handles Ctrl+A (select all) and forwards unhandled scroll wheel events.
func editSubclassProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	val, ok := editSubclassMap.Load(hwnd)
	if !ok {
		return 0
	}
	ne := val.(*win32NativeEdit)

	switch msg {
	case wmKeyDown:
		// Ctrl+A: select all (standard EDIT doesn't support this natively)
		if wParam == vkA {
			state, _, _ := procGetKeyState.Call(0x11) // VK_CONTROL
			if int16(state) < 0 { // high bit set = pressed
				procSendMessageW.Call(hwnd, emSetSel, 0, ^uintptr(0)) // 0, -1
				return 0
			}
		}

	case wmMouseWheel:
		// For multi-line EDIT: if content doesn't need scrolling, forward to parent.
		if ne.multiLine {
			lineCount, _, _ := procSendMessageW.Call(hwnd, emGetLineCount, 0, 0)
			// If content fits in the control (roughly), forward wheel to parent
			if ne.fontSize > 0 {
				visibleLines := int(float64(ne.lastH) / ne.fontSize)
				if int(lineCount) <= visibleLines {
					// Forward to parent window
					procSendMessageW.Call(ne.parentHwnd, wmMouseWheel, wParam, lParam)
					return 0
				}
			}
		} else {
			// Single-line EDIT: always forward wheel to parent
			procSendMessageW.Call(ne.parentHwnd, wmMouseWheel, wParam, lParam)
			return 0
		}
	}

	// Call original wndProc
	ret, _, _ := procCallWindowProcW.Call(ne.origWndProc, hwnd, msg, wParam, lParam)
	return ret
}

// editInset is the pixel inset from the node bounds for the native EDIT
// so the framework's custom border is visible around it.
const editInset = 2.0

// win32NativeEdit wraps a Win32 EDIT control and implements platform.NativeEditText.
type win32NativeEdit struct {
	hwnd        uintptr // EDIT control HWND
	parentHwnd  uintptr // parent window HWND
	origWndProc uintptr // original wndProc before subclassing
	node        *core.Node
	hFont       uintptr // GDI font handle

	multiLine bool
	inputType platform.InputType
	style     uintptr // current window style bits

	fontSize float64 // last set font size (physical px) for vertical centering

	onTextChanged func(string)
	onSubmit      func(string)

	lastX, lastY, lastW, lastH int // cached position to avoid redundant MoveWindow
	hidden                      bool
}

// newNativeEdit creates a hidden Win32 EDIT control as a child of parentHwnd.
// No system border (wsBorder removed) — the framework draws its own border.
func newNativeEdit(parentHwnd uintptr) *win32NativeEdit {
	initNativeEditProcs()
	initGDITextProcs() // ensure procCreateFontW is available

	editClass, _ := syscall.UTF16PtrFromString("EDIT")
	// No wsBorder: framework's editTextPainter draws a custom rounded border.
	style := uintptr(wsChild | esAutoHScroll | wsTabStop)

	hInstance, _, _ := procGetModuleHandleW.Call(0)

	hwnd, _, _ := procCreateWindowExW.Call(
		0, // dwExStyle
		uintptr(unsafe.Pointer(editClass)),
		0, // lpWindowName (no initial text)
		style,
		0, 0, 100, 30, // x, y, width, height — will be updated by AttachToNode
		parentHwnd,
		0, // hMenu
		hInstance,
		0, // lpParam
	)

	ne := &win32NativeEdit{
		hwnd:       hwnd,
		parentHwnd: parentHwnd,
		style:      style,
		lastX:      -1, lastY: -1,
		hidden:     true, // starts hidden — UpdatePosition will show it
	}

	// Subclass the EDIT control to handle Ctrl+A and wheel forwarding.
	editSubclassMap.Store(hwnd, ne)
	ne.origWndProc, _, _ = procSetWindowLongPtrW.Call(
		hwnd, uintptr(0xFFFFFFFFFFFFFFFC),
		syscall.NewCallback(editSubclassProc),
	)

	return ne
}

// ---------- platform.NativeEditText implementation ----------

func (e *win32NativeEdit) AttachToNode(node *core.Node) {
	e.node = node
	// Store self on node so the painter can call UpdatePosition during Paint.
	node.SetData("nativeEdit", e)
	e.UpdatePosition()
}

func (e *win32NativeEdit) Detach() {
	if e.node != nil {
		e.node.SetData("nativeEdit", nil)
	}
	procShowWindow.Call(e.hwnd, SW_HIDE)
	e.node = nil
	e.hidden = true
}

// Hide explicitly hides the native edit control (e.g. when the parent page
// becomes invisible due to a tab switch).
func (e *win32NativeEdit) Hide() {
	if !e.hidden {
		procShowWindow.Call(e.hwnd, SW_HIDE)
		e.hidden = true
	}
}

// UpdatePosition recalculates the native EDIT control's position and size
// based on the node's current layout bounds. It accounts for:
// - Absolute position (walking parent bounds)
// - ScrollView offsets (subtracting scroll position)
// - Viewport clipping (hiding when scrolled out of view)
// - Inset from node bounds (so framework border is visible)
func (e *win32NativeEdit) UpdatePosition() {
	if e.node == nil {
		return
	}
	node := e.node
	b := node.Bounds()
	dpi := 1.0
	if s, ok := node.GetData("dpiScale").(float64); ok && s > 0 {
		dpi = s
	}
	inset := editInset * dpi

	// Walk up parent chain: accumulate absolute position and compute
	// the visible clip rect from any ScrollView ancestor.
	// NOTE: Do NOT subtract scroll offset here — ScrollLayout.Arrange already
	// applies the offset to child bounds (Y = padding - OffsetY).
	absX := b.X + inset
	absY := b.Y + inset
	clipValid := false
	var clipY1, clipY2 float64

	for n := node.Parent(); n != nil; n = n.Parent() {
		nb := n.Bounds()

		// If this ancestor is a ScrollView, compute its viewport as the clip rect.
		// Native controls outside this rect will be hidden.
		if _, ok := n.GetLayout().(*layout.ScrollLayout); ok {
			scrollAbsY := nb.Y
			for p := n.Parent(); p != nil; p = p.Parent() {
				scrollAbsY += p.Bounds().Y
			}
			clipY1 = scrollAbsY
			clipY2 = scrollAbsY + nb.Height
			clipValid = true
		}

		absX += nb.X
		absY += nb.Y
	}

	w := b.Width - inset*2
	h := b.Height - inset*2
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	// For single-line EDIT: size the control to fit the font and center vertically.
	// This avoids text sitting at the top of a tall control.
	if !e.multiLine && e.fontSize > 0 {
		editH := e.fontSize + 6*dpi // font height + small internal padding
		if editH < h {
			absY += (h - editH) / 2 // center vertically
			h = editH
		}
	}

	ix, iy, iw, ih := int(absX), int(absY), int(w), int(h)

	// Viewport clipping: native controls can't be partially clipped,
	// so hide when any part extends outside the ScrollView viewport.
	if clipValid {
		editBottom := absY + h
		if absY < clipY1 || editBottom > clipY2 {
			if !e.hidden {
				procShowWindow.Call(e.hwnd, SW_HIDE)
				e.hidden = true
			}
			return
		}
	}

	// Show if was hidden
	if e.hidden {
		procShowWindow.Call(e.hwnd, SW_SHOW)
		e.hidden = false
	}

	// Only call MoveWindow if position/size actually changed
	if ix != e.lastX || iy != e.lastY || iw != e.lastW || ih != e.lastH {
		procMoveWindow.Call(
			e.hwnd,
			uintptr(ix), uintptr(iy),
			uintptr(iw), uintptr(ih),
			1, // bRepaint
		)
		e.lastX, e.lastY, e.lastW, e.lastH = ix, iy, iw, ih
	}
}

func (e *win32NativeEdit) GetText() string {
	length, _, _ := procSendMessageW.Call(e.hwnd, wmGetTextLength, 0, 0)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	procSendMessageW.Call(e.hwnd, wmGetText, length+1, uintptr(unsafe.Pointer(&buf[0])))
	return syscall.UTF16ToString(buf)
}

func (e *win32NativeEdit) SetText(text string) {
	t, _ := syscall.UTF16PtrFromString(text)
	procSendMessageW.Call(e.hwnd, wmSetText, 0, uintptr(unsafe.Pointer(t)))
}

func (e *win32NativeEdit) SetPlaceholder(text string) {
	// EM_SETCUEBANNER is available on Windows Vista+ (comctl32 v6).
	t, _ := syscall.UTF16PtrFromString(text)
	procSendMessageW.Call(e.hwnd, emSetCueBanner, 1, uintptr(unsafe.Pointer(t)))
}

func (e *win32NativeEdit) SetFont(family string, size float64, weight int) {
	e.fontSize = size
	if e.hFont != 0 {
		procDeleteObject.Call(e.hFont)
	}
	fontName, _ := syscall.UTF16PtrFromString(family)
	if weight <= 0 {
		weight = 400
	}
	e.hFont, _, _ = procCreateFontW.Call(
		uintptr(-int32(size)), // nHeight (negative = character height)
		0,                     // nWidth
		0,                     // nEscapement
		0,                     // nOrientation
		uintptr(weight),       // fnWeight
		0,                     // fdwItalic
		0,                     // fdwUnderline
		0,                     // fdwStrikeOut
		1,                     // fdwCharSet (DEFAULT_CHARSET)
		0,                     // fdwOutputPrecision
		0,                     // fdwClipPrecision
		5,                     // fdwQuality (CLEARTYPE_QUALITY)
		0,                     // fdwPitchAndFamily
		uintptr(unsafe.Pointer(fontName)),
	)
	procSendMessageW.Call(e.hwnd, wmSetFont, e.hFont, 1)
}

func (e *win32NativeEdit) SetTextColor(_ interface{}) {
	// Requires WM_CTLCOLOREDIT handling in parent wndProc.
}

func (e *win32NativeEdit) SetBackgroundColor(_ interface{}) {
	// Requires WM_CTLCOLOREDIT handling in parent wndProc.
}

func (e *win32NativeEdit) SetMultiLine(multiLine bool) {
	if e.multiLine == multiLine {
		return
	}
	e.multiLine = multiLine
	e.recreateControl()
}

func (e *win32NativeEdit) SetMaxLength(max int) {
	if max <= 0 {
		max = 0 // 0 = default limit
	}
	procSendMessageW.Call(e.hwnd, emSetLimitText, uintptr(max), 0)
}

func (e *win32NativeEdit) SetInputType(inputType platform.InputType) {
	if e.inputType == inputType {
		return
	}
	e.inputType = inputType
	e.recreateControl()
}

func (e *win32NativeEdit) SetOnTextChanged(fn func(text string)) {
	e.onTextChanged = fn
}

func (e *win32NativeEdit) SetOnSubmit(fn func(text string)) {
	e.onSubmit = fn
}

func (e *win32NativeEdit) Focus() {
	procSetFocus.Call(e.hwnd)
}

func (e *win32NativeEdit) ClearFocus() {
	procSetFocus.Call(e.parentHwnd)
}

// ---------- Internal helpers ----------

// recreateControl destroys the current EDIT control and creates a new one
// with the updated style flags, preserving text and position.
func (e *win32NativeEdit) recreateControl() {
	// Save state.
	oldText := e.GetText()
	visible, _, _ := procIsWindowVisible.Call(e.hwnd)

	// Determine position from the existing window.
	var rc RECT
	procGetWindowRect.Call(e.hwnd, uintptr(unsafe.Pointer(&rc)))
	pt := POINT{X: rc.Left, Y: rc.Top}
	procScreenToClient.Call(e.parentHwnd, uintptr(unsafe.Pointer(&pt)))
	width := rc.Right - rc.Left
	height := rc.Bottom - rc.Top

	// Destroy old control.
	procDestroyWindow.Call(e.hwnd)

	// Build new style — no wsBorder (framework draws its own).
	style := uintptr(wsChild | wsTabStop | esAutoHScroll)
	if e.multiLine {
		style |= esMultiLine | esAutoVScroll
		style &^= esAutoHScroll
	}
	switch e.inputType {
	case platform.InputTypePassword:
		style |= esPassword
	case platform.InputTypeNumber:
		style |= esNumber
	}

	editClass, _ := syscall.UTF16PtrFromString("EDIT")
	hInstance, _, _ := procGetModuleHandleW.Call(0)

	e.hwnd, _, _ = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(editClass)),
		0,
		style,
		uintptr(pt.X), uintptr(pt.Y),
		uintptr(width), uintptr(height),
		e.parentHwnd,
		0,
		hInstance,
		0,
	)
	e.style = style

	// Re-subclass the new control.
	editSubclassMap.Store(e.hwnd, e)
	e.origWndProc, _, _ = procSetWindowLongPtrW.Call(
		e.hwnd, uintptr(0xFFFFFFFFFFFFFFFC),
		syscall.NewCallback(editSubclassProc),
	)

	// Restore state.
	if oldText != "" {
		e.SetText(oldText)
	}
	if e.hFont != 0 {
		procSendMessageW.Call(e.hwnd, wmSetFont, e.hFont, 1)
	}
	if visible != 0 {
		procShowWindow.Call(e.hwnd, SW_SHOW)
	}
}

// Destroy releases all GDI resources and removes the native control.
func (e *win32NativeEdit) Destroy() {
	if e.hwnd != 0 {
		editSubclassMap.Delete(e.hwnd)
	}
	if e.hFont != 0 {
		procDeleteObject.Call(e.hFont)
		e.hFont = 0
	}
	if e.hwnd != 0 {
		procDestroyWindow.Call(e.hwnd)
		e.hwnd = 0
	}
}
