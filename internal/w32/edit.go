//go:build windows && !nogui

package w32

import (
	"syscall"
	"unsafe"
)

const GWLP_WNDPROC = ^uintptr(3) // -4

// SetWindowProc subclasses a control, returning the previous proc.
func SetWindowProc(hwnd, proc uintptr) uintptr {
	r, _, _ := procSetWindowLongW.Call(hwnd, uintptr(GWLP_WNDPROC), proc)
	return r
}

// CallWindowProc forwards to the original control proc.
func CallWindowProc(proc, hwnd, msg, wp, lp uintptr) uintptr {
	r, _, _ := procCallWindowProcW.Call(proc, hwnd, msg, wp, lp)
	return r
}

var procCallWindowProcW = modUser32.NewProc("CallWindowProcW")
var procGetWindowTextW = modUser32.NewProc("GetWindowTextW")

// GetWindowTextW reads a control's text.
func GetWindowText(hwnd uintptr) string {
	procGetWindowTextLengthW := modUser32.NewProc("GetWindowTextLengthW")
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n <= 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(n+1))
	return syscall.UTF16ToString(buf)
}

// SetFocusTo moves keyboard focus.
func SetFocusTo(hwnd uintptr) {
	modUser32.NewProc("SetFocus").Call(hwnd)
}

// CreateEdit makes a borderless EDIT control with dark-friendly defaults.
func CreateEdit(parent, inst uintptr, id int, text string, x, y, w, h int32, numbers bool) uintptr {
	style := uintptr(WS_CHILD | WS_VISIBLE | WS_TABSTOP | ES_AUTOHSCROLL)
	if numbers {
		style |= ES_NUMBER
	}
	hwnd := CreateWindowEx(0, "EDIT", text, uint32(style), x, y, w, h, parent, uintptr(id), inst)
	return hwnd
}
