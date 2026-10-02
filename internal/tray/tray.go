//go:build windows && !nogui

// Package tray đặt icon bánh sinh nhật vào khay hệ thống và quản lý
// menu chuột phải (Bảng điều khiển / Excel / Thoát…).
//
// V3.6.2 — SỬA LỖI GỐC RỄ "CHUỘT PHẢI KHAY KHÔNG RA MENU": TrackPopupMenu
// từng bị đẩy tham số lệch 1 slot (hwnd vào nReserved → hWnd thực = NULL)
// nên menu không bao giờ hiện, TrackPopupMenu trả 0 ngay lập tức; ngoài ra
// ở chế độ V4 menu còn bị mở 2 lần mỗi lần nhấp phải (cả WM_RBUTTONUP lẫn
// WM_CONTEXTMENU). Nay: thứ tự tham số ĐÚNG theo winuser.h + chỉ mở menu
// từ WM_CONTEXTMENU (mẫu chuẩn NotificationIcon.cpp của Microsoft) + chốt
// chặn chống mở menu kép + log GetLastError cho từng bước để chẩn đoán.
//
// V3.6.1 — đối chiếu tài liệu chính thức Microsoft (learn.microsoft.com:
// ở chế độ sự kiện cũ (V0) và hợp đồng sự kiện bị xử lý nửa vời. Nay làm
// ĐÚNG theo mẫu chuẩn NotificationIcon.cpp của Microsoft (đối chiếu
// learn.microsoft.com: NOTIFYICONDATAW + Shell_NotifyIcon, 02/10/2026):
//  1. NIM_ADD rồi NIM_SETVERSION với uVersion = NOTIFYICON_VERSION_4.
//  2. Ở V4: LOWORD(lParam) = sự kiện (NIN_SELECT, WM_CONTEXTMENU…),
//     HIWORD(lParam) = ID icon, toạ độ anchor nằm ở wParam.
//     NIN_SELECT là cách CHUẨN để bắt "kích hoạt icon" (chuột hoặc bàn phím),
//     WM_CONTEXTMENU là cách CHUẨN cho chuột phải.
//  3. Nếu NIM_SETVERSION thất bại (máy cũ) → quay về hợp đồng V0:
//     lParam = thông điệp chuột (WM_LBUTTONUP, WM_RBUTTONUP…).
//  4. MỌI sự kiện khay đều được ghi vào nhat-ky.log (trừ WM_MOUSEMOVE
//     để tránh phình log) — nếu máy người dùng còn bất thường, log sẽ
//     chỉ đích danh sự kiện nào đến và chế độ icon đang chạy.
package tray

import (
        "fmt"
        "syscall"
        "unsafe"

        "nhac-sinh-nhat/internal/w32"
)

const (
        className  = "NhacSinhNhatTrayWnd"
        windowName = "Nhắc Sinh Nhật"
        wmTray     = w32.WM_APP + 1
)

// Menu command IDs.
const (
        cmdPanel     = 1
        cmdOpenExcel = 2
        cmdTemplate  = 3
        cmdTestAlert = 4
        cmdMuteNow   = 5
        cmdAutostart = 6
        cmdExit      = 7
)

// Callbacks wire tray actions to the app.
type Callbacks struct {
        OpenPanel       func()
        OpenExcel       func()
        RecreateExcel   func()
        TestAlert       func()
        MuteNow         func()
        ToggleAutostart func()
        Exit            func()
        Logf            func(format string, args ...interface{})
}

type Tray struct {
        hwnd    uintptr
        icon    uintptr
        version uint32 // NOTIFYICON_VERSION_4 nếu NIM_SETVERSION thành công, ngược lại 0 (V0)
        menuOpen bool  // chốt chặn: menu khay đang mở (TrackPopupMenu chạy đồng bộ trên 1 thread UI)
        cb      Callbacks
}

var current *Tray

// Create registers the tray icon and its hidden message window.
func Create(hInstance uintptr, cb Callbacks) (*Tray, error) {
        t := &Tray{cb: cb}
        current = t

        wc := w32.WNDCLASSEXW{
                CbSize:      uint32(unsafe.Sizeof(w32.WNDCLASSEXW{})),
                Style:       0,
                LpfnWndProc: syscall.NewCallback(trayWndProc),
                HInstance:   hInstance,
                HCursor:     w32.LoadCursor(w32.MakeIntResource(w32.IDC_ARROW)),
        }
        wc.LpszClassName = w32.UTF16(className)
        if w32.RegisterClassEx(&wc) == 0 {
                return nil, fmt.Errorf("RegisterClassEx thất bại")
        }

        t.hwnd = w32.CreateWindowEx(0, className, windowName, 0, 0, 0, 0, 0, 0, 0, hInstance)
        if t.hwnd == 0 {
                return nil, fmt.Errorf("không tạo được cửa sổ ẩn")
        }

        t.icon = w32.CreateCakeIcon(32)
        if t.icon == 0 {
                t.icon = w32.LoadIcon(w32.MakeIntResource(w32.IDI_APPLICATION))
        }

        // NIM_ADD: KHÔNG đụng vào union uTimeout/uVersion — uVersion chỉ được
        // đặt qua NIM_SETVERSION (bản cũ ghi "UTimeout: 3" là nhầm lẫn, dễ làm
        // shell rơi vào chế độ V3 lời lởi không chuẩn hợp đồng nào cả).
        if !t.modifyIcon(w32.NIM_ADD) {
                return nil, fmt.Errorf("Shell_NotifyIcon thất bại")
        }
        // NIM_SETVERSION → NOTIFYICON_VERSION_4 (chuẩn Vista+, mọi Win10/11 hỗ trợ).
        if t.setVersion(w32.NOTIFYICON_VERSION_4) {
                t.version = w32.NOTIFYICON_VERSION_4
        }
        t.logf("khay hệ thống: đã đặt icon (32px), chế độ sự kiện V%d", t.version)
        return t, nil
}

func (t *Tray) nid() w32.NOTIFYICONDATAW {
        nid := w32.NOTIFYICONDATAW{
                CbSize:           uint32(unsafe.Sizeof(w32.NOTIFYICONDATAW{})),
                HWnd:             t.hwnd,
                UID:              1,
                UFlags:           w32.NIF_MESSAGE | w32.NIF_ICON | w32.NIF_TIP | w32.NIF_SHOWTIP,
                UCallbackMessage: wmTray,
                HIcon:            t.icon,
        }
        copy(nid.SzTip[:], utf16Fixed("Nhắc Sinh Nhật — nhấp để mở bảng điều khiển", 128))
        return nid
}

func (t *Tray) modifyIcon(action uint32) bool {
        nid := t.nid()
        return w32.ShellNotifyIcon(action, &nid)
}

// setVersion chuyển icon sang chế độ sự kiện NOTIFYICON_VERSION_4 —
// dùng đúng mẫu của Microsoft: Shell_NotifyIcon(NIM_SETVERSION, &nid)
// với nid.uVersion = NOTIFYICON_VERSION_4 (union với uTimeout).
func (t *Tray) setVersion(version uint32) bool {
        nid := t.nid()
        nid.UTimeout = version // union uTimeout/uVersion
        return w32.ShellNotifyIcon(w32.NIM_SETVERSION, &nid)
}

func (t *Tray) ShowBalloon(title, text string) {
        nid := w32.NOTIFYICONDATAW{
                CbSize:      uint32(unsafe.Sizeof(w32.NOTIFYICONDATAW{})),
                HWnd:        t.hwnd,
                UID:         1,
                UFlags:      w32.NIF_INFO | w32.NIF_SHOWTIP,
                UTimeout:    3000,
                DWInfoFlags: w32.NIIF_INFO,
        }
        copy(nid.SzInfoTitle[:], utf16Fixed(title, 64))
        copy(nid.SzInfo[:], utf16Fixed(text, 256))
        w32.ShellNotifyIcon(w32.NIM_MODIFY, &nid)
}

// Close removes the icon (call before exit).
func (t *Tray) Close() {
        t.modifyIcon(w32.NIM_DELETE)
        if t.hwnd != 0 {
                w32.DestroyWindow(t.hwnd)
                t.hwnd = 0
        }
        if t.icon != 0 {
                w32.DestroyIcon(t.icon)
                t.icon = 0
        }
}

func (t *Tray) logf(f string, a ...interface{}) {
        if t.cb.Logf != nil {
                t.cb.Logf(f, a...)
        }
}

func utf16Fixed(s string, slots int) []uint16 {
        p, _ := syscall.UTF16FromString(s)
        if len(p) > slots {
                p = p[:slots-1]
        }
        for len(p) < slots {
                p = append(p, 0)
        }
        return p
}

func trayWndProc(hwnd, msg, wp, lp uintptr) (rc uintptr) {
        defer w32.RecoverWndProc("tray") // panic trong UI → log, không giết app
        t := current
        if t == nil {
                return w32.DefWindowProc(hwnd, msg, wp, lp)
        }
        switch msg {
        case w32.WM_DESTROY:
                return 0
        case wmTray:
                t.onTrayEvent(wp, lp)
                return 0
        }
        return w32.DefWindowProc(hwnd, msg, wp, lp)
}

// eventName trả tên sự kiện khay để log dễ đọc (giá trị theo shellapi.h).
func eventName(ev uint32) string {
        switch ev {
        case w32.WM_MOUSEMOVE:
                return "WM_MOUSEMOVE"
        case w32.WM_LBUTTONDOWN:
                return "WM_LBUTTONDOWN"
        case w32.WM_LBUTTONUP:
                return "WM_LBUTTONUP"
        case w32.WM_LBUTTONDBLCLK:
                return "WM_LBUTTONDBLCLK"
        case w32.WM_RBUTTONDOWN:
                return "WM_RBUTTONDOWN"
        case w32.WM_RBUTTONUP:
                return "WM_RBUTTONUP"
        case w32.WM_CONTEXTMENU:
                return "WM_CONTEXTMENU"
        case w32.NIN_SELECT:
                return "NIN_SELECT"
        case w32.NIN_KEYSELECT:
                return "NIN_KEYSELECT"
        case w32.NIN_BALLOONUSERCLICK:
                return "NIN_BALLOONUSERCLICK"
        }
        return fmt.Sprintf("0x%04X", ev)
}

// onTrayEvent xử lý thông điệp callback của khay theo ĐÚNG hợp đồng từng
// chế độ (xem comment đầu file). Mọi sự kiện đều vào log trừ WM_MOUSEMOVE.
func (t *Tray) onTrayEvent(wp, lp uintptr) {
        ev := uint32(lp & 0xFFFF) // LOWORD(lParam) = sự kiện (cả V0 lẫn V4)
        if ev != w32.WM_MOUSEMOVE {
                t.logf("khay: sự kiện %s (wp=0x%X lp=0x%X, chế độ V%d)", eventName(ev), wp, lp, t.version)
        }

        if t.version == w32.NOTIFYICON_VERSION_4 {
                switch ev {
                case w32.NIN_SELECT, w32.NIN_KEYSELECT, w32.WM_LBUTTONDBLCLK:
                        // cách chuẩn để bắt "mở icon": chuột trái HOẶC bàn phím.
                        // Không bắt WM_LBUTTONUP ở V4 — NIN_SELECT đã đủ, tránh mở 2 lần.
                        if t.cb.OpenPanel != nil {
                                t.cb.OpenPanel()
                        }
                case w32.WM_CONTEXTMENU:
                        // chuột phải / phím Menu → menu ngữ cảnh; toạ độ anchor ở wParam
                        t.showContextMenu(w32.GET_X_LPARAM(wp), w32.GET_Y_LPARAM(wp))
                case w32.WM_RBUTTONUP:
                        // Ở V4, shell LUÔN gửi WM_CONTEXTMENU ngay sau WM_RBUTTONUP
                        // (nhat-ky.log máy thật v3.6.1 chứng minh) — nếu mở menu ở
                        // đây thì mỗi lần nhấp phải menu bị mở 2 lần, lần thứ 2
                        // chen vào giữa vòng lặp menu của lần đầu. Chỉ ghi log;
                        // menu do WM_CONTEXTMENU lo (đúng mẫu NotificationIcon.cpp).
                case w32.NIN_BALLOONUSERCLICK:
                        // người dùng bấm vào bóng thông báo → mở bảng điều khiển
                        if t.cb.OpenPanel != nil {
                                t.cb.OpenPanel()
                        }
                }
                return
        }

        // chế độ cũ V0: lParam = thông điệp chuột, wParam = ID icon
        switch ev {
        case w32.WM_LBUTTONUP, w32.WM_LBUTTONDBLCLK:
                if t.cb.OpenPanel != nil {
                        t.cb.OpenPanel()
                }
        case w32.WM_RBUTTONUP, w32.WM_CONTEXTMENU:
                t.showContextMenu(-1, -1)
        }
}

// showContextMenu mở menu chuột phải của icon khay.
// x,y = toạ độ màn hình từ sự kiện V4; nếu không hợp lệ (−1 hoặc gọi từ
// chế độ V0) thì dùng vị trí con trỏ hiện tại.
func (t *Tray) showContextMenu(x, y int32) {
        if t.menuOpen {
                t.logf("khay: BỎ QUA mở menu kép (menu đang mở)")
                return
        }
        t.menuOpen = true
        defer func() { t.menuOpen = false }()

        menu := w32.CreatePopupMenuProc()
        if menu == 0 {
                t.logf("khay: CreatePopupMenu THẤT BẠI")
                return
        }
        addItem := func(id int, label string, checked bool, sep bool) {
                var ok bool
                var gle uint32
                if sep {
                        ok, gle = w32.AppendMenuW(menu, w32.MF_SEPARATOR, 0, 0)
                } else {
                        flags := uintptr(w32.MF_STRING)
                        if checked {
                                flags |= w32.MF_CHECKED
                        }
                        ok, gle = w32.AppendMenuUTF16(menu, flags, uintptr(id), label)
                }
                if !ok {
                        t.logf("khay: AppendMenuW THẤT BẠI cho mục %q, GetLastError=%d", label, gle)
                }
        }
        addItem(cmdPanel, "Bảng điều khiển", false, false)
        addItem(cmdOpenExcel, "Mở danh sách Excel", false, false)
        addItem(cmdTemplate, "Tạo lại file Excel mẫu", false, false)
        addItem(0, "", false, true)
        addItem(cmdTestAlert, "Kiểm tra cảnh báo", false, false)
        addItem(cmdMuteNow, "Tắt nhắc nhở ngay", false, false)
        addItem(0, "", false, true)
        addItem(cmdAutostart, "Tự khởi động cùng Windows", w32.AutostartEnabled(), false)
        addItem(cmdExit, "Thoát (tắt hẳn phần mềm)", false, false)

        // bold default item = Bảng điều khiển
        w32.SetMenuDefaultItem(menu, cmdPanel)

        if x < 0 || y < 0 {
                var pt w32.POINT
                w32.GetCursorPosProc(&pt)
                x, y = pt.X, pt.Y
        }
        t.logf("khay: mở menu tại (%d,%d)", x, y)

        // BẮT BUỘC SetForegroundWindow trước TrackPopupMenu (KB135788): nếu không,
        // menu sẽ không tự mất khi bấm ra ngoài. WM_NULL sau đó để shell gỡ trạng thái.
        w32.SetForegroundWindow(t.hwnd)
        sel, gle := w32.TrackPopupMenuProc(menu, w32.TPM_RETURNCMD|w32.TPM_RIGHTBUTTON|w32.TPM_BOTTOMALIGN,
                uintptr(int64(x)), uintptr(int64(y)), t.hwnd)
        w32.PostMessage(t.hwnd, w32.WM_NULL, 0, 0)
        w32.DestroyMenu(menu)
        if sel == 0 && gle != 0 {
                t.logf("khay: TrackPopupMenu THẤT BẠI, GetLastError=%d (0x%X)", gle, gle)
        } else {
                t.logf("khay: TrackPopupMenu trả về %d", sel)
        }

        switch int(sel) {
        case cmdPanel:
                if t.cb.OpenPanel != nil {
                        t.cb.OpenPanel()
                }
        case cmdOpenExcel:
                if t.cb.OpenExcel != nil {
                        t.cb.OpenExcel()
                }
        case cmdTemplate:
                if t.cb.RecreateExcel != nil {
                        t.cb.RecreateExcel()
                }
        case cmdTestAlert:
                if t.cb.TestAlert != nil {
                        t.cb.TestAlert()
                }
        case cmdMuteNow:
                if t.cb.MuteNow != nil {
                        t.cb.MuteNow()
                }
        case cmdAutostart:
                if t.cb.ToggleAutostart != nil {
                        t.cb.ToggleAutostart()
                }
        case cmdExit:
                if t.cb.Exit != nil {
                        t.cb.Exit()
                }
        }
}
