//go:build windows && !nogui

// Package alertwin vẽ cửa sổ cảnh báo sinh nhật — luôn trên cùng,
// nút "TẮT NHẮC NHỞ" dừng nhạc ngay lập tức.
package alertwin

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"nhac-sinh-nhat/internal/w32"
)

const (
	className = "NhacSinhNhatAlertWnd"
	alertW    = int32(420)
	alertH    = int32(330)
)

// Item is one person to alert.
type Item struct {
	Name     string
	DaysLeft int
	Next     time.Time
	Lunar    string
	IsLunar  bool
	Note     string
	IsTest   bool // hiển thị dạng "kiểm tra" thay vì cảnh báo thật
}

type alert struct {
	hwnd    uintptr
	item    Item
	canvas  *w32.Canvas
	icon    uintptr
	onDone  func()
	hover   bool
	pressed bool
}

var active *alert
var classRegistered bool

// Show displays the alert window (replaces any active one).
func Show(item Item, onDismiss func()) bool {
	if active != nil {
		active.close() // WM_DESTROY sẽ tự gán active = nil
	}
	a := &alert{item: item, onDone: onDismiss, icon: w32.CreateCakeIcon(56)}
	active = a // gán TRƯỚC khi create — WM_PAINT lúc ShowWindow cần thấy nó
	if !a.create() {
		active = nil
		return false
	}
	return true
}

// Active reports whether an alert window is currently visible.
func Active() bool { return active != nil }

// CloseActive closes the current alert without invoking onDismiss.
func CloseActive() {
	if active != nil {
		active.close()
	}
}

func (a *alert) create() bool {
	inst, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	if !classRegistered {
		wc := w32.WNDCLASSEXW{
			CbSize:      uint32(unsafe.Sizeof(w32.WNDCLASSEXW{})),
			Style:       w32.CS_DROPSHADOW,
			LpfnWndProc: syscall.NewCallback(alertWndProc),
			HInstance:   inst,
			HIcon:       w32.CreateCakeIcon(32),
			HIconSm:     w32.CreateCakeIcon(16),
			HCursor:     w32.LoadCursor(w32.MakeIntResource(w32.IDC_ARROW)),
		}
		wc.LpszClassName = w32.UTF16(className)
		if w32.RegisterClassEx(&wc) == 0 {
			return false
		}
		classRegistered = true
	}

	work := w32.MonitorWorkArea(0)
	x := work.Right - alertW - 36
	y := work.Bottom - alertH - 36

	a.hwnd = w32.CreateWindowEx(w32.WS_EX_TOPMOST|w32.WS_EX_TOOLWINDOW, className,
		"Nhắc sinh nhật", w32.WS_POPUP,
		x, y, alertW, alertH, 0, 0, inst)
	if a.hwnd == 0 {
		return false
	}
	w32.SetWindowRgn(a.hwnd, 0, 0, alertW, alertH, 14)
	w32.ShowWindow(a.hwnd, w32.SW_SHOWNA)
	w32.SetWindowPos(a.hwnd, w32.HWND_TOPMOST, 0, 0, 0, 0, w32.SWP_NOMOVE|w32.SWP_NOSIZE)
	w32.SetForegroundWindow(a.hwnd)
	return true
}

func (a *alert) close() {
	h := a.hwnd
	a.hwnd = 0
	if h != 0 {
		w32.DestroyWindow(h)
	}
}

func (a *alert) btnRect() (int32, int32, int32, int32) {
	return 28, alertH - 76, alertW - 56, 52
}

func alertWndProc(hwnd, msg, wp, lp uintptr) (rc uintptr) {
	defer w32.RecoverWndProc("alert") // panic trong UI → log, không giết app
	if active == nil {
		return w32.DefWindowProc(hwnd, msg, wp, lp)
	}
	a := active
	switch msg {
	case w32.WM_DESTROY:
		a.canvas.Free()
		a.canvas = nil
		if active == a {
			active = nil
		}
		return 0

	case w32.WM_ERASEBKGND:
		return 1

	case w32.WM_PAINT:
		a.paint()
		return 0

	case w32.WM_CLOSE:
		// Alt+F4 / đóng bằng cách khác vẫn phải dừng nhạc + đánh dấu đã nhắc
		a.dismiss()
		return 0

	case w32.WM_KEYDOWN:
		switch wp {
		case w32.VK_RETURN, w32.VK_ESCAPE, 0x20:
			a.dismiss()
			return 0
		}

	case w32.WM_MOUSEMOVE:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		bx, by, bw, bh := a.btnRect()
		inside := x >= bx && x < bx+bw && y >= by && y < by+bh
		if inside != a.hover {
			a.hover = inside
			w32.InvalidateRect(hwnd, false)
		}
		w32.TrackMouseLeave(hwnd)
		if inside {
			w32.SetCursor(w32.LoadCursor(w32.MakeIntResource(w32.IDC_HAND)))
		}
		return 0

	case w32.WM_MOUSELEAVE:
		a.hover = false
		w32.InvalidateRect(hwnd, false)
		return 0

	case w32.WM_LBUTTONDOWN:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		bx, by, bw, bh := a.btnRect()
		if x >= bx && x < bx+bw && y >= by && y < by+bh {
			a.pressed = true
			w32.InvalidateRect(hwnd, false)
		}
		return 0

	case w32.WM_LBUTTONUP:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		bx, by, bw, bh := a.btnRect()
		was := a.pressed
		a.pressed = false
		w32.InvalidateRect(hwnd, false)
		if was && x >= bx && x < bx+bw && y >= by && y < by+bh {
			a.dismiss()
		}
		return 0
	}
	return w32.DefWindowProc(hwnd, msg, wp, lp)
}

// dismiss closes the alert and fires the callback (stop music etc.).
func (a *alert) dismiss() {
	cb := a.onDone
	a.onDone = nil
	a.close()
	if cb != nil {
		cb()
	}
}

// drawAlertConfetti rắc vài chấm màu quanh icon bánh — vị trí cố định,
// alpha thấp, tạo cảm giác liên hoan mà không chắn chữ.
func drawAlertConfetti(hdc uintptr) {
	type dot struct {
		x, y, r int32
		c       w32.Color
		a       float64
	}
	dots := []dot{
		{52, 44, 3, w32.CYellow, 0.55},
		{96, 78, 2, w32.CBlue, 0.50},
		{alertW - 60, 40, 3, w32.CPink, 0.50},
		{alertW - 108, 82, 2, w32.CGreen, 0.50},
		{142, 30, 2, w32.Col(0xFF, 0xFF, 0xFF), 0.28},
		{alertW - 150, 34, 2, w32.CAmber, 0.45},
	}
	for _, d := range dots {
		w32.EllipseC(hdc, d.x, d.y, d.r*2, d.r*2, d.c.Blend(w32.CBg, d.a))
	}
}

func (a *alert) paint() {
	var ps w32.PAINTSTRUCT
	hdc := w32.BeginPaint(a.hwnd, &ps)
	defer w32.EndPaint(a.hwnd, &ps)
	if a.canvas == nil {
		a.canvas = w32.NewCanvas(a.hwnd, alertW, alertH)
	}
	if a.canvas == nil {
		return
	}
	c := a.canvas.HDC
	w32.FillRectC(c, 0, 0, alertW, alertH, w32.CBg)

	// top gradient strip
	w32.Gradient(c, 0, 0, alertW, 10, w32.CAccentDk, w32.CPink, false)

	// quầng sáng sau icon bánh + pháo giấy trang trí
	w32.EllipseC(c, alertW/2-44, 12, 88, 88, w32.CCard.Blend(w32.Col(0xFF, 0xFF, 0xFF), 0.06))
	drawAlertConfetti(c)

	// cake icon
	w32.DrawIconEx(c, (alertW-56)/2, 24, a.icon, 56, 56, 0, 0)

	// headline
	headline := "SINH NHẬT SẮP ĐẾN!"
	if a.item.IsTest {
		headline = "KIỂM TRA CẢNH BÁO — THỬ NGHIỆM"
	}
	w32.DrawTextC(c, w32.FSmallSb, headline, 0, 92, alertW, w32.CPink.Blend(w32.CBg, 0.15), w32.DT_CENTER)
	w32.DrawTextC(c, w32.FBig, a.item.Name, 20, 116, alertW-40, w32.CText, w32.DT_CENTER)

	// days line
	daysTxt := "Hôm nay là sinh nhật của " + a.item.Name + "!"
	if a.item.DaysLeft > 0 {
		daysTxt = fmt.Sprintf("Còn %d ngày — %s", a.item.DaysLeft, a.item.Next.Format("02/01/2006"))
	}
	w32.DrawTextC(c, w32.FBody, daysTxt, 20, 158, alertW-40, w32.CTextSoft, w32.DT_CENTER)

	// lunar line
	lunarLine := "Âm lịch: " + a.item.Lunar
	if a.item.IsLunar {
		lunarLine += " (sinh nhật âm lịch)"
	}
	w32.DrawTextC(c, w32.FSmall, lunarLine, 20, 186, alertW-40, w32.CTextDim, w32.DT_CENTER)

	// note
	if a.item.Note != "" {
		w32.DrawTextC(c, w32.FSmall, "“"+a.item.Note+"”", 28, 212, alertW-56, w32.CAmber, w32.DT_CENTER)
	}

	// big stop button
	bx, by, bw, bh := a.btnRect()
	fill := w32.CRed
	if a.hover {
		fill = w32.CRedDk
	}
	if a.pressed {
		fill = w32.CRedDk.Blend(w32.Col(0, 0, 0), 0.25)
	}
	w32.RoundRectBorder(c, bx, by, bw, bh, 10, fill, fill)
	w32.DrawTextC(c, w32.FButton, "TẮT NHẮC NHỞ", bx, by+bh/2-10, bw, w32.Col(0xFF, 0xFF, 0xFF), w32.DT_CENTER)

	// blit the finished frame to screen
	w32.BitBltTo(hdc, 0, 0, alertW, alertH, c, 0, 0)
}
