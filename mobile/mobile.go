//go:build android

// Package mobile is the thin Android lifecycle bridge to the shared Go game.
package mobile

import (
	"path/filepath"
	"sync"
	"time"

	"ancientartofwar/assets"
	"ancientartofwar/internal/original"
	"ancientartofwar/internal/ui"
	"github.com/hajimehoshi/ebiten/v2"
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"
)

type host struct {
	mu                      sync.Mutex
	game                    *ui.Game
	configured, pause, back bool
	savePath                string
	pauseAcks               []chan struct{}
}

var gameHost = &host{}

func init() { ebiten.SetTPS(60); ebiten.SetScreenFilterEnabled(false); enginemobile.SetGame(gameHost) }
func Configure(savePath string) {
	gameHost.mu.Lock()
	gameHost.configured = true
	gameHost.savePath = savePath
	gameHost.mu.Unlock()
}

// Suspend waits for the game thread to commit its save before Java suspends
// the render thread. Merely queuing the request loses that save if Android
// kills the process while the application is in the background.
func Suspend() {
	ack := make(chan struct{})
	gameHost.mu.Lock()
	gameHost.pause = true
	gameHost.pauseAcks = append(gameHost.pauseAcks, ack)
	gameHost.mu.Unlock()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ack:
	case <-timer.C:
	}
}
func Back()  { gameHost.mu.Lock(); gameHost.back = true; gameHost.mu.Unlock() }
func Dummy() {}

func (h *host) Update() error {
	h.mu.Lock()
	ready, pause, back, path := h.configured, h.pause, h.back, h.savePath
	acks := h.pauseAcks
	h.pauseAcks = nil
	h.pause, h.back = false, false
	h.mu.Unlock()
	defer func() {
		for _, ack := range acks {
			close(ack)
		}
	}()
	if h.game == nil && ready {
		library, err := original.Load(assets.Disk)
		if err != nil {
			return err
		}
		h.game = ui.NewWithOptions(library, ui.Options{SavePath: path, DataRoot: filepath.Dir(path), Touch: true, Seed: 1})
	}
	if h.game == nil {
		return nil
	}
	if pause {
		h.game.Suspend()
	}
	if back {
		h.game.Back()
	}
	return h.game.Update()
}
func (h *host) Draw(screen *ebiten.Image) {
	if h.game != nil {
		h.game.Draw(screen)
	}
}
func (h *host) Layout(w, hgt int) (int, int) {
	if h.game != nil {
		return h.game.Layout(w, hgt)
	}
	return max(w, 1), max(hgt, 1)
}
