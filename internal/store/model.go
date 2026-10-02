package store

import (
	"fmt"
	"strings"
	"time"
)

// CalendarKind marks whether a birthday is solar or lunar.
type CalendarKind int

const (
	Solar CalendarKind = iota
	Lunar
)

// Person is one row of danh-sach.xlsx.
type Person struct {
	Name     string // Ho va ten
	Day      int    // ngay sinh (theo loai lich)
	Month    int    // thang sinh (theo loai lich)
	Kind     CalendarKind
	WarnDays int    // nhac truoc bao nhieu ngay (default 3)
	Note     string // ghi chu
	MP3      string // file nhac rieng (duong dan tuyet doi hoac tuong doi voi data dir)
	Row      int    // row index in Excel (1-based, for write-back)
}

// solarMonthLen: số ngày tối đa của mỗi tháng dương lịch
// (tháng 2 tính tối đa 29 — năm nhuận; 30/2 không có thật).
var solarMonthLen = [13]int{0, 31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

// Valid checks day/month ranges AND real calendar dates
// (chặn 31/2, 31/4, 30/2… để NextOccurrence không trả ngày rác).
func (p Person) Valid() error {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return fmt.Errorf("thiếu họ và tên")
	}
	if p.Month < 1 || p.Month > 12 {
		return fmt.Errorf("tháng không hợp lệ: %d", p.Month)
	}
	if p.Kind == Lunar {
		if p.Day < 1 || p.Day > 30 {
			return fmt.Errorf("ngày âm lịch không hợp lệ: %d (tháng âm lịch chỉ có 29–30 ngày)", p.Day)
		}
		return nil
	}
	if p.Day < 1 || p.Day > solarMonthLen[p.Month] {
		return fmt.Errorf("ngày %d/%d không tồn tại trong dương lịch", p.Day, p.Month)
	}
	return nil
}

// DateString returns "dd/MM".
func (p Person) DateString() string {
	return fmt.Sprintf("%02d/%02d", p.Day, p.Month)
}

// KindString returns "DL"/"AL".
func (p Person) KindString() string {
	if p.Kind == Lunar {
		return "AL"
	}
	return "DL"
}

// NextOccurrence returns the next occurrence date (solar) of this birthday,
// strictly in the future (or today if it is today) relative to now.
// For lunar birthdays it finds the next solar date whose lunar date matches
// (day, month) using the Hồ Ngọc Đức algorithm with Vietnam timezone (+7).
func (p Person) NextOccurrence(now time.Time) time.Time {
	tz := 7.0
	y := now.Year()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	best := time.Time{}
	consider := func(c time.Time) {
		if sameDay(c, today) || c.After(today) {
			if best.IsZero() || c.Before(best) {
				best = c
			}
		}
	}

	if p.Kind == Solar {
		// quét tới y+8 để bắt 29/2 (khoảng cách năm nhuận tối đa 8 năm)
		for yy := y - 1; yy <= y+8; yy++ {
			if d, ok := safeDate(yy, p.Month, p.Day); ok {
				consider(d)
				if !best.IsZero() {
					break // đã tìm thấy lần gần nhất, đừng quét xa hơn
				}
			}
		}
	} else {
		// ngày 30 âm lịch có năm thiếu (tháng chỉ 29 ngày) → lùi về 29
		for yy := y - 1; yy <= y+3; yy++ {
			sd, ok := LunarToSolar(p.Day, p.Month, yy, 0, tz)
			if !ok && p.Day == 30 {
				sd, ok = LunarToSolar(29, p.Month, yy, 0, tz)
			}
			if ok {
				consider(sd)
				if !best.IsZero() {
					break
				}
			}
		}
	}

	if best.IsZero() {
		// không bao giờ xảy ra với ngày hợp lệ; sentinel để DaysLeft không tràn số
		return today.AddDate(100, 0, 0)
	}
	return best
}

// DaysLeft returns number of days until next occurrence (0 = today).
func (p Person) DaysLeft(now time.Time) int {
	next := p.NextOccurrence(now)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if next.Year()-today.Year() >= 99 {
		return 36500 // sentinel: ngày không hợp lệ / không tìm thấy
	}
	return int(next.Sub(today).Hours() / 24)
}

// OccurringToday reports whether the birthday is today.
func (p Person) OccurringToday(now time.Time) bool {
	return p.DaysLeft(now) == 0
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func safeDate(y, m, d int) (time.Time, bool) {
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, false
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local)
	if t.Day() != d || int(t.Month()) != m {
		return time.Time{}, false
	}
	return t, true
}
