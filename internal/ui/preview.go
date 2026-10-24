package ui

import (
	"ancientartofwar/internal/game"
	"fmt"
)

// Preview opens a deterministic view for rendering verification. Only the
// tactical preview arranges an artificial encounter; ordinary games never do.
func (g *Game) Preview(name string) error {
	if err := g.Start(0); err != nil {
		return err
	}
	switch name {
	case "title":
		g.Scene = title
	case "campaigns":
		g.Scene = campaigns
	case "briefing":
		g.Scene = briefing
	case "rules":
		g.Scene = rules
	case "leaders":
		g.Scene = leaders
	case "war":
		g.Scene = war
	case "editor":
		g.beginEditor(false)
	case "formations":
		g.openFormation(true)
	case "options":
		g.Return = title
		g.Scene = options
	case "help":
		g.HelpReturn = title
		g.Scene = help
	case "tactical":
		var enemy int
		for i, q := range g.State.Squads {
			if q.Alive() && q.Side == game.Enemy {
				enemy = i
				break
			}
		}
		p := g.State.Squads[g.Selected].Position
		p.X++
		g.State.Squads[enemy].Position = p
		for i := 0; i < 4; i++ {
			g.State.Step()
		}
		b, err := g.State.Zoom(g.Selected)
		if err != nil {
			return err
		}
		g.BattleID = b.ID
		g.Scene = tactical
		g.result(g.State.CommandBattle(b.ID, -1, game.Attack))
	default:
		return fmt.Errorf("unknown preview %q", name)
	}
	return nil
}
