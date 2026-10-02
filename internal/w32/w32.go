//go:build windows && !nogui

package w32

import (
	"runtime/debug"
	"syscall"
	"unsafe"
)

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modGdi32    = syscall.NewLazyDLL("gdi32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modShell32  = syscall.NewLazyDLL("shell32.dll")
	modMsimg32  = syscall.NewLazyDLL("msimg32.dll")
	modDwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	modAdvapi32 = syscall.NewLazyDLL("advapi32.dll")
	modComdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	modWinmm    = syscall.NewLazyDLL("winmm.dll")
	modOle32    = syscall.NewLazyDLL("ole32.dll")
)

var (
	procRegisterClassExW    = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW     = modUser32.NewProc("CreateWindowExW")
	procDefWindowProcW      = modUser32.NewProc("DefWindowProcW")
	procGetMessageW         = modUser32.NewProc("GetMessageW")
	procTranslateMessage    = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW    = modUser32.NewProc("DispatchMessageW")
	procPostQuitMessage     = modUser32.NewProc("PostQuitMessage")
	procPostMessageW        = modUser32.NewProc("PostMessageW")
	procSendMessageW        = modUser32.NewProc("SendMessageW")
	procShowWindow          = modUser32.NewProc("ShowWindow")
	procUpdateWindow        = modUser32.NewProc("UpdateWindow")
	procDestroyWindow       = modUser32.NewProc("DestroyWindow")
	procInvalidateRect      = modUser32.NewProc("InvalidateRect")
	procGetClientRect       = modUser32.NewProc("GetClientRect")
	procGetWindowRect       = modUser32.NewProc("GetWindowRect")
	procClientToScreen      = modUser32.NewProc("ClientToScreen")
	procScreenToClient      = modUser32.NewProc("ScreenToClient")
	procSetWindowPos        = modUser32.NewProc("SetWindowPos")
	procSetForegroundWindow = modUser32.NewProc("SetForegroundWindow")
	procIsWindowVisible     = modUser32.NewProc("IsWindowVisible")
	procIsIconic            = modUser32.NewProc("IsIconic")
	procEnableWindow        = modUser32.NewProc("EnableWindow")
	procLoadCursorW         = modUser32.NewProc("LoadCursorW")
	procLoadIconW           = modUser32.NewProc("LoadIconW")
	procSetCursor           = modUser32.NewProc("SetCursor")
	procTrackMouseEvent     = modUser32.NewProc("TrackMouseEvent")
	procSetTimer            = modUser32.NewProc("SetTimer")
	procKillTimer           = modUser32.NewProc("KillTimer")
	procGetMessagePos       = modUser32.NewProc("GetMessagePos")
	procSetWindowTextW      = modUser32.NewProc("SetWindowTextW")
	procGetWindowLongW      = modUser32.NewProc("GetWindowLongPtrW")
	procSetWindowLongW      = modUser32.NewProc("SetWindowLongPtrW")
	procSetWindowRgn        = modUser32.NewProc("SetWindowRgn")
	procCreateRoundRectRgn  = modGdi32.NewProc("CreateRoundRectRgn")
	procCreateRectRgn       = modGdi32.NewProc("CreateRectRgn")
	procSaveDC              = modGdi32.NewProc("SaveDC")
	procRestoreDC           = modGdi32.NewProc("RestoreDC")
	procSelectClipRgn       = modGdi32.NewProc("SelectClipRgn")
	procGetAsyncKeyState    = modUser32.NewProc("GetAsyncKeyState")
	procMessageBoxW         = modUser32.NewProc("MessageBoxW")
	procMonitorFromWindow   = modUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW     = modUser32.NewProc("GetMonitorInfoW")
	procSystemParametersInf = modUser32.NewProc("SystemParametersInfoW")

	procBeginPaint             = modUser32.NewProc("BeginPaint")
	procEndPaint               = modUser32.NewProc("EndPaint")
	procCreateSolidBrush       = modGdi32.NewProc("CreateSolidBrush")
	procDeleteObject           = modGdi32.NewProc("DeleteObject")
	procSelectObject           = modGdi32.NewProc("SelectObject")
	procGetObjectW             = modGdi32.NewProc("GetObjectW")
	procCreatePen              = modGdi32.NewProc("CreatePen")
	procCreateFontW            = modGdi32.NewProc("CreateFontW")
	procDrawTextW              = modUser32.NewProc("DrawTextW")
	procDrawTextExW            = modUser32.NewProc("DrawTextExW")
	procSetBkMode              = modGdi32.NewProc("SetBkMode")
	procSetBkColor             = modGdi32.NewProc("SetBkColor")
	procSetTextColor           = modGdi32.NewProc("SetTextColor")
	procRectangle              = modGdi32.NewProc("Rectangle")
	procEllipse                = modGdi32.NewProc("Ellipse")
	procRoundRect              = modGdi32.NewProc("RoundRect")
	procMoveToEx               = modGdi32.NewProc("MoveToEx")
	procLineTo                 = modGdi32.NewProc("LineTo")
	procPolygon                = modGdi32.NewProc("Polygon")
	procPolyline               = modGdi32.NewProc("Polyline")
	procSetPixel               = modGdi32.NewProc("SetPixel")
	procFillRect               = modUser32.NewProc("FillRect")
	procDrawIconEx             = modUser32.NewProc("DrawIconEx")
	procPatBlt                 = modGdi32.NewProc("PatBlt")
	procExtCreatePen           = modGdi32.NewProc("ExtCreatePen")
	procCreateDIBSection       = modGdi32.NewProc("CreateDIBSection")
	procCreateCompatibleDC     = modGdi32.NewProc("CreateCompatibleDC")
	procDeleteDC               = modGdi32.NewProc("DeleteDC")
	procBitBlt                 = modGdi32.NewProc("BitBlt")
	procGradientFill           = modMsimg32.NewProc("GradientFill")
	procCreateIconIndirect     = modUser32.NewProc("CreateIconIndirect")
	procDestroyIcon            = modUser32.NewProc("DestroyIcon")
	procGetDC                  = modUser32.NewProc("GetDC")
	procReleaseDC              = modUser32.NewProc("ReleaseDC")
	procCreateCompatibleBitmap = modGdi32.NewProc("CreateCompatibleBitmap")

	procShellNotifyIcon    = modShell32.NewProc("Shell_NotifyIconW")
	procCreatePopupMenu    = modUser32.NewProc("CreatePopupMenu")
	procAppendMenuW        = modUser32.NewProc("AppendMenuW")
	procTrackPopupMenu     = modUser32.NewProc("TrackPopupMenu")
	procGetCursorPos       = modUser32.NewProc("GetCursorPos")
	procDestroyMenu        = modUser32.NewProc("DestroyMenu")
	procCheckMenuItem      = modUser32.NewProc("CheckMenuItem")
	procSetMenuDefaultItem = modUser32.NewProc("SetMenuDefaultItem")

	procCreateMutexW       = modKernel32.NewProc("CreateMutexW")
	procGetLastError       = modKernel32.NewProc("GetLastError")
	procBeepKernel         = modKernel32.NewProc("Beep")
	procGetModuleFileNameW = modKernel32.NewProc("GetModuleFileNameW")
	procExitProcess        = modKernel32.NewProc("ExitProcess")

	procDwmSetWindowAttribute = modDwmapi.NewProc("DwmSetWindowAttribute")

	procRegOpenKeyExW    = modAdvapi32.NewProc("RegOpenKeyExW")
	procRegCloseKey      = modAdvapi32.NewProc("RegCloseKey")
	procRegSetValueExW   = modAdvapi32.NewProc("RegSetValueExW")
	procRegQueryValueExW = modAdvapi32.NewProc("RegQueryValueExW")
	procRegDeleteValueW  = modAdvapi32.NewProc("RegDeleteValueW")

	procGetOpenFileNameW = modComdlg32.NewProc("GetOpenFileNameW")

	procCoInitializeEx = modOle32.NewProc("CoInitializeEx")

	procMciSendStringW = modWinmm.NewProc("mciSendStringW")
)

// Constants (subset used by the app).
const (
	WM_NULL           = 0x0000
	WM_CREATE         = 0x0001
	WM_DESTROY        = 0x0002
	WM_SIZE           = 0x0005
	WM_PAINT          = 0x000F
	WM_CLOSE          = 0x0010
	WM_QUIT           = 0x0012
	WM_ERASEBKGND     = 0x0014
	WM_GETMINMAXINFO  = 0x0024
	WM_SETCURSOR      = 0x0020
	WM_COMMAND        = 0x0111
	WM_TIMER          = 0x0113
	WM_MOUSEMOVE      = 0x0200
	WM_LBUTTONDOWN    = 0x0201
	WM_LBUTTONUP      = 0x0202
	WM_LBUTTONDBLCLK  = 0x0203
	WM_RBUTTONDOWN    = 0x0204
	WM_RBUTTONUP      = 0x0205
	WM_MBUTTONDBLCLK  = 0x0209
	WM_MOUSELEAVE     = 0x02A2
	WM_MOUSEWHEEL     = 0x020A
	WM_CTLCOLOREDIT   = 0x0133
	WM_CTLCOLORSTATIC = 0x0138
	WM_DPICHANGED     = 0x02E0

	WM_KEYDOWN = 0x0100
	WM_KEYUP   = 0x0101
	WM_CHAR    = 0x0102

	WM_APP = 0x8000

	WM_SETICON = 0x0080
	ICON_SMALL = 0
	ICON_BIG   = 1

	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_POPUP            = 0x80000000
	WS_CAPTION          = 0x00C00000
	WS_SYSMENU          = 0x00080000
	WS_MINIMIZEBOX      = 0x00020000
	WS_MAXIMIZEBOX      = 0x00010000
	WS_CHILD            = 0x40000000
	WS_BORDER           = 0x00800000
	WS_EX_APPWINDOW     = 0x00000100
	WS_EX_TOPMOST       = 0x00000008
	WS_EX_TOOLWINDOW    = 0x00000080
	WS_VSCROLL          = 0x00200000
	WS_TABSTOP          = 0x00010000
	WS_GROUP            = 0x00020000

	WS_EX_CLIENTEDGE = 0x00000200

	SW_HIDE       = 0
	SW_SHOW       = 5
	SW_SHOWNA     = 8
	SW_SHOWNORMAL = 1
	SW_RESTORE    = 9

	CS_HREDRAW    = 0x0002
	CS_VREDRAW    = 0x0001
	CS_DROPSHADOW = 0x00020000

	CW_USEDEFAULT = 0x80000000

	HTCLIENT  = 1
	HTCAPTION = 2

	IDC_ARROW = 32512
	IDC_HAND  = 32649
	IDC_IBEAM = 32513

	IDI_APPLICATION = 32512

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	SWP_NOSIZE       = 0x0001
	SWP_NOMOVE       = 0x0002
	SWP_NOZORDER     = 0x0004
	SWP_FRAMECHANGED = 0x0020

	HWND_TOP       = 0
	HWND_TOPMOST   = ^uintptr(0)     // -1
	HWND_NOTOPMOST = ^uintptr(0) - 1 // -2

	GWLP_HINSTANCE = ^uintptr(4) // -6 on 64-bit as uintptr(-6)? handled in code

	FW_NORMAL   = 400
	FW_SEMIBOLD = 600
	FW_BOLD     = 700

	DEFAULT_CHARSET     = 1
	OUT_DEFAULT_PRECIS  = 0
	CLIP_DEFAULT_PRECIS = 0
	CLEARTYPE_QUALITY   = 5
	DEFAULT_PITCH       = 0
	FF_DONTCARE         = 0

	TRANSPARENT = 1
	OPAQUE      = 2

	PS_SOLID = 0
	PS_NULL  = 5

	DC_BRUSH   = 18
	DC_PEN     = 19
	NULL_BRUSH = 5

	DT_LEFT         = 0x0000
	DT_CENTER       = 0x0001
	DT_RIGHT        = 0x0002
	DT_VCENTER      = 0x0004
	DT_SINGLELINE   = 0x0020
	DT_CALCRECT     = 0x0400
	DT_END_ELLIPSIS = 0x8000
	DT_NOCLIP       = 0x0100
	DT_WORDBREAK    = 0x0010
	DT_NOPREFIX     = 0x0800

	SRCCOPY = 0x00CC0020

	DIB_RGB_COLORS = 0
	BI_RGB         = 0

	GRADIENT_FILL_RECT_H = 0
	GRADIENT_FILL_RECT_V = 1

	R2_ALPHABLEND = 0

	TME_LEAVE = 0x0002

	BS_SOLID = 0

	MB_OK              = 0x00000000
	MB_YESNO           = 0x00000004
	MB_YESNOCANCEL     = 0x00000003
	MB_ICONWARNING     = 0x00000030
	MB_ICONINFORMATION = 0x00000040
	MB_ICONQUESTION    = 0x00000020
	MB_SETFOREGROUND   = 0x00010000
	MB_TOPMOST         = 0x00040000

	IDOK     = 1
	IDCANCEL = 2
	IDYES    = 6
	IDNO     = 7

	WM_NCLBUTTONDOWN = 0x00A1
	HTCLOSE          = 20

	VK_ESCAPE = 0x1B
	VK_RETURN = 0x0D
	VK_TAB    = 0x09

	NIM_ADD        = 0x00000000
	NIM_MODIFY     = 0x00000001
	NIM_DELETE     = 0x00000002
	NIM_SETVERSION = 0x00000004
	NIF_MESSAGE    = 0x00000001
	NIF_ICON       = 0x00000002
	NIF_TIP        = 0x00000004
	NIF_INFO       = 0x00000010
	NIF_SHOWTIP    = 0x00000080

	// NOTIFYICON_VERSION_4: hợp đồng sự kiện hiện đại (Vista+), theo mẫu
	// chuẩn NotificationIcon.cpp của Microsoft: NIM_ADD → NIM_SETVERSION.
	// Khi đó LOWORD(lParam) = sự kiện (NIN_SELECT, WM_CONTEXTMENU…),
	// HIWORD(lParam) = ID icon, toạ độ anchor nằm ở wParam.
	NOTIFYICON_VERSION_4 = 4
	NIN_SELECT           = 0x00000400
	NIN_KEYSELECT        = 0x00000401
	NIN_BALLOONUSERCLICK = 0x00000405

	NIIF_INFO = 0x00000001

	TPM_LEFTBUTTON  = 0x0000
	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100
	TPM_BOTTOMALIGN = 0x0004

	MF_SEPARATOR = 0x00000800
	MF_STRING    = 0x00000000
	MF_CHECKED   = 0x00000008
	MF_UNCHECKED = 0x00000000
	MF_BYCOMMAND = 0x00000000

	MONITOR_DEFAULTTONEAREST = 2

	REG_OPTION_NON_VOLATILE = 0x00000000
	KEY_SET_VALUE           = 0x0002
	KEY_QUERY_VALUE         = 0x0001
	REG_SZ                  = 1

	ERROR_ALREADY_EXISTS = 183

	ES_LEFT        = 0x0000
	ES_AUTOHSCROLL = 0x0080
	ES_NUMBER      = 0x2000

	OFN_FILEMUSTEXIST = 0x00001000
	OFN_PATHMUSTEXIST = 0x00000800
	OFN_HIDEREADONLY  = 0x00000004
	OFN_NOCHANGEDIR   = 0x00000008

	COINIT_APARTMENTTHREADED = 0x2

	DWMWA_USE_IMMERSIVE_DARK_MODE_OLD = 19
	DWMWA_USE_IMMERSIVE_DARK_MODE     = 20
	DWMWA_WINDOW_CORNER_PREFERENCE    = 33
	DWMWCP_ROUND                      = 2
)

// Structs.
type POINT struct{ X, Y int32 }

type RECT struct{ Left, Top, Right, Bottom int32 }

func (r RECT) W() int32 { return r.Right - r.Left }
func (r RECT) H() int32 { return r.Bottom - r.Top }

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

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
	LpszMenuName  uintptr
	LpszClassName uintptr
	HIconSm       uintptr
}

type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RGBReserved [32]byte
}

type TRIVERTEX struct {
	X, Y        int32
	Red, Green  uint16
	Blue, Alpha uint16
}

type GRADIENT_RECT struct {
	UpperLeft, LowerRight uint32
}

type MINMAXINFO struct {
	PtReserved, PtMaxSize, PtMaxPosition POINT
	PtMinTrackSize, PtMaxTrackSize       POINT
}

type TRACKMOUSEEVENT struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   uintptr
	DwHoverTime uint32
}

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

type BITMAPINFO struct {
	Header BITMAPINFOHEADER
	Colors [1]uint32
}

type ICONINFO struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeout         uint32 // union uTimeout/uVersion flattened
	SzInfoTitle      [64]uint16
	DWInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

// Helpers.
func UTF16(s string) uintptr {
	p, _ := syscall.UTF16PtrFromString(s)
	return uintptr(unsafe.Pointer(p))
}

func LOWORD(v uintptr) uint16       { return uint16(v) }
func HIWORD(v uintptr) uint16       { return uint16(v >> 16) }
func GET_X_LPARAM(lp uintptr) int32 { return int32(int16(LOWORD(lp))) }
func GET_Y_LPARAM(lp uintptr) int32 { return int32(int16(HIWORD(lp))) }

func HIWORDSigned(v uintptr) int16 { return int16(v >> 16) }

func MakeIntResource(id uint16) uintptr { return uintptr(id) }

func RectIsInside(r RECT, x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

// Syscall wrappers (thin, panic only on truly broken calls).
func RegisterClassEx(wc *WNDCLASSEXW) uint16 {
	r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(wc)))
	return uint16(r)
}

func CreateWindowEx(exStyle uint32, class, title string, style uint32, x, y, w, h int32, parent, menu, inst uintptr) uintptr {
	r, _, _ := procCreateWindowExW.Call(uintptr(exStyle), UTF16(class), UTF16(title), uintptr(style),
		uintptr(int64(x)), uintptr(int64(y)), uintptr(int64(w)), uintptr(int64(h)),
		parent, menu, inst, 0)
	return r
}

func DefWindowProc(hwnd, msg, wp, lp uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func GetMessage(m *MSG, hwnd uintptr) int32 {
	r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(m)), hwnd, 0, 0)
	return int32(r)
}

func TranslateMessage(m *MSG) uintptr {
	r, _, _ := procTranslateMessage.Call(uintptr(unsafe.Pointer(m)))
	return r
}

func DispatchMessage(m *MSG) uintptr {
	r, _, _ := procDispatchMessageW.Call(uintptr(unsafe.Pointer(m)))
	return r
}

func PostQuitMessage(code int32) { procPostQuitMessage.Call(uintptr(code)) }

func PostMessage(hwnd, msg, wp, lp uintptr) uintptr {
	r, _, _ := procPostMessageW.Call(hwnd, msg, wp, lp)
	return r
}

func SendMessage(hwnd, msg, wp, lp uintptr) uintptr {
	r, _, _ := procSendMessageW.Call(hwnd, msg, wp, lp)
	return r
}

func ShowWindow(hwnd uintptr, cmd int32) { procShowWindow.Call(hwnd, uintptr(cmd)) }

func UpdateWindow(hwnd uintptr) { procUpdateWindow.Call(hwnd) }

func DestroyWindow(hwnd uintptr) { procDestroyWindow.Call(hwnd) }

func InvalidateRect(hwnd uintptr, erase bool) {
	procInvalidateRect.Call(hwnd, 0, boolPtr(erase))
}

func GetClientRect(hwnd uintptr) RECT {
	var r RECT
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

func GetWindowRect(hwnd uintptr) RECT {
	var r RECT
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

func ClientToScreen(hwnd uintptr, pt *POINT) {
	procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(pt)))
}

func ScreenToClient(hwnd uintptr, pt *POINT) {
	procScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(pt)))
}

func SetWindowPos(hwnd, after uintptr, x, y, cx, cy int32, flags uint32) {
	procSetWindowPos.Call(hwnd, after, uintptr(int64(x)), uintptr(int64(y)), uintptr(int64(cx)), uintptr(int64(cy)), uintptr(flags))
}

func SetForegroundWindow(hwnd uintptr) { procSetForegroundWindow.Call(hwnd) }

func IsWindowVisible(hwnd uintptr) bool {
	r, _, _ := procIsWindowVisible.Call(hwnd)
	return r != 0
}

// IsIconic reports whether the window is minimized.
func IsIconic(hwnd uintptr) bool {
	r, _, _ := procIsIconic.Call(hwnd)
	return r != 0
}

func EnableWindow(hwnd uintptr, enable bool) { procEnableWindow.Call(hwnd, boolPtr(enable)) }

func LoadCursor(id uintptr) uintptr {
	r, _, _ := procLoadCursorW.Call(0, id)
	return r
}

func SetCursor(h uintptr) { procSetCursor.Call(h) }

func TrackMouseLeave(hwnd uintptr) {
	tme := TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), DwFlags: TME_LEAVE, HwndTrack: hwnd}
	procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
}

func SetTimer(hwnd uintptr, id, ms uintptr) uintptr {
	r, _, _ := procSetTimer.Call(hwnd, id, ms)
	return r
}

func KillTimer(hwnd uintptr, id uintptr) { procKillTimer.Call(hwnd, id) }

func SetWindowText(hwnd uintptr, s string) { procSetWindowTextW.Call(hwnd, UTF16(s)) }

func BeginPaint(hwnd uintptr, ps *PAINTSTRUCT) uintptr {
	r, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(ps)))
	return r
}

func EndPaint(hwnd uintptr, ps *PAINTSTRUCT) { procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(ps))) }

func GetDC(hwnd uintptr) uintptr {
	r, _, _ := procGetDC.Call(hwnd)
	return r
}

func ReleaseDC(hwnd, hdc uintptr) { procReleaseDC.Call(hwnd, hdc) }

func MessageBox(hwnd uintptr, text, caption string, flags uint32) int32 {
	r, _, _ := procMessageBoxW.Call(hwnd, UTF16(text), UTF16(caption), uintptr(flags))
	return int32(r)
}

func MonitorWorkArea(hwnd uintptr) RECT {
	var mi MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	r, _, _ := procMonitorFromWindow.Call(hwnd, uintptr(MONITOR_DEFAULTTONEAREST))
	procGetMonitorInfoW.Call(r, uintptr(unsafe.Pointer(&mi)))
	return mi.RcWork
}

func ScreenSize() (int32, int32) {
	w, _, _ := procSystemParametersInf.Call(0x0048 /*SPI_GETNONCLIENTMETRICS — not used*/, 0, 0, 0)
	_ = w
	// simpler: use GetSystemMetrics
	gsm := modUser32.NewProc("GetSystemMetrics")
	cx, _, _ := gsm.Call(SM_CXSCREEN)
	cy, _, _ := gsm.Call(SM_CYSCREEN)
	return int32(cx), int32(cy)
}

func CreateSolidBrush(color uint32) uintptr {
	r, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return r
}

func DeleteObject(h uintptr) { procDeleteObject.Call(h) }

func boolPtr(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// CrashLog is set by the app (gui_windows.Run) so WndProc panics reach
// nhat-ky.log instead of silently killing the process.
var CrashLog func(format string, args ...interface{})

// CoInitializeEx initializes COM on the calling thread (STA).
// BẮT BUỘC trước khi mở hộp thoại GetOpenFileNameW — không có COM,
// trên Windows 10/11 (OneDrive/shell extension) hộp thoại làm crash process.
// S_FALSE (đã khởi tạo rồi) coi như thành công.
func CoInitializeEx() {
	r, _, _ := procCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED)
	_ = r // S_OK=0, S_FALSE=1, RPC_E_CHANGED_MODE=0x80010106 — đều không fatal
}

// RecoverWndProc chặn panic trong window procedure — nếu không có guard này,
// MỌI panic trong code UI sẽ giết process một cách im lặng (windowsgui không
// có console để in stack). Gọi dạng: defer RecoverWndProc("panel").
func RecoverWndProc(context string) {
	if r := recover(); r != nil {
		if CrashLog != nil {
			CrashLog("PANIC[%s]: %v\n%s", context, r, debug.Stack())
		}
	}
}

// LogPanic ghi log panic cho các hàm không chạy trong WndProc.
func LogPanic(context string) {
	if r := recover(); r != nil {
		if CrashLog != nil {
			CrashLog("PANIC[%s]: %v\n%s", context, r, debug.Stack())
		}
	}
}
