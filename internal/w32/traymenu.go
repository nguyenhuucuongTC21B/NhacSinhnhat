//go:build windows && !nogui

package w32

import (
        "runtime"
        "syscall"
        "unsafe"
)

// ShellNotifyIcon wraps Shell_NotifyIconW.
func ShellNotifyIcon(action uint32, nid *NOTIFYICONDATAW) bool {
        r, _, _ := procShellNotifyIcon.Call(uintptr(action), uintptr(unsafe.Pointer(nid)))
        return r != 0
}

func CreatePopupMenuProc() uintptr {
        r, _, _ := procCreatePopupMenu.Call()
        return r
}

func lastErr(err error) uint32 {
        if e, ok := err.(syscall.Errno); ok {
                return uint32(e)
        }
        return 0
}

// AppendMenuW trả về TRUE/FALSE + GetLastError để caller phát hiện mục nào
// thêm thất bại (menu rỗng sẽ khiến TrackPopupMenu không hiện gì và trả 0
// ngay lập tức).
func AppendMenuW(menu uintptr, flags uintptr, id uintptr, text uintptr) (bool, uint32) {
        r, _, err := procAppendMenuW.Call(menu, flags, id, text)
        return r != 0, lastErr(err)
}

// AppendMenuUTF16 = AppendMenuW với chuỗi Go → UTF-16, an toàn GC
// (UTF16PtrFromString + Call + KeepAlive trong cùng hàm, đúng mẫu chuẩn).
func AppendMenuUTF16(menu, flags, id uintptr, text string) (bool, uint32) {
        p, err := syscall.UTF16PtrFromString(text)
        if err != nil {
                return false, 0
        }
        r, _, callErr := procAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(p)))
        runtime.KeepAlive(p)
        return r != 0, lastErr(callErr)
}

// TrackPopupMenuProc bọc TrackPopupMenu của Windows theo ĐÚNG thứ tự tham
// số trong winuser.h:
//
//      BOOL TrackPopupMenu(HMENU hMenu, UINT uFlags, int x, int y,
//                          int nReserved, HWND hWnd, const RECT *prcRect);
//
// nReserved nằm TRƯỚC hWnd và phải là 0. Bản v3.6.1 từng đẩy hwnd vào slot
// nReserved (hwnd, 0, 0) → hWnd thực = NULL → menu không bao giờ hiện,
// TrackPopupMenu trả 0 ngay lập tức (đúng hiện tượng "chuột phải không ra
// menu" trong nhat-ky.log). Trả về (giá trị chọn, GetLastError) để log
// chỉ đích danh lỗi nếu máy người dùng còn bất thường.
func TrackPopupMenuProc(menu, flags, x, y, hwnd uintptr) (uintptr, uint32) {
        r, _, err := procTrackPopupMenu.Call(menu, flags, x, y, 0 /*nReserved*/, hwnd, 0 /*prcRect*/)
        var lastErr uint32
        if errno, ok := err.(syscall.Errno); ok {
                lastErr = uint32(errno)
        }
        return r, lastErr
}

func GetCursorPosProc(pt *POINT) {
        procGetCursorPos.Call(uintptr(unsafe.Pointer(pt)))
}

func SetMenuDefaultItem(menu uintptr, id int) {
        procSetMenuDefaultItem.Call(menu, uintptr(id), 0)
}

func DestroyIcon(h uintptr) { procDestroyIcon.Call(h) }

func LoadIcon(id uintptr) uintptr {
        r, _, _ := procLoadIconW.Call(0, id)
        return r
}

func DestroyMenu(menu uintptr) { procDestroyMenu.Call(menu) }

const WM_CONTEXTMENU = 0x007B
