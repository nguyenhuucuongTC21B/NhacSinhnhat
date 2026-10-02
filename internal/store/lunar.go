package store

import (
	"fmt"
	"math"
	"time"
)

// Hồ Ngọc Đức's Vietnamese lunar calendar algorithm.
// Ported from amlich-hnd.js (http://www.informatik.uni-leipzig.de/~duc/amlich/).

func jdFromDate(dd, mm, yy int) int {
	a := intDiv(14-mm, 12)
	y := yy + 4800 - a
	m := mm + 12*a - 3
	jd := dd + intDiv(153*m+2, 5) + 365*y + intDiv(y, 4) - intDiv(y, 100) + intDiv(y, 400) - 32045
	if jd < 2299161 {
		jd = dd + intDiv(153*m+2, 5) + 365*y + intDiv(y, 4) - 32083
	}
	return jd
}

func jdToDate(jd int) (int, int, int) {
	var a, b, c, d, e, m, day, month, year int
	if jd > 2299160 {
		a = jd + 32044
		b = intDiv(4*a+3, 146097)
		c = a - intDiv(b*146097, 4)
	} else {
		b = 0
		c = jd + 32082
	}
	d = intDiv(4*c+3, 1461)
	e = c - intDiv(1461*d, 4)
	m = intDiv(5*e+2, 153)
	day = e - intDiv(153*m+2, 5) + 1
	month = m + 3 - 12*intDiv(m, 10)
	year = b*100 + d - 4800 + intDiv(m, 10)
	return day, month, year
}

func newMoonFloat(k int) float64 {
	T := float64(k) / 1236.85
	T2 := T * T
	T3 := T2 * T
	dr := math.Pi / 180
	Jd1 := 2415020.75933 + 29.53058868*float64(k) + 0.0001178*T2 - 0.000000155*T3
	Jd1 = Jd1 + 0.00033*math.Sin((166.56+132.87*T-0.009173*T2)*dr)
	M := 359.2242 + 29.10535608*float64(k) - 0.0000333*T2 - 0.00000347*T3
	Mpr := 306.0253 + 385.81691806*float64(k) + 0.0107306*T2 + 0.00001236*T3
	F := 21.2964 + 390.67050646*float64(k) - 0.0016528*T2 - 0.00000239*T3
	C1 := (0.1734 - 0.000393*T) * math.Sin(M*dr)
	C1 += 0.0021 * math.Sin(2*dr*M)
	C1 -= 0.4068 * math.Sin(Mpr*dr)
	C1 += 0.0161 * math.Sin(dr*2*Mpr)
	C1 -= 0.0004 * math.Sin(dr*3*Mpr)
	C1 += 0.0104 * math.Sin(dr*2*F)
	C1 -= 0.0051 * math.Sin(dr*(M+Mpr))
	C1 -= 0.0074 * math.Sin(dr*(M-Mpr))
	C1 += 0.0004 * math.Sin(dr*(2*F+M))
	C1 -= 0.0004 * math.Sin(dr*(2*F-M))
	C1 -= 0.0006 * math.Sin(dr*(2*F+Mpr))
	C1 += 0.001 * math.Sin(dr*(2*F-Mpr))
	C1 += 0.0005 * math.Sin(dr*(2*Mpr+M))
	var deltat float64
	if T < -11 {
		deltat = 0.001 + 0.000839*T + 0.0002261*T2 - 0.00000845*T3 - 0.000000081*T*T3
	} else {
		deltat = -0.000278 + 0.000265*T + 0.000262*T2
	}
	return Jd1 + C1 - deltat
}

func getNewMoonDay(k int, timeZone float64) int {
	return int(newMoonFloat(k) + 0.5 + timeZone/24.0)
}

func sunLongitudeFloat(jdn float64) float64 {
	T := (jdn - 2451545.0) / 36525
	T2 := T * T
	dr := math.Pi / 180
	M := 357.52910 + 35999.05030*T - 0.0001559*T2 - 0.00000048*T*T2
	L0 := 280.46645 + 36000.76983*T + 0.0003032*T2
	DL := (1.914600 - 0.004817*T - 0.000014*T2) * math.Sin(dr*M)
	DL += (0.019993 - 0.000101*T) * math.Sin(dr*2*M)
	DL += 0.000290 * math.Sin(dr*3*M)
	L := (L0 + DL) * dr
	return L - math.Pi*2*math.Floor(L/(math.Pi*2))
}

func getSunLongitude(dayNumber int, timeZone float64) int {
	return int(sunLongitudeFloat(float64(dayNumber)-0.5-timeZone/24.0) / math.Pi * 6)
}

func getLunarMonth11(yy int, timeZone float64) int {
	off := jdFromDate(31, 12, yy) - 2415021
	k := int(float64(off) / 29.530588853)
	nm := getNewMoonDay(k, timeZone)
	sunLong := getSunLongitude(nm, timeZone)
	if sunLong >= 9 {
		nm = getNewMoonDay(k-1, timeZone)
	}
	return nm
}

func getLeapMonthOffset(a11 int, timeZone float64) int {
	k := int((float64(a11)-2415021.076998695)/29.530588853 + 0.5)
	last := 0
	i := 1
	arc := getSunLongitude(getNewMoonDay(k+i, timeZone), timeZone)
	for {
		last = arc
		i++
		arc = getSunLongitude(getNewMoonDay(k+i, timeZone), timeZone)
		if arc == last || i >= 14 {
			break
		}
	}
	return i - 1
}

// SolarToLunar converts a solar date to Vietnamese lunar date.
// Returns (lunarDay, lunarMonth, lunarYear, isLeapMonth, ok).
func SolarToLunar(dd, mm, yy int, timeZone float64) (int, int, int, int, bool) {
	dayNumber := jdFromDate(dd, mm, yy)
	k := int((float64(dayNumber) - 2415021.076998695) / 29.530588853)
	monthStart := getNewMoonDay(k+1, timeZone)
	if monthStart > dayNumber {
		monthStart = getNewMoonDay(k, timeZone)
	}
	a11 := getLunarMonth11(yy, timeZone)
	b11 := a11
	lunarYear := yy
	if a11 >= monthStart {
		lunarYear = yy
		a11 = getLunarMonth11(yy-1, timeZone)
	} else {
		lunarYear = yy + 1
		b11 = getLunarMonth11(yy+1, timeZone)
	}
	lunarDay := dayNumber - monthStart + 1
	diff := intDiv(monthStart-a11, 29)
	lunarLeap := 0
	lunarMonth := diff + 11
	if b11-a11 > 365 {
		leapMonthDiff := getLeapMonthOffset(a11, timeZone)
		if diff >= leapMonthDiff {
			lunarMonth = diff + 10
			if diff == leapMonthDiff {
				lunarLeap = 1
			}
		}
	}
	if lunarMonth > 12 {
		lunarMonth = lunarMonth - 12
	}
	if lunarMonth >= 11 && diff < 4 {
		lunarYear -= 1
	}
	return lunarDay, lunarMonth, lunarYear, lunarLeap, true
}

// LunarToSolar converts a lunar date to solar. leapMonth: 0 = normal month,
// 1 = the leap (nhuận) variant of that month number.
// Returns the solar date and whether conversion was valid.
func LunarToSolar(lunarDay, lunarMonth, lunarYear, leapMonth int, timeZone float64) (time.Time, bool) {
	if lunarDay < 1 || lunarDay > 30 || lunarMonth < 1 || lunarMonth > 12 {
		return time.Time{}, false
	}

	// a11: new moon beginning lunar month 11 that brackets our lunar year.
	var a11, b11 int
	if lunarMonth < 11 {
		a11 = getLunarMonth11(lunarYear-1, timeZone)
		b11 = getLunarMonth11(lunarYear, timeZone)
	} else {
		a11 = getLunarMonth11(lunarYear, timeZone)
		b11 = getLunarMonth11(lunarYear+1, timeZone)
	}

	// position (in lunations) of the requested month relative to a11
	pos := 0
	switch {
	case lunarMonth == 11:
		pos = 0
	case lunarMonth == 12:
		pos = 1
	default: // months 1..10
		pos = lunarMonth + 1 // 1→2, 2→3, ... 10→11
	}

	// detect leap month inside this lunar year span
	hasLeap := b11-a11 > 365
	if hasLeap {
		leapOff := getLeapMonthOffset(a11, timeZone) // lunation index of the leap month
		leapMonthNum := leapOff - 2
		if leapMonthNum < 0 {
			leapMonthNum += 12
		}
		if leapMonth == 1 {
			// user requested the leap variant
			if lunarMonth != leapMonthNum {
				return time.Time{}, false // this leap month does not exist
			}
			pos = leapOff
		} else if lunarMonth <= 10 && lunarMonth > leapMonthNum {
			pos++ // months after the leap month shift by one lunation
		}
	} else if leapMonth == 1 {
		return time.Time{}, false // no leap month in this lunar year
	}

	// k: new-moon index of a11 — align by iteration so newMoon(k) == a11 exactly
	k := int((float64(a11) - 2415021.076998695) / 29.530588853)
	for k > 0 && getNewMoonDay(k, timeZone) > a11 {
		k--
	}
	for getNewMoonDay(k, timeZone) < a11 {
		k++
	}
	monthStart := getNewMoonDay(k+pos, timeZone)
	d, m, y := jdToDate(monthStart + lunarDay - 1)
	t, ok := safeDate(y, m, d)
	return t, ok
}

// convenience wrappers using Vietnam timezone (+7).
const VNTimeZone = 7.0

// SolarToLunarVN converts solar -> lunar (VN +7).
func SolarToLunarVN(t time.Time) (int, int, int, int) {
	ld, lm, ly, leap, _ := SolarToLunar(t.Day(), int(t.Month()), t.Year(), VNTimeZone)
	return ld, lm, ly, leap
}

// LunarToSolarVN converts lunar -> solar (VN +7). leap 0/1.
func LunarToSolarVN(lunarDay, lunarMonth, lunarYear, leap int) (time.Time, bool) {
	return LunarToSolar(lunarDay, lunarMonth, lunarYear, leap, VNTimeZone)
}

// LunarDateString formats "d/M" lunar, with "N" marker for leap months.
func LunarDateString(day, month, leap int) string {
	if leap == 1 {
		return fmt.Sprintf("%d/%dN", day, month)
	}
	return fmt.Sprintf("%d/%d", day, month)
}

func intDiv(a, b int) int {
	if b == 0 {
		return 0
	}
	q := a / b
	if a%b != 0 && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}
