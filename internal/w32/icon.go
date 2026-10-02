//go:build windows && !nogui

package w32

import (
	"math"
	"syscall"
	"unsafe"
)

// --- tiny software rasterizer for the cake icon ---------------------------

type pixBuf struct {
	pix  []uint8 // BGRA straight alpha
	W, H int
}

func newPix(w, h int) *pixBuf {
	return &pixBuf{pix: make([]uint8, w*h*4), W: w, H: h}
}

// set blends (r,g,b) with coverage a onto pixel (x,y) — premultiplied-ish.
func (p *pixBuf) set(x, y int, r, g, b uint8, a float64) {
	if x < 0 || y < 0 || x >= p.W || y >= p.H || a <= 0 {
		return
	}
	if a > 1 {
		a = 1
	}
	i := (y*p.W + x) * 4
	da := float64(p.pix[i+3]) / 255
	// simple source-over
	oa := a + da*(1-a)
	if oa <= 0 {
		return
	}
	for c, src := range [...]float64{float64(b), float64(g), float64(r)} {
		dst := float64(p.pix[i+c]) / 255
		out := (src*a + dst*da*(1-a)) / oa
		p.pix[i+c] = uint8(out * 255)
	}
	p.pix[i+3] = uint8(oa * 255)
}

func (p *pixBuf) fillRectF(x0, y0, x1, y1 float64, r, g, b uint8) {
	for y := int(math.Floor(y0)); y < int(math.Ceil(y1)); y++ {
		cy := cov(y0, y1, float64(y))
		for x := int(math.Floor(x0)); x < int(math.Ceil(x1)); x++ {
			cx := cov(x0, x1, float64(x))
			p.set(x, y, r, g, b, cx*cy)
		}
	}
}

func cov(a, b, v float64) float64 {
	lo, hi := math.Min(a, b), math.Max(a, b)
	if v+1 <= lo || v >= hi {
		return 0
	}
	if v >= lo && v+1 <= hi {
		return 1
	}
	ov := math.Min(hi, v+1) - math.Max(lo, v)
	if ov < 0 {
		ov = 0
	}
	return ov
}

func (p *pixBuf) fillEllipseF(cx, cy, rx, ry float64, r, g, b uint8) {
	for y := int(cy - ry - 1); y <= int(cy+ry+1); y++ {
		for x := int(cx - rx - 1); x <= int(cx+rx+1); x++ {
			// 3x3 supersample
			acc := 0.0
			for sy := 0; sy < 3; sy++ {
				for sx := 0; sx < 3; sx++ {
					px := float64(x) + (float64(sx)+0.5)/3
					py := float64(y) + (float64(sy)+0.5)/3
					dx := (px - cx) / rx
					dy := (py - cy) / ry
					if dx*dx+dy*dy <= 1 {
						acc += 1.0 / 9
					}
				}
			}
			if acc > 0 {
				p.set(x, y, r, g, b, acc)
			}
		}
	}
}

func (p *pixBuf) fillRoundRectF(x0, y0, x1, y1, rad float64, r, g, b uint8) {
	rad = math.Min(rad, math.Min((x1-x0)/2, (y1-y0)/2))
	p.fillRectF(x0+rad, y0, x1-rad, y1, r, g, b)
	p.fillRectF(x0, y0+rad, x0+rad, y1-rad, r, g, b)
	p.fillRectF(x1-rad, y0+rad, x1, y1-rad, r, g, b)
	p.fillEllipseF(x0+rad, y0+rad, rad, rad, r, g, b)
	p.fillEllipseF(x1-rad, y0+rad, rad, rad, r, g, b)
	p.fillEllipseF(x0+rad, y1-rad, rad, rad, r, g, b)
	p.fillEllipseF(x1-rad, y1-rad, rad, rad, r, g, b)
}

// cakeBGRA paints a cute birthday cake normalized to a 48x48 design grid.
func cakeBGRA(size int) []uint8 {
	s := float64(size) / 48.0
	p := newPix(size, size)
	C := func(hex uint32) (uint8, uint8, uint8) {
		r := uint8((hex >> 16) & 0xFF)
		g := uint8((hex >> 8) & 0xFF)
		b := uint8(hex & 0xFF)
		return r, g, b
	}
	// plate
	pR, pG, pB := C(0xCBD5E1)
	p.fillEllipseF(24*s, 43.5*s, 19*s, 3.2*s, pR, pG, pB)
	// bottom layer
	r1, g1, b1 := C(0xF472B6)
	p.fillRoundRectF(12*s, 29*s, 36*s, 43*s, 3*s, r1, g1, b1)
	// frosting bottom
	fr, fg, fb := C(0xFDF2F8)
	p.fillRoundRectF(12*s, 28*s, 36*s, 31.5*s, 1.5*s, fr, fg, fb)
	for _, dx := range []float64{14.5, 19, 23.5, 28, 32.5} {
		p.fillEllipseF(dx*s, 31.5*s, 1.7*s, 2.1*s, fr, fg, fb)
	}
	// top layer
	r2, g2, b2 := C(0xFBBF24)
	p.fillRoundRectF(16.5*s, 17*s, 31.5*s, 27.5*s, 2.5*s, r2, g2, b2)
	fr2, fg2, fb2 := C(0xFEF3C7)
	p.fillRoundRectF(16.5*s, 16*s, 31.5*s, 19*s, 1.5*s, fr2, fg2, fb2)
	for _, dx := range []float64{19, 23, 27, 30} {
		p.fillEllipseF(dx*s, 19*s, 1.5*s, 1.9*s, fr2, fg2, fb2)
	}
	// sprinkles on bottom layer
	for _, pt := range [][2]float64{{16, 35}, {21, 38}, {26, 34.5}, {31, 37}, {18, 40.5}, {29, 41}, {24, 36.5}} {
		sr, sg, sb := C(0xFEF3C7)
		p.fillEllipseF(pt[0]*s, pt[1]*s, 0.8*s, 0.8*s, sr, sg, sb)
	}
	// candles
	candles := []struct {
		x  float64
		h  float64
		cl uint32
	}{{20, 9.0, 0x60A5FA}, {24, 10.5, 0x34D399}, {28, 8.0, 0xF87171}}
	for _, cd := range candles {
		cr, cg, cb := C(cd.cl)
		p.fillRoundRectF((cd.x-1.1)*s, (27.5-cd.h)*s, (cd.x+1.1)*s, 16.5*s, 1.0*s, cr, cg, cb)
		// flame
		fr3, fg3, fb3 := C(0xFB923C)
		p.fillEllipseF(cd.x*s, (27.5-cd.h)-1.8*s, 1.7*s, 2.6*s, fr3, fg3, fb3)
		yr, yg, yb := C(0xFDE68A)
		p.fillEllipseF(cd.x*s, (27.5-cd.h)-1.5*s, 0.9*s, 1.5*s, yr, yg, yb)
	}
	return p.pix
}

// CreateCakeIcon builds a 32bpp HICON of the cake at the given size.
func CreateCakeIcon(size int) uintptr {
	bgra := cakeBGRA(size)
	hdcScreen := GetDC(0)
	defer ReleaseDC(0, hdcScreen)

	bi := BITMAPINFO{}
	bi.Header.BiSize = uint32(unsafe.Sizeof(bi.Header))
	bi.Header.BiWidth = int32(size)
	bi.Header.BiHeight = -int32(size)
	bi.Header.BiPlanes = 1
	bi.Header.BiBitCount = 32
	bi.Header.BiCompression = BI_RGB

	var bits unsafe.Pointer
	hColor, _, _ := procCreateDIBSection.Call(hdcScreen, uintptr(unsafe.Pointer(&bi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hColor == 0 {
		return 0
	}
	copy((*[1 << 24]uint8)(bits)[:size*size*4], bgra)

	// 1bpp mask (all zeros = use alpha)
	maskHdr := BITMAPINFO{}
	maskHdr.Header.BiSize = uint32(unsafe.Sizeof(maskHdr.Header))
	maskHdr.Header.BiWidth = int32(size)
	maskHdr.Header.BiHeight = int32(size) // bottom-up fine
	maskHdr.Header.BiPlanes = 1
	maskHdr.Header.BiBitCount = 1
	maskHdr.Header.BiCompression = BI_RGB
	maskW := ((size + 31) / 32) * 4 // row bytes
	var maskBits unsafe.Pointer
	hMask, _, _ := procCreateDIBSection.Call(hdcScreen, uintptr(unsafe.Pointer(&maskHdr)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&maskBits)), 0, 0)
	if hMask != 0 && maskBits != nil {
		for i := range maskW * size {
			(*[1 << 24]uint8)(maskBits)[i] = 0
		}
	}

	ii := ICONINFO{FIcon: 1, HbmMask: hMask, HbmColor: hColor}
	hIcon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	DeleteObject(hColor)
	if hMask != 0 {
		DeleteObject(hMask)
	}
	return hIcon
}

var _ = syscall.UTF16FromString
