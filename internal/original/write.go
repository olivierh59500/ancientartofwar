package original

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"ancientartofwar/internal/game"
)

// EncodeCampaign produces the original 1460-byte DOS scenario format. Generated
// campaigns therefore use the same parser and map pieces as the supplied disk.
func EncodeCampaign(c game.Campaign) ([]byte, error) {
	if err := c.Rules.Validate(); err != nil {
		return nil, err
	}
	if len(c.Flags) > 10 || c.Opponent < 0 || c.Opponent > 7 {
		return nil, fmt.Errorf("invalid campaign")
	}
	b := make([]byte, 1460)
	for i := 0; i < 840; i++ {
		b[i] = ' '
	}
	rows := strings.Split(c.Story, "\n")
	at := 0
	for _, row := range rows {
		for len(row) > 32 {
			copy(b[at:min(840, at+32)], row[:32])
			at += 32
			row = row[32:]
			if at >= 840 {
				break
			}
		}
		if at >= 840 {
			break
		}
		copy(b[at:min(840, at+32)], row)
		at += 32
	}
	copy(b[840:1040], c.World.Tiles[:])
	put := func(off, v int) { binary.LittleEndian.PutUint16(b[off:], uint16(int16(v))) }
	for i, q := range c.Squads {
		if q.Count() > 14 {
			return nil, fmt.Errorf("squad %d exceeds 14 men", i+1)
		}
		if q.Alive() && (!q.Position.Valid() || q.Pace < 1 || q.Pace > 3 || q.Formation < 1 || q.Formation > 9 || q.Food < 1 || q.Food > 49 || q.Condition < 1 || q.Condition > 49) {
			return nil, fmt.Errorf("invalid active squad %d", i+1)
		}
		put(1040+i*2, 0)
		if q.Side == game.Player {
			put(1040+i*2, -1)
		}
		put(1080+i*2, q.Pace)
		if !q.Alive() {
			put(1120+i*2, 9999)
		}
		put(1160+i*2, q.Position.X)
		put(1200+i*2, q.Position.Y)
		b[1240+i] = byte(q.Formation)
		b[1260+i] = byte(q.Condition)
		b[1280+i] = byte(q.Food)
		for k, n := range q.Men {
			if n < 0 || n > 14 {
				return nil, fmt.Errorf("invalid troop count")
			}
			b[1360+k*20+i] = byte(n)
		}
	}
	for i := 0; i < 10; i++ {
		put(1300+i*2, -1)
	}
	for i, f := range c.Flags {
		if !f.Position.Valid() {
			return nil, fmt.Errorf("invalid flag position")
		}
		put(1300+i*2, f.Position.X)
		put(1320+i*2, f.Position.Y)
		if f.Side == game.Player {
			put(1340+i*2, -1)
		}
	}
	r := c.Rules
	values := []int{c.Opponent, 0, 0, r.SupplyRange, r.Recruitment, r.Fitness, r.Visibility, r.WaterSpeed, r.MountainSpeed, r.ForestSpeed}
	if r.VillageFood {
		values[1] = 1
	}
	if r.FortFood {
		values[2] = 1
	}
	for i, v := range values {
		put(1440+i*2, v)
	}
	return b, nil
}

type userCampaign struct {
	Version int
	Title   string
	Data    []byte
}

func SaveCampaign(w io.Writer, c game.Campaign) error {
	b, err := EncodeCampaign(c)
	if err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(userCampaign{1, c.Title, b})
}
func (l *Library) LoadCampaign(r io.Reader, id string) (game.Campaign, error) {
	d := json.NewDecoder(io.LimitReader(r, 16384))
	d.DisallowUnknownFields()
	var c userCampaign
	if err := d.Decode(&c); err != nil {
		return game.Campaign{}, err
	}
	if c.Version != 1 || len(c.Title) > 80 {
		return game.Campaign{}, fmt.Errorf("invalid campaign metadata")
	}
	return l.DecodeCampaign(id, c.Title, c.Data)
}
