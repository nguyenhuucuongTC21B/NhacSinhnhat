//go:build windows && !nogui

package w32

import (
	"image/color"
	"syscall"
	"unsafe"
)

// Color is a 24-bit RGB color (0x00BBGGRR when passed to GDI).
type Color uint32

func Col(r, g, b uint32) Color { return Color(r | g<<8 | b<<16) }

func (c Color) R() uint8 { return uint8(c & 0xFF) }
func (c Color) G() uint8 { return uint8((c >> 8) & 0xFF) }
func (c Color) B() uint8 { return uint8((c >> 16) & 0xFF) }

// Blend mixes c onto bg by alpha (0..1).
func (c Color) Blend(bg Color, alpha float64) Color {
	mix := func(a, b uint32) uint32 { return a + uint32(float64(b-a)*alpha) }
	return Col(mix(uint32(c.R()), uint32(bg.R())), mix(uint32(c.G()), uint32(bg.G())), mix(uint32(c.B()), uint32(bg.B())))
}

// Theme colors for the whole app (dark slate + violet/pink accents).
var (
	CBg        = Col(0x0F, 0x17, 0x2A) // slate-900 page
	CCard      = Col(0x1E, 0x29, 0x3B) // slate-800
	CRowAlt    = Col(0x16, 0x20, 0x38)
	CRowHover  = Col(0x27, 0x34, 0x52)
	CRowSel    = Col(0x31, 0x2E, 0x81) // indigo-900
	CBorder    = Col(0x2A, 0x36, 0x52)
	CBorderHi  = Col(0x3B, 0x4A, 0x6B)
	CText      = Col(0xF1, 0xF5, 0xF9)
	CTextSoft  = Col(0xCB, 0xD5, 0xE1)
	CTextDim   = Col(0x94, 0xA3, 0xB8)
	CTextFaint = Col(0x64, 0x74, 0x8B)
	CAccent    = Col(0x8B, 0x5C, 0xF6) // violet-500
	CAccentHi  = Col(0xA7, 0x8B, 0xFA)
	CAccentDk  = Col(0x7C, 0x3A, 0xED)
	CPink      = Col(0xDB, 0x27, 0x77)
	CGreen     = Col(0x10, 0xB9, 0x81)
	CGreenDk   = Col(0x05, 0x96, 0x69)
	CRed       = Col(0xEF, 0x44, 0x44)
	CRedDk     = Col(0xDC, 0x26, 0x26)
	COrange    = Col(0xF9, 0x73, 0x16)
	CYellow    = Col(0xEA, 0xB3, 0x08)
	CAmber     = Col(0xF5, 0x9E, 0x0B)
	CBlue      = Col(0x60, 0xA5, 0xFA)
	CEditBg    = Col(0x0B, 0x14, 0x24)
)

// Font is a lazily-created HFONT.
type Font struct {
	h    uintptr
	H    int32 // negative char height
	Word string
	Bold bool
}

var fonts []*Font

func NewFont(h int32, weight int32, face string) *Font {
	f := &Font{H: h, Word: face, Bold: weight >= 600}
	fonts = append(fonts, f)
	return f
}

var (
	FTitle   = NewFont(-27, FW_BOLD, "Segoe UI")
	FH2      = NewFont(-20, FW_SEMIBOLD, "Segoe UI")
	FBody    = NewFont(-15, FW_NORMAL, "Segoe UI")
	FBodySb  = NewFont(-15, FW_SEMIBOLD, "Segoe UI")
	FSmall   = NewFont(-13, FW_NORMAL, "Segoe UI")
	FSmallSb = NewFont(-13, FW_SEMIBOLD, "Segoe UI")
	FTiny    = NewFont(-12, FW_NORMAL, "Segoe UI")
	FBadge   = NewFont(-14, FW_SEMIBOLD, "Segoe UI")
	FBig     = NewFont(-24, FW_BOLD, "Segoe UI")
	FLabel   = NewFont(-14, FW_SEMIBOLD, "Segoe UI")
	FButton  = NewFont(-15, FW_SEMIBOLD, "Segoe UI")
)

func (f *Font) HFONT() uintptr {
	if f.h == 0 {
		weight := int32(FW_NORMAL)
		if f.Bold {
			weight = FW_SEMIBOLD
		}
		r, _, _ := procCreateFontW.Call(
			uintptr(int64(f.H)), 0, 0, 0, uintptr(weight), 0, 0, 0,
			DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY,
			DEFAULT_PITCH|uintptr(FF_DONTCARE), UTF16(f.Word))
		f.h = r
	}
	return f.h
}

// FreeFonts deletes all font handles (called on window destroy; handles are global anyway).
func FreeFonts() {
	for _, f := range fonts {
		if f.h != 0 {
			DeleteObject(f.h)
			f.h = 0
		}
	}
}

// Canvas is a double-buffered GDI drawing target.
type Canvas struct {
	HDC    uintptr
	Bmp    uintptr
	OldBmp uintptr
	W, H   int32
	bits   unsafe.Pointer
	mirror *imageRGBA // optional Go-side pixel access (nil unless requested)
	hwnd   uintptr
}

type imageRGBA struct {
	Pix  []uint8
	W, H int
}

func NewCanvas(hwnd uintptr, w, h int32) *Canvas {
	if w <= 0 || h <= 0 {
		return nil
	}
	screenDC := GetDC(hwnd)
	if screenDC == 0 {
		return nil
	}
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		ReleaseDC(hwnd, screenDC)
		return nil
	}
	bi := BITMAPINFO{}
	bi.Header.BiSize = uint32(unsafe.Sizeof(bi.Header))
	bi.Header.BiWidth = w
	bi.Header.BiHeight = -h // top-down
	bi.Header.BiPlanes = 1
	bi.Header.BiBitCount = 32
	bi.Header.BiCompression = BI_RGB
	var bits unsafe.Pointer
	bmp, _, _ := procCreateDIBSection.Call(memDC, uintptr(unsafe.Pointer(&bi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 {
		procDeleteDC.Call(memDC)
		ReleaseDC(hwnd, screenDC)
		return nil
	}
	old, _, _ := procSelectObject.Call(memDC, bmp)
	ReleaseDC(hwnd, screenDC)
	return &Canvas{HDC: memDC, Bmp: bmp, OldBmp: old, W: w, H: h, bits: bits, hwnd: hwnd}
}

// Pix exposes the 32bpp BGRA pixel buffer.
func (c *Canvas) Pix() []uint8 {
	if c == nil || c.bits == nil {
		return nil
	}
	return (*[1 << 30]uint8)(c.bits)[: c.W*c.H*4 : c.W*c.H*4]
}

func (c *Canvas) Free() {
	if c == nil {
		return
	}
	procSelectObject.Call(c.HDC, c.OldBmp)
	DeleteObject(c.Bmp)
	procDeleteDC.Call(c.HDC)
	c.HDC, c.Bmp = 0, 0
}

// ---- fill / shape helpers (all coordinates int32, inclusive-exclusive-ish like GDI) ----

func FillRectC(hdc uintptr, x, y, w, h int32, color Color) {
	brush := CreateSolidBrush(uint32(color))
	r := RECT{Left: x, Top: y, Right: x + w, Bottom: y + h}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
	DeleteObject(brush)
}

func FillRectBr(hdc uintptr, r RECT, brush uintptr) {
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
}

func RoundRectC(hdc uintptr, x, y, w, h int32, radius int32, color Color) {
	brush := CreateSolidBrush(uint32(color))
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, uintptr(color))
	oldB, _, _ := procSelectObject.Call(hdc, brush)
	oldP, _, _ := procSelectObject.Call(hdc, pen)
	procRoundRect.Call(hdc, uintptr(int64(x)), uintptr(int64(y)), uintptr(int64(x+w)), uintptr(int64(y+h)), uintptr(int64(radius*2)), uintptr(int64(radius*2)))
	procSelectObject.Call(hdc, oldB)
	procSelectObject.Call(hdc, oldP)
	DeleteObject(brush)
	DeleteObject(pen)
}

// RoundRectBorder draws rounded rect with fill + 1px border color.
func RoundRectBorder(hdc uintptr, x, y, w, h int32, radius int32, fill, border Color) {
	brush := CreateSolidBrush(uint32(fill))
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, uintptr(border))
	oldB, _, _ := procSelectObject.Call(hdc, brush)
	oldP, _, _ := procSelectObject.Call(hdc, pen)
	procRoundRect.Call(hdc, uintptr(int64(x)), uintptr(int64(y)), uintptr(int64(x+w)), uintptr(int64(y+h)), uintptr(int64(radius*2)), uintptr(int64(radius*2)))
	procSelectObject.Call(hdc, oldB)
	procSelectObject.Call(hdc, oldP)
	DeleteObject(brush)
	DeleteObject(pen)
}

func EllipseC(hdc uintptr, x, y, w, h int32, color Color) {
	brush := CreateSolidBrush(uint32(color))
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, uintptr(color))
	oldB, _, _ := procSelectObject.Call(hdc, brush)
	oldP, _, _ := procSelectObject.Call(hdc, pen)
	procEllipse.Call(hdc, uintptr(int64(x)), uintptr(int64(y)), uintptr(int64(x+w)), uintptr(int64(y+h)))
	procSelectObject.Call(hdc, oldB)
	procSelectObject.Call(hdc, oldP)
	DeleteObject(brush)
	DeleteObject(pen)
}

// GradientV / GradientH fill a rect with a two-color gradient.
func Gradient(hdc uintptr, x, y, w, h int32, c1, c2 Color, vertical bool) {
	verts := []TRIVERTEX{
		{X: x, Y: y, Red: uint16(c1.R()) * 257, Green: uint16(c1.G()) * 257, Blue: uint16(c1.B()) * 257, Alpha: 0},
		{X: x + w, Y: y + h, Red: uint16(c2.R()) * 257, Green: uint16(c2.G()) * 257, Blue: uint16(c2.B()) * 257, Alpha: 0},
	}
	mesh := []GRADIENT_RECT{{UpperLeft: 0, LowerRight: 1}}
	mode := uintptr(GRADIENT_FILL_RECT_H)
	if vertical {
		mode = uintptr(GRADIENT_FILL_RECT_V)
	}
	procGradientFill.Call(hdc, uintptr(unsafe.Pointer(&verts[0])), 2, uintptr(unsafe.Pointer(&mesh[0])), 1, mode)
}

// Line draws a 1px line.
func Line(hdc uintptr, x1, y1, x2, y2 int32, color Color) {
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, uintptr(color))
	oldP, _, _ := procSelectObject.Call(hdc, pen)
	procMoveToEx.Call(hdc, uintptr(int64(x1)), uintptr(int64(y1)), 0)
	procLineTo.Call(hdc, uintptr(int64(x2)), uintptr(int64(y2)))
	procSelectObject.Call(hdc, oldP)
	DeleteObject(pen)
}

// Text helpers ------------------------------------------------------------

// MeasureText returns the width/height of s drawn with font f.
func MeasureText(hdc uintptr, f *Font, s string) (int32, int32) {
	oldF, _, _ := procSelectObject.Call(hdc, f.HFONT())
	rc := RECT{Left: 0, Top: 0, Right: 10000, Bottom: 0}
	procDrawTextW.Call(hdc, UTF16(s), uintptr(int32(utf16Len(s))), uintptr(unsafe.Pointer(&rc)), uintptr(DT_CALCRECT|DT_NOPREFIX|DT_SINGLELINE))
	procSelectObject.Call(hdc, oldF)
	return rc.Right - rc.Left, rc.Bottom - rc.Top
}

func utf16Len(s string) int {
	p, _ := syscall.UTF16FromString(s)
	return len(p) - 1
}

// DrawTextC draws s with font f and color col at (x,y) with given width;
// align: 0=left 1=center 2=right; clips to w with ellipsis.
func DrawTextC(hdc uintptr, f *Font, s string, x, y, w int32, col Color, align uint32) int32 {
	oldF, _, _ := procSelectObject.Call(hdc, f.HFONT())
	oldBk, _, _ := procSetBkMode.Call(hdc, TRANSPARENT)
	procSetTextColor.Call(hdc, uintptr(col))
	flags := uintptr(DT_SINGLELINE | DT_NOPREFIX | DT_END_ELLIPSIS | align)
	rc := RECT{Left: x, Top: y, Right: x + w, Bottom: y + 200}
	procDrawTextW.Call(hdc, UTF16(s), uintptr(int32(utf16Len(s))), uintptr(unsafe.Pointer(&rc)), flags)
	procSetBkMode.Call(hdc, oldBk)
	procSelectObject.Call(hdc, oldF)
	return rc.Bottom - rc.Top
}

// DrawTextRect draws multiline text inside rect.
func DrawTextRect(hdc uintptr, f *Font, s string, r RECT, col Color, flags uintptr) {
	oldF, _, _ := procSelectObject.Call(hdc, f.HFONT())
	oldBk, _, _ := procSetBkMode.Call(hdc, TRANSPARENT)
	procSetTextColor.Call(hdc, uintptr(col))
	rc := r
	procDrawTextW.Call(hdc, UTF16(s), uintptr(int32(utf16Len(s))), uintptr(unsafe.Pointer(&rc)), flags|uintptr(DT_NOPREFIX))
	procSetBkMode.Call(hdc, oldBk)
	procSelectObject.Call(hdc, oldF)
}

func TextHeight(hdc uintptr, f *Font, s string) int32 {
	h, _ := MeasureText(hdc, f, s)
	return h
}

// SetWindowRegion rounds the window corners.
func SetWindowRgn(hwnd uintptr, x, y, w, h int32, radius int32) {
	rgn, _, _ := procCreateRoundRectRgn.Call(uintptr(int64(x)), uintptr(int64(y)), uintptr(int64(x+w)), uintptr(int64(y+h)), uintptr(int64(radius*2)), uintptr(int64(radius*2)))
	procSetWindowRgn.Call(hwnd, rgn, 1)
}

// PushClip giới hạn vùng vẽ của hdc vào hình chữ nhật (có thể lồng nhau) —
// dùng để cắt gọn danh sách cuộn, không cho hàng tràn lên nhãn cột/xuống footer.
// Trả về context ID để truyền vào PopClip.
func PushClip(hdc uintptr, x, y, w, h int32) int32 {
	id, _, _ := procSaveDC.Call(hdc)
	rgn, _, _ := procCreateRectRgn.Call(uintptr(int64(x)), uintptr(int64(y)), uintptr(int64(x+w)), uintptr(int64(y+h)))
	procSelectClipRgn.Call(hdc, rgn)
	DeleteObject(rgn) // hệ thống giữ bản sao riêng sau SelectClipRgn
	return int32(int32(id))
}

// PopClip khôi phục vùng vẽ trước đó của PushClip (RestoreDC với -id).
func PopClip(hdc uintptr, id int32) {
	if id != 0 {
		procRestoreDC.Call(hdc, uintptr(int64(-id)))
	}
}

// EnableDarkTitleBar asks DWM for a dark caption (best effort).
func EnableDarkTitleBar(hwnd uintptr) {
	val := uint32(1)
	procDwmSetWindowAttribute.Call(hwnd, uintptr(DWMWA_USE_IMMERSIVE_DARK_MODE), uintptr(unsafe.Pointer(&val)), 4)
	procDwmSetWindowAttribute.Call(hwnd, uintptr(DWMWA_USE_IMMERSIVE_DARK_MODE_OLD), uintptr(unsafe.Pointer(&val)), 4)
}

// SetRoundedCorners xin DWM bo góc cửa sổ (Windows 11) — best effort:
// trên Windows 10 lệnh gọi thất bại lặng lẽ, không ảnh hưởng gì.
func SetRoundedCorners(hwnd uintptr) {
	pref := uint32(DWMWCP_ROUND)
	procDwmSetWindowAttribute.Call(hwnd, uintptr(DWMWA_WINDOW_CORNER_PREFERENCE), uintptr(unsafe.Pointer(&pref)), 4)
}

// ExitProcess kết thúc process NGAY LẶC LUÔN — dùng cho nút Thoát để bảo đảm
// "tắt phần mềm hoàn toàn": khi hộp thoại/popup đang mở có vòng lặp message
// lồng nhau, PostQuitMessage một mình có thể bị nuốt WM_QUIT và app không tắt.
func ExitProcess(code uint32) {
	procExitProcess.Call(uintptr(code))
}

// SetBkColorProc sets the background color used by GDI text.
func SetBkColorProc(hdc uintptr, c Color) { procSetBkColor.Call(hdc, uintptr(c)) }

// SetTextColorProc sets the GDI text color.
func SetTextColorProc(hdc uintptr, c Color) { procSetTextColor.Call(hdc, uintptr(c)) }

var _ = color.RGBA{}
