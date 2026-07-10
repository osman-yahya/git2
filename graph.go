package main

// Commit-graph lane layout. Each commit row gets a slice of cells drawn to
// the left of the commit message, Fork-style: colored vertical lanes with
// rounded connectors where branches fork off and merge back.

type GraphCell struct {
	Ch    rune
	Color int // palette index
}

type GraphRow struct {
	Cells []GraphCell
	Color int // color of the commit's own lane (used for the dot / hash)
	Lane  int
}

type lane struct {
	hash  string // the commit hash this lane is waiting to meet, "" = free
	color int
}

// BuildGraph assigns lanes to commits (which must be in git log --date-order,
// i.e. children before parents) and renders one row of cells per commit.
func BuildGraph(commits []Commit) []GraphRow {
	rows := make([]GraphRow, len(commits))
	var lanes []lane
	nextColor := 0

	allocLane := func(hash string) int {
		for i := range lanes {
			if lanes[i].hash == "" {
				lanes[i] = lane{hash, nextColor % len(lanePalette)}
				nextColor++
				return i
			}
		}
		lanes = append(lanes, lane{hash, nextColor % len(lanePalette)})
		nextColor++
		return len(lanes) - 1
	}

	for idx, c := range commits {
		// find every lane waiting for this commit
		var matches []int
		for i := range lanes {
			if lanes[i].hash == c.Hash {
				matches = append(matches, i)
			}
		}
		var cur int
		if len(matches) == 0 {
			cur = allocLane(c.Hash) // branch tip: open a fresh lane
		} else {
			cur = matches[0]
		}
		color := lanes[cur].color

		// base row: pass-through verticals for every active lane
		cells := make([]GraphCell, len(lanes))
		for i := range lanes {
			if lanes[i].hash != "" {
				cells[i] = GraphCell{'│', lanes[i].color}
			} else {
				cells[i] = GraphCell{' ', 0}
			}
		}

		// horizontal segment between two columns, crossing active lanes
		drawAcross := func(a, b, color int) {
			lo, hi := a, b
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo + 1; i < hi; i++ {
				if cells[i].Ch == '│' {
					cells[i] = GraphCell{'┼', cells[i].Color}
				} else {
					cells[i] = GraphCell{'─', color}
				}
			}
		}

		// sibling lanes that were also waiting for this commit: they fold in
		var siblings []int
		if len(matches) > 1 {
			siblings = matches[1:]
		}
		for _, m := range siblings {
			if m > cur {
				cells[m] = GraphCell{'╯', lanes[m].color}
			} else {
				cells[m] = GraphCell{'╰', lanes[m].color}
			}
			drawAcross(cur, m, lanes[m].color)
			lanes[m].hash = ""
		}

		// continue this lane toward the first parent (or end it)
		if len(c.Parents) == 0 {
			lanes[cur].hash = ""
		} else {
			lanes[cur].hash = c.Parents[0]
		}

		// extra parents of a merge commit branch off to their own lanes
		var extraParents []string
		if len(c.Parents) > 1 {
			extraParents = c.Parents[1:]
		}
		for _, p := range extraParents {
			target := -1
			for i := range lanes {
				if lanes[i].hash == p {
					target = i
					break
				}
			}
			if target >= 0 {
				// joins a lane that already exists
				var ch rune
				if target > cur {
					ch = '┤'
				} else {
					ch = '├'
				}
				if cells[target].Ch == ' ' {
					if target > cur {
						ch = '╮'
					} else {
						ch = '╭'
					}
				}
				cells[target] = GraphCell{ch, lanes[target].color}
				drawAcross(cur, target, lanes[target].color)
			} else {
				t := allocLane(p)
				for len(cells) < len(lanes) {
					cells = append(cells, GraphCell{' ', 0})
				}
				var ch rune
				if t > cur {
					ch = '╮'
				} else {
					ch = '╭'
				}
				cells[t] = GraphCell{ch, lanes[t].color}
				drawAcross(cur, t, lanes[t].color)
			}
		}

		// the commit dot goes on top of everything else
		dot := '●'
		if c.IsMerge() {
			dot = '○'
		}
		if len(cells) <= cur {
			grow := make([]GraphCell, cur+1-len(cells))
			for i := range grow {
				grow[i] = GraphCell{' ', 0}
			}
			cells = append(cells, grow...)
		}
		cells[cur] = GraphCell{dot, color}

		// drop trailing free lanes so rows stay as narrow as possible
		trim := len(cells)
		for trim > 0 && cells[trim-1].Ch == ' ' && (trim-1 >= len(lanes) || lanes[trim-1].hash == "") {
			trim--
		}
		rows[idx] = GraphRow{Cells: cells[:trim], Color: color, Lane: cur}

		for len(lanes) > 0 && lanes[len(lanes)-1].hash == "" {
			lanes = lanes[:len(lanes)-1]
		}
	}
	return rows
}

// GraphWidth returns the widest row (in cells) across all rows, capped.
func GraphWidth(rows []GraphRow, cap int) int {
	w := 1
	for _, r := range rows {
		if len(r.Cells) > w {
			w = len(r.Cells)
		}
	}
	if w > cap {
		w = cap
	}
	return w
}
