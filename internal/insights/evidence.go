package insights

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"bereaucat/internal/store"
)

// EvidenceParams scopes an evidence request. Metric names come from the fixed
// vocabulary below; unknown metrics are rejected, never guessed.
type EvidenceParams struct {
	Metric  string
	CycleID *uuid.UUID
	From    time.Time
	To      time.Time
	UserID  *uuid.UUID // person_* metrics
	StateID *uuid.UUID // open_state
	Rank    *int       // priority_rank
	Day     string     // day metric, YYYY-MM-DD
}

// Evidence resolves one metric into its exact population. Every row set here
// is the same population the report counted — one computation, two views.
func (e *Engine) Evidence(ctx context.Context, project store.Project, p EvidenceParams) (*Evidence, error) {
	d, err := e.load(ctx, project)
	if err != nil {
		return nil, err
	}
	w, cycle := e.resolveWindow(ReportOptions{CycleID: p.CycleID, From: p.From, To: p.To}, d.cycles)
	ev := &Evidence{
		Scope:  e.scope(d, w, cycle),
		Metric: p.Metric,
		Rows:   []EvidenceRow{},
	}

	units := d.leafUnits()
	spansOf := map[uuid.UUID][]span{}
	if cycle != nil {
		for _, t := range units {
			spansOf[t.id] = d.cycleMembership(d.top(t), cycle.ID)
		}
	}
	requireCycle := cycle != nil

	addDelivered := func(t *task, c completion, entered string) {
		row := e.row(d, t)
		at := c.at
		row.CompletedAt = &at
		actor := d.userRef(c.actor)
		row.MarkedCompleteBy = &actor
		for _, u := range d.credits(t, c.at) {
			row.Credits = append(row.Credits, d.userRef(u))
		}
		row.EnteredCycle = entered
		ev.Rows = append(ev.Rows, row)
	}
	enteredLabel := func(t *task) string {
		if cycle == nil {
			return ""
		}
		spans := spansOf[t.id]
		if memberAt(spans, w.start) {
			return "planned"
		}
		if entry := firstEntryAfter(spans, w.start); !entry.IsZero() {
			return "added " + e.dayKey(entry)
		}
		return ""
	}

	switch p.Metric {
	case "delivered", "completed":
		ev.Title = "Delivered"
		ev.HowComputed = "Leaf items that entered a completed-type state during the window and were still completed at the cutoff. Reopened items fall out; a repeated completion counts once."
		ev.SourceEvents = "state_changed"
		ev.Chips = e.windowChips(d, w, cycle, "leaf items", "still completed at cutoff")
		for _, t := range units {
			if c, ok := d.deliveredIn(t, w.start, w.cutoff, spansOf[t.id], requireCycle); ok {
				addDelivered(t, c, enteredLabel(t))
			}
		}

	case "original_plan":
		if cycle == nil {
			return nil, fmt.Errorf("metric %q needs a cycle", p.Metric)
		}
		ev.Title = "Delivered from the original plan"
		ev.HowComputed = "Delivered items that were already in the cycle at its start. Added work never inflates this population's denominator."
		ev.SourceEvents = "state_changed, cycle_added, cycle_removed"
		ev.Chips = e.windowChips(d, w, cycle, "leaf items", "in cycle at start")
		for _, t := range units {
			if !memberAt(spansOf[t.id], w.start) {
				continue
			}
			if c, ok := d.deliveredIn(t, w.start, w.cutoff, spansOf[t.id], true); ok {
				addDelivered(t, c, "planned")
			}
		}

	case "planned":
		if cycle == nil {
			return nil, fmt.Errorf("metric %q needs a cycle", p.Metric)
		}
		ev.Title = "Planned at start"
		ev.HowComputed = "Leaf items in the cycle at its first moment, reconstructed from cycle_added/cycle_removed events."
		ev.SourceEvents = "cycle_added, cycle_removed"
		ev.Chips = e.windowChips(d, w, cycle, "leaf items", "in cycle at start")
		for _, t := range units {
			if memberAt(spansOf[t.id], w.start) {
				ev.Rows = append(ev.Rows, e.rowWithState(d, t, w.cutoff))
			}
		}

	case "added":
		if cycle == nil {
			return nil, fmt.Errorf("metric %q needs a cycle", p.Metric)
		}
		ev.Title = "Added mid-cycle"
		ev.HowComputed = "Leaf items whose first cycle entry is dated after the cycle start and that were not in the cycle at start. An item entering twice counts once."
		ev.SourceEvents = "cycle_added"
		ev.Chips = e.windowChips(d, w, cycle, "leaf items", "entered after start")
		for _, t := range units {
			spans := spansOf[t.id]
			if memberAt(spans, w.start) {
				continue
			}
			if entry := firstEntryAfter(spans, w.start); !entry.IsZero() && entry.Before(w.cutoff) {
				row := e.rowWithState(d, t, w.cutoff)
				row.EnteredCycle = "added " + e.dayKey(entry)
				ev.Rows = append(ev.Rows, row)
			}
		}

	case "descoped":
		if cycle == nil {
			return nil, fmt.Errorf("metric %q needs a cycle", p.Metric)
		}
		ev.Title = "Removed from scope"
		ev.HowComputed = "Leaf items that were in the cycle (at start or added later) and were no longer in it at the cutoff."
		ev.SourceEvents = "cycle_removed"
		ev.Chips = e.windowChips(d, w, cycle, "leaf items", "not in cycle at close")
		for _, t := range units {
			spans := spansOf[t.id]
			entered := firstEntryAfter(spans, w.start)
			wasIn := memberAt(spans, w.start) || (!entered.IsZero() && entered.Before(w.cutoff))
			if wasIn && !memberAt(spans, w.cutoff.Add(-time.Nanosecond)) {
				ev.Rows = append(ev.Rows, e.rowWithState(d, t, w.cutoff))
			}
		}

	case "cancelled":
		if cycle == nil {
			// Range mode: entered a cancelled-type state during the window and
			// still cancelled at the cutoff — the mirror of "completed".
			ev.Title = "Cancelled"
			ev.HowComputed = "Leaf items that entered a cancelled-type state during the window and were still cancelled at the cutoff. Cancelled is never delivery."
			ev.SourceEvents = "state_changed"
			ev.Chips = e.windowChips(d, w, nil, "leaf items", "still cancelled at cutoff")
			for _, t := range units {
				if d.typeAt(t, w.cutoff) != "cancelled" {
					continue
				}
				for _, evn := range t.events {
					if evn.typ == "state_changed" && !evn.at.Before(w.start) && evn.at.Before(w.cutoff) &&
						d.states[uuidFrom(evn.new, "state_id")].Type == "cancelled" {
						ev.Rows = append(ev.Rows, e.rowWithState(d, t, w.cutoff))
						break
					}
				}
			}
			break
		}
		fallthrough

	case "carried", "archived":
		if cycle == nil {
			return nil, fmt.Errorf("metric %q needs a cycle", p.Metric)
		}
		wantType := map[string]string{"carried": "", "cancelled": "cancelled", "archived": "archived"}[p.Metric]
		ev.Title = map[string]string{"carried": "Carried over", "cancelled": "Cancelled", "archived": "Archived"}[p.Metric]
		ev.HowComputed = "Leaf items in the cycle at the cutoff whose state type at that moment matches this outcome. Outcomes are exclusive: completed, open, cancelled and archived partition the scope at close."
		ev.SourceEvents = "state_changed, cycle_added, cycle_removed"
		ev.Chips = e.windowChips(d, w, cycle, "leaf items", "in cycle at close")
		for _, t := range units {
			if !memberAt(spansOf[t.id], w.cutoff.Add(-time.Nanosecond)) {
				continue
			}
			typ := d.typeAt(t, w.cutoff)
			match := false
			if p.Metric == "carried" {
				match = isOpenType(typ)
			} else {
				match = typ == wantType
			}
			if match {
				row := e.rowWithState(d, t, w.cutoff)
				row.EnteredCycle = enteredLabel(t)
				ev.Rows = append(ev.Rows, row)
			}
		}

	case "created":
		ev.Title = "Created"
		ev.HowComputed = "Leaf items created during the window."
		ev.SourceEvents = "task_created"
		ev.Chips = e.windowChips(d, w, cycle, "leaf items", "")
		for _, t := range units {
			if !t.createdAt.Before(w.start) && t.createdAt.Before(w.cutoff) {
				ev.Rows = append(ev.Rows, e.rowWithState(d, t, w.cutoff))
			}
		}

	case "person_delivered":
		if p.UserID == nil {
			return nil, fmt.Errorf("metric %q needs a user", p.Metric)
		}
		u := d.userRef(*p.UserID)
		ev.Title = "Delivered by " + u.Username
		ev.HowComputed = "Delivered items credited to this person: their Contributors, or — when none are recorded — their assignees at completion time."
		ev.SourceEvents = "state_changed, assignee_added, assignee_removed + task contributors"
		ev.Chips = e.windowChips(d, w, cycle, "leaf items", "credited to "+u.Username)
		for _, t := range units {
			c, ok := d.deliveredIn(t, w.start, w.cutoff, spansOf[t.id], requireCycle)
			if !ok {
				continue
			}
			for _, cu := range d.credits(t, c.at) {
				if cu == *p.UserID {
					addDelivered(t, c, enteredLabel(t))
					break
				}
			}
		}

	case "person_open":
		if p.UserID == nil {
			return nil, fmt.Errorf("metric %q needs a user", p.Metric)
		}
		u := d.userRef(*p.UserID)
		now := e.now()
		ev.Title = "Open now, assigned to " + u.Username
		ev.HowComputed = "Open leaf items currently assigned to this person — a snapshot at generation time, independent of the selected window."
		ev.SourceEvents = "current assignees (snapshot)"
		ev.Chips = []string{"assignee = " + u.Username, "snapshot " + e.dayKey(now)}
		for _, t := range units {
			if !isOpenType(d.states[t.stateID].Type) {
				continue
			}
			for _, au := range t.assigneesNow {
				if au != *p.UserID {
					continue
				}
				row := e.row(d, t)
				last := t.createdAt
				for _, evn := range t.events {
					if evn.typ == "state_changed" {
						last = evn.at
					}
				}
				row.AgeDays = int(now.Sub(last).Hours() / 24)
				if !t.dueDate.IsZero() {
					due := t.dueDate
					row.Due = &due
				}
				ev.Rows = append(ev.Rows, row)
				break
			}
		}

	case "open_state", "priority_rank", "aging", "overdue", "due_soon", "blocked":
		e.snapshotMetric(d, ev, p, units)

	case "day":
		day, err := time.ParseInLocation("2006-01-02", p.Day, e.tz)
		if err != nil {
			return nil, fmt.Errorf("metric day needs day=YYYY-MM-DD")
		}
		dayEnd := day.AddDate(0, 0, 1)
		ev.Title = "Events on " + p.Day
		ev.HowComputed = "Completions, cycle entries and removals dated this day on items in scope. Other activity is not listed because it does not move a reported number."
		ev.SourceEvents = "state_changed, cycle_added, cycle_removed"
		ev.Chips = []string{"day = " + p.Day}
		for _, t := range units {
			if c, ok := d.deliveredIn(t, w.start, w.cutoff, spansOf[t.id], requireCycle); ok &&
				!c.at.Before(day) && c.at.Before(dayEnd) {
				addDelivered(t, c, enteredLabel(t))
			}
		}

	default:
		return nil, fmt.Errorf("unknown metric %q", p.Metric)
	}

	ev.Count = len(ev.Rows)
	cx := &ComplexitySum{Of: ev.Count}
	for _, r := range ev.Rows {
		if r.Assessed {
			cx.Sum += r.Complexity
			cx.Assessed++
		} else {
			cx.NotAssessed++
		}
	}
	ev.Cx = cx
	sort.Slice(ev.Rows, func(i, j int) bool {
		a, b := ev.Rows[i], ev.Rows[j]
		if (a.CompletedAt != nil) && (b.CompletedAt != nil) && !a.CompletedAt.Equal(*b.CompletedAt) {
			return a.CompletedAt.After(*b.CompletedAt)
		}
		return a.Key > b.Key
	})
	return ev, nil
}

// snapshotMetric covers the now-based health populations; these are
// representable as task-list filters, so TaskListQuery is set.
func (e *Engine) snapshotMetric(d *projectData, ev *Evidence, p EvidenceParams, units []*task) {
	now := e.now()
	switch p.Metric {
	case "open_state":
		ev.Title = "Open work in a state"
		ev.HowComputed = "Open leaf items currently in the selected state."
		ev.SourceEvents = "current state (snapshot)"
		if p.StateID != nil {
			ev.Chips = []string{"state = " + d.states[*p.StateID].Name, "snapshot " + e.dayKey(now)}
			ev.TaskListQuery = "state_id=" + p.StateID.String()
		}
	case "priority_rank":
		ev.Title = "Open work at a priority rank"
		ev.HowComputed = "Open leaf items whose priority sits at the selected rank. Rank 0 is the explicit unranked bucket."
		ev.SourceEvents = "current priority (snapshot)"
		if p.Rank != nil {
			ev.Chips = []string{fmt.Sprintf("rank = %d", *p.Rank), "snapshot " + e.dayKey(now)}
		}
	case "aging":
		ev.Title = "Longest unchanged in a started state"
		ev.HowComputed = "Open leaf items in a started-type state, aged by their last state change. The state name travels with each row: parked states read as parked."
		ev.SourceEvents = "state_changed"
	case "overdue":
		ev.Title = "Overdue"
		ev.HowComputed = "Open leaf items whose due date has passed."
		ev.SourceEvents = "due date (snapshot)"
	case "due_soon":
		ev.Title = "Due within 7 days"
		ev.HowComputed = "Open leaf items due in the next 7 days."
		ev.SourceEvents = "due date (snapshot)"
	case "blocked":
		ev.Title = "Blocked"
		ev.HowComputed = "Open leaf items with at least one blocker task that is itself still open. Blocked is a relationship, not a state name."
		ev.SourceEvents = "task blocker links"
	}
	for _, t := range units {
		st := d.states[t.stateID]
		if !isOpenType(st.Type) {
			continue
		}
		match := false
		row := e.rowWithState(d, t, now)
		switch p.Metric {
		case "open_state":
			match = p.StateID != nil && t.stateID == *p.StateID
		case "priority_rank":
			match = p.Rank != nil && d.priorities[t.priorityID].Rank == *p.Rank
		case "aging":
			if st.Type == "started" {
				match = true
				last := t.createdAt
				for _, evn := range t.events {
					if evn.typ == "state_changed" {
						last = evn.at
					}
				}
				row.AgeDays = int(now.Sub(last).Hours() / 24)
			}
		case "overdue":
			match = !t.dueDate.IsZero() && now.After(t.dueDate)
		case "due_soon":
			match = !t.dueDate.IsZero() && !now.After(t.dueDate) && t.dueDate.Sub(now).Hours() <= 7*24
		case "blocked":
			match = d.blocked[t.id]
		}
		if match {
			if !t.dueDate.IsZero() {
				due := t.dueDate
				row.Due = &due
			}
			ev.Rows = append(ev.Rows, row)
		}
	}
	if p.Metric == "aging" {
		sort.Slice(ev.Rows, func(i, j int) bool { return ev.Rows[i].AgeDays > ev.Rows[j].AgeDays })
	}
}

func (e *Engine) row(d *projectData, t *task) EvidenceRow {
	cx, ok := t.complexity()
	row := EvidenceRow{
		Key: d.key4(t), Title: t.title,
		State:      d.stateRef(t.stateID),
		Priority:   d.prioRef(t.priorityID),
		Complexity: cx, Assessed: ok,
	}
	for _, u := range t.assigneesNow {
		row.Assignees = append(row.Assignees, d.userRef(u))
	}
	if t.parent != (uuid.UUID{}) {
		if p, ok := d.tasks[t.parent]; ok {
			row.ParentKey = d.key4(p)
			row.ParentTitle = p.title
		}
	}
	return row
}

// rowWithState renders the row with the state the task had at the given
// moment, which is what scope evidence must show.
func (e *Engine) rowWithState(d *projectData, t *task, at time.Time) EvidenceRow {
	row := e.row(d, t)
	row.State = d.stateRef(d.stateAt(t, at))
	return row
}

func (e *Engine) windowChips(d *projectData, w window, cycle *ListCycle, extra ...string) []string {
	chips := []string{}
	if cycle != nil {
		chips = append(chips, "cycle = "+cycle.Title)
	} else {
		chips = append(chips, fmt.Sprintf("%s to %s", e.dayKey(w.start), e.dayKey(w.endEx.AddDate(0, 0, -1))))
	}
	for _, x := range extra {
		if x != "" {
			chips = append(chips, x)
		}
	}
	return chips
}
