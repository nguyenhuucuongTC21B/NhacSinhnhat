//go:build windows && !nogui

package w32

import "unsafe"

func BitBltTo(dst uintptr, dx, dy, dw, dh int32, src uintptr, sx, sy int32) {
	procBitBlt.Call(dst, uintptr(int64(dx)), uintptr(int64(dy)), uintptr(int64(dw)), uintptr(int64(dh)),
		src, uintptr(int64(sx)), uintptr(int64(sy)), uintptr(SRCCOPY))
}

func DrawIconEx(hdc uintptr, x, y int32, hIcon uintptr, w, h int32, step uintptr, flags uintptr) {
	procDrawIconEx.Call(hdc, uintptr(int64(x)), uintptr(int64(y)), hIcon,
		uintptr(int64(w)), uintptr(int64(h)), step, 0, flags)
}

func CreatePenProc(width int32, color Color) (uintptr, uintptr, uintptr) {
	r, _, e := procCreatePen.Call(PS_SOLID, uintptr(width), uintptr(color))
	_ = e
	return r, 0, 0
}

func SelectObjectProc(hdc, obj uintptr) (uintptr, uintptr, uintptr) {
	r, _, e := procSelectObject.Call(hdc, obj)
	_ = e
	return r, 0, 0
}

// LineTo2 draws a line (MoveToEx + LineTo).
func LineTo2(hdc uintptr, x1, y1, x2, y2 int32) {
	procMoveToEx.Call(hdc, uintptr(int64(x1)), uintptr(int64(y1)), 0)
	procLineTo.Call(hdc, uintptr(int64(x2)), uintptr(int64(y2)))
}

var _ = unsafe.Pointer(nil)
