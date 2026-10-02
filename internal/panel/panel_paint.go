//go:build windows && !nogui

package panel

import (
	"fmt"
	"strings"
	"time"

	"nhac-sinh-nhat/internal/store"
	"nhac-sinh-nhat/internal/w32"
)

func (pp *Panel) paint() {
	var ps w32.PAINTSTRUCT
	hdc := w32.BeginPaint(pp.hwnd, &ps)
	defer w32.EndPaint(pp.hwnd, &ps)
	rc := w32.GetClientRect(pp.hwnd)
	if rc.W() <= 0 || rc.H() <= 0 {
		return
	}
	if pp.canvas == nil || pp.canvas.W != rc.W() || pp.canvas.H != rc.H() {
		pp.canvas.Free()
		pp.canvas = w32.NewCanvas(pp.hwnd, rc.W(), rc.H())
	}
	if pp.canvas == nil {
		return
	}
	pp.render(pp.canvas.HDC, rc)
	w32.BitBltTo(hdc, 0, 0, rc.W(), rc.H(), pp.canvas.HDC, 0, 0)
}

func (pp *Panel) ensureIcons() {
	if pp.icon == 0 {
		pp.icon = w32.CreateCakeIcon(32)
	}
	if pp.icon16 == 0 {
		pp.icon16 = w32.CreateCakeIcon(16)
	}
	if pp.icon48 == 0 {
		pp.icon48 = w32.CreateCakeIcon(48)
	}
	if pp.icon72 == 0 {
		pp.icon72 = w32.CreateCakeIcon(72)
	}
}

func (pp *Panel) render(hdc uintptr, rc w32.RECT) {
	pp.ensureIcons()
	pp.layoutButtons(rc.W(), rc.H())
	w := rc.W()
	h := rc.H()

	// page background
	w32.FillRectC(hdc, 0, 0, w, h, w32.CBg)

	pp.drawHeader(hdc, w)
	pp.drawButtons(hdc)
	pp.drawList(hdc, w, h)
	pp.drawFooter(hdc, w, h)
}

func (pp *Panel) drawHeader(hdc uintptr, w int32) {
	// diagonal-ish gradient: horizontal violet → pink
	w32.Gradient(hdc, 0, 0, w, headerH, w32.CAccentDk, w32.CPink, false)

	// decorative translucent bubbles (blend white over gradient mid tone)
	mid := w32.CAccentDk.Blend(w32.CPink, 0.55)
	bubble := mid.Blend(w32.Col(0xFF, 0xFF, 0xFF), 0.10)
	bubble2 := mid.Blend(w32.Col(0xFF, 0xFF, 0xFF), 0.16)
	w32.EllipseC(hdc, w-150, -46, 170, 170, bubble)
	w32.EllipseC(hdc, w-60, 40, 110, 110, bubble2)
	w32.EllipseC(hdc, 210, -60, 120, 120, bubble)

	// pháo giấy sinh nhật — toạ độ cố định nên không nhấp nháy giữa các lần paint
	drawConfetti(hdc, w, mid)

	// quầng sáng sau icon bánh
	w32.EllipseC(hdc, 14, 14, 64, 64, mid.Blend(w32.Col(0xFF, 0xFF, 0xFF), 0.12))

	// cake icon
	w32.DrawIconEx(hdc, 22, 24, pp.icon48, 48, 48, 0, 0)

	// title + subtitle
	w32.DrawTextC(hdc, w32.FTitle, "Nhắc Sinh Nhật", 84, 18, w-84-180, w32.Col(0xFF, 0xFF, 0xFF), w32.DT_LEFT)
	subCol := mid.Blend(w32.Col(0xFF, 0xFF, 0xFF), 0.78)
	sub := "Bảng điều khiển quản lý sinh nhật"
	if pp.deps.Version != "" {
		sub += " • v" + pp.deps.Version
	}
	w32.DrawTextC(hdc, w32.FSmall, sub, 85, 56, w-85-180, subCol, w32.DT_LEFT)

	// count pill on the right
	n := len(pp.rows)
	label := fmt.Sprintf("%d người", n)
	tw, _ := w32.MeasureText(hdc, w32.FSmallSb, label)
	pw, ph := tw+30, int32(32)
	px, py := w-24-pw, (headerH-ph)/2
	pillBg := mid.Blend(w32.Col(0xFF, 0xFF, 0xFF), 0.14)
	pillBd := mid.Blend(w32.Col(0xFF, 0xFF, 0xFF), 0.34)
	w32.RoundRectBorder(hdc, px, py, pw, ph, 16, pillBg, pillBd)
	w32.DrawTextC(hdc, w32.FSmallSb, label, px, py+ph/2-9, pw, w32.Col(0xFF, 0xFF, 0xFF), w32.DT_CENTER)

	// viền dưới header tạo chiều sâu
	w32.Line(hdc, 0, headerH, w, headerH, w32.Col(0x0A, 0x10, 0x1E))
}

// drawConfetti vẽ chấm màu trang trí trên dải header — toạ độ tính theo tỉ lệ
// chiều rộng nên ổn định giữa các lần resize, alpha thấp để không chắn chữ.
func drawConfetti(hdc uintptr, w int32, base w32.Color) {
	type dot struct {
		fx   float64
		y, r int32
		c    w32.Color
		a    float64
	}
	dots := []dot{
		{0.075, 14, 3, w32.CYellow, 0.70},
		{0.125, 70, 2, w32.Col(0xFF, 0xFF, 0xFF), 0.30},
		{0.190, 30, 2, w32.CBlue, 0.75},
		{0.245, 78, 3, w32.CGreen, 0.50},
		{0.315, 18, 2, w32.CYellow, 0.60},
		{0.385, 60, 2, w32.Col(0xFF, 0xFF, 0xFF), 0.28},
		{0.460, 34, 3, w32.CRed, 0.50},
		{0.530, 74, 2, w32.CBlue, 0.60},
		{0.600, 16, 2, w32.CGreen, 0.60},
		{0.665, 52, 3, w32.CYellow, 0.55},
		{0.730, 24, 2, w32.Col(0xFF, 0xFF, 0xFF), 0.30},
	}
	for _, d := range dots {
		x := int32(float64(w) * d.fx)
		w32.EllipseC(hdc, x, d.y, d.r*2, d.r*2, d.c.Blend(base, d.a))
	}
}

// avatarTone chọn màu avatar ổn định theo tên — cùng tên luôn cùng màu.
func avatarTone(name string) w32.Color {
	palette := []w32.Color{w32.CAccent, w32.CPink, w32.CGreen, w32.CBlue, w32.COrange, w32.CAmber}
	var h uint32
	for _, c := range []byte(name) {
		h = h*31 + uint32(c)
	}
	return palette[h%uint32(len(palette))]
}

// avatarInitial lấy chữ cái đầu của tên (hỗ trợ Unicode/tiếng Việt).
func avatarInitial(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return "?"
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0]))
}

func (pp *Panel) drawButtons(hdc uintptr) {
	for _, b := range pp.buttons {
		hover := pp.hoverBtn == int(b.id)
		pressed := pp.pressBtn == int(b.id)
		drawButton(hdc, b, hover, pressed)
	}
}

func drawButton(hdc uintptr, b button, hover, pressed bool) {
	var fill, border, text w32.Color
	switch b.style {
	case stAccent:
		fill, text = w32.CAccent, w32.Col(0xFF, 0xFF, 0xFF)
		border = w32.CAccent
		if hover {
			fill = w32.CAccentHi
		}
		if pressed {
			fill = w32.CAccentDk
		}
	case stGreen:
		fill, text = w32.CGreen, w32.Col(0xFF, 0xFF, 0xFF)
		border = w32.CGreen
		if hover {
			fill = w32.CGreen.Blend(w32.Col(0xFF, 0xFF, 0xFF), 0.18)
		}
		if pressed {
			fill = w32.CGreenDk
		}
	case stOutlineDanger:
		fill, text, border = w32.CCard, w32.CTextSoft, w32.CBorder
		if hover {
			border = w32.CRed
			text = w32.Col(0xF8, 0x71, 0x71)
			fill = w32.CRed.Blend(w32.CCard, 0.10)
		}
	default: // outline
		fill, text, border = w32.CCard, w32.CTextSoft, w32.CBorder
		if hover {
			border = w32.CAccent
			text = w32.Col(0xFF, 0xFF, 0xFF)
			fill = w32.CAccent.Blend(w32.CCard, 0.10)
		}
	}
	w32.RoundRectBorder(hdc, b.x, b.y, b.w, b.h, 9, fill, border)

	// content centered: icon + label
	tw, _ := w32.MeasureText(hdc, w32.FButton, b.label)
	iconW := int32(0)
	if b.icon != icoNone {
		iconW = 16 + 7
	}
	total := iconW + tw
	cx := b.x + (b.w-total)/2
	cy := b.y + b.h/2
	if b.icon != icoNone {
		drawIcon(hdc, b.icon, cx+8, cy, text)
	}
	w32.DrawTextC(hdc, w32.FButton, b.label, cx+iconW, cy-10, b.w, text, w32.DT_LEFT)
}

// drawIcon paints a 16px glyph centered at (cx,cy) using 2px pen lines.
func drawIcon(hdc uintptr, kind iconKind, cx, cy int32, color w32.Color) {
	pen, _, _ := w32.CreatePenProc(2, color)
	old, _, _ := w32.SelectObjectProc(hdc, pen)
	defer func() {
		w32.SelectObjectProc(hdc, old)
		w32.DeleteObject(pen)
	}()
	l := func(x1, y1, x2, y2 int32) { w32.LineTo2(hdc, cx+x1, cy+y1, cx+x2, cy+y2) }
	switch kind {
	case icoPlus:
		l(-6, 0, 6, 0)
		l(0, -6, 0, 6)
	case icoPencil:
		// diagonal body
		l(-6, 6, 3, -3)
		// tip
		l(3, -3, 6, -6)
		l(-6, 6, -3, 6)
		l(-6, 6, -6, 3)
	case icoTrash:
		// lid
		l(-6, -5, 6, -5)
		// handle
		l(-2, -5, -2, -8)
		l(-2, -8, 2, -8)
		l(2, -8, 2, -5)
		// body
		l(-4, -2, -3, 7)
		l(4, -2, 3, 7)
		l(-3, 7, 3, 7)
		// inner lines
		l(-1, 0, -1, 4)
		l(1, 0, 1, 4)
	case icoGrid:
		w32.FillRectC(hdc, cx-7, cy-7, 6, 6, color)
		w32.FillRectC(hdc, cx+1, cy-7, 6, 6, color)
		w32.FillRectC(hdc, cx-7, cy+1, 6, 6, color)
		w32.FillRectC(hdc, cx+1, cy+1, 6, 6, color)
	case icoImport:
		// arrow pointing down into a tray = bring data in
		l(0, -7, 0, 2)  // shaft
		l(-4, -2, 0, 2) // left head
		l(4, -2, 0, 2)  // right head
		// tray
		l(-6, 3, -6, 7)
		l(6, 3, 6, 7)
		l(-6, 7, 6, 7)
	case icoSave:
		// floppy outline
		l(-6, -6, 6, -6)
		l(6, -6, 6, 6)
		l(6, 6, -6, 6)
		l(-6, 6, -6, -6)
		// shutter
		l(-2, -6, -2, -2)
		l(-2, -2, 3, -2)
		l(3, -2, 3, -6)
		// label
		l(-3, 6, -3, 2)
		l(-3, 2, 3, 2)
		l(3, 2, 3, 6)
	}
}

func badgeFor(days int) (text string, tone w32.Color) {
	switch {
	case days >= 36500: // ngày không hợp lệ — không bao giờ nhắc
		return "không hợp lệ", w32.CTextFaint
	case days == 0:
		return "HÔM NAY", w32.CRed
	case days == 1:
		return "NGÀY MAI", w32.COrange
	case days <= 3:
		return fmt.Sprintf("%d ngày", days), w32.COrange
	case days <= 7:
		return fmt.Sprintf("%d ngày", days), w32.CYellow
	default:
		return fmt.Sprintf("%d ngày", days), w32.CGreen
	}
}

func (pp *Panel) drawList(hdc uintptr, w, h int32) {
	lr := w32.RECT{Left: 0, Top: listTop, Right: w, Bottom: h - footerH}

	// column geometry
	badgeRight := w - 26
	lunarRight := badgeRight - 112
	dateRight := lunarRight - 92
	nameRight := dateRight - 120

	// nhãn cột
	hdrY := lr.Top + 14
	w32.DrawTextC(hdc, w32.FSmall, "HỌ VÀ TÊN", 58, hdrY, 300, w32.CTextFaint, w32.DT_LEFT)
	w32.DrawTextC(hdc, w32.FSmall, "SINH NHẬT", dateRight-108, hdrY, 108, w32.CTextFaint, w32.DT_RIGHT)
	w32.DrawTextC(hdc, w32.FSmall, "ÂM LỊCH", lunarRight-78, hdrY, 78, w32.CTextFaint, w32.DT_RIGHT)
	w32.DrawTextC(hdc, w32.FSmall, "CÒN LẠI", badgeRight-84, hdrY, 84, w32.CTextFaint, w32.DT_RIGHT)

	if len(pp.rows) == 0 {
		pp.drawEmptyState(hdc, lr)
		return
	}

	// clip vùng danh sách: hàng cuộn bị cắt gọn tại đây, không tràn lên
	// nhãn cột hay xuống footer
	clip := w32.PushClip(hdc, 0, lr.Top+listHdrH+6, w, lr.H()-listHdrH-6)

	// rows — thẻ bo góc + avatar chữ cái đầu
	for i, r := range pp.rows {
		yTop := lr.Top + listHdrH + 8 + int32(i)*rowH - pp.scrollY
		if yTop+rowH < lr.Top+listHdrH+8 || yTop > lr.Bottom {
			continue
		}
		sel := r.key == pp.selKey
		bg := w32.CCard
		if i%2 == 1 {
			bg = w32.CRowAlt
		}
		border := w32.CBorder
		if i == pp.hoverRow && !sel {
			bg = w32.CRowHover
			border = w32.CBorderHi
		}
		if sel {
			bg = w32.CRowSel
			border = w32.CAccent
		}
		cardX, cardW := int32(12), w-26
		w32.RoundRectBorder(hdc, cardX, yTop+2, cardW, rowH-4, 9, bg, border)
		if sel {
			w32.FillRectC(hdc, cardX+1, yTop+9, 3, rowH-18, w32.CAccent)
		}

		// avatar tròn chữ cái đầu — màu ổn định theo tên
		tone := avatarTone(r.person.Name)
		avX, avD := cardX+10, int32(26)
		avY := yTop + (rowH-avD)/2
		w32.EllipseC(hdc, avX, avY, avD, avD, tone)
		w32.DrawTextC(hdc, w32.FSmallSb, avatarInitial(r.person.Name), avX, avY+5, avD, w32.Col(0xFF, 0xFF, 0xFF), w32.DT_CENTER)

		txtY := yTop + (rowH-20)/2
		// name
		nameX := avX + avD + 10
		w32.DrawTextC(hdc, w32.FBody, r.person.Name, nameX, txtY, nameRight-nameX, w32.CText, w32.DT_LEFT)
		// date + kind tag
		dateTxt := r.person.DateString()
		tw, _ := w32.MeasureText(hdc, w32.FSmall, dateTxt)
		w32.DrawTextC(hdc, w32.FSmall, dateTxt, dateRight-108, txtY+1, 108, w32.CTextSoft, w32.DT_RIGHT)
		kindTxt, kindCol := "(DL)", w32.CTextFaint
		if r.person.Kind == store.Lunar {
			kindTxt, kindCol = "(AL)", w32.CAmber
		}
		w32.DrawTextC(hdc, w32.FTiny, kindTxt, dateRight-108+tw+6, txtY+3, 40, kindCol, w32.DT_LEFT)
		// lunar
		w32.DrawTextC(hdc, w32.FSmall, r.lunarTxt, lunarRight-78, txtY+1, 78, w32.CTextDim, w32.DT_RIGHT)
		// badge
		badgeTxt, toneB := badgeFor(r.daysLeft)
		bw2, _ := w32.MeasureText(hdc, w32.FBadge, badgeTxt)
		pillW, pillH := bw2+22, int32(24)
		px, py := badgeRight-pillW, yTop+(rowH-pillH)/2
		pillBg := toneB.Blend(bg, 0.16)
		pillBd := toneB.Blend(bg, 0.45)
		w32.RoundRectBorder(hdc, px, py, pillW, pillH, 12, pillBg, pillBd)
		w32.DrawTextC(hdc, w32.FBadge, badgeTxt, px, py+(pillH-18)/2-1, pillW, toneB, w32.DT_CENTER)
	}
	w32.PopClip(hdc, clip)

	// scrollbar
	maxS := pp.maxScroll()
	if maxS > 0 {
		viewH := lr.H() - listHdrH
		contentH := pp.contentH()
		thumbH := int32(float64(viewH) * float64(viewH) / float64(contentH))
		if thumbH < 30 {
			thumbH = 30
		}
		trackH := viewH - 8
		thumbY := lr.Top + listHdrH + 4 + int32(float64(pp.scrollY)*float64(trackH-thumbH)/float64(maxS))
		w32.RoundRectBorder(hdc, w-13, thumbY, 6, thumbH, 3, w32.CBorderHi, w32.CBorderHi)
	}
}

func (pp *Panel) drawEmptyState(hdc uintptr, lr w32.RECT) {
	cx := (lr.Left + lr.Right) / 2
	cy := (lr.Top + lr.Bottom) / 2
	w32.DrawIconEx(hdc, cx-36, cy-64, pp.icon72, 72, 72, 0, 0)
	w32.DrawTextC(hdc, w32.FH2, "Chưa có ai trong danh sách", cx-220, cy+18, 440, w32.CTextSoft, w32.DT_CENTER)
	w32.DrawTextC(hdc, w32.FSmall, "Nhấn “Thêm” để nhập người đầu tiên, dùng “Nhập Excel”\nđể nạp file có sẵn, hoặc mở file data/danh-sach.xlsx", cx-280, cy+48, 560, w32.CTextFaint, w32.DT_CENTER)
}

func (pp *Panel) drawFooter(hdc uintptr, w, h int32) {
	top := h - footerH
	w32.FillRectC(hdc, 0, top, w, footerH, w32.CCard)
	w32.Line(hdc, 0, top, w, top, w32.CBorder)

	// thông báo tạm thời (hint) ưu tiên hiển thị ở footer
	// infoRight: mép phải vùng chữ — chừa lại nút Thoát (86px + margin)
	infoRight := w - margin - 86 - 12
	if pp.statusMsg != "" && time.Now().Before(pp.statusAt) {
		w32.EllipseC(hdc, 18, top+footerH/2-4, 8, 8, w32.CAmber)
		w32.DrawTextC(hdc, w32.FSmall, pp.statusMsg, 34, top+footerH/2-9, w-360, w32.CAmber, w32.DT_LEFT)
		info := fmt.Sprintf("%d người • data/danh-sach.xlsx", len(pp.rows))
		w32.DrawTextC(hdc, w32.FSmall, info, w-360, top+footerH/2-9, infoRight-(w-360), w32.CTextFaint, w32.DT_RIGHT)
		return
	}

	// status: nearest birthday
	if best, ok := nearestInfo(pp.rows); ok {
		_, dot := badgeFor(best.daysLeft)
		w32.EllipseC(hdc, 18, top+footerH/2-4, 8, 8, dot)
		text := fmt.Sprintf("Sinh nhật gần nhất: %s — %s", best.person.Name, badgeTextOnly(best.daysLeft))
		w32.DrawTextC(hdc, w32.FSmall, text, 34, top+footerH/2-9, w-440, w32.CTextDim, w32.DT_LEFT)
	} else {
		w32.DrawTextC(hdc, w32.FSmall, "Chưa có sinh nhật nào", 18, top+footerH/2-9, w-440, w32.CTextDim, w32.DT_LEFT)
	}
	info := fmt.Sprintf("%d người • data/danh-sach.xlsx", len(pp.rows))
	w32.DrawTextC(hdc, w32.FSmall, info, w-360, top+footerH/2-9, infoRight-(w-360), w32.CTextFaint, w32.DT_RIGHT)
}

func nearestInfo(rows []row) (row, bool) {
	if len(rows) == 0 {
		return row{}, false
	}
	best := rows[0]
	for _, r := range rows[1:] {
		if r.daysLeft < best.daysLeft {
			best = r
		}
	}
	return best, true
}

func badgeTextOnly(days int) string {
	t, _ := badgeFor(days)
	return t
}
