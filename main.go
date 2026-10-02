package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"nhac-sinh-nhat/internal/reminder"
	"nhac-sinh-nhat/internal/store"
)

// OnExternalReload is set by the platform GUI to be notified when the
// Excel file changed on disk (3s watcher).
var OnExternalReload func(err error)

// RunConfig carries everything the GUI needs.
type RunConfig struct {
	Version  string
	AppName  string
	DataDir  string
	Store    *store.Store
	Engine   *reminder.Engine
	Logf     func(string, ...interface{})
	FirstRun bool
}

func main() {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	dataDir := filepath.Join(exeDir, "data")
	_ = os.MkdirAll(dataDir, 0o755)

	logf := setupLogger(filepath.Join(dataDir, "nhat-ky.log"))
	logf("===== %s v%s =====", AppName, Version)
	logf("thư mục dữ liệu: %s", dataDir)

	st := store.NewStore(dataDir)
	created, err := st.EnsureTemplate()
	if err != nil {
		logf("không tạo được file Excel mẫu: %v", err)
	} else if created {
		logf("lần đầu chạy — đã tạo file mẫu: %s", st.Path())
	}
	if err := st.Load(); err != nil {
		logf("tải danh sách: %v", err)
	} else {
		logf("đã tải %d người từ danh-sach.xlsx", st.Count())
	}

	engine := reminder.NewEngine(dataDir)

	// watch for external Excel changes every 3 seconds
	stop := st.StartWatcher(3*time.Second, func(err error) {
		if err != nil {
			logf("tải lại danh sách lỗi: %v", err)
			return
		}
		logf("tải lại danh sách tự động: %d người", st.Count())
		if OnExternalReload != nil {
			OnExternalReload(err)
		}
	})
	defer close(stop)

	cfg := RunConfig{
		Version:  Version,
		AppName:  AppName,
		DataDir:  dataDir,
		Store:    st,
		Engine:   engine,
		Logf:     logf,
		FirstRun: created,
	}
	Run(cfg)
}

func setupLogger(path string) func(string, ...interface{}) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return func(string, ...interface{}) {}
	}
	lg := log.New(io.MultiWriter(f), "", log.LstdFlags)
	return func(format string, args ...interface{}) {
		lg.Printf(format, args...)
	}
}
