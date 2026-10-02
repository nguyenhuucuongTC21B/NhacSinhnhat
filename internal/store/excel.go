package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
)

const SheetName = "SinhNhat"

var headerCells = []string{
	"Họ và tên", "Ngày/Tháng", "Loại lịch",
	"Nhắc trước (ngày)", "Ghi chú", "File MP3",
}

// Store keeps the person list in memory, synced with the Excel file.
type Store struct {
	mu      sync.RWMutex
	path    string
	dataDir string
	persons []Person
	// internal marker: mtime of our own last write (skip reload echo)
	lastOwnWrite time.Time
}

// NewStore creates the store bound to dataDir/danh-sach.xlsx.
func NewStore(dataDir string) *Store {
	return &Store{
		path:    filepath.Join(dataDir, "danh-sach.xlsx"),
		dataDir: dataDir,
	}
}

// Path returns the Excel file path.
func (s *Store) Path() string { return s.path }

// EnsureTemplate creates a sample Excel file if it does not exist.
// Returns true when a new file was created.
func (s *Store) EnsureTemplate() (bool, error) {
	if _, err := os.Stat(s.path); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return false, err
	}
	f := excelize.NewFile()
	defer f.Close()
	if _, err := f.NewSheet(SheetName); err != nil {
		return false, err
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return false, err
	}
	for i, h := range headerCells {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(SheetName, cell, h); err != nil {
			return false, err
		}
	}
	sample := []Person{
		{Name: "Nguyễn Văn An", Day: 1, Month: 1, Kind: Solar, WarnDays: 3, Note: "Bản mẫu – sửa hoặc xóa"},
		{Name: "Trần Thị Bình", Day: 15, Month: 8, Kind: Lunar, WarnDays: 3, Note: "Sinh nhật âm lịch"},
	}
	for r, p := range sample {
		if err := writePersonRow(f, r+2, p); err != nil {
			return false, err
		}
	}
	if err := styleHeader(f); err != nil {
		return false, err
	}
	if err := f.SaveAs(s.path); err != nil {
		return false, err
	}
	s.lastOwnWrite = time.Now()
	return true, nil
}

func styleHeader(f *excelize.File) error {
	style, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"7C3AED"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(SheetName, "A1", "F1", style); err != nil {
		return err
	}
	widths := map[string]float64{"A": 28, "B": 13, "C": 11, "D": 17, "E": 24, "F": 30}
	for col, w := range widths {
		if err := f.SetColWidth(SheetName, col, col, w); err != nil {
			return err
		}
	}
	return nil
}

func writePersonRow(f *excelize.File, row int, p Person) error {
	cells := []interface{}{
		p.Name,
		p.DateString(),
		p.KindString(),
		p.WarnDays,
		p.Note,
		p.MP3,
	}
	for i, v := range cells {
		cell, err := excelize.CoordinatesToCellName(i+1, row)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(SheetName, cell, v); err != nil {
			return err
		}
	}
	return nil
}

// Load reads (or reloads) the Excel file into memory.
// Có recover: file lạ làm excelize panic thì trả về error thay vì giết process
// (watcher chạy trên goroutine riêng — panic sẽ tắt cả ứng dụng).
func (s *Store) Load() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("lỗi đọc file Excel: %v", r)
		}
	}()
	f, err := excelize.OpenFile(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	rows, err := f.GetRows(SheetName)
	if err != nil {
		return err
	}
	persons := []Person{}
	for i, row := range rows {
		if i == 0 {
			continue // header
		}
		// tolerate short rows
		get := func(idx int) string {
			if idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		name := get(0)
		if name == "" {
			continue
		}
		p := Person{Row: i + 1}
		p.Name = name
		// normalizeDateCell: ô date thật của Excel ("2015-08-18T00:00:00Z") → "18/8"
		d, m, err := ParseDayMonth(normalizeDateCell(get(1)))
		if err == nil {
			p.Day, p.Month = d, m
		}
		p.Kind = ParseKind(get(2))
		if wd, err := strconv.Atoi(get(3)); err == nil && wd >= 0 && wd <= 90 {
			p.WarnDays = wd
		} else {
			p.WarnDays = 3
		}
		p.Note = get(4)
		p.MP3 = get(5)
		// chặn dữ liệu lỗi từ file ngoài (tháng 18, 31/2, 31/4, 30/2…)
		if p.Valid() == nil {
			persons = append(persons, p)
		}
	}
	s.mu.Lock()
	s.persons = persons
	s.mu.Unlock()
	return nil
}

// ParseKind accepts DL/AL and Vietnamese variants.
func ParseKind(s string) CalendarKind {
	t := strings.ToUpper(strings.TrimSpace(s))
	switch t {
	case "AL", "ÂM", "ÂM LỊCH", "AM", "AM LICH", "LUNAR", "N", "NÔNG", "NÔNG LỊCH":
		return Lunar
	default:
		return Solar
	}
}

// ParseDayMonth accepts "5/10", "05-10", "5.10", "10/5/1999" (ignores year).
func ParseDayMonth(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, fmt.Errorf("trống")
	}
	repl := strings.NewReplacer("-", "/", ".", "/", " ", "/")
	parts := strings.Split(repl.Replace(s), "/")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("thiếu ngày/tháng: %q", s)
	}
	d, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("không đọc được ngày/tháng: %q", s)
	}
	return d, m, nil
}

// Snapshot returns a copy of the list sorted by next-occurrence.
func (s *Store) Snapshot() []Person {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Person, len(s.persons))
	copy(out, s.persons)
	return out
}

// Count returns number of persons.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.persons)
}

// ByIndex returns a copy of person at index i.
func (s *Store) ByIndex(i int) (Person, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i < 0 || i >= len(s.persons) {
		return Person{}, false
	}
	return s.persons[i], true
}

// Add inserts a person and writes back to Excel (append at end).
// Nếu ghi file thất bại (vd file đang mở trong Excel) thì hoàn tác danh sách
// trong bộ nhớ để RAM và đĩa không lệch nhau.
func (s *Store) Add(p Person) error {
	if err := p.Valid(); err != nil {
		return err
	}
	s.mu.Lock()
	old := make([]Person, len(s.persons))
	copy(old, s.persons)
	p.Row = 0
	s.persons = append(s.persons, p)
	s.mu.Unlock()
	if err := s.SaveAll(); err != nil {
		s.mu.Lock()
		s.persons = old
		s.mu.Unlock()
		return err
	}
	return nil
}

// Update replaces person at index and writes back (hoàn tác nếu ghi lỗi).
func (s *Store) Update(i int, p Person) error {
	if err := p.Valid(); err != nil {
		return err
	}
	s.mu.Lock()
	if i < 0 || i >= len(s.persons) {
		s.mu.Unlock()
		return fmt.Errorf("chỉ số không hợp lệ")
	}
	old := make([]Person, len(s.persons))
	copy(old, s.persons)
	p.Row = s.persons[i].Row
	s.persons[i] = p
	s.mu.Unlock()
	if err := s.SaveAll(); err != nil {
		s.mu.Lock()
		s.persons = old
		s.mu.Unlock()
		return err
	}
	return nil
}

// Delete removes person at index and writes back (hoàn tác nếu ghi lỗi).
func (s *Store) Delete(i int) error {
	s.mu.Lock()
	if i < 0 || i >= len(s.persons) {
		s.mu.Unlock()
		return fmt.Errorf("chỉ số không hợp lệ")
	}
	old := make([]Person, len(s.persons))
	copy(old, s.persons)
	s.persons = append(s.persons[:i], s.persons[i+1:]...)
	s.mu.Unlock()
	if err := s.SaveAll(); err != nil {
		s.mu.Lock()
		s.persons = old
		s.mu.Unlock()
		return err
	}
	return nil
}

// SaveAll writes the full list back to the Excel file.
func (s *Store) SaveAll() error {
	s.mu.RLock()
	persons := make([]Person, len(s.persons))
	copy(persons, s.persons)
	s.mu.RUnlock()

	f := excelize.NewFile()
	defer f.Close()
	if _, err := f.NewSheet(SheetName); err != nil {
		return err
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return err
	}
	for i, h := range headerCells {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(SheetName, cell, h); err != nil {
			return err
		}
	}
	for r, p := range persons {
		if err := writePersonRow(f, r+2, p); err != nil {
			return err
		}
	}
	if err := styleHeader(f); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if err := f.SaveAs(s.path); err != nil {
		return err
	}
	s.mu.Lock()
	s.lastOwnWrite = time.Now()
	// refresh row indexes
	for i := range s.persons {
		s.persons[i].Row = i + 2
	}
	s.mu.Unlock()
	return nil
}

// StartWatcher polls the file mtime every interval and calls onChanged
// when the file changed externally (not by our own SaveAll).
func (s *Store) StartWatcher(interval time.Duration, onChanged func(err error)) chan struct{} {
	stop := make(chan struct{})
	go func() {
		var lastMod time.Time
		if st, err := os.Stat(s.path); err == nil {
			lastMod = st.ModTime()
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				st, err := os.Stat(s.path)
				if err != nil {
					continue
				}
				if st.ModTime().Equal(lastMod) {
					continue
				}
				lastMod = st.ModTime()
				// ignore echo of our own write
				s.mu.RLock()
				own := s.lastOwnWrite
				s.mu.RUnlock()
				if time.Since(own) < 2*time.Second {
					continue
				}
				err = s.Load()
				if onChanged != nil {
					onChanged(err)
				}
			}
		}
	}()
	return stop
}

// SortByNext sorts persons in place by days left.
func SortByNext(persons []Person, now time.Time) {
	sort.SliceStable(persons, func(i, j int) bool {
		return persons[i].DaysLeft(now) < persons[j].DaysLeft(now)
	})
}
