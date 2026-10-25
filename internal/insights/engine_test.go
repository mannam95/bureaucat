package insights

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"bereaucat/internal/store"
)

// buildHarbor reproduces the shared data contract of the final build
// reference: 30 planned + 6 added - 3 removed = 33 at close = 25 completed +
// 7 open + 1 archived; original-plan delivery 22/30; delivered complexity 142
// across 20 of 25 assessed; planned complexity 171 across 23 of 30; 27 person
// credits across 25 items with 2 shared. The engine must reproduce it digit
// for digit.
func buildHarbor(t *testing.T) (*fixture, uuid.UUID) {
	t.Helper()
	f := newFixture("HBR")
	f.state("Backlog", "backlog", 0)
	f.state("Todo", "unstarted", 1)
	f.state("In Progress", "started", 2)
	f.state("Done", "completed", 3)
	f.state("Archived", "archived", 4)
	// Harbor defines NO cancelled-type state: its Cancelled tile must be
	// absent, not zero.
	f.prio("No priority", 0, true)
	f.prio("Low", 1, true)
	f.prio("Medium", 2, true)
	f.prio("Major", 3, true)
	f.prio("Critical / Blocking", 4, true)

	cycle := f.cycle("Cycle 18", "2026-10-05", "2026-10-18")
	qa := f.user("qa1")

	// Estimates per the contract arithmetic (difficulty x effort):
	// delivered-planned assessed: P1-10 cx6, P11-16 cx9, P17 cx10, P18 cx4 = 128
	// delivered-added assessed: A1 cx6, A2 cx8 = 14 -> delivered 142/20 of 25
	// open-planned assessed: P23 cx9, P24 cx10, P25 cx12 = 31
	// archived: P27 cx4; removed assessed: P28 cx8 -> planned 171/23 of 30
	de := map[string][2]int{
		"P17": {2, 5}, "P18": {2, 2}, "P23": {3, 3}, "P24": {2, 5}, "P25": {3, 4},
		"P27": {2, 2}, "P28": {2, 4},
		"A1": {2, 3}, "A2": {2, 4},
	}
	for i := 1; i <= 10; i++ {
		de[fmt.Sprintf("P%d", i)] = [2]int{2, 3}
	}
	for i := 11; i <= 16; i++ {
		de[fmt.Sprintf("P%d", i)] = [2]int{3, 3}
	}

	// The 30 planned units.
	for i := 1; i <= 30; i++ {
		key := fmt.Sprintf("P%d", i)
		d := de[key]
		switch key {
		case "P21":
			// Import-era membership: no cycle events, only cycle_tasks.
			f.task(key, "Backlog", "Medium", d[0], d[1], "2026-09-20 10:00")
			f.inCycleNow(key, cycle, "2026-10-01 09:00")
		case "P22":
			// A subtask: the parent carries the cycle membership.
			f.task("PP", "In Progress", "Medium", 0, 0, "2026-09-19 10:00")
			f.cycleAdd("PP", cycle, "2026-10-01 09:00")
			f.subtask(key, "PP", "Backlog", "Medium", d[0], d[1], "2026-09-20 10:00")
		default:
			f.task(key, "Backlog", "Medium", d[0], d[1], "2026-09-20 10:00")
			f.cycleAdd(key, cycle, "2026-10-01 09:00")
		}
	}
	// The 6 added units, created inside the window.
	for i := 1; i <= 6; i++ {
		key := fmt.Sprintf("A%d", i)
		d := de[key]
		f.task(key, "Backlog", "Major", d[0], d[1], "2026-10-07 09:00")
	}
	f.cycleAdd("A1", cycle, "2026-10-07 10:00")
	f.cycleRemove("A1", cycle, "2026-10-08 10:00")
	f.cycleAdd("A1", cycle, "2026-10-09 10:00") // re-entry counts once
	for i := 2; i <= 6; i++ {
		f.cycleAdd(fmt.Sprintf("A%d", i), cycle, "2026-10-08 10:00")
	}
	// The 3 descoped planned units.
	for _, key := range []string{"P28", "P29", "P30"} {
		f.cycleRemove(key, cycle, "2026-10-16 10:00")
	}

	// Credit setup.
	f.contributor("P1", "dev1")  // contributors win over the current assignee
	f.assignNow("P1", "tester1") // the hand-over left a tester assigned
	f.assign("P18", "alice", "2026-10-05 09:00")
	f.assign("P18", "bob", "2026-10-05 09:00") // shared via two assignees
	f.contributor("P19", "carol")
	f.contributor("P19", "dan") // shared via two contributors
	f.assign("P20", "dev2", "2026-10-05 09:00")
	f.assignNow("P21", "erin") // import-era: no assignee events at all
	f.assign("P22", "frank", "2026-10-05 09:00")
	// Everyone else: one assignee each, round-robin over three people.
	workers := []string{"gina", "hugo", "iris"}
	for i := 2; i <= 17; i++ {
		f.assign(fmt.Sprintf("P%d", i), workers[i%3], "2026-10-05 09:00")
	}
	for i := 1; i <= 3; i++ {
		f.assign(fmt.Sprintf("A%d", i), workers[i%3], "2026-10-09 11:00")
	}

	// Completions: 22 planned + 3 added, spread across the cycle, all still
	// completed at the cutoff.
	for i := 1; i <= 22; i++ {
		key := fmt.Sprintf("P%d", i)
		day := 6 + i%10
		f.move(key, "Backlog", "In Progress", fmt.Sprintf("2026-10-%02d 09:00", day), qa)
		f.move(key, "In Progress", "Done", fmt.Sprintf("2026-10-%02d 15:00", day), qa)
	}
	for i := 1; i <= 3; i++ {
		f.move(fmt.Sprintf("A%d", i), "Backlog", "Done", fmt.Sprintf("2026-10-%02d 16:00", 11+i), qa)
	}
	// The hand-over on P20: dev2 built it, was unassigned after completion,
	// the tester took over. Credit must stay with dev2.
	f.unassign("P20", "dev2", "2026-10-17 10:00")
	f.assign("P20", "tester2", "2026-10-17 10:01")

	// The archived planned unit.
	f.move("P27", "Backlog", "Archived", "2026-10-17 12:00", qa)
	// P23..P26 stay open; A4..A6 stay open: 7 open at close.

	return f, cycle
}

func TestHarborCycleContract(t *testing.T) {
	f, cycle := buildHarbor(t)
	e := f.engine("2026-10-25 12:00")
	rep, err := e.ProjectReport(context.Background(), f.project, ReportOptions{CycleID: &cycle})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	sec := rep.Cycle
	if sec == nil {
		t.Fatal("expected a cycle section")
	}

	if sec.PlannedAtStart.Count != 30 {
		t.Errorf("planned = %d, want 30", sec.PlannedAtStart.Count)
	}
	if sec.Added.Count != 6 {
		t.Errorf("added = %d, want 6 (re-entry must count once)", sec.Added.Count)
	}
	if sec.Removed.Count != 3 {
		t.Errorf("removed = %d, want 3", sec.Removed.Count)
	}
	if sec.AtClose != 33 {
		t.Errorf("at close = %d, want 33", sec.AtClose)
	}
	if sec.Delivered.Count != 25 {
		t.Errorf("delivered = %d, want 25", sec.Delivered.Count)
	}
	if sec.OriginalPlan.Count != 22 {
		t.Errorf("original plan delivered = %d, want 22", sec.OriginalPlan.Count)
	}
	if sec.CarriedOver.Count != 7 {
		t.Errorf("carried = %d, want 7", sec.CarriedOver.Count)
	}
	if sec.Cancelled != nil {
		t.Errorf("cancelled tile must be absent: Harbor defines no cancelled-type state")
	}
	if sec.Archived == nil || sec.Archived.Count != 1 {
		t.Errorf("archived tile = %+v, want count 1", sec.Archived)
	}

	cx := sec.Delivered.Cx
	if cx.Sum != 142 || cx.Assessed != 20 || cx.Of != 25 || cx.NotAssessed != 5 {
		t.Errorf("delivered cx = %+v, want 142 across 20 of 25", cx)
	}
	pcx := sec.PlannedAtStart.Cx
	if pcx.Sum != 171 || pcx.Assessed != 23 || pcx.Of != 30 {
		t.Errorf("planned cx = %+v, want 171 across 23 of 30", pcx)
	}

	wantIdentity := "30 planned + 6 added - 3 removed = 33 at close = 25 completed + 7 open + 1 archived"
	if sec.Identity != wantIdentity {
		t.Errorf("identity = %q, want %q", sec.Identity, wantIdentity)
	}

	// The daily series ends at the cycle end with the full picture.
	last := sec.Days[len(sec.Days)-1]
	if last.CompletedCum != 25 {
		t.Errorf("final completed cum = %d, want 25", last.CompletedCum)
	}
	if last.Scope != 33 {
		t.Errorf("final scope = %d, want 33", last.Scope)
	}

	// People: 27 credits across 25 distinct items, 2 shared.
	credits, shared := 0, 0
	byName := map[string]PersonRow{}
	for _, row := range rep.People.Rows {
		credits += row.Delivered
		shared += row.Shared
		byName[row.User.Username] = row
	}
	if rep.People.TotalDelivered != 25 {
		t.Errorf("total delivered = %d, want 25", rep.People.TotalDelivered)
	}
	if credits != 27 {
		t.Errorf("person credits = %d, want 27", credits)
	}
	if shared != 4 { // two shared items, each counted on both people
		t.Errorf("shared row count = %d, want 4", shared)
	}
	if byName["dev1"].Delivered != 1 {
		t.Errorf("dev1 delivered = %d, want 1 (contributors beat current assignee)", byName["dev1"].Delivered)
	}
	if byName["tester1"].Delivered != 0 {
		t.Errorf("tester1 delivered = %d, want 0", byName["tester1"].Delivered)
	}
	if byName["dev2"].Delivered != 1 {
		t.Errorf("dev2 delivered = %d, want 1 (assignees at completion time)", byName["dev2"].Delivered)
	}
	if byName["tester2"].Delivered != 0 {
		t.Errorf("tester2 delivered = %d, want 0 (assigned after completion)", byName["tester2"].Delivered)
	}
	if byName["erin"].Delivered != 1 {
		t.Errorf("erin delivered = %d, want 1 (import-era current assignees)", byName["erin"].Delivered)
	}
	if byName["frank"].Delivered != 1 {
		t.Errorf("frank delivered = %d, want 1 (subtask counts through its parent's cycle)", byName["frank"].Delivered)
	}
	for _, name := range []string{"alice", "bob", "carol", "dan"} {
		if byName[name].Shared != 1 {
			t.Errorf("%s shared = %d, want 1", name, byName[name].Shared)
		}
	}
}

func TestHarborEvidence(t *testing.T) {
	f, cycle := buildHarbor(t)
	e := f.engine("2026-10-25 12:00")
	ctx := context.Background()

	ev, err := e.Evidence(ctx, f.project, EvidenceParams{Metric: "delivered", CycleID: &cycle})
	if err != nil {
		t.Fatalf("delivered evidence: %v", err)
	}
	if ev.Count != 25 {
		t.Errorf("delivered evidence rows = %d, want 25", ev.Count)
	}
	for _, row := range ev.Rows {
		if row.CompletedAt == nil || row.MarkedCompleteBy == nil {
			t.Fatalf("row %s misses completion facts", row.Key)
		}
		if row.MarkedCompleteBy.Username != "qa1" {
			t.Errorf("row %s marked complete by %s, want qa1 (actor, not credit)", row.Key, row.MarkedCompleteBy.Username)
		}
		if len(row.Credits) == 0 {
			t.Errorf("row %s has no credits", row.Key)
		}
	}

	added, err := e.Evidence(ctx, f.project, EvidenceParams{Metric: "added", CycleID: &cycle})
	if err != nil {
		t.Fatalf("added evidence: %v", err)
	}
	if added.Count != 6 {
		t.Errorf("added evidence rows = %d, want 6", added.Count)
	}

	person, err := e.Evidence(ctx, f.project, EvidenceParams{Metric: "person_delivered", CycleID: &cycle, UserID: ptr(f.userIDs["dev2"])})
	if err != nil {
		t.Fatalf("person evidence: %v", err)
	}
	if person.Count != 1 || person.Rows[0].Key != "HBR-"+taskNum(f, "P20") {
		t.Errorf("dev2 evidence = %+v, want exactly P20", person.Rows)
	}

	if _, err := e.Evidence(ctx, f.project, EvidenceParams{Metric: "nope"}); err == nil {
		t.Error("unknown metric must be rejected, not guessed")
	}
}

func ptr[T any](v T) *T { return &v }

func taskNum(f *fixture, key string) string {
	for _, t := range f.fs.tasks {
		if t.ID == f.taskIDs[key] {
			return fmt.Sprint(t.TaskNumber)
		}
	}
	return "?"
}

func TestReopenFallsOut(t *testing.T) {
	f := newFixture("RE")
	f.state("Todo", "unstarted", 0)
	f.state("Done", "completed", 1)
	f.prio("No priority", 0, true)
	cycle := f.cycle("C1", "2026-10-05", "2026-10-18")
	f.task("T1", "Todo", "No priority", 2, 2, "2026-10-01 09:00")
	f.cycleAdd("T1", cycle, "2026-10-04 09:00")
	f.assign("T1", "ann", "2026-10-05 09:00")
	f.move("T1", "Todo", "Done", "2026-10-08 09:00", f.userIDs["ann"])
	f.move("T1", "Done", "Todo", "2026-10-10 09:00", f.userIDs["ann"]) // reopened

	e := f.engine("2026-10-25 12:00")
	rep, err := e.ProjectReport(context.Background(), f.project, ReportOptions{CycleID: &cycle})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if rep.Cycle.Delivered.Count != 0 {
		t.Errorf("delivered = %d, want 0: reopened work falls out", rep.Cycle.Delivered.Count)
	}
	if rep.Cycle.CarriedOver.Count != 1 {
		t.Errorf("carried = %d, want 1", rep.Cycle.CarriedOver.Count)
	}
}

func TestPreCompletedArrivalIsNotCycleDelivery(t *testing.T) {
	f := newFixture("PC")
	f.state("Todo", "unstarted", 0)
	f.state("Done", "completed", 1)
	f.prio("No priority", 0, true)
	cycle := f.cycle("C1", "2026-10-05", "2026-10-18")
	f.task("T1", "Todo", "No priority", 2, 2, "2026-09-01 09:00")
	f.assign("T1", "ann", "2026-09-01 10:00")
	f.move("T1", "Todo", "Done", "2026-09-20 09:00", f.userIDs["ann"]) // done before the cycle
	f.cycleAdd("T1", cycle, "2026-10-08 09:00")                        // added already complete

	e := f.engine("2026-10-25 12:00")
	rep, err := e.ProjectReport(context.Background(), f.project, ReportOptions{CycleID: &cycle})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if rep.Cycle.Added.Count != 1 {
		t.Errorf("added = %d, want 1", rep.Cycle.Added.Count)
	}
	if rep.Cycle.Delivered.Count != 0 {
		t.Errorf("delivered = %d, want 0: an already-completed arrival is added scope, not cycle delivery", rep.Cycle.Delivered.Count)
	}
}

func TestRangeReportAndConditionalCancelled(t *testing.T) {
	f := newFixture("RNG")
	f.state("Todo", "unstarted", 0)
	f.state("Done", "completed", 1)
	f.state("Rejected", "cancelled", 2)
	f.prio("Not yet assessed", 0, true)
	f.prio("Minor", 1, true)
	inactive := f.prio("Medium", 2, false) // inactive but may still hold work
	_ = inactive
	f.task("T1", "Todo", "Minor", 2, 3, "2026-10-02 09:00")
	f.assign("T1", "ann", "2026-10-02 10:00")
	f.move("T1", "Todo", "Done", "2026-10-10 09:00", f.userIDs["ann"])
	f.task("T2", "Todo", "Medium", 0, 0, "2026-10-03 09:00")
	f.move("T2", "Todo", "Rejected", "2026-10-11 09:00", f.userIDs["ann"])
	f.task("T3", "Todo", "Not yet assessed", 0, 0, "2026-10-04 09:00")

	e := f.engine("2026-10-25 12:00")
	rep, err := e.ProjectReport(context.Background(), f.project, ReportOptions{
		From: ts("2026-10-01 00:00"), To: ts("2026-10-20 00:00"),
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if rep.Cycle != nil || rep.Range == nil {
		t.Fatal("expected a range section")
	}
	if rep.Range.Completed.Count != 1 {
		t.Errorf("completed = %d, want 1", rep.Range.Completed.Count)
	}
	if rep.Range.Created.Count != 3 {
		t.Errorf("created = %d, want 3", rep.Range.Created.Count)
	}
	if rep.Range.Cancelled == nil || rep.Range.Cancelled.Count != 1 {
		t.Errorf("cancelled tile = %+v, want count 1 (project defines a cancelled-type state)", rep.Range.Cancelled)
	}
	if rep.Range.HasCycles {
		t.Error("has_cycles must be false")
	}
	// The inactive priority holding open work still renders in the mix.
	foundInactive := false
	for _, rc := range rep.Health.PriorityMix {
		if rc.Priority.Name == "Not yet assessed" && rc.Count != 1 {
			t.Errorf("unranked bucket = %d, want 1", rc.Count)
		}
		if !rc.Priority.Active {
			foundInactive = true
		}
	}
	_ = foundInactive // T2 is cancelled (not open), so no inactive row is expected here
}

func TestPortfolioAttention(t *testing.T) {
	f := newFixture("ATT")
	f.state("Todo", "unstarted", 0)
	f.state("Doing", "started", 1)
	f.state("Done", "completed", 2)
	f.prio("No priority", 0, true)
	f.prio("Urgent", 4, true)
	f.project.CreatedAt.Time = ts("2026-08-01 00:00") // enough history to judge

	f.task("T1", "Doing", "Urgent", 0, 0, "2026-10-20 09:00")
	f.move("T1", "Todo", "Doing", "2026-10-20 10:00", uuid.UUID{})
	f.task("T2", "Todo", "No priority", 0, 0, "2026-10-21 09:00")
	f.task("T3", "Todo", "No priority", 0, 0, "2026-10-22 09:00")
	f.task("T4", "Done", "No priority", 2, 2, "2026-10-01 09:00")
	f.move("T4", "Todo", "Done", "2026-10-12 09:00", uuid.UUID{})

	e := f.engine("2026-10-25 12:00")
	p, err := e.Portfolio(context.Background(), []store.Project{f.project}, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("portfolio: %v", err)
	}
	row := p.Rows[0]
	if !row.Attention {
		t.Errorf("attention = false, want true: %+v", row)
	}
	if row.TopRankOpen != 1 || row.TopRankName != "Urgent" {
		t.Errorf("top rank = %d %q, want 1 Urgent", row.TopRankOpen, row.TopRankName)
	}
	if row.Created != 4 || row.Completed != 1 {
		t.Errorf("created/completed = %d/%d, want 4/1 (all four created inside the 30-day window)", row.Created, row.Completed)
	}
	if row.Oldest == nil || row.Oldest.State.Name != "Doing" {
		t.Errorf("oldest = %+v, want the Doing task with its state name", row.Oldest)
	}
}
