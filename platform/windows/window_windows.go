package windows

import (
	"fmt"
	"image"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/huanfeng/wind-ui/core"
	"github.com/huanfeng/wind-ui/layout"
	"github.com/huanfeng/wind-ui/platform"
	"github.com/huanfeng/wind-ui/render/gg"
)

// Win32 constants
const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_POPUP            = 0x80000000
	WS_EX_LAYERED       = 0x00080000
	WS_EX_TOPMOST       = 0x00000008

	WM_DESTROY     = 0x0002
	WM_SIZE        = 0x0005
	WM_PAINT       = 0x000F
	WM_CLOSE       = 0x0010
	WM_ERASEBKGND  = 0x0014
	WM_KEYDOWN     = 0x0100
	WM_KEYUP       = 0x0101
	WM_MOUSEMOVE   = 0x0200
	WM_LBUTTONDOWN = 0x0201
	WM_LBUTTONUP   = 0x0202
	WM_RBUTTONDOWN = 0x0204
	WM_RBUTTONUP   = 0x0205
	WM_MOUSEWHEEL  = 0x020A
	WM_DPICHANGED  = 0x02E0
	WM_USER        = 0x0400
	WM_TIMER       = 0x0113
	WM_APP_PAINT   = WM_USER + 1

	CS_HREDRAW    = 0x0002
	CS_VREDRAW    = 0x0001
	CS_DBLCLKS    = 0x0008 // 允许窗口接收双击消息（WM_LBUTTONDBLCLK 已在 tray_windows.go 中声明）
	IDC_ARROW     = 32512
	SW_SHOW       = 5
	SW_HIDE       = 0
	CW_USEDEFAULT = ^0x7fffffff
	COLOR_WINDOW  = 5
	SRCCOPY       = 0x00CC0020
	DIB_RGB_COLORS = 0
	BI_RGB         = 0

	SW_MINIMIZE = 6
	SW_MAXIMIZE = 3
	SW_RESTORE  = 9

	SWP_NOMOVE   = 0x0002
	SWP_NOSIZE   = 0x0001
	SWP_NOZORDER = 0x0004

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	GWL_STYLE = -16

	WS_MAXIMIZE = 0x01000000

	HWND_TOPMOST   = ^uintptr(0) // -1 as uintptr
	HWND_NOTOPMOST = ^uintptr(1) // -2 as uintptr
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW  = user32.NewProc("RegisterClassExW")
	procCreateWindowExW   = user32.NewProc("CreateWindowExW")
	procShowWindow        = user32.NewProc("ShowWindow")
	procDestroyWindow     = user32.NewProc("DestroyWindow")
	procDefWindowProcW    = user32.NewProc("DefWindowProcW")
	procGetMessageW       = user32.NewProc("GetMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessageW  = user32.NewProc("DispatchMessageW")
	procPostMessageW      = user32.NewProc("PostMessageW")
	procPostQuitMessage   = user32.NewProc("PostQuitMessage")
	procGetClientRect     = user32.NewProc("GetClientRect")
	procBeginPaint        = user32.NewProc("BeginPaint")
	procEndPaint          = user32.NewProc("EndPaint")
	procGetDC             = user32.NewProc("GetDC")
	procReleaseDC         = user32.NewProc("ReleaseDC")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procGetSystemMetrics  = user32.NewProc("GetSystemMetrics")
	procMoveWindow        = user32.NewProc("MoveWindow")
	procLoadCursorW       = user32.NewProc("LoadCursorW")
	procSetWindowTextW    = user32.NewProc("SetWindowTextW")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procIsWindowVisible   = user32.NewProc("IsWindowVisible")
	procGetWindowRect     = user32.NewProc("GetWindowRect")
	procScreenToClient    = user32.NewProc("ScreenToClient")
	procSetTimer          = user32.NewProc("SetTimer")
	procKillTimer         = user32.NewProc("KillTimer")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procBitBlt             = gdi32.NewProc("BitBlt")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procDeleteObject       = gdi32.NewProc("DeleteObject")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

// WNDCLASSEXW is the Windows WNDCLASSEXW structure.
type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

// MSG is the Windows MSG structure.
type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

// POINT is the Windows POINT structure.
type POINT struct {
	X, Y int32
}

// RECT is the Windows RECT structure.
type RECT struct {
	Left, Top, Right, Bottom int32
}

// PAINTSTRUCT is the Windows PAINTSTRUCT structure.
type PAINTSTRUCT struct {
	HDC         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

// BITMAPINFOHEADER is the Windows BITMAPINFOHEADER structure.
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// BITMAPINFO is the Windows BITMAPINFO structure.
type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}

// windowMap stores HWND -> *win32Window mapping for wndProc lookups.
var windowMap sync.Map

// classRegistered tracks whether the window class has been registered.
var classRegistered bool
var className, _ = syscall.UTF16PtrFromString("WindUIWindowClass")

// win32Window implements platform.Window for Windows.
type win32Window struct {
	hwnd     uintptr
	plat     *WindowsPlatform
	opts     platform.WindowOptions

	contentView  *core.Node
	textRenderer core.TextRenderer
	dpiScale     float64 // DPI scale factor (1.0 at 96 DPI, 1.5 at 144 DPI)
	dpiScaled    bool    // true after node tree has been DPI-scaled

	lastHoverNode   *core.Node // tracks which node the mouse is over for HoverEnter/Exit
	capturedNode    *core.Node // pointer capture: receives all Move/Up events after ActionDown
	focusManager    *core.FocusManager // 键盘焦点管理器

	onClose        func() bool
	onResize       func(w, h int)
	onDPIChanged   func(dpi float64)
	onFocusChanged func(focused bool)

	// Cached rendering buffers — reused across frames, recreated on resize only.
	cachedImage *image.RGBA     // canvas backing buffer (avoids re-alloc per frame)
	dibMemDC    uintptr         // cached memory DC for presentation
	dibBitmap   uintptr         // cached DIB section HBITMAP
	dibBits     unsafe.Pointer  // pointer to DIB pixel data
	dibWidth    int             // cached DIB width
	dibHeight   int             // cached DIB height

	// Animation frame loop (~60fps when active, stopped when idle).
	animTimerID   uintptr
	animators     []*core.ValueAnimator
	lastFrameTime time.Time

	mu sync.Mutex
}

// registerWindowClass registers the Win32 window class once.
func registerWindowClass() error {
	if classRegistered {
		return nil
	}

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	hCursor, _, _ := procLoadCursorW.Call(0, IDC_ARROW)

	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         CS_HREDRAW | CS_VREDRAW | CS_DBLCLKS, // CS_DBLCLKS 使窗口能接收 WM_LBUTTONDBLCLK 消息
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		HCursor:       hCursor,
		HbrBackground: COLOR_WINDOW + 1,
		LpszClassName: className,
	}

	ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		return err
	}
	classRegistered = true
	return nil
}

// newWin32Window creates a new Win32 window.
func newWin32Window(plat *WindowsPlatform, opts platform.WindowOptions) (*win32Window, error) {
	runtime.LockOSThread()

	if err := registerWindowClass(); err != nil {
		return nil, err
	}

	w := &win32Window{
		plat: plat,
		opts: opts,
	}

	hInstance, _, _ := procGetModuleHandleW.Call(0)

	// Determine window styles
	// WS_CLIPCHILDREN excludes child window areas from parent painting,
	// so native EDIT controls aren't covered by our BitBlt.
	style := uintptr(WS_OVERLAPPEDWINDOW | 0x02000000) // WS_CLIPCHILDREN = 0x02000000
	exStyle := uintptr(0)

	if opts.Frameless {
		style = WS_POPUP | WS_VISIBLE
	}
	if opts.TopMost {
		exStyle |= WS_EX_TOPMOST
	}
	if opts.Transparent {
		exStyle |= WS_EX_LAYERED
	}
	if opts.NoActivate {
		exStyle |= 0x08000000 // WS_EX_NOACTIVATE
	}

	// Determine position
	x := CW_USEDEFAULT
	y := CW_USEDEFAULT
	if opts.X != 0 || opts.Y != 0 {
		x = opts.X
		y = opts.Y
	}

	// Determine size — scale dp to physical pixels using system DPI.
	// After DPI awareness is enabled, CreateWindowExW expects physical pixels.
	sysDPI := 96.0
	if procGetDpiForWindow.Find() == nil {
		// Before window exists, use GetDpiForSystem or default
		getDpiForSystem := syscall.NewLazyDLL("user32.dll").NewProc("GetDpiForSystem")
		if getDpiForSystem.Find() == nil {
			d, _, _ := getDpiForSystem.Call()
			if d > 0 {
				sysDPI = float64(d)
			}
		}
	}
	width := opts.Width
	height := opts.Height
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}
	// Scale from dp to physical pixels
	width = int(DpToPx(float64(width), sysDPI))
	height = int(DpToPx(float64(height), sysDPI))

	title, _ := syscall.UTF16PtrFromString(opts.Title)

	hwnd, _, err := procCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		style,
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		0, // parent
		0, // menu
		hInstance,
		0, // lpParam
	)
	if hwnd == 0 {
		return nil, err
	}

	w.hwnd = hwnd
	windowMap.Store(hwnd, w)

	// Enable Win11 rounded corners for frameless windows via DWM
	if opts.Frameless {
		dwmapi := syscall.NewLazyDLL("dwmapi.dll")
		dwmSetWindowAttribute := dwmapi.NewProc("DwmSetWindowAttribute")
		if dwmSetWindowAttribute.Find() == nil {
			// DWMWA_WINDOW_CORNER_PREFERENCE = 33, DWMWCP_ROUND = 2
			cornerPref := int32(2)
			dwmSetWindowAttribute.Call(hwnd, 33, uintptr(unsafe.Pointer(&cornerPref)), 4)
		}
	}

	return w, nil
}

// wndProc is the Win32 window procedure callback.
func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	val, ok := windowMap.Load(hwnd)
	if !ok {
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	}
	w := val.(*win32Window)

	switch msg {
	case WM_CLOSE:
		if w.onClose != nil {
			if !w.onClose() {
				return 0 // Prevent close
			}
		}
		procDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		w.releaseCachedDIB()
		w.cachedImage = nil
		windowMap.Delete(hwnd)
		// Post WM_QUIT to exit the message loop
		procPostQuitMessage.Call(0)
		return 0

	case WM_DPICHANGED:
		// wParam: LOWORD = 新 X DPI，HIWORD = 新 Y DPI
		newDPI := float64(int16(wParam & 0xFFFF))
		newScale := newDPI / 96.0
		if newScale > 0 && newScale != w.dpiScale {
			w.dpiScale = newScale
			// 使用 core.RescaleNodeDPI 重新缩放节点树（从原始 dp 值计算，避免浮点累积误差）
			if w.contentView != nil {
				core.RescaleNodeDPI(w.contentView, newScale)
			}
			// lParam points to a RECT with the suggested new window position/size
			if lParam != 0 {
				suggestedRect := (*RECT)(unsafe.Pointer(lParam))
				procSetWindowPos.Call(hwnd, 0,
					uintptr(suggestedRect.Left), uintptr(suggestedRect.Top),
					uintptr(suggestedRect.Right-suggestedRect.Left),
					uintptr(suggestedRect.Bottom-suggestedRect.Top),
					SWP_NOZORDER)
			}
			if w.onDPIChanged != nil {
				w.onDPIChanged(newDPI)
			}
		}
		return 0

	case WM_SIZE:
		width := int(lParam & 0xFFFF)
		height := int((lParam >> 16) & 0xFFFF)
		if w.onResize != nil {
			w.onResize(width, height)
		}
		w.render()
		return 0

	case WM_PAINT:
		var ps PAINTSTRUCT
		procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		w.render()
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0

	case WM_APP_PAINT:
		w.render()
		return 0

	case WM_TIMER:
		w.tickAnimations()
		return 0

	case WM_ERASEBKGND:
		return 1 // We handle background painting ourselves

	case WM_KEYDOWN:
		vk := int(wParam)
		w.dispatchKey(core.ActionKeyDown, vk)
		return 0

	case WM_KEYUP:
		vk := int(wParam)
		w.dispatchKey(core.ActionKeyUp, vk)
		return 0

	case WM_MOUSEMOVE:
		x := int(int16(lParam & 0xFFFF))
		y := int(int16((lParam >> 16) & 0xFFFF))
		// MK_LBUTTON (0x0001) indicates left button is held during move
		if wParam&0x0001 != 0 {
			w.dispatchMotion(core.ActionMove, float64(x), float64(y), core.MouseButtonLeft)
		} else {
			w.dispatchMotion(core.ActionHoverMove, float64(x), float64(y), core.MouseButtonLeft)
		}
		return 0

	case WM_LBUTTONDOWN:
		x := int(int16(lParam & 0xFFFF))
		y := int(int16((lParam >> 16) & 0xFFFF))
		w.dispatchMotion(core.ActionDown, float64(x), float64(y), core.MouseButtonLeft)
		return 0

	case WM_LBUTTONUP:
		x := int(int16(lParam & 0xFFFF))
		y := int(int16((lParam >> 16) & 0xFFFF))
		w.dispatchMotion(core.ActionUp, float64(x), float64(y), core.MouseButtonLeft)
		return 0

	case WM_LBUTTONDBLCLK:
		// 鼠标左键双击：发送 ActionDoubleClick 事件
		x := int(int16(lParam & 0xFFFF))
		y := int(int16((lParam >> 16) & 0xFFFF))
		w.dispatchMotion(core.ActionDoubleClick, float64(x), float64(y), core.MouseButtonLeft)
		return 0

	case WM_RBUTTONDOWN:
		x := int(int16(lParam & 0xFFFF))
		y := int(int16((lParam >> 16) & 0xFFFF))
		w.dispatchMotion(core.ActionDown, float64(x), float64(y), core.MouseButtonRight)
		return 0

	case WM_RBUTTONUP:
		x := int(int16(lParam & 0xFFFF))
		y := int(int16((lParam >> 16) & 0xFFFF))
		w.dispatchMotion(core.ActionUp, float64(x), float64(y), core.MouseButtonRight)
		// 右键抬起时额外发送 ActionRightClick，方便 widget handler 直接处理上下文菜单
		w.dispatchMotion(core.ActionRightClick, float64(x), float64(y), core.MouseButtonRight)
		return 0

	case WM_MOUSEWHEEL:
		// wParam high word is wheel delta (positive = scroll up, negative = scroll down)
		delta := int16(wParam >> 16)
		// WHEEL_DELTA is 120; normalize to notch count
		notches := float64(delta) / 120.0
		// lParam contains SCREEN coordinates — convert to client coordinates
		pt := POINT{X: int32(int16(lParam & 0xFFFF)), Y: int32(int16((lParam >> 16) & 0xFFFF))}
		procScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
		w.dispatchScroll(float64(pt.X), float64(pt.Y), 0, notches)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

// dispatchMotion creates and dispatches a MotionEvent through the node tree
// using the 3-phase dispatch (hit-test → intercept → handle+bubble).
func (w *win32Window) dispatchMotion(action core.MotionAction, x, y float64, button core.MouseButton) {
	if w.contentView == nil {
		return
	}

	// Hover tracking: send HoverEnter/HoverExit when the deepest hit node changes.
	// HoverExit bubbles UP to all ancestors so parent containers (ScrollView etc.)
	// can clear their hover state when the mouse leaves their bounds.
	if action == core.ActionHoverMove {
		hitPoint := core.Point{X: x, Y: y}
		currentTarget := core.HitTest(w.contentView, hitPoint)
		if currentTarget != w.lastHoverNode {
			needRepaint := false
			// Send HoverExit to old target AND its ancestors
			if w.lastHoverNode != nil {
				exitEvt := core.NewMotionEvent(core.ActionHoverExit, x, y)
				for node := w.lastHoverNode; node != nil; node = node.Parent() {
					if h := node.GetHandler(); h != nil {
						if h.OnEvent(node, exitEvt) {
							needRepaint = true
						}
					}
				}
			}
			// Send HoverEnter to new target only (not ancestors)
			if currentTarget != nil {
				enterEvt := core.NewMotionEvent(core.ActionHoverEnter, x, y)
				if h := currentTarget.GetHandler(); h != nil {
					if h.OnEvent(currentTarget, enterEvt) {
						needRepaint = true
					}
				}
			}
			w.lastHoverNode = currentTarget
			if needRepaint {
				w.Invalidate()
			}
		}
	}

	// Pointer capture: after ActionDown, subsequent Move/Up go to the node
	// that consumed ActionDown, so drag works even outside the original view.
	if (action == core.ActionMove || action == core.ActionUp) && w.capturedNode != nil {
		evt := core.NewMotionEvent(action, x, y)
		evt.Button = button
		evt.RawX = x
		evt.RawY = y
		consumed := false
		if h := w.capturedNode.GetHandler(); h != nil {
			consumed = h.OnEvent(w.capturedNode, evt)
		}
		if action == core.ActionUp {
			w.capturedNode = nil
		}
		if consumed {
			w.Invalidate()
		}
		return
	}

	evt := core.NewMotionEvent(action, x, y)
	evt.Button = button
	evt.RawX = x
	evt.RawY = y

	// Dispatch through the normal 3-phase event system.
	// For ActionDown, capture the consuming node for subsequent Move/Up.
	if action == core.ActionDown {
		consumer, consumed := core.DispatchEventCapture(w.contentView, evt, core.Point{X: x, Y: y})
		w.capturedNode = consumer
		if consumed {
			w.Invalidate()
		}
	} else {
		consumed := core.DispatchEvent(w.contentView, evt, core.Point{X: x, Y: y})
		if consumed {
			w.Invalidate()
		}
	}
}

// getKeyState 返回指定虚拟键的当前状态。
// 当返回值最高位为 1 时，表示该键处于按下状态。
func getKeyState(vk int) bool {
	// 确保 procGetKeyState 已初始化（由 initNativeEditProcs 懒加载）
	initNativeEditProcs()
	ret, _, _ := procGetKeyState.Call(uintptr(vk))
	return (ret & 0x8000) != 0
}

// dispatchKey 处理键盘事件分发。
// Tab/Shift+Tab 触发焦点导航，其他键分发到当前焦点节点。
func (w *win32Window) dispatchKey(action core.KeyAction, vkCode int) {
	if w.contentView == nil {
		return
	}

	// Tab 键（VK_TAB = 0x09）处理焦点导航
	if vkCode == 0x09 && action == core.ActionKeyDown {
		if w.focusManager != nil {
			// 检查 Shift 键（VK_SHIFT = 0x10）判断导航方向
			shift := getKeyState(0x10)
			w.focusManager.MoveFocus(w.contentView, !shift)
			w.contentView.MarkDirty()
			w.postAppPaint()
		}
		return
	}

	// 其他键分发到当前焦点节点
	evt := core.NewKeyEvent(action, vkCode)
	if w.focusManager != nil {
		focused := w.focusManager.Current()
		if focused != nil {
			if h := focused.GetHandler(); h != nil {
				h.OnEvent(focused, evt)
			}
		}
	}
}

// postAppPaint 向消息队列投递一个 WM_APP_PAINT 消息触发重绘。
func (w *win32Window) postAppPaint() {
	procPostMessageW.Call(w.hwnd, WM_APP_PAINT, 0, 0)
}

// GetFocusManager 返回窗口的焦点管理器，供外部访问当前焦点节点。
func (w *win32Window) GetFocusManager() *core.FocusManager {
	return w.focusManager
}

// dispatchScroll creates and dispatches a ScrollEvent through the node tree.
func (w *win32Window) dispatchScroll(x, y, deltaX, deltaY float64) {
	if w.contentView == nil {
		return
	}
	evt := core.NewScrollEvent(x, y, deltaX, deltaY)
	consumed := core.DispatchEvent(w.contentView, evt, core.Point{X: x, Y: y})
	if consumed {
		w.Invalidate() // repaint after scroll
	}
}

// ---------- platform.Window implementation ----------

func (w *win32Window) SetContentView(root *core.Node) {
	w.mu.Lock()
	w.contentView = root
	w.mu.Unlock()
	root.SetInvalidator(w)
	w.Invalidate()
}

func (w *win32Window) SetTitle(title string) {
	t, _ := syscall.UTF16PtrFromString(title)
	procSetWindowTextW.Call(w.hwnd, uintptr(unsafe.Pointer(t)))
}

func (w *win32Window) SetIcon(icon *core.ImageResource) {
	// Phase 1: stub — setting window icon requires CreateIconFromResourceEx
}

func (w *win32Window) Show() {
	procShowWindow.Call(w.hwnd, SW_SHOW)
	// Force immediate render after showing (ShowWindow triggers WM_PAINT,
	// but we also render explicitly to ensure content is visible immediately)
	w.render()
}

func (w *win32Window) Hide() {
	procShowWindow.Call(w.hwnd, SW_HIDE)
}

func (w *win32Window) Close() {
	procDestroyWindow.Call(w.hwnd)
}

func (w *win32Window) Minimize() {
	procShowWindow.Call(w.hwnd, SW_MINIMIZE)
}

func (w *win32Window) Maximize() {
	procShowWindow.Call(w.hwnd, SW_MAXIMIZE)
}

func (w *win32Window) Restore() {
	procShowWindow.Call(w.hwnd, SW_RESTORE)
}

func (w *win32Window) SetSize(width, height int) {
	procSetWindowPos.Call(w.hwnd, 0,
		0, 0, uintptr(width), uintptr(height),
		SWP_NOMOVE|SWP_NOZORDER)
}

func (w *win32Window) SetPosition(x, y int) {
	procSetWindowPos.Call(w.hwnd, 0,
		uintptr(x), uintptr(y), 0, 0,
		SWP_NOSIZE|SWP_NOZORDER)
}

func (w *win32Window) Center() {
	width, height := w.getWindowSize()
	screenW, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	screenH, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	x := (int(screenW) - width) / 2
	y := (int(screenH) - height) / 2
	w.SetPosition(x, y)
}

func (w *win32Window) IsVisible() bool {
	ret, _, _ := procIsWindowVisible.Call(w.hwnd)
	return ret != 0
}

func (w *win32Window) IsFocused() bool {
	fg, _, _ := procGetForegroundWindow.Call()
	return fg == w.hwnd
}

func (w *win32Window) GetSize() core.Size {
	width, height := w.getClientSize()
	return core.Size{Width: float64(width), Height: float64(height)}
}

func (w *win32Window) GetPosition() core.Point {
	var r RECT
	procGetWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
	return core.Point{X: float64(r.Left), Y: float64(r.Top)}
}

var procGetDpiForWindow *syscall.LazyProc

func init() {
	procGetDpiForWindow = syscall.NewLazyDLL("user32.dll").NewProc("GetDpiForWindow")
}

func (w *win32Window) GetDPI() float64 {
	if procGetDpiForWindow.Find() == nil {
		dpi, _, _ := procGetDpiForWindow.Call(w.hwnd)
		if dpi > 0 {
			return float64(dpi)
		}
	}
	return 96.0
}

func (w *win32Window) SetOnClose(fn func() bool) {
	w.onClose = fn
}

func (w *win32Window) SetOnResize(fn func(w, h int)) {
	w.onResize = fn
}

func (w *win32Window) SetOnDPIChanged(fn func(dpi float64)) {
	w.onDPIChanged = fn
}

func (w *win32Window) SetOnFocusChanged(fn func(focused bool)) {
	w.onFocusChanged = fn
}

func (w *win32Window) NativeHandle() uintptr {
	return w.hwnd
}

func (w *win32Window) Invalidate() {
	procPostMessageW.Call(w.hwnd, WM_APP_PAINT, 0, 0)
}

func (w *win32Window) InvalidateRect(rect core.Rect) {
	// Phase 1: invalidate entire window
	w.Invalidate()
}

// StartAnimator registers an animation with the window's frame loop.
// The animation is automatically ticked at ~60fps and removed when complete.
func (w *win32Window) StartAnimator(anim *core.ValueAnimator) {
	if anim == nil {
		return
	}
	anim.Start()
	w.animators = append(w.animators, anim)
	w.ensureFrameTimer()
}

// RequestFrame ensures at least one more frame tick will occur.
func (w *win32Window) RequestFrame() {
	w.ensureFrameTimer()
}

// ensureFrameTimer starts the ~60fps WM_TIMER if not already running.
func (w *win32Window) ensureFrameTimer() {
	if w.animTimerID != 0 {
		return
	}
	// SetTimer(hwnd, nIDEvent=1, uElapse=16ms, lpTimerFunc=NULL)
	ret, _, _ := procSetTimer.Call(w.hwnd, 1, 16, 0)
	if ret != 0 {
		w.animTimerID = ret
		w.lastFrameTime = time.Now()
	}
}

// stopFrameTimer stops the animation timer.
func (w *win32Window) stopFrameTimer() {
	if w.animTimerID != 0 {
		procKillTimer.Call(w.hwnd, 1)
		w.animTimerID = 0
	}
}

// tickAnimations advances all active animators and triggers repaint.
func (w *win32Window) tickAnimations() {
	now := time.Now()
	elapsed := now.Sub(w.lastFrameTime)
	w.lastFrameTime = now

	anyRunning := false
	n := 0
	for _, anim := range w.animators {
		if anim.Tick(elapsed) {
			w.animators[n] = anim
			n++
			anyRunning = true
		}
	}
	for i := n; i < len(w.animators); i++ {
		w.animators[i] = nil
	}
	w.animators = w.animators[:n]

	if !anyRunning {
		w.stopFrameTimer()
	}

	// Animation callbacks call node.Invalidate() which registers dirty rects.
	w.render()
}

// ---------- Render pipeline ----------

func (w *win32Window) render() {
	w.mu.Lock()
	contentView := w.contentView
	w.mu.Unlock()

	if contentView == nil {
		return
	}

	width, height := w.getClientSize()
	if width <= 0 || height <= 0 {
		return
	}

	// Update DPI scale factor
	w.dpiScale = w.GetDPI() / 96.0
	if w.dpiScale < 1.0 {
		w.dpiScale = 1.0
	}

	// Scale dp values in the node tree to physical pixels (once).
	if !w.dpiScaled {
		core.ScaleNodeDPI(contentView, w.dpiScale)
		w.dpiScaled = true
	}

	// Create canvas with cached text renderer
	if w.textRenderer == nil {
		w.textRenderer = w.plat.CreateTextRenderer()
	}

	// 初始化焦点管理器（仅一次）
	if w.focusManager == nil {
		w.focusManager = core.NewFocusManager()
	}

	root := contentView

	// Expose TextMeasurer on the root node so Painter.Measure() can
	// perform accurate text measurement during the layout pass.
	if core.GetTextMeasurer(root) == nil {
		root.SetData("textMeasurer", core.NewTextMeasurer(w.textRenderer))
	}

	// Determine if we need full or partial repaint.
	sizeChanged := w.cachedImage == nil ||
		w.cachedImage.Bounds().Dx() != width ||
		w.cachedImage.Bounds().Dy() != height

	dirtyRects, dirtyFull := root.PopDirtyRegion()
	fullRepaint := dirtyFull || sizeChanged

	// No dirty regions and no size change — skip rendering entirely.
	if !fullRepaint && len(dirtyRects) == 0 {
		return
	}

	// Layout pass: only run Measure + Arrange if layout is dirty
	// (structural/sizing changes). Pure visual changes (hover, color) skip this.
	if fullRepaint || root.IsLayoutDirty() {
		widthSpec := core.MeasureSpec{Mode: core.MeasureModeExact, Size: float64(width)}
		heightSpec := core.MeasureSpec{Mode: core.MeasureModeExact, Size: float64(height)}
		layout.MeasureChild(root, widthSpec, heightSpec)
		if l := root.GetLayout(); l != nil {
			l.Arrange(root, core.Rect{Width: float64(width), Height: float64(height)})
		}
		root.SetBounds(core.Rect{Width: float64(width), Height: float64(height)})
		clearLayoutDirty(root)
	}

	if fullRepaint {
		// --- Full repaint path (resize, first frame, large dirty area) ---
		var canvas *gg.GGCanvas
		if sizeChanged {
			fmt.Printf("[WindUI] canvas resized: %dx%d (%.1f MB RGBA)\n",
				width, height, float64(width*height*4)/(1024*1024))
			canvas = gg.NewGGCanvas(width, height, w.textRenderer)
		} else {
			canvas = gg.NewGGCanvasForImage(w.cachedImage, w.textRenderer)
		}
		PaintNode(root, canvas)
		w.cachedImage = canvas.Target()
		w.present(w.cachedImage)
	} else {
		// --- Partial repaint path (dirty regions only) ---
		canvas := gg.NewGGCanvasRetained(w.cachedImage, w.textRenderer)
		for _, r := range dirtyRects {
			canvas.ClearRect(r)
		}
		PaintNodeDirty(root, canvas, dirtyRects, 0, 0)
		w.cachedImage = canvas.Target()
		w.presentDirty(w.cachedImage, dirtyRects)
	}
}


// PaintNode 递归绘制节点树到画布上（含 overlay 处理）。
// 实现已统一到 core.PaintNode，此处保留为兼容性包装。
func PaintNode(node *core.Node, canvas core.Canvas) {
	core.PaintNode(node, canvas)
}

// PaintNodeDirty repaints all visible nodes that spatially intersect the dirty
// regions. The dirty flags (IsDirty/IsChildDirty) are NOT used for culling —
// every node overlapping a dirty rect must be repainted because something else
// in that area may have changed (e.g. a dialog closed, revealing background).
// Culling is purely spatial: nodes entirely outside all dirty rects are skipped.
func PaintNodeDirty(node *core.Node, canvas core.Canvas, dirtyRects []core.Rect, ox, oy float64) {
	if node.GetVisibility() != core.Visible {
		return
	}

	b := node.Bounds()
	absRect := core.Rect{X: ox + b.X, Y: oy + b.Y, Width: b.Width, Height: b.Height}

	// Spatial culling: skip subtree if entirely outside all dirty rects.
	if !rectIntersectsAny(absRect, dirtyRects) {
		return
	}

	canvas.Save()
	canvas.Translate(b.X, b.Y)

	if p := node.GetPainter(); p != nil {
		p.Paint(node, canvas)
	}

	if node.GetData("paintsChildren") == nil {
		childOX, childOY := ox+b.X, oy+b.Y
		var overlays []*core.Node
		for _, child := range node.Children() {
			if child.GetData("isOverlay") != nil {
				overlays = append(overlays, child)
				continue
			}
			PaintNodeDirty(child, canvas, dirtyRects, childOX, childOY)
		}
		// Paint overlays last, at full parent bounds (on top of all content).
		for _, overlay := range overlays {
			overlay.SetBounds(core.Rect{X: 0, Y: 0, Width: b.Width, Height: b.Height})
			overlay.SetMeasuredSize(core.Size{Width: b.Width, Height: b.Height})
			PaintNodeDirty(overlay, canvas, dirtyRects, childOX, childOY)
		}
	}

	canvas.Restore()
	node.ClearDirty()
}

// clearLayoutDirty recursively clears layoutDirty flags after a layout pass.
func clearLayoutDirty(node *core.Node) {
	node.ClearLayoutDirty()
	for _, child := range node.Children() {
		if child.IsLayoutDirty() {
			clearLayoutDirty(child)
		}
	}
}

// rectIntersectsAny reports whether rect overlaps with any of the dirty rects.
func rectIntersectsAny(rect core.Rect, dirtyRects []core.Rect) bool {
	for _, dr := range dirtyRects {
		if rect.Overlaps(dr) {
			return true
		}
	}
	return false
}

// present copies the RGBA image to the window using GDI.
// The memory DC and DIB section are cached and reused across frames;
// they are only recreated when the window size changes.
func (w *win32Window) present(img *image.RGBA) {
	if img == nil {
		return
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	hdc, _, _ := procGetDC.Call(w.hwnd)
	if hdc == 0 {
		return
	}
	defer procReleaseDC.Call(w.hwnd, hdc)

	// Ensure cached DIB section matches the current dimensions.
	if w.dibWidth != width || w.dibHeight != height {
		fmt.Printf("[WindUI] DIB resized: %dx%d (%.1f MB BGRA)\n",
			width, height, float64(width*height*4)/(1024*1024))
		w.releaseCachedDIB()

		memDC, _, _ := procCreateCompatibleDC.Call(hdc)
		if memDC == 0 {
			return
		}

		bmi := BITMAPINFO{
			BmiHeader: BITMAPINFOHEADER{
				BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
				BiWidth:       int32(width),
				BiHeight:      -int32(height), // top-down
				BiPlanes:      1,
				BiBitCount:    32,
				BiCompression: BI_RGB,
			},
		}

		var bits unsafe.Pointer
		hBitmap, _, _ := procCreateDIBSection.Call(
			memDC,
			uintptr(unsafe.Pointer(&bmi)),
			DIB_RGB_COLORS,
			uintptr(unsafe.Pointer(&bits)),
			0,
			0,
		)
		if hBitmap == 0 {
			procDeleteDC.Call(memDC)
			return
		}

		procSelectObject.Call(memDC, hBitmap)

		w.dibMemDC = memDC
		w.dibBitmap = hBitmap
		w.dibBits = bits
		w.dibWidth = width
		w.dibHeight = height
	}

	// Full-frame RGBA -> BGRA copy + BitBlt
	copyRGBAtoBGRA(img, w.dibBits, w.dibWidth, 0, 0, width, height)

	procBitBlt.Call(
		hdc,
		0, 0,
		uintptr(width), uintptr(height),
		w.dibMemDC,
		0, 0,
		SRCCOPY,
	)
}

// presentDirty copies only the dirty rectangles from the RGBA image to the
// window, avoiding a full-frame pixel copy and BitBlt.
func (w *win32Window) presentDirty(img *image.RGBA, dirtyRects []core.Rect) {
	if img == nil || w.dibBits == nil {
		// Fallback to full present if DIB not ready.
		w.present(img)
		return
	}

	hdc, _, _ := procGetDC.Call(w.hwnd)
	if hdc == 0 {
		return
	}
	defer procReleaseDC.Call(w.hwnd, hdc)

	for _, r := range dirtyRects {
		x0 := max(int(r.X), 0)
		y0 := max(int(r.Y), 0)
		x1 := min(int(r.X+r.Width+0.5), w.dibWidth)
		y1 := min(int(r.Y+r.Height+0.5), w.dibHeight)
		if x0 >= x1 || y0 >= y1 {
			continue
		}

		copyRGBAtoBGRA(img, w.dibBits, w.dibWidth, x0, y0, x1, y1)

		rw := x1 - x0
		rh := y1 - y0
		procBitBlt.Call(
			hdc,
			uintptr(x0), uintptr(y0),
			uintptr(rw), uintptr(rh),
			w.dibMemDC,
			uintptr(x0), uintptr(y0),
			SRCCOPY,
		)
	}
}

// copyRGBAtoBGRA copies a rectangular region from an RGBA image to a BGRA
// DIB buffer, swapping R and B channels.
func copyRGBAtoBGRA(src *image.RGBA, dibBits unsafe.Pointer, dibW, x0, y0, x1, y1 int) {
	pix := src.Pix
	stride := src.Stride
	dibStride := dibW * 4
	totalDIBBytes := dibW * src.Bounds().Dy() * 4
	dst := unsafe.Slice((*byte)(dibBits), totalDIBBytes)
	rowPixels := x1 - x0

	for y := y0; y < y1; y++ {
		srcOff := y*stride + x0*4
		dstOff := y*dibStride + x0*4

		// Batch process: read/write 4 bytes at a time via uint32.
		// On little-endian x64, RGBA bytes [R,G,B,A] = uint32(A<<24|B<<16|G<<8|R).
		// BGRA bytes [B,G,R,A] = swap bits 0-7 (R) with bits 16-23 (B).
		srcU32 := unsafe.Slice((*uint32)(unsafe.Pointer(&pix[srcOff])), rowPixels)
		dstU32 := unsafe.Slice((*uint32)(unsafe.Pointer(&dst[dstOff])), rowPixels)
		for i := 0; i < rowPixels; i++ {
			v := srcU32[i]
			// Swap R and B channels, keep G and A in place.
			dstU32[i] = (v & 0xFF00FF00) | ((v & 0xFF) << 16) | ((v >> 16) & 0xFF)
		}
	}
}

// releaseCachedDIB frees the cached GDI resources (memory DC + DIB section).
func (w *win32Window) releaseCachedDIB() {
	if w.dibBitmap != 0 {
		procDeleteObject.Call(w.dibBitmap)
		w.dibBitmap = 0
	}
	if w.dibMemDC != 0 {
		procDeleteDC.Call(w.dibMemDC)
		w.dibMemDC = 0
	}
	w.dibBits = nil
	w.dibWidth = 0
	w.dibHeight = 0
}

// ---------- Helpers ----------

// getClientSize returns the client area dimensions.
func (w *win32Window) getClientSize() (int, int) {
	var r RECT
	procGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
	return int(r.Right - r.Left), int(r.Bottom - r.Top)
}

// getWindowSize returns the full window dimensions (including frame).
func (w *win32Window) getWindowSize() (int, int) {
	var r RECT
	procGetWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
	return int(r.Right - r.Left), int(r.Bottom - r.Top)
}
