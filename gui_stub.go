//go:build !windows || nogui

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xuri/excelize/v2"

	"nhac-sinh-nhat/internal/reminder"
	"nhac-sinh-nhat/internal/store"
)

// Run is the headless stub: on Linux/-tags nogui builds we only support
// the E2E self-test mode (NSN_E2E=1).
func Run(conf RunConfig) {
	if os.Getenv("NSN_E2E") == "1" {
		os.Exit(runE2E(conf))
	}
	fmt.Println("[Nhac Sinh Nhat] Bản build này KHÔNG có giao diện (headless).")
	fmt.Println("Đây chỉ là bản dùng để kiểm thử tự động (E2E) trên Linux.")
	fmt.Println("Bản giao diện Windows phải build KHÔNG dùng -tags nogui.")
	select {}
}

func runE2E(conf RunConfig) int {
	fails := 0
	check := func(name string, ok bool, extra ...interface{}) {
		if ok {
			fmt.Printf("E2E PASS: %s\n", name)
		} else {
			fails++
			fmt.Printf("E2E FAIL: %s %v\n", name, extra)
		}
	}

	// 0) E2E phải độc lập giữa các lần chạy: xóa sạch dữ liệu cũ
	//    (danh-sach.xlsx, da-nhac.json, các thư mục t2/t3/t4…)
	_ = os.RemoveAll(conf.DataDir)

	// 1) template + load (tạo lại file mẫu sau khi xóa)
	created, err := conf.Store.EnsureTemplate()
	check("tao-file-mau", err == nil && created, err)
	err = conf.Store.Load()
	check("tai-danh-sach", err == nil && conf.Store.Count() >= 2, conf.Store.Count())

	// 2) add / update / delete
	base := conf.Store.Count()
	p := store.Person{Name: "E2E Tester", Day: 25, Month: 12, Kind: store.Solar, WarnDays: 3}
	err = conf.Store.Add(p)
	check("them-nguoi", err == nil && conf.Store.Count() == base+1, err)
	snap := conf.Store.Snapshot()
	idx := -1
	for i, s := range snap {
		if s.Name == "E2E Tester" {
			idx = i
		}
	}
	err = conf.Store.Update(idx, store.Person{Name: "E2E Tester", Day: 1, Month: 1, Kind: store.Solar, WarnDays: 2})
	check("cap-nhat-nguoi", err == nil, err)
	err = conf.Store.Delete(idx)
	check("xoa-nguoi", err == nil && conf.Store.Count() == base, err)

	// 3) save/reload round-trip
	err = conf.Store.SaveAll()
	check("ghi-excel", err == nil, err)
	err = conf.Store.Load()
	check("doc-lai-excel", err == nil && conf.Store.Count() == base, conf.Store.Count())

	// 4) solar next occurrence: birthday today
	today := time.Now()
	solar := store.Person{Day: today.Day(), Month: int(today.Month()), Kind: store.Solar}
	check("ngay-xay-ra-toi-hom-nay", solar.DaysLeft(today) == 0, solar.DaysLeft(today))

	// solar in 5 days
	future := today.AddDate(0, 0, 5)
	fut := store.Person{Day: future.Day(), Month: int(future.Month()), Kind: store.Solar}
	check("con-5-ngay", fut.DaysLeft(today) == 5, fut.DaysLeft(today))

	// 5) lunar conversions (Hồ Ngọc Đức): mùng 1 Tết 2020–2030
	tetTable := []struct {
		solar string
		year  int
	}{
		{"2020-01-25", 2020}, {"2021-02-12", 2021}, {"2022-02-01", 2022},
		{"2023-01-22", 2023}, {"2024-02-10", 2024}, {"2025-01-29", 2025},
		{"2026-02-17", 2026}, {"2027-02-06", 2027}, {"2028-01-26", 2028},
		{"2029-02-13", 2029}, {"2030-02-02", 2030}, // 2030: VN mừng Tết 02/02 (múi giờ +7, TQ 03/02)
	}
	for _, tt := range tetTable {
		d, _ := time.Parse("2006-01-02", tt.solar)
		ld, lm, ly, leap := store.SolarToLunarVN(d)
		check("tet-"+tt.solar, ld == 1 && lm == 1 && ly == tt.year && leap == 0, ld, lm, ly, leap)
	}

	ld, lm, ly, leap := store.SolarToLunarVN(time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local))
	check("solar->lunar-tet-2025", ld == 1 && lm == 1 && ly == 2025 && leap == 0, ld, lm, ly, leap)
	sd, ok := store.LunarToSolarVN(15, 8, 2025, 0)
	check("lunar->solar-trung-thu-2025", ok && sd.Format("2006-01-02") == "2025-10-06", sd)
	sd, ok = store.LunarToSolarVN(8, 4, 2027, 0)
	check("lunar->solar-8-4-2027", ok && sd.Format("2006-01-02") == "2027-05-13", sd)
	sd, ok = store.LunarToSolarVN(1, 1, 2027, 0)
	check("lunar->solar-tet-2027", ok && sd.Format("2006-01-02") == "2027-02-06", sd)

	// leap month: tháng 6 nhuận 2025 → 25/07/2025 = 1/6N
	ld2, lm2, _, leap2 := store.SolarToLunarVN(time.Date(2025, 7, 25, 0, 0, 0, 0, time.Local))
	check("thang-nhuan-2025", ld2 == 1 && lm2 == 6 && leap2 == 1, ld2, lm2, leap2)

	// lunar birthday next occurrence: 8/4 AL
	lunarPerson := store.Person{Day: 8, Month: 4, Kind: store.Lunar}
	nx := lunarPerson.NextOccurrence(today)
	ldl, lml, _, leapl := store.SolarToLunarVN(nx)
	check("sinh-nhat-am-lich", ldl == 8 && lml == 4 && leapl == 0, nx, ldl, lml, leapl)

	// 6) reminder candidates + notify-once
	eng := reminder.NewEngine(conf.DataDir)
	todayPerson := store.Person{Name: "Hom Nay", Day: today.Day(), Month: int(today.Month()), Kind: store.Solar, WarnDays: 3}
	cands := eng.Candidates([]store.Person{todayPerson}, today)
	check("ung-vien-hom-nay", len(cands) == 1, len(cands))
	eng.MarkNotified("Hom Nay")
	cands = eng.Candidates([]store.Person{todayPerson}, today)
	check("da-nhac-thi-bo-qua", len(cands) == 0, len(cands))

	// 7) import từ file Excel ngoài (cột xáo trộn, tên cột có dấu,
	//    dòng lỗi ngày/tháng, dòng trùng trong file)
	importSrc := filepath.Join(conf.DataDir, "import-src.xlsx")
	ef := excelize.NewFile()
	sh := "Khach"
	if _, err := ef.NewSheet(sh); err != nil {
		check("tao-file-import", false, err)
	}
	_ = ef.DeleteSheet("Sheet1")
	impRows := [][]interface{}{
		{"Ghi chú", "Tên khách mời", "Loại lịch", "Nhắc trước", "Tháng", "Ngày sinh", "File nhạc"},
		{"test nhập", "Lê Thị Nhập", "AL", 5, 8, 15, ""},
		{"", "Trần Gộp", "DL", 3, 12, 25, ""},
		{"", "Dòng Lỗi", "DL", 3, 13, 40, ""}, // ngày 40 / tháng 13 — sai
		{"", "Trần Gộp", "DL", 3, 12, 25, ""}, // trùng dòng 3
	}
	for r, rv := range impRows {
		for c, v := range rv {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = ef.SetCellValue(sh, cell, v)
		}
	}
	if err := ef.SaveAs(importSrc); err != nil {
		check("tao-file-import", false, err)
	}
	ef.Close()
	persons, warns, err := store.ReadPersonsFromExcel(importSrc)
	okImp := err == nil && len(persons) == 2 && len(warns) == 2
	if okImp && (persons[0].Name != "Lê Thị Nhập" || persons[0].Day != 15 || persons[0].Month != 8 ||
		persons[0].Kind != store.Lunar || persons[0].WarnDays != 5) {
		okImp = false
	}
	check("doc-file-ngoai-cot-xao-tron", okImp, err, len(persons), len(warns), persons)

	n, sk, err := conf.Store.ImportPersons(persons, false) // gộp
	check("nhap-gop", err == nil && n == 2 && sk == 0, n, sk, err)
	n, sk, err = conf.Store.ImportPersons(persons, false) // gộp lần nữa → hết trùng store
	check("nhap-gop-trung-store", err == nil && n == 0 && sk == 2, n, sk, err)
	n, sk, err = conf.Store.ImportPersons(persons, true) // thay thế
	check("nhap-thay-the", err == nil && n == 2 && sk == 0 && conf.Store.Count() == 2, n, sk, conf.Store.Count(), err)
	err = conf.Store.Load() // file trên đĩa phải khớp danh sách sau nhập
	check("ghi-excel-sau-nhap", err == nil && conf.Store.Count() == 2, conf.Store.Count(), err)

	// file KHÔNG có dòng tiêu đề → giả định đúng thứ tự cột mẫu (A..F)
	noHdr := filepath.Join(conf.DataDir, "no-header.xlsx")
	ef2 := excelize.NewFile()
	sh2 := "SinhNhat"
	_, _ = ef2.NewSheet(sh2)
	_ = ef2.DeleteSheet("Sheet1")
	raw := [][]interface{}{
		{"Nguyễn Không Tiêu Đề", "5/10", "DL", 2, "", ""},
		{"Phạm Âm Lịch", "1/2", "AL", 3, "ghi chú", ""},
	}
	for r, rv := range raw {
		for c, v := range rv {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = ef2.SetCellValue(sh2, cell, v)
		}
	}
	_ = ef2.SaveAs(noHdr)
	ef2.Close()
	persons2, _, err := store.ReadPersonsFromExcel(noHdr)
	okNh := err == nil && len(persons2) == 2 &&
		persons2[0].Day == 5 && persons2[0].Month == 10 && persons2[0].WarnDays == 2 &&
		persons2[1].Kind == store.Lunar
	check("doc-file-khong-tieu-de", okNh, err, persons2)

	// 8) cơ chế nhắc: chặn dữ liệu lỗi, 29/2, 30 âm lịch, ngày xa
	badDir := filepath.Join(conf.DataDir, "t2")
	_ = os.MkdirAll(badDir, 0o755)
	st2 := store.NewStore(badDir)
	ef3 := excelize.NewFile()
	sh3 := "SinhNhat"
	_, _ = ef3.NewSheet(sh3)
	_ = ef3.DeleteSheet("Sheet1")
	rows3 := [][]interface{}{
		{"Họ và tên", "Ngày/Tháng", "Loại lịch", "Nhắc trước (ngày)", "Ghi chú", "File MP3"},
		{"Rac Thang 18", "5/18", "DL", 3, "", ""},      // tháng 18 — phải bị bỏ qua
		{"Ngay 31 Thang 2", "31/2", "DL", 3, "", ""},   // 31/2 không có thật — bị chặn
		{"Ngay 31 Thang 4", "31/4", "DL", 3, "", ""},   // 31/4 không có thật — bị chặn
		{"Ngay 30 Thang 2", "30/2", "DL", 3, "", ""},   // 30/2 không có thật — bị chặn
		{"Nguyễn Văn A", "18/8/1980", "DL", 3, "", ""}, // hợp lệ
		{"Hop Le 29 Thang 2", "29/2", "DL", 3, "", ""}, // 29/2 hợp lệ (năm nhuận)
	}
	for r, rv := range rows3 {
		for c, v := range rv {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = ef3.SetCellValue(sh3, cell, v)
		}
	}
	_ = ef3.SaveAs(filepath.Join(badDir, "danh-sach.xlsx"))
	ef3.Close()
	err = st2.Load()
	check("load-chan-ngay-khong-co-that", err == nil && st2.Count() == 2, err, st2.Count())
	snap2 := st2.Snapshot()
	if len(snap2) == 2 {
		dlA := snap2[0].DaysLeft(today)
		check("18-8-1980-daysleft-hop-le", dlA >= 0 && dlA <= 366, dlA)
	}
	feb29 := store.Person{Day: 29, Month: 2, Kind: store.Solar}
	dlF := feb29.DaysLeft(today)
	check("29-2-daysleft-rac", dlF >= 0 && dlF <= 1500, dlF)
	lun30 := store.Person{Day: 30, Month: 3, Kind: store.Lunar}
	dlL := lun30.DaysLeft(today)
	check("am-lich-30-3-daysleft-rac", dlL >= 0 && dlL <= 1500, dlL)

	eng2 := reminder.NewEngine(badDir)
	farDate := today.AddDate(0, 0, 100) // luôn cách ~100 ngày → không bao giờ nằm trong cửa sổ nhắc
	far := store.Person{Name: "Xa Voi", Day: farDate.Day(), Month: int(farDate.Month()), Kind: store.Solar, WarnDays: 3}
	check("khong-nhac-ngay-xa-100", len(eng2.Candidates([]store.Person{far}, today)) == 0, far.DaysLeft(today))
	bad := store.Person{Day: 31, Month: 2, Kind: store.Solar, WarnDays: 3}
	check("31-2-khong-bao-dong", len(eng2.Candidates([]store.Person{bad}, today)) == 0, bad.DaysLeft(today))
	nearDate := today.AddDate(0, 0, 2)
	nearP := store.Person{Name: "Gan Voi", Day: nearDate.Day(), Month: int(nearDate.Month()), Kind: store.Solar, WarnDays: 3}
	cs := eng2.Candidates([]store.Person{nearP}, today)
	check("co-nhac-trong-cua-so-2-ngay", len(cs) == 1 && cs[0].DaysLeft == 2, len(cs), cs)
	eng2.MarkNotified("Gan Voi")
	check("da-tat-thi-khong-nhac-lai", len(eng2.Candidates([]store.Person{nearP}, today)) == 0, nil)

	// 9) Nearest phải bỏ qua người có ngày không hợp lệ (popup kiểm tra không được hiện rác)
	_, okNear := reminder.Nearest([]store.Person{bad}, today)
	check("nearest-bo-qua-ngay-rac", !okNear, okNear)
	_, okNear2 := reminder.Nearest([]store.Person{bad, nearP}, today)
	check("nearest-chon-nguoi-hop-le", okNear2, okNear2)

	// 10) WarnDays = 0 hợp lệ: chỉ nhắc ĐÚNG ngày sinh nhật
	warn0Today := store.Person{Name: "Warn0 Hom Nay", Day: today.Day(), Month: int(today.Month()), Kind: store.Solar, WarnDays: 0}
	check("warn-0-nhac-dung-ngay", len(eng2.Candidates([]store.Person{warn0Today}, today)) == 1, nil)
	warn0Future := store.Person{Name: "Warn0 T roi", Day: nearDate.Day(), Month: int(nearDate.Month()), Kind: store.Solar, WarnDays: 0}
	check("warn-0-khong-nhac-som", len(eng2.Candidates([]store.Person{warn0Future}, today)) == 0, nil)

	// 11) hai người TRÙNG TÊN: sửa/xóa theo chỉ số phải đúng người
	dupDir := filepath.Join(conf.DataDir, "t3")
	_ = os.MkdirAll(dupDir, 0o755)
	st3 := store.NewStore(dupDir)
	_ = st3.Add(store.Person{Name: "Trung Ten", Day: 5, Month: 10, Kind: store.Solar, WarnDays: 3})
	_ = st3.Add(store.Person{Name: "Trung Ten", Day: 20, Month: 3, Kind: store.Solar, WarnDays: 7})
	_ = st3.Update(1, store.Person{Name: "Trung Ten", Day: 21, Month: 3, Kind: store.Solar, WarnDays: 7})
	_ = st3.Load()
	dups := st3.Snapshot()
	okDup := len(dups) == 2 && dups[0].Day == 5 && dups[0].Month == 10 && dups[1].Day == 21 && dups[1].Month == 3
	check("trung-ten-update-dung-nguoi", okDup, dups)
	_ = st3.Delete(0)
	_ = st3.Load()
	dups = st3.Snapshot()
	check("trung-ten-xoa-dung-nguoi", len(dups) == 1 && dups[0].Day == 21, dups)

	// 12) ô date thật của Excel (ISO) trong danh-sach.xlsx cũng phải đọc được
	isoDir := filepath.Join(conf.DataDir, "t4")
	_ = os.MkdirAll(isoDir, 0o755)
	st4 := store.NewStore(isoDir)
	ef4 := excelize.NewFile()
	sh4 := "SinhNhat"
	_, _ = ef4.NewSheet(sh4)
	_ = ef4.DeleteSheet("Sheet1")
	rows4 := [][]interface{}{
		{"Họ và tên", "Ngày/Tháng", "Loại lịch", "Nhắc trước (ngày)", "Ghi chú", "File MP3"},
		{"Nguyen ISO", "1980-08-18T00:00:00Z", "DL", 3, "", ""},
	}
	for r, rv := range rows4 {
		for c, v := range rv {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = ef4.SetCellValue(sh4, cell, v)
		}
	}
	_ = ef4.SaveAs(filepath.Join(isoDir, "danh-sach.xlsx"))
	ef4.Close()
	err = st4.Load()
	isoList := st4.Snapshot()
	okIso := err == nil && len(isoList) == 1 && isoList[0].Day == 18 && isoList[0].Month == 8
	check("load-o-date-iso", okIso, err, isoList)

	// 13) file Excel HỎNG / KHÔNG ĐÚNG LOẠI: phải trả về error, tuyệt đối
	//     không được crash process (triệu chứng cũ: bấm Nhập Excel → phần
	//     mềm tự tắt). Guard recover trong ReadPersonsFromExcel/Load.
	garbage := filepath.Join(conf.DataDir, "hong.xlsx")
	_ = os.WriteFile(garbage, []byte("day khong phai file zip/excel"), 0o644)
	gp, gw, err := store.ReadPersonsFromExcel(garbage)
	check("file-hong-khong-crash", err != nil && len(gp) == 0 && len(gw) == 0, err, gp)

	corruptDir := filepath.Join(conf.DataDir, "t5")
	_ = os.MkdirAll(corruptDir, 0o755)
	_ = os.WriteFile(filepath.Join(corruptDir, "danh-sach.xlsx"), []byte{0x50, 0x4B, 0x00, 0x01}, 0o644)
	st5 := store.NewStore(corruptDir)
	err = st5.Load()
	check("load-file-hong-khong-crash", err != nil, err)

	// 14) file EXCEL "thực tế" của người dùng: STT + Họ và Tên +
	//     Ngày sinh + Tháng (cột riêng) + Loại lịch + Nhắc trước
	realDir := filepath.Join(conf.DataDir, "t6")
	_ = os.MkdirAll(realDir, 0o755)
	ef6 := excelize.NewFile()
	sh6 := "Khach Hang"
	_, _ = ef6.NewSheet(sh6)
	_ = ef6.DeleteSheet("Sheet1")
	rows6 := [][]interface{}{
		{"STT", "Họ và Tên", "Ngày sinh", "Tháng", "Loại lịch", "Nhắc trước (ngày)"},
		{1, "Nguyễn Thực Tế", 15, 8, "AL", 5},
		{2, "Trần Thực Tế 2", 3, 12, "DL", 2},
		{3, "Nguyễn Thực Tế", 15, 8, "AL", 5}, // trùng dòng 2
	}
	for r, rv := range rows6 {
		for c, v := range rv {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = ef6.SetCellValue(sh6, cell, v)
		}
	}
	_ = ef6.SaveAs(filepath.Join(realDir, "danh-sach.xlsx"))
	ef6.Close()
	rp, rw, err := store.ReadPersonsFromExcel(filepath.Join(realDir, "danh-sach.xlsx"))
	okReal := err == nil && len(rp) == 2 && len(rw) == 1 &&
		rp[0].Name == "Nguyễn Thực Tế" && rp[0].Day == 15 && rp[0].Month == 8 &&
		rp[0].Kind == store.Lunar && rp[0].WarnDays == 5 &&
		rp[1].Day == 3 && rp[1].Month == 12
	check("doc-file-header-thuc-te-cot-rieng", okReal, err, rp, rw)

	// 15) "Nhắc trước" ngoài 0–90 trong file phải bị chốt về 3
	clampDir := filepath.Join(conf.DataDir, "t7")
	_ = os.MkdirAll(clampDir, 0o755)
	ef7 := excelize.NewFile()
	sh7 := "SinhNhat"
	_, _ = ef7.NewSheet(sh7)
	_ = ef7.DeleteSheet("Sheet1")
	rows7 := [][]interface{}{
		{"Họ và tên", "Ngày/Tháng", "Loại lịch", "Nhắc trước (ngày)", "Ghi chú", "File MP3"},
		{"Warn 999", "5/10", "DL", 999, "", ""},
		{"Warn Am", "6/10", "DL", -5, "", ""},
	}
	for r, rv := range rows7 {
		for c, v := range rv {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = ef7.SetCellValue(sh7, cell, v)
		}
	}
	_ = ef7.SaveAs(filepath.Join(clampDir, "danh-sach.xlsx"))
	ef7.Close()
	st7 := store.NewStore(clampDir)
	_ = st7.Load()
	clamped := st7.Snapshot()
	okClamp := len(clamped) == 2 && clamped[0].WarnDays == 3 && clamped[1].WarnDays == 3
	check("load-chot-warn-ngoai-khoang", okClamp, clamped)

	// 16) ParseDayMonth chấp nhận các định dạng phổ biến
	p1, m1, e1 := store.ParseDayMonth("15/8")
	p2, m2, e2 := store.ParseDayMonth(" 5.10 ")
	p3, m3, e3 := store.ParseDayMonth("05-10")
	p4, m4, e4 := store.ParseDayMonth("18/8/1980")
	_, _, e5 := store.ParseDayMonth("")
	_, _, e6 := store.ParseDayMonth("abc")
	okParse := e1 == nil && p1 == 15 && m1 == 8 &&
		e2 == nil && p2 == 5 && m2 == 10 &&
		e3 == nil && p3 == 5 && m3 == 10 &&
		e4 == nil && p4 == 18 && m4 == 8 &&
		e5 != nil && e6 != nil
	check("parse-ngay-thang-dinh-dang", okParse, p1, m1, p2, m2, p3, m3, p4, m4, e1, e2, e3, e4, e5, e6)

	if fails == 0 {
		fmt.Println("E2E: TẤT CẢ ĐỀU PASS ✔")
		return 0
	}
	fmt.Printf("E2E: %d test FAIL ✘\n", fails)
	return 1
}
