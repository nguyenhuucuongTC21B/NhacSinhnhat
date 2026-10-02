//go:build windows && !nogui

// Package panel vẽ bảng điều khiển sinh nhật bằng GDI thuần — chủ đề tối,
// gradient tím/hồng, danh sách có badge đếm ngược, nút bo tròn có hover.
package panel

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"nhac-sinh-nhat/internal/store"
	"nhac-sinh-nhat/internal/w32"
)

const (
	className   = "NhacSinhNhatPanelWnd"
	wmAppReload = w32.WM_APP + 10 // external data changed
)

const (
	headerH  int32 = 96
	toolbarY int32 = 112
	toolbarH int32 = 40
	listTop  int32 = 168
	listHdrH int32 = 30
	rowH     int32 = 40
	footerH  int32 = 40
	margin   int32 = 16
)

// Deps are the services the panel needs.
type Deps struct {
	HInstance     uintptr
	Store         *store.Store
	Version       string // hiển thị trên header (vd "3.3.0")
	Logf          func(string, ...interface{})
	OnDataChanged func() // after add/edit/delete → re-run reminder scan
	OnOpenExcel   func() // open the Excel file with default app
	OnExit        func() // Thoát — tắt hẳn phần mềm (khác với đóng cửa sổ)
}

type buttonID int

const (
	btnAdd buttonID = iota
	btnEdit
	btnDelete
	btnImport
	btnOpenExcel
	btnSaveExcel
	btnExit // nút Thoát ở footer — đường thoát luôn sẵn có
	btnCount
)

type btnStyle int

const (
	stAccent btnStyle = iota
	stGreen
	stOutline
	stOutlineDanger
)

type iconKind int

const (
	icoNone iconKind = iota
	icoPlus
	icoPencil
	icoTrash
	icoImport
	icoGrid
	icoSave
)

type button struct {
	id    buttonID
	x, y  int32
	w, h  int32
	label string
	icon  iconKind
	style btnStyle
}

type row struct {
	person   store.Person
	key      string // định danh ổn định: tên|loại|ngày|nhắc|ghi chú|mp3
	daysLeft int
	next     time.Time
	lunarTxt string
}

// personKey tạo khóa định danh cho từng người — tránh nhầm người khi danh sách
// có hai người trùng tên (khác ngày sinh / ghi chú / nhạc).
func personKey(p store.Person) string {
	return strings.ToLower(strings.TrimSpace(p.Name)) + "|" + p.KindString() + "|" + p.DateString() +
		"|" + strconv.Itoa(p.WarnDays) + "|" + p.Note + "|" + p.MP3
}

type Panel struct {
	deps      Deps
	hwnd      uintptr
	icon      uintptr
	icon16    uintptr
	icon48    uintptr
	icon72    uintptr
	canvas    *w32.Canvas
	rows      []row
	scrollY   int32
	selKey    string
	hoverBtn  int // buttonID, -1 none
	pressBtn  int // -1 none
	hoverRow  int
	buttons   [int(btnCount)]button
	visible   bool
	statusMsg string    // gợi ý/tình trạng tạm thời ở footer
	statusAt  time.Time // hết hạn sau vài giây
}

var p *Panel
var panelClassRegistered bool

// Open creates (once) and shows/foregrounds the panel window.
func Open(d Deps) {
	// tạo mới nếu chưa có HOẶC cửa sổ đã bị đóng (✕) trước đó
	if p == nil || p.hwnd == 0 {
		p = &Panel{deps: d, hoverBtn: -1, pressBtn: -1, hoverRow: -1}
		if !p.create() {
			p = nil
			return
		}
	}
	p.refresh()
	if w32.IsIconic(p.hwnd) {
		// đang minimize → khôi phục, nếu không click khay sẽ có cảm giác "không ăn"
		w32.ShowWindow(p.hwnd, w32.SW_RESTORE)
	} else {
		w32.ShowWindow(p.hwnd, w32.SW_SHOW)
	}
	w32.SetForegroundWindow(p.hwnd)
	p.visible = true
}

// Refresh re-reads the store and repaints (called after external reload).
func Refresh() {
	if p != nil {
		p.refresh()
	}
}

func (pp *Panel) create() bool {
	inst := pp.deps.HInstance
	if pp.icon == 0 {
		pp.icon = w32.CreateCakeIcon(32)
	}
	if pp.icon16 == 0 {
		pp.icon16 = w32.CreateCakeIcon(16)
	}
	if !panelClassRegistered {
		wc := w32.WNDCLASSEXW{
			CbSize:        uint32(unsafe.Sizeof(w32.WNDCLASSEXW{})),
			Style:         w32.CS_HREDRAW | w32.CS_VREDRAW,
			LpfnWndProc:   syscall.NewCallback(panelWndProc),
			HInstance:     inst,
			HIcon:         pp.icon,
			HIconSm:       pp.icon16, // icon bánh ở thanh tiêu đề
			HCursor:       w32.LoadCursor(w32.MakeIntResource(w32.IDC_ARROW)),
			HbrBackground: 0,
		}
		wc.LpszClassName = w32.UTF16(className)
		if w32.RegisterClassEx(&wc) == 0 {
			pp.logf("RegisterClassEx thất bại")
			return false
		}
		panelClassRegistered = true // đăng ký ĐÚNG 1 LẦN — gọi lại sẽ lỗi và panel không mở được
	}

	work := w32.MonitorWorkArea(0)
	W, H := int32(680), int32(640)
	x := work.Left + (work.W()-W)/2
	y := work.Top + (work.H()-H)/3

	pp.hwnd = w32.CreateWindowEx(0, className,
		"Nhắc Sinh Nhật — Bảng điều khiển",
		w32.WS_OVERLAPPEDWINDOW,
		x, y, W, H, 0, 0, inst)
	if pp.hwnd == 0 {
		pp.logf("CreateWindowEx thất bại")
		return false
	}
	w32.EnableDarkTitleBar(pp.hwnd)
	w32.SetRoundedCorners(pp.hwnd) // bo góc cửa sổ trên Windows 11
	pp.ensureIcons()
	// icon bánh sinh nhật đồng bộ: tiêu đề + taskbar + Alt-Tab
	w32.SendMessage(pp.hwnd, w32.WM_SETICON, w32.ICON_SMALL, pp.icon16)
	w32.SendMessage(pp.hwnd, w32.WM_SETICON, w32.ICON_BIG, pp.icon48)
	w32.ShowWindow(pp.hwnd, w32.SW_SHOWNORMAL)
	w32.UpdateWindow(pp.hwnd)
	pp.refresh()
	pp.logf("đã mở bảng điều khiển")
	return true
}

func (pp *Panel) logf(f string, a ...interface{}) {
	if pp.deps.Logf != nil {
		pp.deps.Logf("panel: "+f, a...)
	}
}

// refresh re-snapshots the store sorted by days left.
func (pp *Panel) refresh() {
	now := time.Now()
	persons := pp.deps.Store.Snapshot()
	store.SortByNext(persons, now)
	rows := make([]row, 0, len(persons))
	for _, per := range persons {
		dl := per.DaysLeft(now)
		next := per.NextOccurrence(now)
		ld, lm, _, leap := store.SolarToLunarVN(next)
		rows = append(rows, row{
			person:   per,
			key:      personKey(per),
			daysLeft: dl,
			next:     next,
			lunarTxt: store.LunarDateString(ld, lm, leap),
		})
	}
	pp.rows = rows
	w32.InvalidateRect(pp.hwnd, false)
}

// hint hiển thị thông báo tạm thời ở footer (tự ẩn sau ~5 giây).
func (pp *Panel) hint(msg string) {
	pp.statusMsg = msg
	pp.statusAt = time.Now().Add(5 * time.Second)
	w32.SetTimer(pp.hwnd, 7, 5100)
	w32.InvalidateRect(pp.hwnd, false)
}

// ---- layout --------------------------------------------------------------

func (pp *Panel) layoutButtons(w, h int32) {
	y := toolbarY
	bx := margin
	bw := int32(98)
	pp.buttons[btnAdd] = button{id: btnAdd, x: bx, y: y, w: bw, h: toolbarH, label: "Thêm", icon: icoPlus, style: stAccent}
	pp.buttons[btnEdit] = button{id: btnEdit, x: bx + bw + 8, y: y, w: bw, h: toolbarH, label: "Sửa", icon: icoPencil, style: stOutline}
	pp.buttons[btnDelete] = button{id: btnDelete, x: bx + (bw+8)*2, y: y, w: bw, h: toolbarH, label: "Xóa", icon: icoTrash, style: stOutlineDanger}

	// right group: Nhập Excel · Mở Excel · Lưu Excel
	impW, openW, saveW := int32(106), int32(100), int32(106)
	saveX := w - margin - saveW
	openX := saveX - openW - 8
	impX := openX - impW - 8
	pp.buttons[btnSaveExcel] = button{id: btnSaveExcel, x: saveX, y: y, w: saveW, h: toolbarH, label: "Lưu Excel", icon: icoSave, style: stGreen}
	pp.buttons[btnOpenExcel] = button{id: btnOpenExcel, x: openX, y: y, w: openW, h: toolbarH, label: "Mở Excel", icon: icoGrid, style: stOutline}
	pp.buttons[btnImport] = button{id: btnImport, x: impX, y: y, w: impW, h: toolbarH, label: "Nhập Excel", icon: icoImport, style: stOutline}

	// nút Thoát ở góc phải footer: tắt HẲN phần mềm (khác với nút ✕
	// chỉ đóng cửa sổ, app vẫn chạy trong khay). Luôn sẵn có kể cả khi
	// menu chuột phải của khay hệ thống không hoạt động trên máy nào đó.
	pp.buttons[btnExit] = button{
		id: btnExit, x: w - margin - 86, y: h - footerH + 6,
		w: 86, h: footerH - 12, label: "Thoát", icon: icoNone, style: stOutlineDanger,
	}
}

func (pp *Panel) listRect() w32.RECT {
	rc := w32.GetClientRect(pp.hwnd)
	return w32.RECT{Left: 0, Top: listTop, Right: rc.Right, Bottom: rc.Bottom - footerH}
}

func (pp *Panel) contentH() int32 {
	return listHdrH + int32(len(pp.rows))*rowH
}

func (pp *Panel) maxScroll() int32 {
	lr := pp.listRect()
	m := pp.contentH() - (lr.H() - listHdrH)
	if m < 0 {
		m = 0
	}
	return m
}

func (pp *Panel) rowAt(x, y int32) int {
	lr := pp.listRect()
	if y < lr.Top+listHdrH || y >= lr.Bottom {
		return -1
	}
	idx := int((y - lr.Top - listHdrH + pp.scrollY) / rowH)
	if idx < 0 || idx >= len(pp.rows) {
		return -1
	}
	if x > lr.Right-14 { // scrollbar zone
		return -1
	}
	return idx
}

func (pp *Panel) buttonAt(x, y int32) buttonID {
	for _, b := range pp.buttons {
		if x >= b.x && x < b.x+b.w && y >= b.y && y < b.y+b.h {
			return b.id
		}
	}
	return -1
}

// ---- message handling ----------------------------------------------------

func panelWndProc(hwnd, msg, wp, lp uintptr) (rc uintptr) {
	defer w32.RecoverWndProc("panel") // panic trong UI → log, không giết app
	if p == nil {
		return w32.DefWindowProc(hwnd, msg, wp, lp)
	}
	switch msg {
	case w32.WM_CLOSE:
		w32.DestroyWindow(hwnd)
		return 0

	case w32.WM_DESTROY:
		p.hwnd = 0
		p.canvas.Free()
		p.canvas = nil
		p.visible = false
		p = nil // cho phép Open() tạo lại cửa sổ ở lần mở sau
		return 0

	case w32.WM_TIMER:
		if wp == 7 { // hết hạn thông báo footer
			w32.KillTimer(hwnd, 7)
			if p != nil && !p.statusAt.IsZero() && !time.Now().Before(p.statusAt) {
				p.statusMsg = ""
				p.statusAt = time.Time{}
				w32.InvalidateRect(hwnd, false)
			}
			return 0
		}

	case w32.WM_SIZE:
		p.canvas.Free()
		p.canvas = nil
		w32.InvalidateRect(hwnd, false)
		return 0

	case w32.WM_ERASEBKGND:
		return 1

	case w32.WM_PAINT:
		p.paint()
		return 0

	case w32.WM_GETMINMAXINFO:
		mmi := (*w32.MINMAXINFO)(unsafe.Pointer(lp)) //nolint:gosec // MSG-provided pointer
		// 680 = tổng chiều rộng 2 nhóm nút; hẹp hơn là chúng chìm vào nhau
		mmi.PtMinTrackSize = w32.POINT{X: 680, Y: 500}
		return 0

	case w32.WM_MOUSEMOVE:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		p.onMouseMove(x, y)
		return 0

	case w32.WM_MOUSELEAVE:
		p.hoverBtn, p.hoverRow = -1, -1
		w32.InvalidateRect(hwnd, false)
		return 0

	case w32.WM_LBUTTONDOWN:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		p.onLButtonDown(x, y)
		return 0

	case w32.WM_LBUTTONUP:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		p.onLButtonUp(x, y)
		return 0

	case w32.WM_LBUTTONDBLCLK:
		x, y := w32.GET_X_LPARAM(lp), w32.GET_Y_LPARAM(lp)
		if idx := p.rowAt(x, y); idx >= 0 {
			p.openEditDialog(idx)
		}
		return 0

	case w32.WM_MOUSEWHEEL:
		delta := int32(int16(uint16(uint32(wp) >> 16)))
		step := (delta / 120) * rowH * 2
		p.scrollY -= step
		if p.scrollY < 0 {
			p.scrollY = 0
		}
		if m := p.maxScroll(); p.scrollY > m {
			p.scrollY = m
		}
		w32.InvalidateRect(hwnd, false)
		return 0

	case wmAppReload:
		p.refresh()
		return 0
	}
	return w32.DefWindowProc(hwnd, msg, wp, lp)
}

func (pp *Panel) onMouseMove(x, y int32) {
	btn := pp.buttonAt(x, y)
	row := pp.rowAt(x, y)
	if btn != buttonID(pp.hoverBtn) || row != pp.hoverRow {
		pp.hoverBtn = int(btn)
		pp.hoverRow = row
		w32.InvalidateRect(pp.hwnd, false)
	}
	w32.TrackMouseLeave(pp.hwnd)
	if btn >= 0 {
		w32.SetCursor(w32.LoadCursor(w32.MakeIntResource(w32.IDC_HAND)))
	} else if row >= 0 {
		w32.SetCursor(w32.LoadCursor(w32.MakeIntResource(w32.IDC_ARROW)))
	}
}

func (pp *Panel) onLButtonDown(x, y int32) {
	btn := pp.buttonAt(x, y)
	if btn >= 0 {
		pp.pressBtn = int(btn)
		w32.InvalidateRect(pp.hwnd, false)
		return
	}
	if idx := pp.rowAt(x, y); idx >= 0 {
		pp.selKey = pp.rows[idx].key
		w32.InvalidateRect(pp.hwnd, false)
		return
	}
	// scrollbar track: jump the view toward the click position
	lr := pp.listRect()
	if x >= lr.Right-14 && pp.maxScroll() > 0 {
		rel := float64(y-(lr.Top+listHdrH)) / float64(lr.H()-listHdrH)
		pp.scrollY = int32(rel * float64(pp.maxScroll()))
		w32.InvalidateRect(pp.hwnd, false)
	}
}

func (pp *Panel) onLButtonUp(x, y int32) {
	btn := pp.buttonAt(x, y)
	pressed := buttonID(pp.pressBtn)
	pp.pressBtn = -1
	w32.InvalidateRect(pp.hwnd, false)
	if btn >= 0 && btn == pressed {
		pp.runButton(btn)
	}
}

func (pp *Panel) runButton(id buttonID) {
	switch id {
	case btnAdd:
		pp.openEditDialog(-1)
	case btnEdit:
		if idx := pp.selectedIndex(); idx >= 0 {
			pp.openEditDialog(idx)
		} else {
			pp.hint("Hãy nhấp chọn một người trong danh sách trước khi sửa.")
		}
	case btnDelete:
		if idx := pp.selectedIndex(); idx >= 0 {
			pp.confirmDelete(idx)
		} else {
			pp.hint("Hãy nhấp chọn một người trong danh sách trước khi xóa.")
		}
	case btnImport:
		pp.importExcel()
	case btnOpenExcel:
		if pp.deps.OnOpenExcel != nil {
			pp.deps.OnOpenExcel()
		}

	case btnSaveExcel:
		if err := pp.deps.Store.SaveAll(); err != nil {
			pp.logf("lưu Excel lỗi: %v", err)
			pp.hint("Không lưu được file Excel — có thể file đang mở, hãy đóng nó rồi thử lại.")
			w32.MessageBox(pp.hwnd,
				"Không lưu được file Excel:\n"+err.Error()+
					"\n\nMẹo: nếu file đang mở trong Excel, hãy đóng nó lại rồi bấm “Lưu Excel” lần nữa.",
				"Lưu Excel — lỗi", w32.MB_OK|w32.MB_ICONWARNING|w32.MB_SETFOREGROUND)
		} else {
			pp.hint("Đã lưu danh sách vào data/danh-sach.xlsx.")
			pp.logf("đã lưu danh sách vào Excel")
		}
		pp.refresh()

	case btnExit:
		pp.confirmExit()
	}
}

// confirmExit hỏi lại trước khi tắt hẳn phần mềm — đây là thao tác khó
// làm lại (phải mở lại app), nên không tắt ngay không hỏi.
func (pp *Panel) confirmExit() {
	res := w32.MessageBox(pp.hwnd,
		"Tắt hoàn toàn Nhắc Sinh Nhật?\n\n"+
			"Phần mềm sẽ KHÔNG nhắc sinh nhật nữa cho đến khi bạn mở lại\n"+
			"(hoặc khởi động lại máy, nếu đã bật “Tự khởi động cùng Windows”).",
		"Thoát phần mềm", w32.MB_YESNO|w32.MB_ICONQUESTION|w32.MB_SETFOREGROUND)
	if res != w32.IDYES {
		return
	}
	pp.logf("thoát từ nút Thoát trên bảng điều khiển")
	if pp.deps.OnExit != nil {
		pp.deps.OnExit()
	}
}

func (pp *Panel) selectedIndex() int {
	for i, r := range pp.rows {
		if r.key == pp.selKey {
			return i
		}
	}
	return -1
}

// storeIndexOfKey tìm vị trí người dùng trong store theo khóa định danh
// (dùng được cả khi danh sách có người trùng tên).
func (pp *Panel) storeIndexOfKey(key string) int {
	if key == "" {
		return -1
	}
	snap := pp.deps.Store.Snapshot()
	for i, per := range snap {
		if personKey(per) == key {
			return i
		}
	}
	return -1
}

func (pp *Panel) confirmDelete(idx int) {
	r := pp.rows[idx]
	title := "Xóa người này?"
	text := fmt.Sprintf("Xóa \"%s\" (%s) khỏi danh sách sinh nhật?", r.person.Name, r.person.DateString())
	res := w32.MessageBox(pp.hwnd, text, title, w32.MB_OK|4 /*MB_YESNO*/ |0x30 /*icon warning*/)
	if res != w32.IDYES {
		return
	}
	// tra vị trí thật trong store theo khóa (chính xác cả khi trùng tên)
	i := pp.storeIndexOfKey(r.key)
	if i < 0 {
		pp.hint("Người này không còn trong danh sách — đang làm mới.")
		pp.selKey = ""
		pp.refresh()
		return
	}
	if err := pp.deps.Store.Delete(i); err != nil {
		pp.logf("xóa lỗi: %v", err)
		w32.MessageBox(pp.hwnd,
			"Không xóa được (không ghi được file Excel):\n"+err.Error()+
				"\n\nMẹo: nếu file đang mở trong Excel, hãy đóng nó lại rồi thử lại.",
			"Xóa — lỗi", w32.MB_OK|w32.MB_ICONWARNING|w32.MB_SETFOREGROUND)
		return
	}
	pp.selKey = ""
	pp.refresh()
	pp.hint("Đã xóa \"" + r.person.Name + "\" khỏi danh sách.")
	if pp.deps.OnDataChanged != nil {
		pp.deps.OnDataChanged()
	}
}

// importExcel cho phép nạp danh sách từ một file .xlsx có sẵn
// (chế độ Gộp thêm hoặc Thay thế toàn bộ).
func (pp *Panel) importExcel() {
	path, errDlg := w32.BrowseForExcelFile(pp.hwnd, "Chọn file Excel (.xlsx) có sẵn dữ liệu sinh nhật")
	if errDlg != nil {
		pp.logf("nhập excel: hộp thoại lỗi: %v", errDlg)
		w32.MessageBox(pp.hwnd,
			"Không mở được hộp thoại chọn file:\n"+errDlg.Error()+
				"\n\nChi tiết đã ghi vào data/nhat-ky.log — hãy gửi file này cho người hỗ trợ nếu lỗi lặp lại.",
			"Nhập Excel — lỗi", w32.MB_OK|w32.MB_ICONWARNING|w32.MB_SETFOREGROUND)
		return
	}
	if path == "" {
		return // người dùng bấm Hủy
	}
	persons, warns, err := store.ReadPersonsFromExcel(path)
	if err != nil {
		w32.MessageBox(pp.hwnd,
			"Không đọc được file:\n"+err.Error()+"\n\nHãy chắc chắn đây là file .xlsx.",
			"Nhập Excel — lỗi", w32.MB_OK|w32.MB_ICONWARNING|w32.MB_SETFOREGROUND)
		return
	}
	if len(persons) == 0 {
		msg := "Không tìm thấy dòng dữ liệu hợp lệ nào trong file.\n\nMỗi dòng cần: Họ và tên + Ngày/Tháng (ví dụ 15/8).\nDòng đầu tiên nên là tiêu đề cột (Họ và tên, Ngày/Tháng, Loại lịch, …)."
		if len(warns) > 0 {
			msg += "\n\nChi tiết:\n- " + strings.Join(warns[:min(len(warns), 5)], "\n- ")
		}
		w32.MessageBox(pp.hwnd, msg, "Nhập Excel", w32.MB_OK|w32.MB_ICONWARNING|w32.MB_SETFOREGROUND)
		return
	}

	q := fmt.Sprintf("Đọc được %d người hợp lệ từ:\n%s", len(persons), filepath.Base(path))
	if len(warns) > 0 {
		q += fmt.Sprintf("\n(%d dòng sẽ bỏ qua vì lỗi hoặc thiếu dữ liệu)", len(warns))
	}
	q += "\n\nNhấn [Có]    = Thay thế TOÀN BỘ danh sách hiện tại\nNhấn [Không] = Gộp thêm vào danh sách hiện tại\nNhấn [Hủy]   = Không nhập"
	res := w32.MessageBox(pp.hwnd, q, "Nhập Excel — chọn cách nhập",
		w32.MB_YESNOCANCEL|w32.MB_ICONQUESTION|w32.MB_SETFOREGROUND)
	if res == w32.IDCANCEL {
		return
	}
	replace := res == w32.IDYES

	n, sk, err := pp.deps.Store.ImportPersons(persons, replace)
	if err != nil {
		w32.MessageBox(pp.hwnd, "Nhập thất bại:\n"+err.Error(), "Nhập Excel — lỗi",
			w32.MB_OK|w32.MB_ICONWARNING|w32.MB_SETFOREGROUND)
		return
	}
	pp.logf("nhập Excel từ %s: %d người (bỏ qua %d), thay thế=%v", filepath.Base(path), n, sk, replace)

	msg := fmt.Sprintf("Đã nhập %d người vào danh sách.", n)
	if sk > 0 {
		msg += fmt.Sprintf("\nBỏ qua %d dòng (trùng hoặc dữ liệu lỗi).", sk)
	}
	w32.MessageBox(pp.hwnd, msg, "Nhập Excel — hoàn tất",
		w32.MB_OK|w32.MB_ICONINFORMATION|w32.MB_SETFOREGROUND)

	pp.selKey = ""
	pp.refresh()
	if pp.deps.OnDataChanged != nil {
		pp.deps.OnDataChanged()
	}
}
