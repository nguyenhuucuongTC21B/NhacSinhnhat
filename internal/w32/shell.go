//go:build windows && !nogui

package w32

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// OPENFILENAMEW (x64 layout; no unions — natural alignment matches MSVC).
type OPENFILENAMEW struct {
	LStructSize       uint32
	HwndOwner         uintptr
	HInstance         uintptr
	LpstrFilter       uintptr
	LpstrCustomFilter uintptr
	NMaxCustomFilter  uint32
	NFilterIndex      uint32
	LpstrFile         uintptr
	NMaxFile          uint32
	LpstrFileTitle    uintptr
	NMaxFileTitle     uint32
	LpstrInitialDir   uintptr
	LpstrTitle        uintptr
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       uintptr
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    uintptr
	PvReserved        uintptr
	DwReserved        uint32
	FlagsEx           uint32
}

// ---------------------------------------------------------------------------
// Hộp thoại chọn file — PHIÊN BẢN v3.5.0
//
// Lịch sử lỗi (bài học xương máu — KHÔNG viết lại bảng vtable bằng trí nhớ):
//  - v3.4.0: bảng vtable hiểu sai → "SetTitle" thực chất gọi SetFolder → crash.
//  - v3.4.1: bảng "sửa lại" vẫn sai (thiếu Advise/Unadvise ở slot 7-8 làm MỌI
//    slot từ 8 trở đi lệch +2) VÀ SetFileTypes bị đảo thứ tự tham số — chữ ký
//    chuẩn là SetFileTypes(UINT cFileTypes, const COMDLG_FILTERSPEC *rg) —
//    SỐ LƯỢNG TRƯỚC, MẢNG SAU — code truyền (mảng, số lượng) → Windows đọc
//    "mảng" tại địa chỉ 0x2 → access violation NGAY TRƯỚC KHI Show chạy →
//    process chết ngay sau dòng log "mở", không bao giờ thấy "Show trả về".
//
// Giải pháp 3 lớp:
//  1. IFileOpenDialog (COM hiện đại) — vtable chép NGUYÊN VĂN từ
//     shobjidl_core.idl (xem comment trong openFileViaCom), log từng bước COM.
//  2. Fallback GetOpenFileNameW nếu COM trả lỗi.
//  3. KHÔNG BAO GIỜ im lặng: mọi bước ghi nhat-ky.log; thất bại → trả error
//     → caller hiện MessageBox cho người dùng.
// ---------------------------------------------------------------------------

var (
	procCommDlgExtendedError = modComdlg32.NewProc("CommDlgExtendedError")
	procCoCreateInstance     = modOle32.NewProc("CoCreateInstance")
	procCoTaskMemFree        = modOle32.NewProc("CoTaskMemFree")
)

// COM GUIDs.
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	clsidFileOpenDialog = guid{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = guid{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
)

const (
	clsctxInprocServer = 0x1
	sigdnFileSysPath   = 0x80058000

	fosNoChangedDir    = 0x0008
	fosForceFileSystem = 0x0040
	fosPathMustExist   = 0x0800
	fosFileMustExist   = 0x1000

	hrCancelled = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED=1223)
)

// vcall gọi phương thức thứ idx trong COM vtable của obj.
// Bố cục vtable: [0]=QueryInterface [1]=AddRef [2]=Release rồi tới các
// phương thức riêng theo thứ tự khai báo trong IDL.
func vcall(obj uintptr, idx int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(idx)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

func utf16Bytes(s string) []uint16 {
	p, err := syscall.UTF16FromString(s)
	if err != nil {
		p = []uint16{0}
	}
	return p
}

// ptrToString đọc chuỗi UTF-16 kết thúc NUL do Windows cấp phát.
func ptrToString(p uintptr) string {
	if p == 0 {
		return ""
	}
	n := 0
	for *(*uint16)(unsafe.Pointer(p + uintptr(n)*2)) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(p)), n))
}

// comLog ghi từng bước COM của hộp thoại vào nhat-ky.log — nếu có crash cứng
// (access violation trong native, Go không bắt được) thì dòng cuối cùng ghi
// trước khi chết sẽ chỉ đích danh bước gây lỗi.
func comLog(f string, a ...interface{}) {
	if CrashLog != nil {
		CrashLog("hộp thoại COM: "+f, a...)
	}
}

// openFileViaCom mở hộp thoại chọn file bằng IFileOpenDialog (COM hiện đại —
// cùng hộp thoại Explorer dùng).
//
// ⚠️ BẢN ĐỒ VTABLE — CHÉP NGUYÊN VĂN TỪ shobjidl_core.idl (Windows SDK).
// Sai MỘT slot = Windows gọi nhầm hàm khác với tham số sai kiểu → access
// violation → process chết ngay (không thể recover trong Go). Vui lòng KHÔNG
// dò lại bằng trí nhớ — mở shobjidl_core.idl/idl ra đối chiếu nếu cần sửa.
//
//	interface IModalWindow : IUnknown {   // IUnknown chiếm slot 0,1,2
//	    HRESULT Show(HWND hwndOwner);                                            //  3
//	}
//	interface IFileDialog : IModalWindow {
//	    HRESULT SetFileTypes(UINT cFileTypes, const COMDLG_FILTERSPEC *rg);      //  4 ← SỐ LƯỢNG TRƯỚC, MẢNG SAU
//	    HRESULT SetFileTypeIndex(UINT iFileType);                                //  5
//	    HRESULT GetFileTypeIndex(UINT *piFileType);                              //  6
//	    HRESULT Advise(IFileDialogEvents *pfde, DWORD *pdwCookie);               //  7 ← v3.4.1 thiếu 2 slot này
//	    HRESULT Unadvise(DWORD dwCookie);                                        //  8
//	    HRESULT SetOptions(FILEOPENDIALOGOPTIONS fos);                           //  9
//	    HRESULT GetOptions(FILEOPENDIALOGOPTIONS *pfos);                         // 10
//	    HRESULT SetDefaultFolder(IShellItem *psi);                               // 11
//	    HRESULT SetFolder(IShellItem *psi);                                      // 12
//	    HRESULT GetCurrentSelection(IShellItem **ppsi);                          // 13
//	    HRESULT SetFileName(LPCWSTR pszName);                                    // 14
//	    HRESULT SetTitle(LPCWSTR pszTitle);                                      // 15
//	    HRESULT SetOkButtonLabel(LPCWSTR pszText);                               // 16
//	    HRESULT SetFileNameLabel(LPCWSTR pszText);                               // 17
//	    HRESULT GetResult(IShellItem **ppsi);                                    // 18
//	    HRESULT AddPlace(IShellItem *psi, FDAP fdap);                            // 19
//	    HRESULT SetDefaultExtension(LPCWSTR pszDefaultExtension);                // 20
//	    HRESULT Close(HRESULT hr);                                               // 21
//	    HRESULT SetClientGuid(REFGUID guid);                                     // 22
//	    HRESULT ClearClientData(void);                                           // 23
//	    HRESULT SetFilter(IShellItemFilter *pFilter);                            // 24
//	}
//	interface IFileOpenDialog : IFileDialog {
//	    HRESULT GetResults(IShellItemArray **ppenum);                            // 25
//	    HRESULT GetSelectedItems(IShellItemArray **ppsam);                       // 26
//	}
//	interface IShellItem : IUnknown {     // IUnknown chiếm slot 0,1,2
//	    HRESULT BindToHandler(GBH bh, REFIID riid, void **ppv);                  //  3
//	    HRESULT GetParent(IShellItem **ppsi);                                    //  4
//	    HRESULT GetDisplayName(SIGDN sigdnName, LPWSTR *ppszName);               //  5
//	    HRESULT GetAttributes(SFGAO sfgaoMask, SFGAO *psfgaoAttribs);            //  6
//	    HRESULT Compare(IShellItem *psi, SICHINT hint, int *piOrder);            //  7
//	}
func openFileViaCom(owner uintptr, title, filterName, filterSpec string) (string, error) {
	defer LogPanic("file-dialog-com")

	// 1) CoCreateInstance(CLSID_FileOpenDialog, NULL, CLSCTX_INPROC_SERVER,
	//                    IID_IFileOpenDialog, &dlg)
	var dlg uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)),
		uintptr(unsafe.Pointer(&dlg)))
	comLog("CoCreateInstance(FileOpenDialog) → 0x%08X, dlg=0x%X", uint32(hr), dlg)
	if hr != 0 || dlg == 0 {
		return "", fmt.Errorf("CoCreateInstance(FileOpenDialog) = 0x%08X", uint32(hr))
	}
	// KeepAlive đăng ký TRƯỚC → chạy SAU cùng (sau Release) — dữ liệu filter
	// và tiêu đề phải sống đến khi đối tượng COM được giải phóng.
	defer runtime.KeepAlive(filterName)
	defer runtime.KeepAlive(filterSpec)
	defer runtime.KeepAlive(title)
	defer vcall(dlg, 2) // Release dialog (chạy trước KeepAlive)

	// 2) SetTitle(15) — LPCWSTR
	t16 := utf16Bytes(title)
	vcall(dlg, 15, uintptr(unsafe.Pointer(&t16[0])))

	// 3) SetFileTypes(4) — COMDLG_FILTERSPEC {pszName, pszSpec}, 2 nhóm lọc.
	//    ⚠️ THỨ TỰ THAM SỐ CHUẨN: (UINT cFileTypes, const COMDLG_FILTERSPEC*)
	//    = SỐ LƯỢNG TRƯỚC, MẢNG SAU — v3.4.1 đảo ngược → đọc "mảng" tại địa
	//    chỉ 0x2 → access violation ngay trước Show (crash chính của v3.4.1).
	type comFilter struct{ name, spec *uint16 }
	n16 := utf16Bytes(filterName)
	s16 := utf16Bytes(filterSpec)
	an16 := utf16Bytes("Tất cả tệp")
	as16 := utf16Bytes("*.*")
	specs := []comFilter{
		{&n16[0], &s16[0]},
		{&an16[0], &as16[0]},
	}
	hrSet := vcall(dlg, 4, uintptr(len(specs)), uintptr(unsafe.Pointer(&specs[0])))
	comLog("SetFileTypes(2 nhóm lọc) → 0x%08X", uint32(hrSet))

	// 4) GetOptions(10) → SetOptions(9): bắt buộc file + đường dẫn tồn tại.
	//    (v3.4.1 gọi 12/11 — thật ra là SetFolder/SetDefaultFolder nhận
	//    IShellItem* nhưng nhận về số nhỏ/địa chỉ rác → crash.)
	var opts uint32
	hrGet := vcall(dlg, 10, uintptr(unsafe.Pointer(&opts)))
	comLog("GetOptions → 0x%08X, opts=0x%08X", uint32(hrGet), opts)
	if hrGet == 0 {
		opts |= fosFileMustExist | fosPathMustExist | fosForceFileSystem | fosNoChangedDir
		hrSetOpts := vcall(dlg, 9, uintptr(opts))
		comLog("SetOptions(0x%08X) → 0x%08X", opts, uint32(hrSetOpts))
	}

	// 5) Show(3) — modal, tự chạy message loop; giữ tham chiếu buffer tới sau Show
	comLog("Show… (owner=0x%X)", owner)
	hr = vcall(dlg, 3, owner)
	runtime.KeepAlive(specs)
	runtime.KeepAlive(n16)
	runtime.KeepAlive(s16)
	runtime.KeepAlive(an16)
	runtime.KeepAlive(as16)
	runtime.KeepAlive(t16)
	comLog("Show → 0x%08X", uint32(hr))
	if uint32(hr) == hrCancelled {
		return "", nil // người dùng bấm Hủy — không phải lỗi
	}
	if hr != 0 {
		return "", fmt.Errorf("IFileOpenDialog::Show = 0x%08X", uint32(hr))
	}

	// 6) GetResult(18) → IShellItem — đúng mẫu chuẩn Microsoft cho chọn 1 file.
	//    (v3.4.1 gọi 22 — thật ra là SetClientGuid, không trả kết quả gì; sau
	//    đó gọi "Next" trên con trỏ nil — cơ may crash lần nữa.)
	var item uintptr
	hrRes := vcall(dlg, 18, uintptr(unsafe.Pointer(&item)))
	comLog("GetResult → 0x%08X, item=0x%X", uint32(hrRes), item)
	if hrRes != 0 || item == 0 {
		return "", fmt.Errorf("IFileOpenDialog::GetResult = 0x%08X", uint32(hrRes))
	}
	defer vcall(item, 2) // Release IShellItem

	// 7) IShellItem::GetDisplayName(5, SIGDN_FILESYSPATH) → đường dẫn TUYỆT ĐỐI
	var pPath uintptr
	hrName := vcall(item, 5, sigdnFileSysPath, uintptr(unsafe.Pointer(&pPath)))
	if hrName != 0 || pPath == 0 {
		comLog("GetDisplayName(SIGDN_FILESYSPATH) → 0x%08X", uint32(hrName))
		return "", fmt.Errorf("IShellItem::GetDisplayName = 0x%08X", uint32(hrName))
	}
	path := ptrToString(pPath)
	procCoTaskMemFree.Call(pPath)
	return path, nil
}

// cdErrText dịch mã CommDlgExtendedError thành chữ dễ hiểu.
func cdErrText(code uint32) string {
	switch code {
	case 0xFFFF:
		return "hộp thoại không thể khởi tạo"
	case 0x0001:
		return "cấu trúc dữ liệu sai kích thước"
	case 0x0002:
		return "khởi tạo hộp thoại thất bại (thiếu bộ nhớ?)"
	case 0x0003, 0x0004:
		return "thiếu template/hinstance"
	case 0x0005, 0x0006, 0x0007, 0x0008:
		return "không nạp được tài nguyên hộp thoại"
	case 0x0009, 0x000A:
		return "không cấp phát được bộ nhớ"
	case 0x3001:
		return "subclass danh sách thất bại"
	case 0x3002:
		return "tên file không hợp lệ"
	case 0x3003:
		return "bộ đệm tên file quá nhỏ"
	}
	return "lỗi không rõ"
}

// openFileLegacy là hộp thoại GetOpenFileNameW cũ — dùng làm phương án dự phòng.
func openFileLegacy(owner uintptr, title, filter string) (string, error) {
	defer LogPanic("file-dialog-legacy")
	buf := make([]uint16, 32768)
	f16 := syscall.StringToUTF16(filter)
	t16 := syscall.StringToUTF16(title)
	ofn := OPENFILENAMEW{
		LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})),
		HwndOwner:   owner,
		LpstrFilter: uintptr(unsafe.Pointer(&f16[0])),
		LpstrFile:   uintptr(unsafe.Pointer(&buf[0])),
		NMaxFile:    uint32(len(buf)),
		LpstrTitle:  uintptr(unsafe.Pointer(&t16[0])),
		Flags:       OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST | OFN_HIDEREADONLY | OFN_NOCHANGEDIR,
	}
	r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	runtime.KeepAlive(f16)
	runtime.KeepAlive(t16)
	runtime.KeepAlive(buf)
	if r == 0 {
		code, _, _ := procCommDlgExtendedError.Call()
		if code == 0 {
			return "", nil // người dùng bấm Hủy
		}
		return "", fmt.Errorf("GetOpenFileNameW lỗi 0x%04X — %s", uint16(code), cdErrText(uint32(code)))
	}
	return syscall.UTF16ToString(buf), nil
}

// browseForFile: COM trước, legacy dự phòng, luôn log. "" + nil = người dùng Hủy.
func browseForFile(owner uintptr, title, filterName, filterSpec, legacyFilter string) (string, error) {
	if CrashLog != nil {
		CrashLog("hộp thoại chọn file: mở (owner=0x%X, %s)", owner, title)
	}
	CoInitializeEx()
	path, errCom := openFileViaCom(owner, title, filterName, filterSpec)
	if errCom == nil {
		if CrashLog != nil {
			if path == "" {
				CrashLog("hộp thoại chọn file: người dùng bấm Hủy (COM)")
			} else {
				CrashLog("hộp thoại chọn file: đã chọn %q (COM)", path)
			}
		}
		return path, nil
	}
	// COM thất bại → thử hộp thoại legacy
	comLog("COM thất bại (%v) → thử hộp thoại legacy", errCom)
	path, errLegacy := openFileLegacy(owner, title, legacyFilter)
	if errLegacy == nil {
		if CrashLog != nil {
			CrashLog("hộp thoại chọn file: COM lỗi (%v) — đã dùng legacy, kết quả %q", errCom, path)
		}
		return path, nil
	}
	return "", fmt.Errorf("%v; dự phòng legacy: %v", errCom, errLegacy)
}

// BrowseForFile shows an open-file dialog (MP3/WAV).
// Trả về ("", nil) khi người dùng Hủy; error khi hộp thoại KHÔNG mở được.
func BrowseForFile(owner uintptr, title string) (string, error) {
	return browseForFile(owner, title,
		"Tệp âm thanh (*.mp3; *.wav)", "*.mp3;*.wav",
		"Tệp âm thanh (*.mp3; *.wav)\x00*.mp3;*.wav\x00Tất cả tệp\x00*.*\x00\x00")
}

// BrowseForExcelFile shows an open-file dialog filtered to .xlsx.
// Trả về ("", nil) khi người dùng Hủy; error khi hộp thoại KHÔNG mở được.
func BrowseForExcelFile(owner uintptr, title string) (string, error) {
	return browseForFile(owner, title,
		"Tệp Excel (*.xlsx)", "*.xlsx",
		"Tệp Excel (*.xlsx)\x00*.xlsx\x00Tất cả tệp\x00*.*\x00\x00")
}

// ExePath returns the full path of the running executable.
func ExePath() string {
	buf := make([]uint16, 1024)
	n, _, _ := procGetModuleFileNameW.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// --- single instance ------------------------------------------------------

// CreateSingleInstanceMutex returns true when we are the first instance.
// Dùng errno TRỰC TIẾP từ Call — gọi GetLastError riêng qua syscall có thể
// bị Go runtime clobber giữa hai lần gọi.
func CreateSingleInstanceMutex(name string) bool {
	_, _, err := procCreateMutexW.Call(0, 0, UTF16(name))
	return err != syscall.Errno(ERROR_ALREADY_EXISTS)
}

// --- registry autostart ---------------------------------------------------

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const autostartValue = "NhacSinhNhat"

func openRunKey(access uint32) (uintptr, error) {
	var hKey uintptr
	r, _, err := procRegOpenKeyExW.Call(
		uintptr(0x80000001), // HKEY_CURRENT_USER
		UTF16(runKeyPath), 0, uintptr(access), uintptr(unsafe.Pointer(&hKey)))
	if r != 0 {
		return 0, err
	}
	return hKey, nil
}

// AutostartEnabled reports whether the Run entry exists.
func AutostartEnabled() bool {
	hKey, err := openRunKey(KEY_QUERY_VALUE)
	if err != nil {
		return false
	}
	defer procRegCloseKey.Call(hKey)
	var typ uint32
	r, _, _ := procRegQueryValueExW.Call(hKey, UTF16(autostartValue), 0, uintptr(unsafe.Pointer(&typ)), 0, 0)
	return r == 0
}

// SetAutostart writes or removes the Run entry (quoted exe path).
func SetAutostart(enable bool) {
	hKey, err := openRunKey(KEY_SET_VALUE)
	if err != nil {
		return
	}
	defer procRegCloseKey.Call(hKey)
	if !enable {
		procRegDeleteValueW.Call(hKey, UTF16(autostartValue))
		return
	}
	exe := `"` + ExePath() + `"`
	s, _ := syscall.UTF16FromString(exe)
	data := unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*2)
	procRegSetValueExW.Call(hKey, UTF16(autostartValue), 0, uintptr(REG_SZ),
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
}
