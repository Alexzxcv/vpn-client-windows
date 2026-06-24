//go:build windows

package main

import (
	"log/slog"
	"runtime"
	"sync"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

// hasUI reports whether a native UI window can be opened.
func hasUI() bool { return true }

// --- Win32 plumbing (kept local; go-webview2's w32 helpers are internal) ---

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procGetWindowLongPtrW   = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW   = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procShowWindow          = user32.NewProc("ShowWindow")
	procIsZoomed            = user32.NewProc("IsZoomed")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procCreateIconFromResEx = user32.NewProc("CreateIconFromResourceEx")
	procLookupIconIdFromDir = user32.NewProc("LookupIconIdFromDirectoryEx")
)

// gwlStyle is GWL_STYLE (-16). Kept as a typed var so it can be converted to
// uintptr at runtime (a negative constant cannot be converted directly).
var gwlStyle = int32(-16)

const (
	wsCaption     = 0x00C00000
	wsThickFrame  = 0x00040000
	wsMinimizeBox = 0x00020000
	wsMaximizeBox = 0x00010000
	wsSysMenu     = 0x00080000

	swpNoMove     = 0x0002
	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	swpFrameChng  = 0x0020

	swMinimize = 6
	swRestore  = 9

	wmClose         = 0x0010
	wmSetIcon       = 0x0080
	wmNCLButtonDown = 0x00A1
	wmSysCommand    = 0x0112

	scMaximize = 0xF030
	scRestore  = 0xF120

	htCaption = 2

	iconSmall = 0
	iconBig   = 1
)

// makeFrameless strips the system title bar/caption while keeping a thin native
// sizing frame (WS_THICKFRAME) so the window can still be resized and minimized
// by the OS. The React UI draws its own title bar on top. We deliberately do NOT
// touch WM_NCCALCSIZE/WM_NCHITTEST (those live in go-webview2's fixed wndproc),
// so dragging-resize and close stay handled by the OS — the reliable path.
func makeFrameless(hwnd uintptr) {
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlStyle))
	style &^= uintptr(wsCaption)
	style |= uintptr(wsThickFrame | wsMinimizeBox | wsMaximizeBox | wsSysMenu)
	procSetWindowLongPtrW.Call(hwnd, uintptr(gwlStyle), style)
	// Apply the new frame and let WebView2 re-fill the (now larger) client area.
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		uintptr(swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChng))
}

// setWindowIconFromICO loads an HICON from in-memory .ico bytes and assigns it as
// the window's big/small icon (taskbar + Alt-Tab).
func setWindowIconFromICO(hwnd uintptr, ico []byte, cx, cy int, big bool) {
	if len(ico) == 0 {
		return
	}
	// Find the best-matching directory entry for the requested size.
	id, _, _ := procLookupIconIdFromDir.Call(
		uintptr(unsafe.Pointer(&ico[0])),
		1, // fIcon = TRUE
		uintptr(int32(cx)),
		uintptr(int32(cy)),
		0, // LR_DEFAULTCOLOR
	)
	if id == 0 || int(id) >= len(ico) {
		return
	}
	h, _, _ := procCreateIconFromResEx.Call(
		uintptr(unsafe.Pointer(&ico[int(id)])),
		uintptr(len(ico))-id,
		1, // fIcon = TRUE
		0x00030000,
		uintptr(int32(cx)),
		uintptr(int32(cy)),
		0, // LR_DEFAULTCOLOR
	)
	if h == 0 {
		return
	}
	which := uintptr(iconSmall)
	if big {
		which = uintptr(iconBig)
	}
	procSendMessageW.Call(hwnd, wmSetIcon, which, h)
}

// windowManager owns the lifecycle of the WebView2 window. The window can be
// closed (minimize-to-tray) and re-opened from the tray without exiting the
// process. Only one window exists at a time.
type windowManager struct {
	log   *slog.Logger
	title string
	url   string

	mu   sync.Mutex
	wv   webview2.WebView
	hwnd uintptr
	open bool
}

func newWindowManager(log *slog.Logger, title, url string) *windowManager {
	return &windowManager{log: log, title: title, url: url}
}

// Open shows the window. If one is already open it is a no-op (the existing
// window stays). Open blocks on its own locked OS thread until the window is
// closed; callers run it in a dedicated goroutine.
func (m *windowManager) Open() {
	m.mu.Lock()
	if m.open {
		m.mu.Unlock()
		return
	}
	m.open = true
	m.mu.Unlock()

	// WebView2 requires its message loop on a single OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug: false,
		WindowOptions: webview2.WindowOptions{
			Title:  m.title,
			Width:  440,
			Height: 760,
			IconId: 0,
			Center: true,
		},
	})
	if w == nil {
		m.log.Error("failed to create WebView2 window (is the WebView2 runtime installed?)")
		m.mu.Lock()
		m.open = false
		m.mu.Unlock()
		return
	}

	hwnd := uintptr(w.Window())

	// Frameless: drop the system caption, keep a thin native sizing frame.
	makeFrameless(hwnd)
	// Brand icon (taskbar / Alt-Tab) from the embedded SAPN .ico.
	setWindowIconFromICO(hwnd, appIcon, 16, 16, false)
	setWindowIconFromICO(hwnd, appIcon, 32, 32, true)

	// Native bridge for the React title bar:
	//  - windowMinimize / windowMaximize / windowClose: window controls.
	//  - windowStartDrag: begin an OS move-drag from the custom title bar
	//    (-webkit-app-region is not supported in WebView2, so we use the
	//    classic ReleaseCapture + WM_NCLBUTTONDOWN(HTCAPTION) trick).
	_ = w.Bind("windowMinimize", func() { m.Minimize() })
	_ = w.Bind("windowMaximize", func() { m.ToggleMaximize() })
	_ = w.Bind("windowClose", func() { m.Close() })
	_ = w.Bind("windowStartDrag", func() {
		procReleaseCapture.Call()
		procSendMessageW.Call(hwnd, wmNCLButtonDown, htCaption, 0)
	})

	m.mu.Lock()
	m.wv = w
	m.hwnd = hwnd
	m.mu.Unlock()

	w.Navigate(m.url)
	w.Run() // blocks until the window is closed

	w.Destroy()
	m.mu.Lock()
	m.wv = nil
	m.hwnd = 0
	m.open = false
	m.mu.Unlock()
}

// Close hides the window to the tray (the process keeps running). It posts
// WM_CLOSE to the window's own thread, which is cross-thread safe — unlike
// webview.Terminate(), which posts the quit to the CALLING goroutine's message
// queue (the HTTP handler), so the window loop never saw it and the X did
// nothing. WM_CLOSE -> DestroyWindow -> Run() returns; the tray stays.
func (m *windowManager) Close() {
	if h := m.handle(); h != 0 {
		procPostMessageW.Call(h, wmClose, 0, 0)
	}
}

// Minimize minimizes the window. Safe to call cross-thread (ShowWindow posts to
// the window's own thread).
func (m *windowManager) Minimize() {
	if h := m.handle(); h != 0 {
		procShowWindow.Call(h, swMinimize)
	}
}

// ToggleMaximize maximizes the window, or restores it if already maximized.
func (m *windowManager) ToggleMaximize() {
	h := m.handle()
	if h == 0 {
		return
	}
	zoomed, _, _ := procIsZoomed.Call(h)
	cmd := uintptr(scMaximize)
	if zoomed != 0 {
		cmd = scRestore
	}
	procPostMessageW.Call(h, wmSysCommand, cmd, 0)
}

func (m *windowManager) handle() uintptr {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hwnd
}

// IsOpen reports whether a window is currently shown.
func (m *windowManager) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.open
}
