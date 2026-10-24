package game

import "fmt"

func (s *State) controllable(id int) (*Squad, error) {
	if s.Outcome != Running {
		return nil, fmt.Errorf("the campaign has ended")
	}
	if id < 0 || id >= MaxSquads {
		return nil, fmt.Errorf("invalid squad")
	}
	q := &s.Squads[id]
	if !q.Alive() || q.Side != Player {
		return nil, fmt.Errorf("select an allied squad")
	}
	if q.BattleID != 0 {
		return nil, fmt.Errorf("squad is engaged in battle")
	}
	return q, nil
}

// TraceRoute preserves the route drawn by the player (including dangerous
// crossings). A horizontal step is omitted once every five steps, as in 14a3.
func TraceRoute(from Point, waypoints []Point) ([]Point, error) {
	var route []Point
	for _, to := range waypoints {
		if !to.Valid() {
			return nil, fmt.Errorf("destination outside the map")
		}
		i := 0
		for from != to {
			i++
			p := from
			if i%5 != 0 {
				p.X += sign(to.X - p.X)
			}
			p.Y += sign(to.Y - p.Y)
			if p == from {
				continue
			}
			route = append(route, p)
			from = p
			if len(route) > MaxRouteSteps {
				return nil, fmt.Errorf("route exceeds %d steps", MaxRouteSteps)
			}
		}
	}
	return route, nil
}

func (s *State) Move(id int, waypoints []Point) error {
	q, err := s.controllable(id)
	if err != nil {
		return err
	}
	route, err := TraceRoute(q.Position, waypoints)
	if err != nil {
		return err
	}
	used := 0
	for i := range s.Squads {
		if i != id {
			used += len(s.Squads[i].Route)
		}
	}
	if used+len(route) > MaxRouteSteps {
		return fmt.Errorf("shared route memory full (%d/%d steps)", used, MaxRouteSteps)
	}
	return s.installDrawnRoute(q, route)
}

func (s *State) routeCapacity(id int) int {
	used := 0
	for i, q := range s.Squads {
		if i != id {
			used += len(q.Route)
		}
	}
	return max(0, MaxRouteSteps-used)
}

func (s *State) SetPace(id, pace int) error {
	q, err := s.controllable(id)
	if err != nil {
		return err
	}
	if pace < 0 || pace > 3 {
		return fmt.Errorf("invalid pace")
	}
	if pace == 0 {
		q.Stopped = true
		return nil
	}
	if q.Condition < 5 && pace < 3 || q.Condition < 15 && pace < 2 {
		return fmt.Errorf("squad is too tired for that pace")
	}
	q.Pace = pace
	q.Stopped = false
	return nil
}

func (s *State) SetFormation(id, formation int) error {
	q, err := s.controllable(id)
	if err != nil {
		return err
	}
	if formation < 1 || formation > 9 {
		return fmt.Errorf("formation must be 1..9")
	}
	q.Formation = formation
	return nil
}

func (s *State) Detach(id int, men [4]int) (int, error) {
	q, err := s.controllable(id)
	if err != nil {
		return -1, err
	}
	total := 0
	for k, n := range men {
		if n < 0 || n > q.Men[k] {
			return -1, fmt.Errorf("not enough %s", Kind(k))
		}
		total += n
	}
	if total == 0 || total >= q.Count() {
		return -1, fmt.Errorf("leave at least one man in each squad")
	}
	free := -1
	for i := MaxSquads - 1; i >= 0; i-- {
		if !s.Squads[i].Alive() && s.Squads[i].BattleID == 0 {
			free = i
			break
		}
	}
	if free < 0 {
		return -1, fmt.Errorf("all 20 squad slots are occupied")
	}
	newSquad := Squad{ID: free, Side: q.Side, Position: q.Position, Men: men, Pace: 2, Condition: q.Condition, Food: q.Food, Formation: q.Formation, Visible: true, AITarget: -1}
	newSquad.Position.X++
	if newSquad.Position.X > 70 {
		newSquad.Position.X -= 2
	}
	for k, n := range men {
		q.Men[k] -= n
	}
	s.Squads[free] = newSquad
	s.ReservedSquad = free
	s.refreshOriginalCounts()
	return free, nil
}

// Transfer follows the DOS join rule: incoming men inherit the receiver's food
// and condition. Both squads are checked before any mutation.
func (s *State) Transfer(from, to int, men [4]int) error {
	a, err := s.controllable(from)
	if err != nil {
		return err
	}
	b, err := s.controllable(to)
	if err != nil {
		return err
	}
	if from == to || Distance(a.Position, b.Position) > 8 {
		return fmt.Errorf("squads must be adjacent")
	}
	total := 0
	for k, n := range men {
		if n < 0 || n > a.Men[k] {
			return fmt.Errorf("not enough %s", Kind(k))
		}
		total += n
	}
	if total == 0 || b.Count()+total > 14 {
		return fmt.Errorf("a squad can contain at most 14 men")
	}
	for k, n := range men {
		a.Men[k] -= n
		b.Men[k] += n
	}
	s.ReservedSquad = to
	s.refreshOriginalCounts()
	if !a.Alive() {
		s.originalCancelRoutes(a)
		a.AITarget = -1
	}
	return nil
}

func (s *State) detachForAI(id int) {
	q := &s.Squads[id]
	if s.stationarySquads(q.Position.Cell(), Enemy) > 2 {
		return
	}
	found := false
	free := -1
	for i := MaxSquads - 1; i >= 0; i-- {
		if !s.Squads[i].Alive() && i != s.ReservedSquad {
			if found {
				free = i
				break
			}
			found = true
		}
	}
	if free < 0 {
		return
	}
	for _, k := range []Kind{Barbarian, Knight, Archer, Spy} {
		if q.Men[k] > 0 {
			// 1731 resets the destination's active flags, but retains its old
			// clocks, targets and route offsets. It only copies these fields
			// from the parent, rather than cloning all of the parent state.
			copySquad := s.Squads[free]
			copySquad.ID = free
			copySquad.Side, copySquad.Position = s.originalSide(q), q.Position
			copySquad.Visible = copySquad.Side == Player || s.Rules.Visibility == 80 || copySquad.Sighted
			copySquad.Condition, copySquad.Food, copySquad.Formation = q.Condition, q.Food, q.Formation
			copySquad.Route = nil
			copySquad.BattleID = 0
			copySquad.Cooldown = 0
			copySquad.AIActive = false
			copySquad.AIPathActive = false
			copySquad.FineActive = false
			copySquad.Stopped = false
			copySquad.Pace = 2
			if copySquad.Position.X > 70 {
				copySquad.Position.X -= 2
			}
			copySquad.Men = [4]int{}
			copySquad.Men[k] = 1
			s.Squads[free] = copySquad
			q.Men[k]--
			s.normalizeDeadSquads()
			return
		}
	}
}

func (s *State) EditFormation(id, index int, p Point) error {
	if id < 6 || id > 9 {
		return fmt.Errorf("only formations 6..9 can be edited")
	}
	if index < 0 || index >= 14 || p.X < 0 || p.X > 79 || p.Y < 0 || p.Y > 79 {
		return fmt.Errorf("formation position outside grid")
	}
	s.Formations[id].X[index] = p.X
	s.Formations[id].Y[index] = p.Y
	return nil
}
