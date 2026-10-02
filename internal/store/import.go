package store

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Nhập danh sách từ file Excel có sẵn của người dùng.
// Hỗ trợ: thứ tự cột bất kỳ, tên cột tiếng Việt có/không dấu,
// cột "Ngày/Tháng" gộp hoặc cột "Ngày" + "Tháng" riêng,
// giá trị dạng 15/8, 15-8, 15.8, 15/8/1999, ô date của Excel.

var isoDateRe = regexp.MustCompile(`(\d{4})-(\d{1,2})-(\d{1,2})`)

var vnAccent = strings.NewReplacer(
	"à", "a", "á", "a", "ả", "a", "ã", "a", "ạ", "a",
	"ă", "a", "ằ", "a", "ắ", "a", "ẳ", "a", "ẵ", "a", "ặ", "a",
	"â", "a", "ầ", "a", "ấ", "a", "ẩ", "a", "ẫ", "a", "ậ", "a",
	"è", "e", "é", "e", "ẻ", "e", "ẽ", "e", "ẹ", "e",
	"ê", "e", "ề", "e", "ế", "e", "ể", "e", "ễ", "e", "ệ", "e",
	"ì", "i", "í", "i", "ỉ", "i", "ĩ", "i", "ị", "i",
	"ò", "o", "ó", "o", "ỏ", "o", "õ", "o", "ọ", "o",
	"ô", "o", "ồ", "o", "ố", "o", "ổ", "o", "ỗ", "o", "ộ", "o",
	"ơ", "o", "ờ", "o", "ớ", "o", "ở", "o", "ỡ", "o", "ợ", "o",
	"ù", "u", "ú", "u", "ủ", "u", "ũ", "u", "ụ", "u",
	"ư", "u", "ừ", "u", "ứ", "u", "ử", "u", "ữ", "u", "ự", "u",
	"ỳ", "y", "ý", "y", "ỷ", "y", "ỹ", "y", "ỵ", "y",
	"đ", "d",
	"À", "a", "Á", "a", "Ả", "a", "Ã", "a", "Ạ", "a",
	"Ă", "a", "Ằ", "a", "Ắ", "a", "Ẳ", "a", "Ẵ", "a", "Ặ", "a",
	"Â", "a", "Ầ", "a", "Ấ", "a", "Ẩ", "a", "Ẫ", "a", "Ậ", "a",
	"È", "e", "É", "e", "Ẻ", "e", "Ẽ", "e", "Ẹ", "e",
	"Ê", "e", "Ề", "e", "Ế", "e", "Ể", "e", "Ễ", "e", "Ệ", "e",
	"Ì", "i", "Í", "i", "Ỉ", "i", "Ĩ", "i", "Ị", "i",
	"Ò", "o", "Ó", "o", "Ỏ", "o", "Õ", "o", "Ọ", "o",
	"Ô", "o", "Ồ", "o", "Ố", "o", "Ổ", "o", "Ỗ", "o", "Ộ", "o",
	"Ơ", "o", "Ờ", "o", "Ớ", "o", "Ở", "o", "Ỡ", "o", "Ợ", "o",
	"Ù", "u", "Ú", "u", "Ủ", "u", "Ũ", "u", "Ụ", "u",
	"Ư", "u", "Ừ", "u", "Ứ", "u", "Ử", "u", "Ữ", "u", "Ự", "u",
	"Ỳ", "y", "Ý", "y", "Ỷ", "y", "Ỹ", "y", "Ỵ", "y",
	"Đ", "d",
)

// deaccent bỏ dấu + viết thường để so khớp tên cột.
func deaccent(s string) string { return strings.ToLower(vnAccent.Replace(s)) }

// cellInt đọc số nguyên từ ô Excel ("15", "15.0", 15).
func cellInt(v string) (int, bool) {
	t := strings.TrimSpace(v)
	if t == "" {
		return 0, false
	}
	if n, e := strconv.Atoi(t); e == nil {
		return n, true
	}
	if fl, e := strconv.ParseFloat(t, 64); e == nil {
		return int(fl), true
	}
	return 0, false
}

// normalizeDateCell chuyển ô date của Excel ("2015-10-05T00:00:00Z")
// thành "05/10" để ParseDayMonth đọc được.
func normalizeDateCell(v string) string {
	t := strings.TrimSpace(v)
	if m := isoDateRe.FindStringSubmatch(t); m != nil {
		return m[3] + "/" + m[2]
	}
	return t
}

// colMap là vị trí các cột trong file nguồn (-1 = không có).
type colMap struct{ name, date, kind, warn, note, mp3, day, month int }

func defaultColMap() colMap {
	return colMap{name: 0, date: 1, kind: 2, warn: 3, note: 4, mp3: 5, day: -1, month: -1}
}

// detectColumns dò vị trí cột từ dòng tiêu đề bằng từ khóa (đã bỏ dấu).
func detectColumns(header []string) colMap {
	cm := colMap{name: -1, date: -1, kind: -1, warn: -1, note: -1, mp3: -1, day: -1, month: -1}
	hd := make([]string, len(header))
	for i, h := range header {
		hd[i] = deaccent(h)
	}
	used := map[int]bool{}
	take := func(dst *int, subs ...string) {
		if *dst >= 0 {
			return
		}
		for j := range hd {
			if used[j] {
				continue
			}
			for _, s := range subs {
				if strings.Contains(hd[j], s) {
					*dst = j
					used[j] = true
					return
				}
			}
		}
	}
	// thứ tự ưu tiên tránh nhầm "Nhắc trước (ngày)" là cột ngày
	take(&cm.warn, "truoc", "warn")
	take(&cm.note, "ghi chu", "note")
	take(&cm.mp3, "mp3", "music", "bai hat", "nhac")
	take(&cm.name, "ho va ten", "ho ten", "ten", "name")
	take(&cm.kind, "loai", "kind", "lich")

	// cột "Ngày" và "Tháng" riêng, hay cột "Ngày/Tháng" gộp?
	dayCand, monthCand := -1, -1
	for j := range hd {
		if used[j] {
			continue
		}
		if monthCand < 0 && strings.Contains(hd[j], "thang") {
			monthCand = j
		}
		if dayCand < 0 && strings.Contains(hd[j], "ngay") {
			dayCand = j
		}
	}
	if dayCand >= 0 && monthCand >= 0 && dayCand != monthCand {
		cm.day, cm.month = dayCand, monthCand
		used[dayCand], used[monthCand] = true, true
	} else {
		take(&cm.date, "ngay/thang", "ngay thang", "sinh nhat", "date", "birthday", "ngay")
	}
	return cm
}

func (cm colMap) matchedCount() int {
	n := 0
	for _, v := range []int{cm.name, cm.date, cm.kind, cm.warn, cm.note, cm.mp3, cm.day, cm.month} {
		if v >= 0 {
			n++
		}
	}
	return n
}

func rowEmpty(r []string) bool {
	for _, c := range r {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// ReadPersonsFromExcel đọc danh sách người từ file .xlsx bất kỳ.
// Trả về danh sách người hợp lệ + danh sách cảnh báo các dòng bị bỏ qua.
// Có recover: file .xlsx lạ/hỏng làm excelize panic thì trả về error thay vì
// giết process (triệu chứng cũ: bấm Nhập Excel → phần mềm tự tắt).
func ReadPersonsFromExcel(src string) (persons []Person, warns []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			persons, warns = nil, nil
			err = fmt.Errorf("không đọc được file (%v)", r)
		}
	}()
	f, err := excelize.OpenFile(src)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var rows [][]string
	for _, sh := range f.GetSheetList() {
		r, err := f.GetRows(sh)
		if err != nil || len(r) == 0 {
			continue
		}
		rows = r
		break
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("file không có dữ liệu")
	}

	// có dòng tiêu đề không? (≥2 ô khớp từ khóa cột)
	cm := detectColumns(rows[0])
	header := cm.matchedCount() >= 2
	start := 0
	if header {
		start = 1
		if cm.name < 0 {
			cm.name = 0
		}
		if cm.date < 0 && cm.day < 0 {
			cm.date = 1
		}
	} else {
		cm = defaultColMap() // không có tiêu đề → giả định đúng thứ tự mẫu
	}

	get := func(r []string, idx int) string {
		if idx >= 0 && idx < len(r) {
			return strings.TrimSpace(r[idx])
		}
		return ""
	}

	persons = []Person{}
	warns = []string{}
	seen := map[string]bool{}
	for i, r := range rows[start:] {
		lineNo := i + start + 1
		name := get(r, cm.name)
		if name == "" {
			if !rowEmpty(r) {
				warns = append(warns, fmt.Sprintf("dòng %d: thiếu họ tên", lineNo))
			}
			continue
		}

		var d, m int
		if cm.day >= 0 || cm.month >= 0 {
			ok1, ok2 := true, true
			if cm.day >= 0 {
				d, ok1 = cellInt(get(r, cm.day))
			}
			if cm.month >= 0 {
				m, ok2 = cellInt(get(r, cm.month))
			}
			if !ok1 || !ok2 || d <= 0 || m <= 0 {
				warns = append(warns, fmt.Sprintf("dòng %d: \"%s\" thiếu/ sai ngày hoặc tháng", lineNo, name))
				continue
			}
		} else {
			var err error
			d, m, err = ParseDayMonth(normalizeDateCell(get(r, cm.date)))
			if err != nil {
				warns = append(warns, fmt.Sprintf("dòng %d: \"%s\" ngày/tháng không đọc được (%s)", lineNo, name, get(r, cm.date)))
				continue
			}
		}

		p := Person{Name: name, Day: d, Month: m, WarnDays: 3}
		if cm.kind >= 0 {
			p.Kind = ParseKind(get(r, cm.kind))
		}
		if cm.warn >= 0 {
			if v, ok := cellInt(get(r, cm.warn)); ok && v >= 0 && v <= 90 {
				p.WarnDays = v
			}
		}
		p.Note = get(r, cm.note)
		p.MP3 = get(r, cm.mp3)

		if err := p.Valid(); err != nil {
			warns = append(warns, fmt.Sprintf("dòng %d: %s", lineNo, err.Error()))
			continue
		}
		key := strings.ToLower(p.Name) + "|" + p.KindString() + "|" + p.DateString()
		if seen[key] {
			warns = append(warns, fmt.Sprintf("dòng %d: \"%s\" trùng trong file, bỏ qua", lineNo, name))
			continue
		}
		seen[key] = true
		persons = append(persons, p)
	}

	const maxWarns = 10
	if len(warns) > maxWarns {
		warns = append(warns[:maxWarns], fmt.Sprintf("… và %d dòng bỏ qua khác", len(warns)-maxWarns))
	}
	return persons, warns, nil
}

// ImportPersons nhập danh sách vào store rồi ghi ra Excel.
// replace=true: thay toàn bộ danh sách hiện tại.
// replace=false: gộp thêm, bỏ qua người trùng (tên + loại lịch + ngày).
// Trả về số người đã nhập và số dòng bỏ qua.
func (s *Store) ImportPersons(in []Person, replace bool) (imported, skipped int, err error) {
	key := func(p Person) string {
		return strings.ToLower(strings.TrimSpace(p.Name)) + "|" + p.KindString() + "|" + p.DateString()
	}

	s.mu.RLock()
	cur := make([]Person, len(s.persons))
	copy(cur, s.persons)
	s.mu.RUnlock()

	var out []Person
	seen := map[string]bool{}
	if replace {
		out = []Person{}
	} else {
		out = cur
		for _, p := range cur {
			seen[key(p)] = true
		}
	}
	for _, p := range in {
		if e := p.Valid(); e != nil {
			skipped++
			continue
		}
		k := key(p)
		if seen[k] {
			skipped++
			continue
		}
		seen[k] = true
		p.Row = 0
		out = append(out, p)
		imported++
	}
	if imported == 0 {
		return 0, skipped, nil
	}
	s.mu.Lock()
	s.persons = out
	s.mu.Unlock()
	if err := s.SaveAll(); err != nil {
		// hoàn tác — tránh RAM và file Excel lệch nhau
		s.mu.Lock()
		s.persons = cur
		s.mu.Unlock()
		return imported, skipped, err
	}
	return imported, skipped, nil
}
