// Package original decodes original data into native Go structures.
package original

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"strings"

	"ancientartofwar/internal/disk"
	"ancientartofwar/internal/game"
)

type Library struct {
	Files      map[string][]byte
	Campaigns  []game.Campaign
	Leaders    [8]game.Leader
	Formations [10]game.Formation
	Tiles      [72][]byte
}

func Load(data []byte) (*Library, error) {
	files, err := disk.Read(data)
	if err != nil {
		return nil, err
	}
	l := &Library{Files: make(map[string][]byte)}
	for _, f := range files {
		l.Files[f.Name] = f.Data
	}
	m := l.Files["M"]
	if len(m) != 72*164 {
		return nil, fmt.Errorf("M: expected 72 map tiles, got %d bytes", len(m))
	}
	for i := range l.Tiles {
		b := m[i*164 : (i+1)*164]
		if word(b, 0) != 4 || word(b, 2) != 10 {
			return nil, fmt.Errorf("M: invalid tile %d", i+1)
		}
		l.Tiles[i] = append([]byte(nil), b[4:]...)
	}
	f := l.Files["F"]
	if len(f) != 420 {
		return nil, fmt.Errorf("F: expected 420 bytes")
	}
	for n := range l.Formations {
		for i := 0; i < 14; i++ {
			k := int(f[n*14+i])
			if k > 2 {
				return nil, fmt.Errorf("F: invalid troop kind")
			}
			l.Formations[n].Kind[i] = [...]game.Kind{game.Knight, game.Barbarian, game.Archer}[k]
			l.Formations[n].X[i] = int(f[140+n*14+i])
			l.Formations[n].Y[i] = int(f[280+n*14+i])
		}
	}
	o := l.Files["O2"]
	if len(o) < 0x3a7 {
		return nil, fmt.Errorf("O2: missing leader parameters")
	}
	names := []string{"Athena", "Alexander the Great", "Geronimo", "Crazy Ivan", "Julius Caesar", "Genghis Khan", "Napoleon Bonaparte", "Sun Tzu"}
	desc := []int{0, 0x44, 0x90, 0xde, 0x121, 0x169, 0x1b7, 0x200, 0x247}
	for i := range l.Leaders {
		leader := &l.Leaders[i]
		leader.Name = names[i]
		leader.Description = strings.Join(strings.Fields(strings.ReplaceAll(string(o[desc[i]:desc[i+1]]), "\x00", " ")), " ")
		for j := range leader.Parameters {
			leader.Parameters[j] = word(o, 0x277+i*38+j*2)
		}
	}
	titles := l.Files["TITLES"]
	if len(titles) != 876 {
		return nil, fmt.Errorf("TITLES: expected 876 bytes")
	}
	for n := 0; n < 11; n++ {
		id := string(titles[n*3 : n*3+2])
		title := cleanRows(titles[36+96+n*64 : 36+128+n*64])
		c, err := l.DecodeCampaign(id, title, l.Files[id])
		if err != nil {
			return nil, err
		}
		l.Campaigns = append(l.Campaigns, c)
	}
	return l, nil
}

func word(b []byte, i int) int { return int(int16(binary.LittleEndian.Uint16(b[i:]))) }

func (l *Library) DecodeCampaign(id, title string, b []byte) (game.Campaign, error) {
	c := game.Campaign{ID: id, Title: title, Tactical: game.TacticalResources{Primary: l.Files["O6"], Auxiliary: l.Files["O7"]}}
	for layout, name := range [...]string{"P0", "P4", "P5", "P2", "P1", "PA", "P6", "P8", "P3", "P7", "P9", "PB", "PC", "PD"} {
		c.Tactical.Backgrounds[layout] = l.Files[name]
	}
	if len(b) != 1460 {
		return c, fmt.Errorf("%s: expected 1460 bytes, got %d", id, len(b))
	}
	c.Story = cleanRows(b[:840])
	copy(c.World.Tiles[:], b[840:1040])
	if err := l.BuildWorld(&c.World); err != nil {
		return c, fmt.Errorf("%s: %w", id, err)
	}
	for i := 0; i < 20; i++ {
		s := &c.Squads[i]
		s.ID = i
		s.Position = game.Point{X: word(b, 1160+i*2), Y: word(b, 1200+i*2)}
		s.Side = game.Enemy
		if word(b, 1040+i*2) == -1 {
			s.Side = game.Player
		}
		s.Pace = word(b, 1080+i*2)
		s.Formation = int(b[1240+i])
		s.Condition = int(b[1260+i])
		s.Food = int(b[1280+i])
		s.AITarget = -1
		for k := 0; k < 4; k++ {
			s.Men[k] = int(b[1360+k*20+i])
		}
		if word(b, 1120+i*2) == 9999 {
			s.Men = [4]int{}
		}
		if s.Alive() && (!s.Position.Valid() || s.Count() > 14 || s.Pace < 1 || s.Pace > 3 || s.Formation < 1 || s.Formation > 9 || s.Condition < 1 || s.Condition > 49 || s.Food < 1 || s.Food > 49) {
			return c, fmt.Errorf("%s: invalid squad %d", id, i+1)
		}
	}
	for i := 0; i < 10; i++ {
		x := word(b, 1300+i*2)
		if x == -1 {
			continue
		}
		f := game.Flag{Position: game.Point{X: x, Y: word(b, 1320+i*2)}, Side: game.Enemy}
		if word(b, 1340+i*2) == -1 {
			f.Side = game.Player
		}
		if !f.Position.Valid() {
			return c, fmt.Errorf("%s: invalid flag", id)
		}
		c.Flags = append(c.Flags, f)
	}
	c.Opponent = word(b, 1440)
	c.Rules = game.Rules{VillageFood: word(b, 1442) != 0, FortFood: word(b, 1444) != 0, SupplyRange: word(b, 1446), Recruitment: word(b, 1448), Fitness: word(b, 1450), Visibility: word(b, 1452), WaterSpeed: word(b, 1454), MountainSpeed: word(b, 1456), ForestSpeed: word(b, 1458)}
	if c.Opponent < 0 || c.Opponent > 7 {
		return c, fmt.Errorf("%s: invalid opponent", id)
	}
	if err := c.Rules.Validate(); err != nil {
		return c, fmt.Errorf("%s: %w", id, err)
	}
	return c, nil
}

func cleanRows(b []byte) string {
	var lines []string
	for len(b) > 0 {
		n := min(32, len(b))
		var line strings.Builder
		for _, ch := range b[:n] {
			switch {
			case ch == 1:
			case ch == 9:
				line.WriteByte(' ')
			case ch >= 32 && ch <= 126:
				line.WriteByte(ch)
			default:
				line.WriteByte(' ')
			}
		}
		lines = append(lines, strings.TrimSpace(line.String()))
		b = b[n:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (l *Library) BuildWorld(w *game.World) error {
	for cell, id := range w.Tiles {
		if id < 1 || id > 72 {
			return fmt.Errorf("invalid tile %d at cell %d", id, cell)
		}
		tile := l.Tiles[id-1]
		x, y := cell%10*8, cell/10*10
		for row := 0; row < 10; row++ {
			for plane := 0; plane < 2; plane++ {
				copy(w.Pixels[plane*16000+(y+row)*80+x:], tile[row*16+plane*8:row*16+plane*8+8])
			}
		}
	}
	return nil
}

var CGA = [4]color.RGBA{{0, 0, 0, 255}, {0, 170, 170, 255}, {170, 0, 170, 255}, {255, 255, 255, 255}}

// MapImage reconstructs the original 320x400 interlaced CGA framebuffer.
func MapImage(w *game.World) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, 320, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 320; x++ {
			b := w.Pixels[(y%2)*16000+(y/2)*80+x/4]
			im.SetRGBA(x, y, CGA[(b>>uint(6-(x%4)*2))&3])
		}
	}
	return im
}
