package reminder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"nhac-sinh-nhat/internal/store"
)

// Candidate is a person entering the reminder window.
type Candidate struct {
	Person   store.Person
	DaysLeft int
	Next     time.Time
}

// Engine computes reminder candidates and remembers who was notified today.
type Engine struct {
	mu       sync.Mutex
	dataDir  string
	notified map[string]bool // "2006-01-02|name"
}

// NewEngine creates the engine using dataDir to persist notified state.
func NewEngine(dataDir string) *Engine {
	e := &Engine{dataDir: dataDir, notified: map[string]bool{}}
	e.load()
	return e
}

type notifiedFile struct {
	Date  string   `json:"date"`
	Names []string `json:"names"`
}

func (e *Engine) statePath() string {
	return filepath.Join(e.dataDir, "da-nhac.json")
}

func (e *Engine) load() {
	b, err := os.ReadFile(e.statePath())
	if err != nil {
		return
	}
	var nf notifiedFile
	if json.Unmarshal(b, &nf) == nil {
		today := time.Now().Format("2006-01-02")
		if nf.Date == today {
			for _, n := range nf.Names {
				e.notified[today+"|"+n] = true
			}
		}
	}
}

func (e *Engine) saveLocked(today string) {
	names := []string{}
	for k := range e.notified {
		if len(k) > 11 && k[:10] == today {
			names = append(names, k[11:])
		}
	}
	nf := notifiedFile{Date: today, Names: names}
	if b, err := json.Marshal(nf); err == nil {
		_ = os.WriteFile(e.statePath(), b, 0o644)
	}
}

// MarkNotified records that a person was alerted today (persists across restarts).
func (e *Engine) MarkNotified(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	e.notified[today+"|"+name] = true
	e.saveLocked(today)
}

// ResetToday clears the notified marks (nút "Tắt nhắc nhở ngay" không cần,
// nhưng hữu ích khi người dùng sửa file rồi muốn được nhắc lại).
func (e *Engine) ResetToday() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.notified = map[string]bool{}
	e.saveLocked(time.Now().Format("2006-01-02"))
}

// WasNotified reports whether a person was already alerted today.
func (e *Engine) WasNotified(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.notified[time.Now().Format("2006-01-02")+"|"+name]
}

// Candidates returns everyone whose next birthday is within warnDays
// (including today), sorted by days left, excluding already-notified ones.
// Lưu ý: WarnDays = 0 là HỢP LỆ — chỉ nhắc đúng ngày sinh nhật.
// Chỉ giá trị âm (không hợp lệ) mới được nâng về mặc định 3.
func (e *Engine) Candidates(persons []store.Person, now time.Time) []Candidate {
	today := now.Format("2006-01-02")
	out := []Candidate{}
	for _, p := range persons {
		dl := p.DaysLeft(now)
		warn := p.WarnDays
		if warn < 0 {
			warn = 3
		}
		// dl < 0: dữ liệu lỗi/zero-time — tuyệt đối không nhắc
		if dl < 0 || dl > warn {
			continue
		}
		if e.notified[today+"|"+p.Name] {
			continue
		}
		out = append(out, Candidate{
			Person:   p,
			DaysLeft: dl,
			Next:     p.NextOccurrence(now),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].DaysLeft < out[j].DaysLeft
	})
	return out
}

// Nearest returns the person with the fewest days left (anyone in list).
// Người có ngày không hợp lệ (DaysLeft sentinel 36500) bị bỏ qua;
// ok=false khi danh sách rỗng hoặc không ai có ngày hợp lệ.
func Nearest(persons []store.Person, now time.Time) (Candidate, bool) {
	best := Candidate{}
	found := false
	for _, p := range persons {
		dl := p.DaysLeft(now)
		if dl < 0 || dl >= 36500 { // ngày rác / không hợp lệ
			continue
		}
		if !found || dl < best.DaysLeft {
			best = Candidate{Person: p, DaysLeft: dl, Next: p.NextOccurrence(now)}
			found = true
		}
	}
	return best, found
}
