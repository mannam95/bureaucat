package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"bereaucat/internal/store"
)

// Engine computes insights from the store. It is stateless; every call loads
// what it needs. tz is the instance reporting timezone (REPORT_TIMEZONE): two
// people in one meeting must see identical numbers, so the viewer's browser
// timezone is never used.
type Engine struct {
	store store.Querier
	tz    *time.Location
	// now is injectable for tests; defaults to time.Now.
	now func() time.Time
}

func New(s store.Querier, tz *time.Location) *Engine {
	if tz == nil {
		tz = time.UTC
	}
	return &Engine{store: s, tz: tz, now: time.Now}
}

// activityTypes is the event vocabulary the reconstruction needs.
var activityTypes = []string{
	"task_created", "state_changed",
	"cycle_added", "cycle_removed",
	"assignee_added", "assignee_removed",
	"comment_created",
}

// countingRule is the one-line scope footer, identical everywhere.
const countingRule = "Counts leaf items: a parent with subtasks counts through its subtasks. Cancelled is never delivery; archived is its own bucket; unknown estimates are never zero."

// creditRule explains the person table.
const creditRule = "A completed item credits its Contributors; with none recorded, it credits its assignees at completion. Shared items count once per credited person; the total counts each item once."

type stateInfo struct {
	ID       uuid.UUID
	Name     string
	Type     string
	Color    string
	Position int
}

type prioInfo struct {
	ID     uuid.UUID
	Name   string
	Rank   int
	Color  string
	Active bool
}

type event struct {
	typ   string
	at    time.Time
	actor uuid.UUID
	old   map[string]any
	new   map[string]any
}

type task struct {
	id           uuid.UUID
	number       int32
	title        string
	parent       uuid.UUID // zero when top-level
	children     []uuid.UUID
	stateID      uuid.UUID // current
	priorityID   uuid.UUID
	difficulty   int32
	effort       int32
	createdAt    time.Time
	dueDate      time.Time // zero when unset
	assigneesNow []uuid.UUID
	contributors []uuid.UUID
	events       []event
	cycleNow     uuid.UUID // zero when none
	cycleAddedAt time.Time
}

func (t *task) complexity() (int, bool) {
	if t.difficulty > 0 && t.effort > 0 {
		return int(t.difficulty * t.effort), true
	}
	return 0, false
}

type member struct {
	UserRef
	role string
}

type projectData struct {
	id         uuid.UUID
	key        string
	name       string
	iconURL    *string
	createdAt  time.Time
	states     map[uuid.UUID]stateInfo
	stateList  []stateInfo // display order: type -> position -> name
	priorities map[uuid.UUID]prioInfo
	prioList   []prioInfo // rank order
	tasks      map[uuid.UUID]*task
	order      []uuid.UUID // stable task order (by number desc)
	cycles     []ListCycle
	users      map[uuid.UUID]UserRef
	members    []member
	blocked    map[uuid.UUID]bool
	hasAreas   bool
	hasLabels  bool
}

// ListCycle is a loaded cycle with parsed dates.
type ListCycle struct {
	ID    uuid.UUID
	Title string
	Start time.Time // 00:00 reporting tz
	EndEx time.Time // day AFTER end_date, 00:00 reporting tz (exclusive)
}

// typeOrder fixes the lifecycle display order.
var typeOrder = map[string]int{
	"backlog": 0, "unstarted": 1, "started": 2, "verification": 3,
	"completed": 4, "cancelled": 5, "archived": 6,
}

// openTypes are the state types that count as open work. Verification is open:
// built but still being validated is not delivered.
func isOpenType(t string) bool {
	return t == "backlog" || t == "unstarted" || t == "started" || t == "verification"
}

// isWorkingType marks the types where an item is actively in flight (started
// or verification) — the population the aging panels watch.
func isWorkingType(t string) bool {
	return t == "started" || t == "verification"
}

func jsonMap(b []byte) map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// load pulls one whole project into memory.
func (e *Engine) load(ctx context.Context, p store.Project) (*projectData, error) {
	d := &projectData{
		id:         p.ID,
		key:        p.ProjectKey,
		name:       p.Name,
		createdAt:  p.CreatedAt.Time,
		states:     map[uuid.UUID]stateInfo{},
		priorities: map[uuid.UUID]prioInfo{},
		tasks:      map[uuid.UUID]*task{},
		users:      map[uuid.UUID]UserRef{},
		blocked:    map[uuid.UUID]bool{},
	}

	states, err := e.store.ListProjectStates(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("states: %w", err)
	}
	for _, s := range states {
		si := stateInfo{ID: s.ID, Name: s.Name, Type: s.StateType,
			Color: s.Color.String, Position: int(s.Position)}
		d.states[s.ID] = si
		d.stateList = append(d.stateList, si)
	}
	sort.Slice(d.stateList, func(i, j int) bool {
		a, b := d.stateList[i], d.stateList[j]
		if typeOrder[a.Type] != typeOrder[b.Type] {
			return typeOrder[a.Type] < typeOrder[b.Type]
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.Name < b.Name
	})

	prios, err := e.store.ListProjectPriorities(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("priorities: %w", err)
	}
	for _, pr := range prios {
		pi := prioInfo{ID: pr.ID, Name: pr.Name, Rank: int(pr.Rank),
			Color: pr.Color.String, Active: pr.Active}
		d.priorities[pr.ID] = pi
		d.prioList = append(d.prioList, pi)
	}
	sort.Slice(d.prioList, func(i, j int) bool { return d.prioList[i].Rank < d.prioList[j].Rank })

	rows, err := e.store.ListInsightsTasks(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("tasks: %w", err)
	}
	for _, r := range rows {
		t := &task{
			id: r.ID, number: r.TaskNumber, title: r.Title,
			stateID: r.StateID, priorityID: r.PriorityID,
			difficulty: r.Difficulty, effort: r.Effort,
			createdAt: r.CreatedAt.Time,
		}
		if r.ParentTaskID.Valid {
			t.parent = uuid.UUID(r.ParentTaskID.Bytes)
		}
		if r.DueDate.Valid {
			t.dueDate = r.DueDate.Time
		}
		d.tasks[r.ID] = t
		d.order = append(d.order, r.ID)
	}
	sort.Slice(d.order, func(i, j int) bool {
		return d.tasks[d.order[i]].number > d.tasks[d.order[j]].number
	})
	for id, t := range d.tasks {
		if t.parent != (uuid.UUID{}) {
			if parent, ok := d.tasks[t.parent]; ok {
				parent.children = append(parent.children, id)
			} else {
				// Parent deleted: treat the child as standalone.
				t.parent = uuid.UUID{}
			}
		}
	}

	acts, err := e.store.ListInsightsActivity(ctx, store.ListInsightsActivityParams{
		ProjectID: p.ID, Types: activityTypes,
	})
	if err != nil {
		return nil, fmt.Errorf("activity: %w", err)
	}
	for _, a := range acts {
		t, ok := d.tasks[a.TaskID]
		if !ok {
			continue
		}
		t.events = append(t.events, event{
			typ: a.ActivityType, at: a.CreatedAt.Time, actor: a.ActorID,
			old: jsonMap(a.OldValue), new: jsonMap(a.NewValue),
		})
	}

	assignees, err := e.store.ListInsightsAssignees(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("assignees: %w", err)
	}
	for _, a := range assignees {
		if t, ok := d.tasks[a.TaskID]; ok {
			t.assigneesNow = append(t.assigneesNow, a.UserID)
		}
	}
	contribs, err := e.store.ListInsightsContributors(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("contributors: %w", err)
	}
	for _, c := range contribs {
		if t, ok := d.tasks[c.TaskID]; ok {
			t.contributors = append(t.contributors, c.UserID)
		}
	}

	cycleTasks, err := e.store.ListInsightsCycleTasks(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("cycle tasks: %w", err)
	}
	for _, ct := range cycleTasks {
		if t, ok := d.tasks[ct.TaskID]; ok {
			t.cycleNow = ct.CycleID
			t.cycleAddedAt = ct.AddedAt.Time
		}
	}

	cycles, err := e.store.ListInsightsCycles(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("cycles: %w", err)
	}
	for _, c := range cycles {
		start := time.Date(c.StartDate.Time.Year(), c.StartDate.Time.Month(), c.StartDate.Time.Day(), 0, 0, 0, 0, e.tz)
		end := time.Date(c.EndDate.Time.Year(), c.EndDate.Time.Month(), c.EndDate.Time.Day(), 0, 0, 0, 0, e.tz).AddDate(0, 0, 1)
		d.cycles = append(d.cycles, ListCycle{ID: c.ID, Title: c.Title, Start: start, EndEx: end})
	}

	blockedIDs, err := e.store.ListInsightsBlockedTasks(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("blockers: %w", err)
	}
	for _, id := range blockedIDs {
		d.blocked[id] = true
	}

	memberRows, err := e.store.ListProjectMembers(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("members: %w", err)
	}
	for _, m := range memberRows {
		ref := UserRef{ID: m.UserID, Username: m.Username, FirstName: m.FirstName, LastName: m.LastName}
		if m.AvatarUrl.Valid {
			v := m.AvatarUrl.String
			ref.AvatarURL = &v
		}
		d.users[m.UserID] = ref
		d.members = append(d.members, member{UserRef: ref, role: m.Role})
	}

	areas, err := e.store.ListProjectAreas(ctx, p.ID)
	if err == nil {
		d.hasAreas = len(areas) > 0
	}
	labels, err := e.store.ListProjectLabels(ctx, p.ID)
	if err == nil {
		d.hasLabels = len(labels) > 0
	}

	// Resolve people referenced by credit/assignment who are no longer
	// members (they still appear on historical rows).
	missing := map[uuid.UUID]bool{}
	for _, t := range d.tasks {
		for _, u := range append(append([]uuid.UUID{}, t.assigneesNow...), t.contributors...) {
			if _, ok := d.users[u]; !ok {
				missing[u] = true
			}
		}
		for _, ev := range t.events {
			if _, ok := d.users[ev.actor]; !ok {
				missing[ev.actor] = true
			}
		}
	}
	if len(missing) > 0 {
		ids := make([]uuid.UUID, 0, len(missing))
		for u := range missing {
			ids = append(ids, u)
		}
		users, err := e.store.ListUsersByIDs(ctx, ids)
		if err == nil {
			for _, u := range users {
				ref := UserRef{ID: u.ID, Username: u.Username, FirstName: u.FirstName, LastName: u.LastName}
				if u.AvatarUrl.Valid {
					v := u.AvatarUrl.String
					ref.AvatarURL = &v
				}
				d.users[u.ID] = ref
			}
		}
	}

	return d, nil
}

// ---------- replay ----------

func uuidFrom(m map[string]any, key string) uuid.UUID {
	if m == nil {
		return uuid.UUID{}
	}
	s, _ := m[key].(string)
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}
	}
	return id
}

// statePeriod is a half-open interval [from, next.from) in a task's state
// timeline.
type statePeriod struct {
	from    time.Time
	stateID uuid.UUID
}

// stateTimeline reconstructs the task's state over time. The first period
// starts at task creation; with no state_changed events the current state is
// assumed to have held since creation (import-era history).
func (d *projectData) stateTimeline(t *task) []statePeriod {
	initial := t.stateID
	for _, ev := range t.events {
		if ev.typ == "state_changed" {
			if old := uuidFrom(ev.old, "state_id"); old != (uuid.UUID{}) {
				initial = old
			}
			break
		}
	}
	tl := []statePeriod{{from: t.createdAt, stateID: initial}}
	for _, ev := range t.events {
		if ev.typ != "state_changed" {
			continue
		}
		if next := uuidFrom(ev.new, "state_id"); next != (uuid.UUID{}) {
			tl = append(tl, statePeriod{from: ev.at, stateID: next})
		}
	}
	return tl
}

func (d *projectData) stateAt(t *task, at time.Time) uuid.UUID {
	tl := d.stateTimeline(t)
	cur := tl[0].stateID
	for _, p := range tl {
		if p.from.After(at) {
			break
		}
		cur = p.stateID
	}
	return cur
}

func (d *projectData) typeAt(t *task, at time.Time) string {
	return d.states[d.stateAt(t, at)].Type
}

// completion is one entry into a completed-type state.
type completion struct {
	at      time.Time
	actor   uuid.UUID
	stateID uuid.UUID
}

// completions lists every event where the task entered a completed-type state
// (by the current state-type mapping, as documented).
func (d *projectData) completions(t *task) []completion {
	var out []completion
	for _, ev := range t.events {
		if ev.typ != "state_changed" {
			continue
		}
		next := uuidFrom(ev.new, "state_id")
		if d.states[next].Type == "completed" {
			out = append(out, completion{at: ev.at, actor: ev.actor, stateID: next})
		}
	}
	return out
}

// span is a half-open membership interval [from, to); to zero = still open.
type span struct {
	from, to time.Time
}

func (s span) covers(at time.Time) bool {
	return !s.from.After(at) && (s.to.IsZero() || s.to.After(at))
}

// cycleMembership reconstructs when the task (top-level) was in the given
// cycle. Events win; when the task sits in the cycle today without any event
// mentioning that cycle (import-era), the membership is anchored at
// cycle_tasks.added_at.
func (d *projectData) cycleMembership(t *task, cycleID uuid.UUID) []span {
	var spans []span
	open := -1
	sawEvent := false
	for _, ev := range t.events {
		var ref uuid.UUID
		switch ev.typ {
		case "cycle_added":
			ref = uuidFrom(ev.new, "cycle_id")
			if ref != cycleID {
				continue
			}
			sawEvent = true
			if open == -1 {
				spans = append(spans, span{from: ev.at})
				open = len(spans) - 1
			}
		case "cycle_removed":
			ref = uuidFrom(ev.old, "cycle_id")
			if ref != cycleID {
				continue
			}
			sawEvent = true
			if open == -1 {
				// Removed without a recorded addition: member since creation.
				spans = append(spans, span{from: t.createdAt, to: ev.at})
			} else {
				spans[open].to = ev.at
				open = -1
			}
		}
	}
	if !sawEvent && t.cycleNow == cycleID {
		from := t.cycleAddedAt
		if from.IsZero() {
			from = t.createdAt
		}
		spans = append(spans, span{from: from})
	}
	// Reconcile with the present: events may end before a silent re-add.
	if sawEvent && open == -1 && t.cycleNow == cycleID {
		last := t.cycleAddedAt
		if len(spans) > 0 && !spans[len(spans)-1].to.IsZero() && last.After(spans[len(spans)-1].to) {
			spans = append(spans, span{from: last})
		}
	}
	return spans
}

func memberAt(spans []span, at time.Time) bool {
	for _, s := range spans {
		if s.covers(at) {
			return true
		}
	}
	return false
}

// firstEntryAfter returns the first membership start strictly after at; zero
// time when none.
func firstEntryAfter(spans []span, at time.Time) time.Time {
	var best time.Time
	for _, s := range spans {
		if s.from.After(at) && (best.IsZero() || s.from.Before(best)) {
			best = s.from
		}
	}
	return best
}

// top returns the planning unit whose cycle membership governs this task: the
// parent for a subtask, the task itself otherwise.
func (d *projectData) top(t *task) *task {
	if t.parent != (uuid.UUID{}) {
		if p, ok := d.tasks[t.parent]; ok {
			return p
		}
	}
	return t
}

// leafUnits are the counted work items: tasks without children.
func (d *projectData) leafUnits() []*task {
	var out []*task
	for _, id := range d.order {
		t := d.tasks[id]
		if len(t.children) == 0 {
			out = append(out, t)
		}
	}
	return out
}

// assigneesAt replays assignee events backwards from the current set to find
// the initial set, then forward to the asked moment. With no events the
// current assignees stand in (import-era fallback).
func (d *projectData) assigneesAt(t *task, at time.Time) []uuid.UUID {
	var evs []event
	for _, ev := range t.events {
		if ev.typ == "assignee_added" || ev.typ == "assignee_removed" {
			evs = append(evs, ev)
		}
	}
	if len(evs) == 0 {
		return t.assigneesNow
	}
	set := map[uuid.UUID]bool{}
	for _, u := range t.assigneesNow {
		set[u] = true
	}
	// Reverse to the initial set.
	for i := len(evs) - 1; i >= 0; i-- {
		ev := evs[i]
		switch ev.typ {
		case "assignee_added":
			delete(set, uuidFrom(ev.new, "user_id"))
		case "assignee_removed":
			set[uuidFrom(ev.old, "user_id")] = true
		}
	}
	// Forward to the asked moment.
	for _, ev := range evs {
		if ev.at.After(at) {
			break
		}
		switch ev.typ {
		case "assignee_added":
			set[uuidFrom(ev.new, "user_id")] = true
		case "assignee_removed":
			delete(set, uuidFrom(ev.old, "user_id"))
		}
	}
	out := make([]uuid.UUID, 0, len(set))
	for u := range set {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// credits resolves who a delivered item credits: Contributors first, assignees
// at completion time as the fallback.
func (d *projectData) credits(t *task, completedAt time.Time) []uuid.UUID {
	if len(t.contributors) > 0 {
		out := append([]uuid.UUID{}, t.contributors...)
		sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
		return out
	}
	return d.assigneesAt(t, completedAt)
}

// deliveredIn finds the first qualifying completion for a unit: entered a
// completed-type state inside [from, cutoff) and still completed-type at the
// cutoff (reopened work falls out). inCycle additionally requires membership
// at the completion moment.
func (d *projectData) deliveredIn(t *task, from, cutoff time.Time, spans []span, requireCycle bool) (completion, bool) {
	if d.states[d.stateAt(t, cutoff)].Type != "completed" {
		return completion{}, false
	}
	for _, c := range d.completions(t) {
		if c.at.Before(from) || !c.at.Before(cutoff) {
			continue
		}
		if requireCycle && !memberAt(spans, c.at) {
			continue
		}
		return c, true
	}
	return completion{}, false
}

func (d *projectData) userRef(id uuid.UUID) UserRef {
	if u, ok := d.users[id]; ok {
		return u
	}
	return UserRef{ID: id, Username: "unknown"}
}

func (d *projectData) stateRef(id uuid.UUID) StateRef {
	s := d.states[id]
	return StateRef{ID: s.ID, Name: s.Name, Type: s.Type, Color: s.Color}
}

func (d *projectData) prioRef(id uuid.UUID) *PriorityRef {
	p, ok := d.priorities[id]
	if !ok {
		return nil
	}
	return &PriorityRef{ID: p.ID, Name: p.Name, Rank: p.Rank, Color: p.Color, Active: p.Active}
}

// hasType says whether the project defines any state of the given type; a
// missing type means the matching tile is absent, never zero.
func (d *projectData) hasType(stateType string) bool {
	for _, s := range d.stateList {
		if s.Type == stateType {
			return true
		}
	}
	return false
}

// topRank is the highest active rank the project defines (normally 4).
func (d *projectData) topRank() (int, string) {
	best, name := -1, ""
	for _, p := range d.prioList {
		if p.Rank > best && p.Rank > 0 {
			best, name = p.Rank, p.Name
		}
	}
	return best, name
}

func (d *projectData) key4(t *task) string {
	return fmt.Sprintf("%s-%d", d.key, t.number)
}

// dayKey formats a time as the reporting-timezone date.
func (e *Engine) dayKey(t time.Time) string {
	return t.In(e.tz).Format("2006-01-02")
}

// weekStart returns the Monday 00:00 of the week containing t.
func (e *Engine) weekStart(t time.Time) time.Time {
	t = t.In(e.tz)
	wd := (int(t.Weekday()) + 6) % 7 // Monday=0
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, e.tz)
	return day.AddDate(0, 0, -wd)
}
