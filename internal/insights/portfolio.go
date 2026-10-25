package insights

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"bereaucat/internal/store"
)

// attentionRule is shown, not hidden, so it can be challenged.
const attentionRule = "A project needs attention when it has open items at its highest priority rank and created more than it completed over the window."

// Portfolio computes question 0 over the given projects (the handler passes
// the viewer's memberships — same scope, every answer).
func (e *Engine) Portfolio(ctx context.Context, projects []store.Project, from, to time.Time) (*Portfolio, error) {
	now := e.now()
	if from.IsZero() || to.IsZero() {
		today := time.Date(now.In(e.tz).Year(), now.In(e.tz).Month(), now.In(e.tz).Day(), 0, 0, 0, 0, e.tz)
		to = today
		from = today.AddDate(0, 0, -29)
	}
	w := window{start: from, endEx: to.AddDate(0, 0, 1), cutoff: to.AddDate(0, 0, 1)}
	if now.Before(w.cutoff) {
		w.cutoff = now
	}

	out := &Portfolio{
		Scope: Scope{
			From: w.start, To: w.endEx.AddDate(0, 0, -1), Cutoff: w.cutoff,
			Timezone: e.tz.String(), GeneratedAt: now, CountingRule: countingRule,
		},
		Rule: attentionRule,
		Rows: []PortfolioRow{},
	}
	out.Delivered.ByProject = []ProjectCount{}

	for _, p := range projects {
		d, err := e.load(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("project %s: %w", p.ProjectKey, err)
		}
		row := e.portfolioRow(d, p, w)
		out.Rows = append(out.Rows, row)

		out.Delivered.Total += row.Completed
		out.Delivered.Cancelled += row.Cancelled
		out.Delivered.ByProject = append(out.Delivered.ByProject, ProjectCount{ProjectKey: d.key, Count: row.Completed})
	}
	sort.Slice(out.Delivered.ByProject, func(i, j int) bool {
		return out.Delivered.ByProject[i].Count > out.Delivered.ByProject[j].Count
	})
	// Attention rows first, then alphabetical: stable across weeks so
	// screenshots compare.
	sort.Slice(out.Rows, func(i, j int) bool {
		a, b := out.Rows[i], out.Rows[j]
		if a.Attention != b.Attention {
			return a.Attention
		}
		return a.ProjectKey < b.ProjectKey
	})
	return out, nil
}

func (e *Engine) portfolioRow(d *projectData, p store.Project, w window) PortfolioRow {
	now := e.now()
	row := PortfolioRow{
		ProjectKey:  d.key,
		ProjectName: d.name,
		HasCycles:   len(d.cycles) > 0,
		Members:     len(d.members),
		HistoryDays: int(now.Sub(p.CreatedAt.Time).Hours()/24) + 1,
	}
	topRank, topName := d.topRank()
	row.TopRankName = topName

	for _, c := range e.cycleRefs(d.cycles) {
		if c.Status == "active" {
			cc := c
			row.ActiveCycle = &cc
			break
		}
	}

	units := d.leafUnits()
	actors := map[uuid.UUID]bool{}
	var oldest *AgingRow
	for _, t := range units {
		st := d.states[t.stateID]
		switch {
		case isOpenType(st.Type):
			row.Open++
			if p, ok := d.priorities[t.priorityID]; ok && topRank > 0 && p.Rank == topRank {
				row.TopRankOpen++
			}
			if d.blocked[t.id] {
				row.Blocked++
			}
			if !t.dueDate.IsZero() && now.After(t.dueDate) {
				row.Overdue++
			}
			if st.Type == "started" {
				last := t.createdAt
				for _, ev := range t.events {
					if ev.typ == "state_changed" {
						last = ev.at
					}
				}
				days := int(now.Sub(last).Hours() / 24)
				if oldest == nil || days > oldest.Days {
					oldest = &AgingRow{Key: d.key4(t), Title: t.title,
						State: d.stateRef(t.stateID), Days: days}
				}
			}
		case st.Type == "archived":
			row.Archived++
		}

		if _, ok := d.deliveredIn(t, w.start, w.cutoff, nil, false); ok {
			row.Completed++
		}
		if !t.createdAt.Before(w.start) && t.createdAt.Before(w.cutoff) {
			row.Created++
		}
		if d.states[t.stateID].Type == "cancelled" {
			for _, ev := range t.events {
				if ev.typ == "state_changed" && !ev.at.Before(w.start) && ev.at.Before(w.cutoff) &&
					d.states[uuidFrom(ev.new, "state_id")].Type == "cancelled" {
					row.Cancelled++
					break
				}
			}
		}
	}
	for _, t := range d.tasks {
		for _, ev := range t.events {
			if !ev.at.Before(w.start) && ev.at.Before(w.cutoff) {
				actors[ev.actor] = true
			}
		}
	}
	row.ActivePeople = len(actors)
	row.Oldest = oldest
	row.Weekly = e.weekly(d, units, 4)

	windowDays := int(w.cutoff.Sub(w.start).Hours() / 24)
	row.LowHistory = row.HistoryDays < 14 || (row.Created == 0 && row.Completed == 0 && row.ActivePeople == 0)
	switch {
	case row.LowHistory && row.HistoryDays < windowDays:
		row.Trend = "no_history"
	case row.Created > row.Completed:
		row.Trend = "filling"
	case row.Completed > row.Created:
		row.Trend = "draining"
	default:
		row.Trend = "steady"
	}

	if row.TopRankOpen > 0 && row.Created > row.Completed && !row.LowHistory {
		row.Attention = true
		row.AttentionWhy = fmt.Sprintf("%d open %s and %d created vs %d completed over the window",
			row.TopRankOpen, topName, row.Created, row.Completed)
	}
	return row
}

// personSlice is one person's numbers over one project window. One computation
// feeds the person view, the people table and the workspace report, so the
// surfaces can never disagree.
type personSlice struct {
	mine        []*task
	completedOf map[uuid.UUID]completion
	topRank     int
	openNow     int
	comments    int
	created     int
}

// personSliceOf computes the slice. With a cycle, delivery requires membership
// at the completion moment — the same rule the cycle section and the evidence
// drawer apply.
func (e *Engine) personSliceOf(d *projectData, w window, cycle *ListCycle, userID uuid.UUID) personSlice {
	s := personSlice{completedOf: map[uuid.UUID]completion{}}
	units := d.leafUnits()
	topRank, _ := d.topRank()
	for _, t := range units {
		var spans []span
		if cycle != nil {
			spans = d.cycleMembership(d.top(t), cycle.ID)
		}
		c, ok := d.deliveredIn(t, w.start, w.cutoff, spans, cycle != nil)
		if !ok {
			continue
		}
		for _, u := range d.credits(t, c.at) {
			if u == userID {
				s.mine = append(s.mine, t)
				s.completedOf[t.id] = c
				if p, ok := d.priorities[t.priorityID]; ok && topRank > 0 && p.Rank >= topRank-1 {
					s.topRank++
				}
				break
			}
		}
	}
	for _, t := range units {
		if isOpenType(d.states[t.stateID].Type) {
			for _, u := range t.assigneesNow {
				if u == userID {
					s.openNow++
				}
			}
		}
	}
	for _, t := range d.tasks {
		for _, ev := range t.events {
			if ev.actor != userID || ev.at.Before(w.start) || !ev.at.Before(w.cutoff) {
				continue
			}
			switch ev.typ {
			case "comment_created":
				s.comments++
			case "task_created":
				s.created++
			}
		}
	}
	return s
}

// workedWith counts the people who shared credit on the slice's delivered
// items, most-shared first.
func workedWith(d *projectData, s personSlice, userID uuid.UUID, peers map[uuid.UUID]int, refs map[uuid.UUID]UserRef) {
	for _, t := range s.mine {
		for _, u := range d.credits(t, s.completedOf[t.id].at) {
			if u != userID {
				peers[u]++
				refs[u] = d.userRef(u)
			}
		}
	}
}

func sortedUserCounts(peers map[uuid.UUID]int, refs map[uuid.UUID]UserRef) []UserCount {
	out := make([]UserCount, 0, len(peers))
	for u, n := range peers {
		out = append(out, UserCount{User: refs[u], Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].User.Username < out[j].User.Username
	})
	return out
}

// PersonReport computes one member's slice of one project.
func (e *Engine) PersonReport(ctx context.Context, project store.Project, userID uuid.UUID, opts ReportOptions) (*PersonReport, error) {
	d, err := e.load(ctx, project)
	if err != nil {
		return nil, err
	}
	w, cycle := e.resolveWindow(opts, d.cycles)

	s := e.personSliceOf(d, w, cycle, userID)
	rep := &PersonReport{
		Scope:      e.scope(d, w, cycle),
		User:       d.userRef(userID),
		CreditRule: creditRule,
		ByCycle:    []CycleCount{},
		Delivered:  Tile{Metric: "person_delivered", Count: len(s.mine), Cx: cxOver(s.mine)},
		TopRank:    s.topRank,
		OpenNow:    s.openNow,
		Comments:   s.comments,
		Created:    s.created,
		Bands:      bandsOver(s.mine),
	}

	peers, refs := map[uuid.UUID]int{}, map[uuid.UUID]UserRef{}
	workedWith(d, s, userID, peers, refs)
	rep.WorkedWith = sortedUserCounts(peers, refs)

	// Completions per week over the window (at most 8 points to stay plottable).
	rep.Weekly = []WeekPoint{}
	weekIdx := map[string]int{}
	for ws := e.weekStart(w.start); ws.Before(w.cutoff); ws = ws.AddDate(0, 0, 7) {
		weekIdx[e.dayKey(ws)] = len(rep.Weekly)
		rep.Weekly = append(rep.Weekly, WeekPoint{WeekStart: e.dayKey(ws)})
	}
	for _, t := range s.mine {
		if i, ok := weekIdx[e.dayKey(e.weekStart(s.completedOf[t.id].at))]; ok {
			rep.Weekly[i].Completed++
		}
	}

	// Completions by cycle: a subtask counts in its parent's cycle.
	for _, c := range d.cycles {
		count := 0
		for _, t := range s.mine {
			if memberAt(d.cycleMembership(d.top(t), c.ID), s.completedOf[t.id].at) {
				count++
			}
		}
		if count > 0 {
			refs := e.cycleRefs([]ListCycle{c})
			rep.ByCycle = append(rep.ByCycle, CycleCount{Cycle: refs[0], Count: count})
		}
	}
	return rep, nil
}
