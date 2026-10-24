package game

import "fmt"

func nextFineSlot(index int) int {
	index++
	if index > 999 {
		return 0
	}
	return index
}

// 4b07 scans the circular, shared 1,000-byte movement buffer. The global draw
// cursor and the initial two-byte gap affect allocation even when it is empty.
func originalFineSlot(pool *[1000]byte, cursor int) int {
	best, length := nextFineSlot(cursor+1), 0
	start := best
	for {
		at, count := start, 0
		for pool[at] == 0 && at != cursor {
			count++
			at = nextFineSlot(at)
		}
		if count > length {
			best, length = start, count
		}
		for pool[at] != 0 && at != cursor {
			at = nextFineSlot(at)
		}
		if at == cursor {
			return best
		}
		start = at
	}
}

// 14a3 writes signed movement bytes and returns the last written slot. The
// occupied-byte check occurs after advancing the ring pointer.
func originalWriteFine(pool *[1000]byte, from, cursor Point, slot, limit int) int {
	x, y, phase := from.X+1, from.Y, 0
	for x != cursor.X || y != cursor.Y {
		if limit == 0 {
			return slot
		}
		code := 0
		phase++
		if phase < 5 {
			code = sign(cursor.X - x)
			x += code
		} else {
			phase = 0
		}
		if y < cursor.Y {
			code += 80
			y++
		} else if cursor.Y < y {
			code -= 80
			y--
		}
		if code != 0 {
			slot = nextFineSlot(slot)
			if pool[slot] != 0 {
				if slot == 0 {
					return 999
				}
				return slot - 1
			}
			pool[slot] = byte(code)
			limit--
		}
	}
	return slot
}

// 84e4 allocates an AI segment and installs its terminator. When the following
// byte is occupied, that terminator replaces the final movement byte.
func (s *State) originalInstallFine(q *Squad, cursor Point, limit int) bool {
	slot := originalFineSlot(&s.RoutePool, s.RouteCursor)
	q.FineOffset = slot
	end := originalWriteFine(&s.RoutePool, q.Position, cursor, slot, limit)
	if end == slot {
		return false
	}
	q.FineOffset = nextFineSlot(slot)
	tail := nextFineSlot(end)
	if s.RoutePool[tail] != 0 {
		tail = end
	}
	s.RoutePool[tail] = 99
	q.FineActive, q.Stopped, q.MoveClock = true, false, 1
	s.refreshFineView(q)
	return true
}

func (s *State) refreshFineView(q *Squad) {
	q.Route = nil
	p, at := q.Position, q.FineOffset
	for count := 0; count < 1000; count++ {
		code := int(int8(s.RoutePool[at]))
		if code == 0 || code == 99 {
			return
		}
		if code >= 79 {
			p.Y++
			code -= 80
		} else if code < -78 {
			p.Y--
			code += 80
		}
		p.X += code
		q.Route = append(q.Route, p)
		at = nextFineSlot(at)
	}
}

func (s *State) clearFine(q *Squad) {
	if q.FineActive {
		at := q.FineOffset
		for count := 0; count < 1000; count++ {
			s.RoutePool[at] = 0
			at = nextFineSlot(at)
			if s.RoutePool[at] == 0 {
				break
			}
		}
	}
	q.FineActive, q.Stopped, q.Route = false, false, nil
}

// 6273 consumes a terminator on its own movement update. An enemy finishing a
// segment puts itself next in the original commander's round-robin schedule.
func (s *State) takeFineStep(q *Squad) {
	code := int(int8(s.RoutePool[q.FineOffset]))
	s.RoutePool[q.FineOffset] = 0
	if code == 99 || code == 0 {
		q.FineActive, q.Route = false, nil
		if q.Side == Enemy {
			s.AICursor = q.ID - 1
		}
		return
	}
	if code >= 79 {
		q.Position.Y++
		code -= 80
	} else if code < -78 {
		q.Position.Y--
		code += 80
	}
	q.Position.X += code
	q.FineOffset = nextFineSlot(q.FineOffset)
	if len(q.Route) != 0 {
		q.Route = q.Route[1:]
	}
}

// installDrawnRoute puts a completed player/demo route in the same shared
// buffer. The public order is atomic when there is insufficient room.
func (s *State) installDrawnRoute(q *Squad, route []Point) error {
	candidate := State{RoutePool: s.RoutePool, RouteCursor: s.RouteCursor}
	previous := *q
	candidate.clearFine(&previous)
	if len(route) == 0 {
		s.clearFine(q)
		return nil
	}
	slot := originalFineSlot(&candidate.RoutePool, candidate.RouteCursor)
	at, from := nextFineSlot(slot), q.Position
	for _, point := range route {
		dx, dy := point.X-from.X, point.Y-from.Y
		if abs(dx) > 1 || abs(dy) > 1 || dx == 0 && dy == 0 || candidate.RoutePool[at] != 0 {
			return fmt.Errorf("shared route memory full")
		}
		candidate.RoutePool[at] = byte(dy*80 + dx)
		at, from = nextFineSlot(at), point
	}
	if candidate.RoutePool[at] != 0 {
		return fmt.Errorf("shared route memory full")
	}
	candidate.RoutePool[at] = 99
	s.RoutePool, s.RouteCursor = candidate.RoutePool, nextFineSlot(at)
	q.Route, q.FineOffset, q.FineActive, q.Stopped, q.MoveClock = route, nextFineSlot(slot), true, false, 1
	return nil
}
