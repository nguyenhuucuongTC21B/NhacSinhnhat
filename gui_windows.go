//go:build windows && !nogui

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"nhac-sinh-nhat/internal/alertwin"
	"nhac-sinh-nhat/internal/melody"
	"nhac-sinh-nhat/internal/panel"
	"nhac-sinh-nhat/internal/reminder"
	"nhac-sinh-nhat/internal/store"
	"nhac-sinh-nhat/internal/tray"
	"nhac-sinh-nhat/internal/w32"
)

const (
	mainClassName = "NhacSinhNhatMainWnd"
	wmReload      = w32.WM_APP + 20
	wmRecheck     = w32.WM_APP + 21
)

var (
	cfg       RunConfig
	mainHwnd  uintptr
	hInstance uintptr
	trRef     *tray.Tray // để callback dùng balloon tip sau khi khay đã tạo
)

// Run boots the full Windows GUI: tray, panel, alerts, reminder timers.
func Run(conf RunConfig) {
	runtime.LockOSThread()
	cfg = conf
	// mọi panic trong WndProc phải ghi vào nhat-ky.log thay vì giết app im lặng
	w32.CrashLog = logf

	// COM (STA) cho thread UI — BẮT BUỘC trước GetOpenFileNameW/ShellExecute;
	// thiếu COM → hộp thoại chọn file crash process trên Win10/11 (OneDrive,
	// shell extension) — lỗi "bấm Nhập Excel là phần mềm tự tắt"
	w32.CoInitializeEx()

	if !w32.CreateSingleInstanceMutex("Global\\NhacSinhNhatSingleInstance") {
		logf("phát hiện phiên bản khác đang chạy — thoát")
		w32.MessageBox(0,
			"Nhắc Sinh Nhật đang chạy rồi!\n\nHãy nhìn khay hệ thống (mặt clock) — nhấp vào icon bánh sinh nhật để mở bảng điều khiển.",
			"Nhắc Sinh Nhật", w32.MB_OK|w32.MB_ICONINFORMATION|w32.MB_SETFOREGROUND|w32.MB_TOPMOST)
		return
	}

	hm, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	hInstance = hm

	wc := w32.WNDCLASSEXW{
		CbSize:      uint32(unsafe.Sizeof(w32.WNDCLASSEXW{})),
		LpfnWndProc: syscall.NewCallback(appWndProc),
		HInstance:   hInstance,
		HCursor:     w32.LoadCursor(w32.MakeIntResource(w32.IDC_ARROW)),
	}
	wc.LpszClassName = w32.UTF16(mainClassName)
	if w32.RegisterClassEx(&wc) == 0 {
		logf("RegisterClassEx main thất bại")
		return
	}
	// message-only window
	mainHwnd = w32.CreateWindowEx(0, mainClassName, "nsn-main", 0, 0, 0, 0, 0, ^uintptr(2) /*HWND_MESSAGE*/, 0, hInstance)
	if mainHwnd == 0 {
		logf("không tạo được cửa sổ chính")
		return
	}

	// tray icon
	var tr *tray.Tray
	tr, err := tray.Create(hInstance, tray.Callbacks{
		OpenPanel: func() {
			panel.Open(panelDeps())
			logf("tray: mở bảng điều khiển")
		},
		OpenExcel:     openExcel,
		RecreateExcel: recreateExcel,
		TestAlert:     testAlert,
		MuteNow:       muteNow,
		ToggleAutostart: func() {
			w32.SetAutostart(!w32.AutostartEnabled())
			logf("tự khởi động cùng Windows: %v", w32.AutostartEnabled())
		},
		Exit: exitApp,
		Logf: logf,
	})
	if err != nil {
		logf("lỗi khay hệ thống: %v", err)
		w32.MessageBox(0, "Không đặt được icon khay hệ thống: "+err.Error(), "Nhắc Sinh Nhật", w32.MB_OK|w32.MB_ICONWARNING)
		return
	}
	trRef = tr

	// external Excel reload → refresh panel + rescan
	OnExternalReload = func(err error) {
		w32.PostMessage(mainHwnd, wmReload, 0, 0)
	}

	// first run → open panel automatically + balloon tip so users see the app exists
	if cfg.FirstRun {
		w32.SetTimer(mainHwnd, 99, 1500)
	}

	// periodic reminder scan + startup scan (2s delay)
	w32.SetTimer(mainHwnd, 98, 2000)
	w32.SetTimer(mainHwnd, 1, 20*60*1000)

	logf("giao diện đã sẵn sàng — icon khay đang chạy")

	var msg w32.MSG
	for {
		r := w32.GetMessage(&msg, 0)
		if r <= 0 {
			break
		}
		w32.TranslateMessage(&msg)
		w32.DispatchMessage(&msg)
	}
	logf("đã thoát")
}

func logf(f string, a ...interface{}) {
	if cfg.Logf != nil {
		cfg.Logf(f, a...)
	}
}

// exitApp tắt HOÀN TOÀN phần mềm: dừng nhạc, đóng popup cảnh báo, gỡ icon
// khay, rồi ExitProcess — dùng chung cho menu chuột phải khay VÀ nút Thoát
// trên bảng điều khiển. (Đóng cửa sổ bảng điều khiển KHÔNG thoát app —
// app vẫn chạy trong khay; chỉ exitApp mới tắt hẳn.)
func exitApp() {
	logf("thoát theo yêu cầu người dùng")
	melody.Stop()
	alertwin.CloseActive()
	if trRef != nil {
		trRef.Close()
	}
	logf("đã thoát — hẹn gặp lại!")
	// ExitProcess bảo đảm TẮT HOÀN TOÀN: nếu hộp thoại Thêm/Sửa hay popup
	// cảnh báo đang mở, vòng lặp message lồng nhau có thể nuốt WM_QUIT
	// khiến PostQuitMessage một mình không đủ.
	w32.ExitProcess(0)
}

func panelDeps() panel.Deps {
	return panel.Deps{
		HInstance: hInstance,
		Store:     cfg.Store,
		Version:   Version,
		Logf:      logf,
		OnDataChanged: func() {
			w32.PostMessage(mainHwnd, wmRecheck, 0, 0)
		},
		OnOpenExcel: openExcel,
		OnExit:      exitApp,
	}
}

func appWndProc(hwnd, msg, wp, lp uintptr) (rc uintptr) {
	defer w32.RecoverWndProc("app") // panic không được giết app im lặng nữa
	switch msg {
	case w32.WM_TIMER:
		switch wp {
		case 98:
			w32.KillTimer(hwnd, wp)
			scanAlerts()
		case 99: // lần đầu chạy: giới thiệu + mở luôn bảng điều khiển
			w32.KillTimer(hwnd, wp)
			showStartupBalloon()
			panel.Open(panelDeps())
		case 1:
			scanAlerts()
		}
		return 0
	case wmReload:
		panel.Refresh()
		scanAlerts()
		return 0
	case wmRecheck:
		scanAlerts()
		return 0
	case w32.WM_DESTROY:
		return 0
	}
	return w32.DefWindowProc(hwnd, msg, wp, lp)
}

func showStartupBalloon() {
	if trRef != nil {
		trRef.ShowBalloon("Nhắc Sinh Nhật đang chạy",
			"Icon bánh sinh nhật nằm ở khay hệ thống — nhấp vào đó để mở bảng điều khiển.")
	}
}

// scanAlerts shows the next pending alert (one at a time).
func scanAlerts() {
	if alertwin.Active() {
		return
	}
	cands := cfg.Engine.Candidates(cfg.Store.Snapshot(), time.Now())
	if len(cands) == 0 {
		return
	}
	logf("quét nhắc: %d ứng viên — hiện: %s (còn %d ngày)",
		len(cands), cands[0].Person.Name, cands[0].DaysLeft)
	showAlert(cands[0])
}

func toAlertItem(c reminder.Candidate) alertwin.Item {
	ld, lm, _, leap := store.SolarToLunarVN(c.Next)
	return alertwin.Item{
		Name:     c.Person.Name,
		DaysLeft: c.DaysLeft,
		Next:     c.Next,
		Lunar:    store.LunarDateString(ld, lm, leap),
		IsLunar:  c.Person.Kind == store.Lunar,
		Note:     c.Person.Note,
	}
}

// startMusic picks: personal MP3 → shared data/nhac.mp3 → built-in melody.
func startMusic(p store.Person) {
	candidates := []string{}
	if p.MP3 != "" {
		if filepath.IsAbs(p.MP3) {
			candidates = append(candidates, p.MP3)
		} else {
			candidates = append(candidates, filepath.Join(cfg.DataDir, p.MP3))
			candidates = append(candidates, filepath.Join(cfg.DataDir, filepath.Base(p.MP3)))
		}
	}
	candidates = append(candidates, filepath.Join(cfg.DataDir, "nhac.mp3"))
	for _, f := range candidates {
		if st, err := os.Stat(f); err == nil && !st.IsDir() {
			melody.PlayFile(f)
			logf("nhạc: %s", f)
			return
		}
	}
	melody.PlayBuiltIn()
	logf("nhạc: giai điệu có sẵn (không tìm thấy MP3)")
}

func showAlert(c reminder.Candidate) {
	startMusic(c.Person)
	if !alertwin.Show(toAlertItem(c), func() {
		melody.Stop()
		cfg.Engine.MarkNotified(c.Person.Name)
		logf("đã tắt nhắc nhở cho %s", c.Person.Name)
		w32.PostMessage(mainHwnd, wmRecheck, 0, 0)
	}) {
		// không tạo được cửa sổ → bắt buộc dừng nhạc, nếu không nhạc kêu mãi
		melody.Stop()
		cfg.Engine.MarkNotified(c.Person.Name)
		logf("không hiện được popup — đã dừng nhạc và ghi dấu đã nhắc")
		return
	}
	logf("cảnh báo: %s — còn %d ngày", c.Person.Name, c.DaysLeft)
}

func openExcel() {
	path := cfg.Store.Path()
	if _, err := os.Stat(path); err != nil {
		if _, err2 := cfg.Store.EnsureTemplate(); err2 == nil {
			_ = cfg.Store.Load()
			panel.Refresh()
		}
	}
	r, _, _ := procShellExecuteW.Call(mainHwnd, w32.UTF16("open"), w32.UTF16(path), 0, 0, 5)
	// ShellExecuteW trả về giá trị > 32 khi thành công — trước giờ kết quả
	// bị bỏ qua nên "Mở Excel" có thể im lặng không làm gì khi Windows
	// không có app gắn với .xlsx (hết lỗi "bấm Mở Excel không ăn").
	if r <= 32 {
		logf("mở Excel thất bại (ShellExecute = %d): %s", int32(r), path)
		w32.MessageBox(mainHwnd,
			"Không mở được file Excel:\n"+path+
				"\n\nHãy mở file thủ công từ thư mục data của ứng dụng.",
			"Mở Excel — lỗi", w32.MB_OK|w32.MB_ICONWARNING|w32.MB_SETFOREGROUND)
		return
	}
	logf("mở danh sách Excel: %s", path)
}

func recreateExcel() {
	res := w32.MessageBox(mainHwnd,
		"Tạo lại file Excel mẫu sẽ XÓA danh sách hiện tại.\nTiếp tục?",
		"Tạo lại file Excel mẫu", w32.MB_OK|4|w32.MB_ICONWARNING)
	if res != 6 {
		return
	}
	_ = os.Remove(cfg.Store.Path())
	if _, err := cfg.Store.EnsureTemplate(); err != nil {
		logf("tạo lại mẫu lỗi: %v", err)
		return
	}
	if err := cfg.Store.Load(); err != nil {
		logf("tải lại mẫu lỗi: %v", err)
	}
	panel.Refresh()
	logf("đã tạo lại file Excel mẫu")
}

func testAlert() {
	best, ok := reminder.Nearest(cfg.Store.Snapshot(), time.Now())
	if !ok {
		w32.MessageBox(mainHwnd,
			"Không có ai trong danh sách (hoặc chưa ai có ngày sinh hợp lệ).\nHãy thêm người hoặc sửa ngày sinh trước.",
			"Kiểm tra cảnh báo", w32.MB_OK|w32.MB_ICONINFORMATION)
		return
	}
	it := toAlertItem(best)
	it.IsTest = true // ghi rõ là thử nghiệm, không phải cảnh báo thật
	startMusic(best.Person)
	if !alertwin.Show(it, func() {
		melody.Stop()
		logf("kiểm tra cảnh báo: đã tắt")
	}) {
		melody.Stop()
		w32.MessageBox(mainHwnd, "Không hiện được cửa sổ kiểm tra cảnh báo.", "Kiểm tra cảnh báo", w32.MB_OK|w32.MB_ICONWARNING)
		return
	}
	logf("kiểm tra cảnh báo: %s — còn %d ngày (không đánh dấu đã nhắc)", best.Person.Name, best.DaysLeft)
}

func muteNow() {
	melody.Stop()
	alertwin.CloseActive()
	// mark everyone currently pending as notified for today
	cands := cfg.Engine.Candidates(cfg.Store.Snapshot(), time.Now())
	for _, c := range cands {
		cfg.Engine.MarkNotified(c.Person.Name)
	}
	logf("tắt nhắc nhở ngay: %d cảnh báo hôm nay", len(cands))
	if trRef != nil {
		if len(cands) > 0 {
			trRef.ShowBalloon("Đã tắt nhắc nhở",
				fmt.Sprintf("Đã tắt %d cảnh báo của hôm nay. Các ngày khác vẫn được nhắc bình thường.", len(cands)))
		} else {
			trRef.ShowBalloon("Tắt nhắc nhở", "Hôm nay không còn cảnh báo nào đang chờ.")
		}
	}
}

var procShellExecuteW = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
