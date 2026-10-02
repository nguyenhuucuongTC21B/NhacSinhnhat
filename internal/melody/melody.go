//go:build windows && !nogui

package melody

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	procMciSendStringW = syscall.NewLazyDLL("winmm.dll").NewProc("mciSendStringW")
	procBeep           = syscall.NewLazyDLL("kernel32.dll").NewProc("Beep")
)

func mci(cmd string) string {
	buf := make([]uint16, 256)
	p, _ := syscall.UTF16PtrFromString(cmd)
	procMciSendStringW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	return syscall.UTF16ToString(buf)
}

var (
	mu       sync.Mutex
	aliasOn  bool
	stopChan chan struct{} // closed when built-in melody should stop
	beepWG   sync.WaitGroup
)

// PlayFile stops any current sound and loops the given MP3/WAV file.
func PlayFile(path string) {
	Stop()
	mu.Lock()
	mci(`open "` + path + `" type mpegvideo alias nsnmusic`)
	if mci("status nsnmusic mode") == "" {
		mci("close nsnmusic")
		mci(`open "` + path + `" type waveaudio alias nsnmusic`)
	}
	if mci("status nsnmusic mode") == "" {
		mci("close nsnmusic")
		mu.Unlock()
		// file MP3 hỏng/không đọc được — không được IM LẶNG: chuyển sang
		// giai điệu có sẵn để người dùng vẫn biết có cảnh báo
		PlayBuiltIn()
		return
	}
	aliasOn = true
	mci("play nsnmusic")
	stopPoll := make(chan struct{})
	mu.Unlock()

	go func() {
		t := time.NewTicker(700 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stopPoll:
				return
			case <-t.C:
				mu.Lock()
				if !aliasOn {
					mu.Unlock()
					return
				}
				if mci("status nsnmusic mode") == "stopped" {
					mci("seek nsnmusic to start")
					mci("play nsnmusic")
				}
				mu.Unlock()
			}
		}
	}()

	mu.Lock()
	if stopChan == nil {
		stopChan = stopPoll
	} else {
		close(stopPoll)
	}
	mu.Unlock()
}

// note is one melody entry: frequency (0 = rest) + duration ms.
type note struct {
	freq int
	ms   int
}

// happyBirthday is the built-in fallback melody (Happy Birthday in C major).
var happyBirthday = []note{
	{262, 300}, {262, 150}, {294, 450}, {262, 450}, {349, 450}, {330, 850},
	{262, 300}, {262, 150}, {294, 450}, {262, 450}, {392, 450}, {349, 850},
	{262, 300}, {262, 150}, {523, 450}, {440, 450}, {349, 450}, {349, 450}, {330, 850},
	{466, 300}, {466, 150}, {440, 450}, {349, 450}, {392, 450}, {349, 1100},
	{0, 500},
}

// PlayBuiltIn loops the built-in beep melody until Stop.
func PlayBuiltIn() {
	Stop()
	mu.Lock()
	ch := make(chan struct{})
	stopChan = ch
	mu.Unlock()
	beepWG.Add(1)
	go func() {
		defer beepWG.Done()
		for {
			for _, n := range happyBirthday {
				mu.Lock()
				done := stopChan == nil
				mu.Unlock()
				if done {
					return
				}
				if n.freq > 0 {
					procBeep.Call(uintptr(n.freq), uintptr(n.ms))
				} else {
					time.Sleep(time.Duration(n.ms) * time.Millisecond)
				}
			}
		}
	}()
}

// Stop silences everything.
func Stop() {
	mu.Lock()
	ch := stopChan
	stopChan = nil
	aliasOn = false
	mu.Unlock()
	if ch != nil {
		close(ch)
	}
	mci("stop nsnmusic")
	mci("close nsnmusic")
	beepWG.Wait()
}

// Playing reports whether any sound is active.
func Playing() bool {
	mu.Lock()
	defer mu.Unlock()
	return aliasOn || stopChan != nil
}
