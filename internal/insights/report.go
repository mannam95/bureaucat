package insights

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"bereaucat/internal/store"
)

// ReportOptions selects the window of a project report: a cycle, or a date
// range (From/To are dates in the reporting timezone; To is inclusive).
type ReportOptions struct {
	CycleID *uuid.UUID
	From    time.Time
	To      time.Time
}

// window is the resolved reporting interval. endEx is exclusive; cutoff is
// min(endEx, now) so an active cycle reports "so far".
type window struct {
	start, endEx, cutoff time.Time
}

func (e *Engine) resolveWindow(opts ReportOptions, cycles []ListCycle) (window, *ListCycle) {
	now := e.now()
	if opts.CycleID != nil {
		for i := range cycles {
			if cycles[i].ID == *opts.CycleID {
				c := cycles[i]
				w := window{start: c.Start, endEx: c.EndEx, cutoff: c.EndEx}
				if now.Before(w.cutoff) {
					w.cutoff = now
				}
				return w, &c
			}
		}
	}
	from, to := opts.From, opts.To
	if from.IsZero() || to.IsZero() {
		// Default: last 30 days ending today.
		today := time.Date(now.In(e.tz).Year(), now.In(e.tz).Month(), now.In(e.tz).Day(), 0, 0, 0, 0, e.tz)
		to = today
		from = today.AddDate(0, 0, -29)
	}
	w := window{start: from, endEx: to.AddDate(0, 0, 1), cutoff: to.AddDate(0, 0, 1)}
	if now.Before(w.cutoff) {
		w.cutoff = now
	}
	return w, nil
}

func (e *Engine) scope(d *projectData, w window, cycle *ListCycle) Scope {
	s := Scope{
		ProjectKey:   d.key,
		ProjectName:  d.name,
		From:         w.start,
		To:           w.endEx.AddDate(0, 0, -1),
		Cutoff:       w.cutoff,
		Timezone:     e.tz.String(),
		GeneratedAt:  e.now(),
		CountingRule: countingRule,
	}
	if cycle != nil {
		id := cycle.ID
		s.CycleID = &id
		s.CycleTitle = cycle.Title
	}
	return s
}

// cycleRefs decorates the project's cycles with their derived status.
func (e *Engine) cycleRefs(cycles []ListCycle) []CycleRef {
	now := e.now()
	out := make([]CycleRef, 0, len(cycles))
	for _, c := range cycles {
		status := "active"
		if now.Before(c.Start) {
			status = "upcoming"
		} else if !now.Before(c.EndEx) {
			status = "completed"
		}
		out = append(out, CycleRef{
			ID: c.ID, Title: c.Title, Status: status,
			StartDate: e.dayKey(c.Start),
			EndDate:   e.dayKey(c.EndEx.AddDate(0, 0, -1)),
		})
	}
	return out
}

// cxOver sums complexity with coverage over a unit set.
func cxOver(units []*task) *ComplexitySum {
	out := &ComplexitySum{Of: len(units)}
	for _, t := range units {
		if cx, ok := t.complexity(); ok {
			out.Sum += cx
			out.Assessed++
		} else {
			out.NotAssessed++
		}
	}
	return out
}

func bandsOver(units []*task) []BandCount {
	bands := []struct {
		name     string
		min, max int
	}{{"1-4", 1, 4}, {"5-9", 5, 9}, {"10-16", 10, 16}, {"17+", 17, 1 << 30}}
	out := make([]BandCount, 0, 5)
	counts := make([]int, len(bands))
	notAssessed := 0
	for _, t := range units {
		cx, ok := t.complexity()
		if !ok {
			notAssessed++
			continue
		}
		for i, b := range bands {
			if cx >= b.min && cx <= b.max {
				counts[i]++
				break
			}
		}
	}
	for i, b := range bands {
		out = append(out, BandCount{Band: b.name, Count: counts[i]})
	}
	out = append(out, BandCount{Band: "not_assessed", Count: notAssessed})
	return out
}

// ProjectReport computes the full project answer.
func (e *Engine) ProjectReport(ctx context.Context, project store.Project, opts ReportOptions) (*ProjectReport, error) {
	d, err := e.load(ctx, project)
	if err != nil {
		return nil, err
	}
	w, cycle := e.resolveWindow(opts, d.cycles)

	rep := &ProjectReport{
		Scope:     e.scope(d, w, cycle),
		Cycles:    e.cycleRefs(d.cycles),
		HasAreas:  d.hasAreas,
		HasLabels: d.hasLabels,
	}

	units := d.leafUnits()
	var delivered []*task
	deliveredAt := map[uuid.UUID]completion{}

	if cycle != nil {
		sec := e.cycleSection(d, w, cycle, units, deliveredAt)
		rep.Cycle = sec
		for _, t := range units {
			if _, ok := deliveredAt[t.id]; ok {
				delivered = append(delivered, t)
			}
		}
	} else {
		sec := e.rangeSection(d, w, units, deliveredAt)
		rep.Range = sec
		for _, t := range units {
			if _, ok := deliveredAt[t.id]; ok {
				delivered = append(delivered, t)
			}
		}
	}

	rep.People = e.personTable(d, w, delivered, deliveredAt)
	rep.Health = e.health(d)
	return rep, nil
}

func (e *Engine) cycleSection(d *projectData, w window, cycle *ListCycle, units []*task, deliveredAt map[uuid.UUID]completion) *CycleSection {
	spansOf := map[uuid.UUID][]span{}
	for _, t := range units {
		spansOf[t.id] = d.cycleMembership(d.top(t), cycle.ID)
	}

	var planned, added, removed, atClose, deliveredList, originalPlan []*task
	var cancelled, archived, carried []*task
	for _, t := range units {
		spans := spansOf[t.id]
		inAtStart := memberAt(spans, w.start)
		inAtClose := memberAt(spans, w.cutoff.Add(-time.Nanosecond))
		entered := firstEntryAfter(spans, w.start)
		addedInWin := !inAtStart && !entered.IsZero() && entered.Before(w.cutoff)

		if inAtStart {
			planned = append(planned, t)
		}
		if addedInWin {
			added = append(added, t)
		}
		if (inAtStart || addedInWin) && !inAtClose {
			removed = append(removed, t)
		}
		if inAtClose {
			atClose = append(atClose, t)
		}

		if c, ok := d.deliveredIn(t, w.start, w.cutoff, spans, true); ok {
			deliveredAt[t.id] = c
			deliveredList = append(deliveredList, t)
			if inAtStart {
				originalPlan = append(originalPlan, t)
			}
		}
		if inAtClose {
			switch d.typeAt(t, w.cutoff) {
			case "cancelled":
				cancelled = append(cancelled, t)
			case "archived":
				archived = append(archived, t)
			case "completed":
				// counted via deliveredList
			default:
				carried = append(carried, t)
			}
		}
	}

	sec := &CycleSection{
		Delivered:      Tile{Metric: "delivered", Count: len(deliveredList), Cx: cxOver(deliveredList)},
		OriginalPlan:   Tile{Metric: "original_plan", Count: len(originalPlan)},
		PlannedAtStart: Tile{Metric: "planned", Count: len(planned), Cx: cxOver(planned)},
		Added:          Tile{Metric: "added", Count: len(added)},
		Removed:        Tile{Metric: "descoped", Count: len(removed)},
		CarriedOver:    Tile{Metric: "carried", Count: len(carried), Cx: cxOver(carried)},
		AtClose:        len(atClose),
		Bands:          bandsOver(deliveredList),
	}
	if d.hasType("cancelled") {
		t := Tile{Metric: "cancelled", Count: len(cancelled)}
		sec.Cancelled = &t
	}
	if d.hasType("archived") {
		t := Tile{Metric: "archived", Count: len(archived)}
		sec.Archived = &t
	}
	completedAtClose := 0
	for _, t := range atClose {
		if d.typeAt(t, w.cutoff) == "completed" {
			completedAtClose++
		}
	}
	sec.Identity = fmt.Sprintf("%d planned + %d added - %d removed = %d at close = %d completed + %d open",
		len(planned), len(added), len(removed), len(atClose), completedAtClose, len(carried))
	if sec.Cancelled != nil {
		sec.Identity += fmt.Sprintf(" + %d cancelled", sec.Cancelled.Count)
	}
	if sec.Archived != nil {
		sec.Identity += fmt.Sprintf(" + %d archived", sec.Archived.Count)
	}

	// Daily series across the cycle, capped at the cutoff day.
	for day := w.start; day.Before(w.endEx); day = day.AddDate(0, 0, 1) {
		dayEnd := day.AddDate(0, 0, 1)
		if day.After(w.cutoff) {
			break
		}
		pt := DayPoint{Date: e.dayKey(day), Weekend: day.Weekday() == time.Saturday || day.Weekday() == time.Sunday}
		for _, t := range units {
			spans := spansOf[t.id]
			if memberAt(spans, dayEnd.Add(-time.Nanosecond)) {
				pt.Scope++
			}
			if c, ok := deliveredAt[t.id]; ok {
				if c.at.Before(dayEnd) {
					pt.CompletedCum++
				}
				if !c.at.Before(day) && c.at.Before(dayEnd) {
					pt.Completed++
				}
			}
			entered := firstEntryAfter(spans, w.start)
			if !entered.IsZero() && !entered.Before(day) && entered.Before(dayEnd) {
				pt.Added++
			}
			for _, s := range spans {
				if !s.to.IsZero() && !s.to.Before(day) && s.to.Before(dayEnd) {
					pt.Removed++
				}
			}
		}
		sec.Days = append(sec.Days, pt)
	}
	return sec
}

func (e *Engine) rangeSection(d *projectData, w window, units []*task, deliveredAt map[uuid.UUID]completion) *RangeSection {
	var deliveredList, created, cancelled []*task
	for _, t := range units {
		if c, ok := d.deliveredIn(t, w.start, w.cutoff, nil, false); ok {
			deliveredAt[t.id] = c
			deliveredList = append(deliveredList, t)
		}
		if !t.createdAt.Before(w.start) && t.createdAt.Before(w.cutoff) {
			created = append(created, t)
		}
		if d.typeAt(t, w.cutoff) == "cancelled" {
			for _, ev := range t.events {
				if ev.typ == "state_changed" && !ev.at.Before(w.start) && ev.at.Before(w.cutoff) {
					if d.states[uuidFrom(ev.new, "state_id")].Type == "cancelled" {
						cancelled = append(cancelled, t)
						break
					}
				}
			}
		}
	}

	sec := &RangeSection{
		Completed: Tile{Metric: "completed", Count: len(deliveredList), Cx: cxOver(deliveredList)},
		Created:   Tile{Metric: "created", Count: len(created)},
		Bands:     bandsOver(deliveredList),
		HasCycles: len(d.cycles) > 0,
	}
	if d.hasType("cancelled") {
		t := Tile{Metric: "cancelled", Count: len(cancelled)}
		sec.Cancelled = &t
	}

	for day := w.start; day.Before(w.endEx) && !day.After(w.cutoff); day = day.AddDate(0, 0, 1) {
		dayEnd := day.AddDate(0, 0, 1)
		pt := DayPoint{Date: e.dayKey(day), Weekend: day.Weekday() == time.Saturday || day.Weekday() == time.Sunday}
		for _, t := range units {
			if c, ok := deliveredAt[t.id]; ok {
				if c.at.Before(dayEnd) {
					pt.CompletedCum++
				}
				if !c.at.Before(day) && c.at.Before(dayEnd) {
					pt.Completed++
				}
			}
		}
		sec.Days = append(sec.Days, pt)
	}
	return sec
}

func (e *Engine) personTable(d *projectData, w window, delivered []*task, deliveredAt map[uuid.UUID]completion) PersonTable {
	type acc struct {
		row  PersonRow
		seen bool
	}
	rows := map[uuid.UUID]*acc{}
	get := func(u uuid.UUID) *acc {
		a, ok := rows[u]
		if !ok {
			a = &acc{row: PersonRow{User: d.userRef(u), Cx: &ComplexitySum{}}}
			rows[u] = a
		}
		return a
	}
	for _, m := range d.members {
		get(m.ID)
	}

	topRank, _ := d.topRank()
	table := PersonTable{CreditRule: creditRule, TotalCx: cxOver(delivered), TotalDelivered: len(delivered)}

	for _, t := range delivered {
		c := deliveredAt[t.id]
		credits := d.credits(t, c.at)
		for _, u := range credits {
			a := get(u)
			a.row.Delivered++
			if len(credits) > 1 {
				a.row.Shared++
			}
			a.row.Cx.Of++
			if cx, ok := t.complexity(); ok {
				a.row.Cx.Sum += cx
				a.row.Cx.Assessed++
			} else {
				a.row.Cx.NotAssessed++
			}
			if p, ok := d.priorities[t.priorityID]; ok && topRank > 0 && p.Rank >= topRank-1 {
				a.row.TopRank++
			}
		}
	}
	// Open at end, comments and created over the window.
	for _, t := range d.leafUnits() {
		if isOpenType(d.typeAt(t, w.cutoff)) {
			for _, u := range d.assigneesAt(t, w.cutoff) {
				get(u).row.OpenAtEnd++
			}
		}
	}
	for _, t := range d.tasks {
		for _, ev := range t.events {
			if ev.at.Before(w.start) || !ev.at.Before(w.cutoff) {
				continue
			}
			switch ev.typ {
			case "comment_created":
				get(ev.actor).row.Comments++
			case "task_created":
				get(ev.actor).row.Created++
			}
			if _, ok := rows[ev.actor]; ok {
				rows[ev.actor].row.HasActivity = true
			}
		}
	}
	for u, a := range rows {
		_ = u
		if a.row.Delivered > 0 || a.row.Comments > 0 || a.row.Created > 0 {
			a.row.HasActivity = true
		}
		if a.row.Cx.Of == 0 {
			a.row.Cx = nil
		}
	}

	for _, a := range rows {
		table.Rows = append(table.Rows, a.row)
	}
	sort.Slice(table.Rows, func(i, j int) bool {
		a, b := table.Rows[i], table.Rows[j]
		if a.Delivered != b.Delivered {
			return a.Delivered > b.Delivered
		}
		return a.User.Username < b.User.Username
	})
	return table
}

func (e *Engine) health(d *projectData) Health {
	now := e.now()
	h := Health{}
	byState := map[uuid.UUID]int{}
	byPrio := map[uuid.UUID]int{}
	units := d.leafUnits()
	for _, t := range units {
		st := d.states[t.stateID]
		if !isOpenType(st.Type) {
			continue
		}
		h.Open++
		byState[t.stateID]++
		byPrio[t.priorityID]++
		if st.Type == "started" {
			h.InStarted++
		}
		if d.blocked[t.id] {
			h.Blocked++
		}
		if !t.dueDate.IsZero() {
			late := int(now.Sub(t.dueDate).Hours() / 24)
			if late > 0 {
				h.Overdue++
				h.Due = append(h.Due, e.dueRow(d, t, late))
			} else if late >= -7 {
				h.Due = append(h.Due, e.dueRow(d, t, late))
			}
		}
		if st.Type == "started" {
			last := t.createdAt
			for _, ev := range t.events {
				if ev.typ == "state_changed" {
					last = ev.at
				}
			}
			h.Aging = append(h.Aging, AgingRow{
				Key: d.key4(t), Title: t.title, State: d.stateRef(t.stateID),
				Days:     int(now.Sub(last).Hours() / 24),
				Assignee: firstUser(d, t.assigneesNow),
			})
		}
	}
	for _, s := range d.stateList {
		if isOpenType(s.Type) && byState[s.ID] > 0 {
			h.OpenByState = append(h.OpenByState, StateCount{State: StateRef{ID: s.ID, Name: s.Name, Type: s.Type, Color: s.Color}, Count: byState[s.ID]})
		}
	}
	for _, p := range d.prioList {
		if byPrio[p.ID] > 0 {
			h.PriorityMix = append(h.PriorityMix, RankCount{
				Priority: PriorityRef{ID: p.ID, Name: p.Name, Rank: p.Rank, Color: p.Color, Active: p.Active},
				Count:    byPrio[p.ID],
			})
		}
	}
	sort.Slice(h.Aging, func(i, j int) bool { return h.Aging[i].Days > h.Aging[j].Days })
	if len(h.Aging) > 8 {
		h.Aging = h.Aging[:8]
	}
	sort.Slice(h.Due, func(i, j int) bool { return h.Due[i].LateDays > h.Due[j].LateDays })
	if len(h.Due) > 8 {
		h.Due = h.Due[:8]
	}
	h.Weekly = e.weekly(d, units, 4)
	return h
}

func (e *Engine) dueRow(d *projectData, t *task, late int) DueRow {
	return DueRow{Key: d.key4(t), Title: t.title, Due: t.dueDate, LateDays: late,
		Assignee: firstUser(d, t.assigneesNow)}
}

func firstUser(d *projectData, ids []uuid.UUID) *UserRef {
	if len(ids) == 0 {
		return nil
	}
	u := d.userRef(ids[0])
	return &u
}

// weekly builds created-vs-completed for the last n ISO weeks, ending with the
// current week.
func (e *Engine) weekly(d *projectData, units []*task, n int) []WeekPoint {
	now := e.now()
	thisWeek := e.weekStart(now)
	points := make([]WeekPoint, n)
	index := map[string]int{}
	for i := 0; i < n; i++ {
		ws := thisWeek.AddDate(0, 0, -7*(n-1-i))
		points[i] = WeekPoint{WeekStart: e.dayKey(ws)}
		index[e.dayKey(ws)] = i
	}
	for _, t := range units {
		if i, ok := index[e.dayKey(e.weekStart(t.createdAt))]; ok {
			points[i].Created++
		}
		if d.states[t.stateID].Type == "completed" {
			cs := d.completions(t)
			if len(cs) > 0 {
				if i, ok := index[e.dayKey(e.weekStart(cs[len(cs)-1].at))]; ok {
					points[i].Completed++
				}
			}
		}
	}
	return points
}
