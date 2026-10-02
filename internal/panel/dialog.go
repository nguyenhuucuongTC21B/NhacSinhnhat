//go:build windows && !nogui

package panel

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"nhac-sinh-nhat/internal/store"
	"nhac-sinh-nhat/internal/w32"
)

const (
	dlgClassName = "NhacSinhNhatPanelDlg"
	dlgW, dlgH   = int32(440), int32(508)
	dlgTitleH    = int32(46)
	dlgFieldH    = int32(38)

	wmDlgSave   = w32.WM_APP + 11
	wmDlgCancel = w32.WM_APP + 12
)

// control ids
const (
	idcName = 101 + iota
	idcDate
	idcWarn
	idcNote
	idcMP3
)

type editDlg struct {
	owner    uintptr
	hwnd     uintptr
	inst     uintptr
	mode     int    // 0 = add, 1 = edit
	rowIdx   int    // index into panel rows (for edit)
	origKey  string // khóa định danh của người đang sửa (chống trùng tên)
	storeIdx int

	canvas  *w32.Canvas
	edits   map[int]uintptr
	oldProc map[uintptr]uintptr
	focus   int
	kind    store.CalendarKind
	icon24  uintptr // icon bánh trên thanh tiêu đề hộp thoại

	hoverClose, hoverSave, hoverCancel, hoverBrowse bool
	pressSave, pressCancel, pressBrowse             bool

	errMsg string
}

var dlg *editDlg
var dlgClassRegistered bool

// openEditDialog shows the add (idx<0) / edit modal dialog.
func (pp *Panel) openEditDialog(rowIdx int) {
	if dlg != nil {
		w32.SetForegroundWindow(dlg.hwnd)
		return
	}
	d := &editDlg{
		owner:   pp.hwnd,
		inst:    pp.deps.HInstance,
		mode:    0,
		rowIdx:  rowIdx,
		edits:   map[int]uintptr{},
		oldProc: map[uintptr]uintptr{},
		kind:    store.Solar,
	}
	if rowIdx >= 0 && rowIdx < len(pp.rows) {
		d.mode = 1
		d.origKey = pp.rows[rowIdx].key // chốt định danh ngay lúc mở (tránh nhầm người khi trùng tên)
	}
	if !d.create(pp) {
		dlg = nil
		return
	}
}

func (d *editDlg) create(pp *Panel) bool {
	if !dlgClassRegistered {
		wc := w32.WNDCLASSEXW{
			CbSize:      uint32(unsafe.Sizeof(w32.WNDCLASSEXW{})),
			Style:       w32.CS_DROPSHADOW,
			LpfnWndProc: syscall.NewCallback(dlgWndProc),
			HInstance:   d.inst,
			HCursor:     w32.LoadCursor(w32.MakeIntResource(w32.IDC_ARROW)),
		}
		wc.LpszClassName = w32.UTF16(dlgClassName)
		if w32.RegisterClassEx(&wc) == 0 {
			pp.logf("dialog: RegisterClassEx thất bại")
			return false
		}
		dlgClassRegistered = true // đăng ký ĐÚNG 1 LẦN — nếu không, Thêm/Sửa chỉ mở được lần đầu
	}

	// center on owner
	orc := w32.GetWindowRect(d.owner)
	x := orc.Left + (orc.W()-dlgW)/2
	y := orc.Top + (orc.H()-dlgH)/3

	title := "Thêm người mới"
	d.storeIdx = -1
	if d.mode == 1 {
		title = "Sửa thông tin"
	}

	d.hwnd = w32.CreateWindowEx(0, dlgClassName, title,
		w32.WS_POPUP, x, y, dlgW, dlgH, d.owner, 0, d.inst)
	if d.hwnd == 0 {
		pp.logf("dialog: CreateWindowEx thất bại")
		return false
	}
	w32.SetWindowRgn(d.hwnd, 0, 0, dlgW, dlgH, 14)
	w32.EnableDarkTitleBar(d.hwnd)
	// icon bánh sinh nhật đồng bộ với panel (Alt-Tab, thanh tác vụ)
	if d.icon24 == 0 {
		d.icon24 = w32.CreateCakeIcon(24)
	}
	w32.SendMessage(d.hwnd, w32.WM_SETICON, w32.ICON_SMALL, w32.CreateCakeIcon(16))
	w32.SendMessage(d.hwnd, w32.WM_SETICON, w32.ICON_BIG, d.icon24)

	// create EDIT controls
	mk := func(id int, val string, x, y, w int32, numbers bool) {
		h := w32.CreateEdit(d.hwnd, d.inst, id, val, x+2, y+2, w-4, dlgFieldH-4, numbers)
		d.edits[id] = h
	}
	mk(idcName, "", 16, 78, dlgW-32, false)
	mk(idcDate, "", 16, 150, 130, false)
	mk(idcWarn, "3", 16, 222, 90, true)
	mk(idcNote, "", 16, 294, dlgW-32, false)
	mk(idcMP3, "", 16, 366, 270, false)

	// prefill in edit mode
	if d.mode == 1 {
		r := pp.rows[d.rowIdx]
		d.kind = r.person.Kind
		w32.SetWindowText(d.edits[idcName], r.person.Name)
		w32.SetWindowText(d.edits[idcDate], r.person.DateString())
		w32.SetWindowText(d.edits[idcWarn], strconv.Itoa(r.person.WarnDays))
		w32.SetWindowText(d.edits[idcNote], r.person.Note)
		w32.SetWindowText(d.edits[idcMP3], r.person.MP3)
	}

	// subclass edits: Enter → save, Esc → cancel, Tab → next
	subProc := syscall.NewCallback(dlgEditProc)
	for _, h := range d.edits {
		d.oldProc[h] = w32.SetWindowProc(h, subProc)
	}

	w32.EnableWindow(d.owner, false)
	w32.ShowWindow(d.hwnd, w32.SW_SHOW)
	w32.SetForegroundWindow(d.hwnd)
	w32.SetFocusTo(d.edits[idcName])
	dlg = d
	return true
}

func (d *editDlg) close() {
	if d == nil {
		return
	}
	hwnd := d.hwnd
	d.hwnd = 0
	if hwnd != 0 {
		w32.DestroyWindow(hwnd)
	}
}

func dlgWndProc(hwnd, msg, wp, lp uintptr) (rc uintptr) {
	defer w32.RecoverWndProc("dialog") // panic trong UI → log, không giết app
	if dlg == nil {
		return w32.DefWindowProc(hwnd, msg, wp, lp)
	}
	d := dlg
	switch msg {
	case w32.WM_DESTROY:
		// GỠ SUBCLASS các edit TRƯỚC khi dlg = nil — nếu không, các
		// message teardown của edit đi vào dlgEditProc với dlg=nil và
		// không bao giờ tới được proc gốc của EDIT (nguồn lỗi khi mở
		// hộp thoại lần thứ hai trở đi).
		for h, old := range d.oldProc {
			if old != 0 {
				w32.SetWindowProc(h, old)
			}
		}
		d.oldProc = map[uintptr]uintptr{}
		d.canvas.Free()
		d.canvas = nil
		w32.EnableWindow(d.owner, true)
		w32.SetForegroundWindow(d.owner)
		dlg = nil
		return 0

	case w32.WM_ERASEBKGND:
		return 1

	case w32.WM_PAINT:
		d.paint()
		return 0

	case w32.WM_CTLCOLOREDIT:
		hdc := wp
		w32.SetBkColorProc(hdc, w32.CEditBg)
		w32.SetTextColorProc(hdc, w32.CText)
		return editBrushHandle()

	case w32.WM_COMMAND:
		hi := uint16(uint32(wp) >> 16) // notification code
		id := int(uint16(uint32(wp)))
		if hi == 0x0100 { // EN_SETFOCUS
			d.focus = id
			w32.InvalidateRect(hwnd, false)
		} else if hi == 0x0200 { // EN_KILLFOCUS
			if d.focus == id {
				d.focus = 0
			}
			w32.InvalidateRect(hwnd, false)
		}
		return 0

	case w32.WM_MOUSEMOVE:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		hc := d.closeRect().contains(x, y)
		hs := d.saveRect().contains(x, y)
		hcl := d.cancelRect().contains(x, y)
		hbr := d.browseRect().contains(x, y)
		hseg := d.segRect().contains(x, y)
		if hc != d.hoverClose || hs != d.hoverSave || hcl != d.hoverCancel || hbr != d.hoverBrowse {
			d.hoverClose, d.hoverSave, d.hoverCancel, d.hoverBrowse = hc, hs, hcl, hbr
			w32.InvalidateRect(hwnd, false)
		}
		w32.TrackMouseLeave(hwnd)
		if hc || hs || hcl || hbr || hseg {
			w32.SetCursor(w32.LoadCursor(w32.MakeIntResource(w32.IDC_HAND)))
		}
		return 0

	case w32.WM_MOUSELEAVE:
		d.hoverClose, d.hoverSave, d.hoverCancel, d.hoverBrowse = false, false, false, false
		w32.InvalidateRect(hwnd, false)
		return 0

	case w32.WM_LBUTTONDOWN:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		d.pressSave = d.saveRect().contains(x, y)
		d.pressCancel = d.cancelRect().contains(x, y)
		d.pressBrowse = d.browseRect().contains(x, y)
		if d.closeRect().contains(x, y) {
			w32.PostMessage(hwnd, wmDlgCancel, 0, 0)
			return 0
		}
		if d.segRect().contains(x, y) {
			if x < d.segRect().x+dlgSegW() {
				d.kind = store.Solar
			} else {
				d.kind = store.Lunar
			}
			w32.InvalidateRect(hwnd, false)
			return 0
		}
		w32.InvalidateRect(hwnd, false)
		return 0

	case w32.WM_LBUTTONUP:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		save := d.pressSave && d.saveRect().contains(x, y)
		cancel := d.pressCancel && d.cancelRect().contains(x, y)
		browse := d.pressBrowse && d.browseRect().contains(x, y)
		d.pressSave, d.pressCancel, d.pressBrowse = false, false, false
		w32.InvalidateRect(hwnd, false)
		switch {
		case save:
			w32.PostMessage(hwnd, wmDlgSave, 0, 0)
		case cancel:
			w32.PostMessage(hwnd, wmDlgCancel, 0, 0)
		case browse:
			d.pickMP3()
		}
		return 0

	case wmDlgSave:
		d.save()
		return 0

	case wmDlgCancel:
		d.close()
		return 0
	}
	return w32.DefWindowProc(hwnd, msg, wp, lp)
}

var editBrush uintptr

func editBrushHandle() uintptr {
	if editBrush == 0 {
		editBrush = w32.CreateSolidBrush(uint32(w32.CEditBg))
	}
	return editBrush
}

// dlgEditProc subclasses the EDIT controls for Enter/Esc/Tab.
func dlgEditProc(hwnd, msg, wp, lp uintptr) uintptr {
	defer w32.RecoverWndProc("dialog-edit")
	if dlg == nil {
		return w32.DefWindowProc(hwnd, msg, wp, lp)
	}
	old := dlg.oldProc[hwnd]
	if old == 0 {
		// không biết proc gốc — trả về an toàn thay vì CallWindowProc(0)
		return w32.DefWindowProc(hwnd, msg, wp, lp)
	}
	switch msg {
	case w32.WM_KEYDOWN:
		switch wp {
		case w32.VK_RETURN:
			w32.PostMessage(dlg.hwnd, wmDlgSave, 0, 0)
			return 0
		case w32.VK_ESCAPE:
			w32.PostMessage(dlg.hwnd, wmDlgCancel, 0, 0)
			return 0
		case w32.VK_TAB:
			order := []int{idcName, idcDate, idcWarn, idcNote, idcMP3}
			cur := -1
			for i, id := range order {
				if dlg.edits[id] == hwnd {
					cur = i
				}
			}
			if cur >= 0 {
				next := (cur + 1) % len(order)
				w32.SetFocusTo(dlg.edits[order[next]])
			}
			return 0
		}
	}
	return w32.CallWindowProc(old, hwnd, msg, wp, lp)
}

// geometry -----------------------------------------------------------------

func dlgSegW() int32 { return 95 }

type irect struct{ x, y, w, h int32 }

func (r irect) contains(x, y int32) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

func (d *editDlg) closeRect() irect  { return irect{dlgW - 46, 8, 38, 30} }
func (d *editDlg) segRect() irect    { return irect{158, 150, dlgSegW() * 2, dlgFieldH} }
func (d *editDlg) browseRect() irect { return irect{dlgW - 16 - 120, 366, 120, dlgFieldH} }
func (d *editDlg) saveRect() irect   { return irect{dlgW - 16 - 120, dlgH - 58, 120, 42} }
func (d *editDlg) cancelRect() irect { return irect{dlgW - 16 - 120 - 8 - 92, dlgH - 58, 92, 42} }

// actions ------------------------------------------------------------------

func (d *editDlg) pickMP3() {
	f, errDlg := w32.BrowseForFile(d.hwnd, "Chọn file MP3 riêng cho người này")
	if errDlg != nil {
		// KHÔNG im lặng nữa: hiện lỗi rõ ràng + ghi log
		d.showError("Không mở được hộp thoại chọn file")
		w32.MessageBox(d.hwnd,
			"Không mở được hộp thoại chọn file MP3:\n"+errDlg.Error()+
				"\n\nChi tiết đã ghi vào data/nhat-ky.log.",
			"Duyệt MP3 — lỗi", w32.MB_OK|w32.MB_ICONWARNING|w32.MB_SETFOREGROUND)
		return
	}
	if f != "" {
		w32.SetWindowText(d.edits[idcMP3], f)
		w32.InvalidateRect(d.hwnd, false)
	}
}

func (d *editDlg) save() {
	name := strings.TrimSpace(w32.GetWindowText(d.edits[idcName]))
	dateStr := strings.TrimSpace(w32.GetWindowText(d.edits[idcDate]))
	warnStr := strings.TrimSpace(w32.GetWindowText(d.edits[idcWarn]))
	note := w32.GetWindowText(d.edits[idcNote])
	mp3 := strings.TrimSpace(w32.GetWindowText(d.edits[idcMP3]))

	day, month, err := store.ParseDayMonth(dateStr)
	if err != nil {
		d.showError("Ngày sinh không đúng — hãy nhập dạng ngày/tháng, ví dụ 15/8")
		return
	}
	warn := 3
	if warnStr != "" {
		if v, e := strconv.Atoi(warnStr); e == nil && v >= 0 && v <= 90 {
			warn = v
		}
	}
	person := store.Person{
		Name:     name,
		Day:      day,
		Month:    month,
		Kind:     d.kind,
		WarnDays: warn,
		Note:     note,
		MP3:      mp3,
	}
	if err := person.Valid(); err != nil {
		d.showError(err.Error())
		return
	}

	if p == nil || p.deps.Store == nil {
		return
	}
	st := p.deps.Store
	if d.mode == 1 {
		// tra lại vị trí theo khóa đã chốt khi mở — đúng người cả khi trùng tên
		idx := p.storeIndexOfKey(d.origKey)
		if idx < 0 {
			d.showError("Người này vừa bị xóa hoặc thay đổi ngoài ứng dụng — hãy đóng hộp thoại rồi thử lại.")
			return
		}
		err = st.Update(idx, person)
	} else {
		err = st.Add(person)
	}
	if err != nil {
		if strings.Contains(err.Error(), "bind") || strings.Contains(err.Error(), "open") || strings.Contains(err.Error(), "save") {
			d.showError("Không ghi được file Excel (có thể đang mở trong Excel). Hãy đóng file rồi thử lại.")
		} else {
			d.showError(err.Error())
		}
		return
	}

	p.selKey = personKey(person)
	p.refresh()
	if p.deps.OnDataChanged != nil {
		p.deps.OnDataChanged()
	}
	d.close()
}

func (d *editDlg) showError(msg string) {
	d.errMsg = msg
	w32.InvalidateRect(d.hwnd, false)
}

// painting -----------------------------------------------------------------

func (d *editDlg) paint() {
	var ps w32.PAINTSTRUCT
	hdc := w32.BeginPaint(d.hwnd, &ps)
	defer w32.EndPaint(d.hwnd, &ps)
	if d.canvas == nil {
		d.canvas = w32.NewCanvas(d.hwnd, dlgW, dlgH)
	}
	if d.canvas == nil {
		return
	}
	c := d.canvas.HDC
	w32.FillRectC(c, 0, 0, dlgW, dlgH, w32.CBg)

	// title bar
	title := "Thêm người mới"
	if d.mode == 1 {
		title = "Sửa thông tin"
	}
	w32.FillRectC(c, 0, 0, dlgW, dlgTitleH, w32.CCard)
	w32.Gradient(c, 0, dlgTitleH-3, dlgW, 3, w32.CAccentDk, w32.CPink, false)
	if d.icon24 == 0 {
		d.icon24 = w32.CreateCakeIcon(24)
	}
	w32.DrawIconEx(c, 14, (dlgTitleH-24)/2, d.icon24, 24, 24, 0, 0)
	w32.DrawTextC(c, w32.FBodySb, title, 48, 12, 300, w32.CText, w32.DT_LEFT)
	if d.hoverClose {
		w32.RoundRectBorder(c, dlgW-46, 8, 38, 30, 8, w32.CRed, w32.CRed)
	} else {
		w32.RoundRectBorder(c, dlgW-46, 8, 38, 30, 8, w32.CCard, w32.CCard)
	}
	// draw ✕ as two crossed lines
	pen, _, _ := w32.CreatePenProc(2, w32.CTextDim)
	oldPen, _, _ := w32.SelectObjectProc(c, pen)
	ccx, ccy := int32(dlgW)-46+19, int32(8+15)
	w32.LineTo2(c, ccx-5, ccy-5, ccx+5, ccy+5)
	w32.LineTo2(c, ccx+5, ccy-5, ccx-5, ccy+5)
	w32.SelectObjectProc(c, oldPen)
	w32.DeleteObject(pen)

	// labels
	label := func(txt string, x, y int32) {
		w32.DrawTextC(c, w32.FSmall, txt, x, y, 380, w32.CTextDim, w32.DT_LEFT)
	}
	label("Họ và tên *", 16, 56)
	label("Ngày / Tháng (vd: 15/8)", 16, 128)
	label("Nhắc trước (ngày)", 16, 200)
	label("Ghi chú", 16, 272)
	label("File MP3 riêng (tuỳ chọn)", 16, 344)

	// edits
	for id := range d.edits {
		g := d.editRect(id)
		border := w32.CBorderHi
		if d.focus == id {
			border = w32.CAccent
		}
		w32.RoundRectBorder(c, g.x, g.y, g.w, g.h, 8, w32.CEditBg, border)
	}

	// segmented DL/AL
	sr := d.segRect()
	w32.RoundRectBorder(c, sr.x, sr.y, sr.w, sr.h, 8, w32.CEditBg, w32.CBorderHi)
	activeTxt := w32.Col(0xFF, 0xFF, 0xFF)
	if d.kind == store.Solar {
		w32.RoundRectBorder(c, sr.x+2, sr.y+2, dlgSegW()-4, sr.h-4, 7, w32.CAccent, w32.CAccent)
		w32.DrawTextC(c, w32.FSmallSb, "Dương lịch", sr.x, sr.y+9, dlgSegW(), activeTxt, w32.DT_CENTER)
		w32.DrawTextC(c, w32.FSmall, "Âm lịch", sr.x+dlgSegW(), sr.y+9, dlgSegW(), w32.CTextDim, w32.DT_CENTER)
	} else {
		w32.RoundRectBorder(c, sr.x+dlgSegW()+2, sr.y+2, dlgSegW()-4, sr.h-4, 7, w32.CAccent, w32.CAccent)
		w32.DrawTextC(c, w32.FSmall, "Dương lịch", sr.x, sr.y+9, dlgSegW(), w32.CTextDim, w32.DT_CENTER)
		w32.DrawTextC(c, w32.FSmallSb, "Âm lịch", sr.x+dlgSegW(), sr.y+9, dlgSegW(), activeTxt, w32.DT_CENTER)
	}
	w32.DrawTextC(c, w32.FTiny, "Sinh nhật theo dương/âm lịch", sr.x, sr.y+dlgFieldH+4, sr.w, w32.CTextFaint, w32.DT_CENTER)

	// warn hint
	w32.DrawTextC(c, w32.FTiny, "ngày — 0 = chỉ nhắc đúng ngày sinh nhật", 116, 232, 320, w32.CTextFaint, w32.DT_LEFT)

	// error
	if d.errMsg != "" {
		w32.DrawTextC(c, w32.FSmall, "⚠ "+d.errMsg, 16, 414, dlgW-32, w32.CRed, w32.DT_LEFT)
	}

	// buttons
	drawButton(c, button{id: btnAdd, x: d.saveRect().x, y: d.saveRect().y, w: d.saveRect().w, h: d.saveRect().h,
		label: "Lưu lại"}, d.hoverSave, d.pressSave)
	drawButton(c, button{id: btnEdit, x: d.cancelRect().x, y: d.cancelRect().y, w: d.cancelRect().w, h: d.cancelRect().h,
		label: "Hủy"}, d.hoverCancel, d.pressCancel)
	drawButton(c, button{id: btnOpenExcel, x: d.browseRect().x, y: d.browseRect().y, w: d.browseRect().w, h: d.browseRect().h,
		label: "Duyệt…"}, d.hoverBrowse, d.pressBrowse)

	// blit the finished frame to screen
	w32.BitBltTo(hdc, 0, 0, dlgW, dlgH, c, 0, 0)
}

func (d *editDlg) editRect(id int) irect {
	switch id {
	case idcName:
		return irect{16, 78, dlgW - 32, dlgFieldH}
	case idcDate:
		return irect{16, 150, 130, dlgFieldH}
	case idcWarn:
		return irect{16, 222, 90, dlgFieldH}
	case idcNote:
		return irect{16, 294, dlgW - 32, dlgFieldH}
	case idcMP3:
		return irect{16, 366, 270, dlgFieldH}
	}
	return irect{}
}

var (
	_ = fmt.Sprintf
	_ = unsafe.Pointer(nil)
	_ = w32.CText
)
