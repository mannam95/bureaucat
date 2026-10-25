package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"bereaucat/internal/store"
)

// fakeStore satisfies store.Querier by embedding it (nil: any un-stubbed call
// panics, which is exactly what a test should do) and overriding the queries
// the engine uses.
type fakeStore struct {
	store.Querier
	states       []store.ListProjectStatesRow
	priorities   []store.ListProjectPrioritiesRow
	tasks        []store.ListInsightsTasksRow
	activity     []store.ListInsightsActivityRow
	assignees    []store.ListInsightsAssigneesRow
	contributors []store.ListInsightsContributorsRow
	cycleTasks   []store.ListInsightsCycleTasksRow
	cycles       []store.ListInsightsCyclesRow
	blocked      []uuid.UUID
	members      []store.ListProjectMembersRow
	users        map[uuid.UUID]store.ListUsersByIDsRow
}

func (f *fakeStore) ListProjectStates(ctx context.Context, projectID uuid.UUID) ([]store.ListProjectStatesRow, error) {
	return f.states, nil
}
func (f *fakeStore) ListProjectPriorities(ctx context.Context, projectID uuid.UUID) ([]store.ListProjectPrioritiesRow, error) {
	return f.priorities, nil
}
func (f *fakeStore) ListInsightsTasks(ctx context.Context, projectID uuid.UUID) ([]store.ListInsightsTasksRow, error) {
	return f.tasks, nil
}
func (f *fakeStore) ListInsightsActivity(ctx context.Context, arg store.ListInsightsActivityParams) ([]store.ListInsightsActivityRow, error) {
	want := map[string]bool{}
	for _, t := range arg.Types {
		want[t] = true
	}
	out := []store.ListInsightsActivityRow{}
	for _, a := range f.activity {
		if want[a.ActivityType] {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Time.Before(out[j].CreatedAt.Time) })
	return out, nil
}
func (f *fakeStore) ListInsightsAssignees(ctx context.Context, projectID uuid.UUID) ([]store.ListInsightsAssigneesRow, error) {
	return f.assignees, nil
}
func (f *fakeStore) ListInsightsContributors(ctx context.Context, projectID uuid.UUID) ([]store.ListInsightsContributorsRow, error) {
	return f.contributors, nil
}
func (f *fakeStore) ListInsightsCycleTasks(ctx context.Context, projectID uuid.UUID) ([]store.ListInsightsCycleTasksRow, error) {
	return f.cycleTasks, nil
}
func (f *fakeStore) ListInsightsCycles(ctx context.Context, projectID uuid.UUID) ([]store.ListInsightsCyclesRow, error) {
	return f.cycles, nil
}
func (f *fakeStore) ListInsightsBlockedTasks(ctx context.Context, projectID uuid.UUID) ([]uuid.UUID, error) {
	return f.blocked, nil
}
func (f *fakeStore) ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]store.ListProjectMembersRow, error) {
	return f.members, nil
}
func (f *fakeStore) ListProjectAreas(ctx context.Context, projectID uuid.UUID) ([]store.ProjectArea, error) {
	return nil, nil
}
func (f *fakeStore) ListProjectLabels(ctx context.Context, projectID uuid.UUID) ([]store.ProjectLabel, error) {
	return nil, nil
}
func (f *fakeStore) ListUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]store.ListUsersByIDsRow, error) {
	out := []store.ListUsersByIDsRow{}
	for _, id := range ids {
		if u, ok := f.users[id]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}

// fixture is a small DSL for building a project history.
type fixture struct {
	fs        *fakeStore
	projectID uuid.UUID
	project   store.Project
	stateIDs  map[string]uuid.UUID
	prioIDs   map[string]uuid.UUID
	taskIDs   map[string]uuid.UUID
	userIDs   map[string]uuid.UUID
	actor     uuid.UUID
	seq       int
}

func newFixture(key string) *fixture {
	pid := uuid.New()
	f := &fixture{
		fs:        &fakeStore{users: map[uuid.UUID]store.ListUsersByIDsRow{}},
		projectID: pid,
		project: store.Project{
			ID: pid, ProjectKey: key, Name: key + " project",
			CreatedAt: pgtype.Timestamptz{Time: ts("2026-01-01 00:00"), Valid: true},
		},
		stateIDs: map[string]uuid.UUID{},
		prioIDs:  map[string]uuid.UUID{},
		taskIDs:  map[string]uuid.UUID{},
		userIDs:  map[string]uuid.UUID{},
	}
	f.actor = f.user("system")
	return f
}

func ts(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

func (f *fixture) state(name, stateType string, position int) uuid.UUID {
	id := uuid.New()
	f.stateIDs[name] = id
	f.fs.states = append(f.fs.states, store.ListProjectStatesRow{
		ID: id, ProjectID: f.projectID, StateType: stateType, Name: name,
		Position: int32(position),
	})
	return id
}

func (f *fixture) prio(name string, rank int, active bool) uuid.UUID {
	id := uuid.New()
	f.prioIDs[name] = id
	f.fs.priorities = append(f.fs.priorities, store.ListProjectPrioritiesRow{
		ID: id, ProjectID: f.projectID, Name: name, Rank: int32(rank), Active: active,
	})
	return id
}

func (f *fixture) user(name string) uuid.UUID {
	if id, ok := f.userIDs[name]; ok {
		return id
	}
	id := uuid.New()
	f.userIDs[name] = id
	f.fs.users[id] = store.ListUsersByIDsRow{ID: id, Username: name, FirstName: name, LastName: "T"}
	f.fs.members = append(f.fs.members, store.ListProjectMembersRow{
		ProjectID: f.projectID, UserID: id, Role: "member",
		Username: name, FirstName: name, LastName: "T",
	})
	return id
}

// task adds a task. state/prio are names; difficulty/effort set the estimate.
func (f *fixture) task(key, stateName, prioName string, difficulty, effort int, createdAt string) uuid.UUID {
	f.seq++
	id := uuid.New()
	f.taskIDs[key] = id
	f.fs.tasks = append(f.fs.tasks, store.ListInsightsTasksRow{
		ID: id, TaskNumber: int32(f.seq), Title: "Task " + key,
		StateID:    f.stateIDs[stateName],
		PriorityID: f.prioIDs[prioName],
		Difficulty: int32(difficulty), Effort: int32(effort),
		CreatedAt: pgtype.Timestamptz{Time: ts(createdAt), Valid: true},
	})
	return id
}

func (f *fixture) subtask(key, parentKey, stateName, prioName string, difficulty, effort int, createdAt string) uuid.UUID {
	id := f.task(key, stateName, prioName, difficulty, effort, createdAt)
	for i := range f.fs.tasks {
		if f.fs.tasks[i].ID == id {
			f.fs.tasks[i].ParentTaskID = pgtype.UUID{Bytes: f.taskIDs[parentKey], Valid: true}
		}
	}
	return id
}

func (f *fixture) cycle(title, start, end string) uuid.UUID {
	id := uuid.New()
	f.fs.cycles = append(f.fs.cycles, store.ListInsightsCyclesRow{
		ID: id, Title: title,
		StartDate: pgtype.Date{Time: ts(start + " 00:00"), Valid: true},
		EndDate:   pgtype.Date{Time: ts(end + " 00:00"), Valid: true},
	})
	return id
}

func jb(m map[string]any) []byte {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(m)
	return b
}

func (f *fixture) event(taskKey, typ, at string, actor uuid.UUID, old, new map[string]any) {
	if actor == (uuid.UUID{}) {
		actor = f.actor
	}
	f.fs.activity = append(f.fs.activity, store.ListInsightsActivityRow{
		TaskID: f.taskIDs[taskKey], ActivityType: typ,
		OldValue: jb(old), NewValue: jb(new),
		CreatedAt: pgtype.Timestamptz{Time: ts(at), Valid: true},
		ActorID:   actor,
	})
}

// cycleAdd/cycleRemove record membership events.
func (f *fixture) cycleAdd(taskKey string, cycleID uuid.UUID, at string) {
	f.event(taskKey, "cycle_added", at, uuid.UUID{}, nil, map[string]any{"cycle_id": cycleID.String()})
}
func (f *fixture) cycleRemove(taskKey string, cycleID uuid.UUID, at string) {
	f.event(taskKey, "cycle_removed", at, uuid.UUID{}, map[string]any{"cycle_id": cycleID.String()}, nil)
}

// move records a state change by state names, with the actor who did it.
func (f *fixture) move(taskKey, fromState, toState, at string, actor uuid.UUID) {
	f.event(taskKey, "state_changed", at, actor,
		map[string]any{"state_id": f.stateIDs[fromState].String(), "name": fromState},
		map[string]any{"state_id": f.stateIDs[toState].String(), "name": toState})
	// Keep the current state in sync with the last event applied.
	for i := range f.fs.tasks {
		if f.fs.tasks[i].ID == f.taskIDs[taskKey] {
			f.fs.tasks[i].StateID = f.stateIDs[toState]
		}
	}
}

func (f *fixture) assign(taskKey, userName, at string) {
	id := f.user(userName)
	f.event(taskKey, "assignee_added", at, uuid.UUID{}, nil,
		map[string]any{"user_id": id.String(), "username": userName})
	f.fs.assignees = append(f.fs.assignees, store.ListInsightsAssigneesRow{
		TaskID: f.taskIDs[taskKey], UserID: id,
	})
}

func (f *fixture) unassign(taskKey, userName, at string) {
	id := f.user(userName)
	f.event(taskKey, "assignee_removed", at, uuid.UUID{},
		map[string]any{"user_id": id.String(), "username": userName}, nil)
	for i, a := range f.fs.assignees {
		if a.TaskID == f.taskIDs[taskKey] && a.UserID == id {
			f.fs.assignees = append(f.fs.assignees[:i], f.fs.assignees[i+1:]...)
			break
		}
	}
}

// assignNow records a current assignee without any event (import-era).
func (f *fixture) assignNow(taskKey, userName string) {
	id := f.user(userName)
	f.fs.assignees = append(f.fs.assignees, store.ListInsightsAssigneesRow{
		TaskID: f.taskIDs[taskKey], UserID: id,
	})
}

func (f *fixture) contributor(taskKey, userName string) {
	id := f.user(userName)
	f.fs.contributors = append(f.fs.contributors, store.ListInsightsContributorsRow{
		TaskID: f.taskIDs[taskKey], UserID: id,
	})
}

// inCycleNow records current membership (cycle_tasks) without events.
func (f *fixture) inCycleNow(taskKey string, cycleID uuid.UUID, addedAt string) {
	f.fs.cycleTasks = append(f.fs.cycleTasks, store.ListInsightsCycleTasksRow{
		CycleID: cycleID, TaskID: f.taskIDs[taskKey],
		AddedAt: pgtype.Timestamptz{Time: ts(addedAt), Valid: true},
	})
}

func (f *fixture) engine(now string) *Engine {
	e := New(f.fs, time.UTC)
	fixed := ts(now)
	e.now = func() time.Time { return fixed }
	return e
}

func (f *fixture) keyOf(n int, prefix string) string {
	return fmt.Sprintf("%s%d", prefix, n)
}
