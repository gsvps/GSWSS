//go:build windows

package tray

import (
	"runtime"
	"sync"
)

var ui = &uiThread{}

type uiThread struct {
	once sync.Once
	ch   chan func()
}

func (u *uiThread) start() {
	u.once.Do(func() {
		u.ch = make(chan func(), 8)
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			for fn := range u.ch {
				fn()
			}
		}()
	})
}

func (u *uiThread) run(fn func()) {
	done := make(chan struct{})
	u.ch <- func() {
		defer close(done)
		fn()
	}
	<-done
}

func startUIThread() {
	ui.start()
}

func runOnUI(fn func()) {
	ui.run(fn)
}
