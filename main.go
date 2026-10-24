package main

import (
	"flag"
	"log"

	"ancientartofwar/assets"
	"ancientartofwar/internal/original"
	"ancientartofwar/internal/ui"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	campaign := flag.Int("campaign", -1, "start a campaign directly (0..10)")
	seed := flag.Uint64("seed", 1, "deterministic simulation seed")
	save := flag.String("save", "savegame.json", "native save file")
	smoke := flag.Int("smoke", 0, "exit after this many frames")
	screenshot := flag.String("screenshot", "", "write a PNG before exiting smoke mode")
	demo := flag.Bool("demo", false, "automatically command both armies")
	touch := flag.Bool("touch", false, "show touch controls for desktop/device verification")
	preview := flag.String("preview", "", "open a view for rendering checks: title, war, tactical, editor, formations, leaders, rules")
	flag.Parse()
	l, err := original.Load(assets.Disk)
	if err != nil {
		log.Fatal(err)
	}
	g := ui.New(l, *save, *seed)
	g.Touch = *touch
	g.SmokeFrames = *smoke
	g.Screenshot = *screenshot
	g.Demo = *demo
	if *campaign >= 0 {
		if err := g.Start(*campaign); err != nil {
			log.Fatal(err)
		}
	}
	if *preview != "" {
		if err := g.Preview(*preview); err != nil {
			log.Fatal(err)
		}
	}
	if *smoke > 0 {
		ebiten.SetRunnableOnUnfocused(true)
	}
	windowWidth := 1024
	if *touch {
		windowWidth = 1724
	}
	ebiten.SetWindowSize(windowWidth, 768)
	ebiten.SetWindowTitle("The Ancient Art of War — Go / Ebitengine")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
