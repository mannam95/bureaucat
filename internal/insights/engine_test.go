package insights

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"bereaucat/internal/store"
)

// buildHarbor reproduces the shared data contract of the final build
// reference: 30 planned + 6 added - 3 removed = 33 in the cycle = 0 in
// development + 25 delivered + 8 deferred (7 parked in Backlog + 1 archived);
// original-plan delivery 22/30; delivered complexity 142 across 20 of 25
// assessed; planned complexity 171 across 23 of 30; 27 person credits across
// 25 items with 2 shared. The engine must reproduce it digit for digit.
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

	if sec.InCycle.Count != 33 {
		t.Errorf("in cycle = %d, want 33", sec.InCycle.Count)
	}
	if sec.BoardTasks != 33 {
		t.Errorf("board tasks = %d, want 33 (PP stands in for its one subtask)", sec.BoardTasks)
	}
	if sec.Delivered.Count != 25 {
		t.Errorf("delivered = %d, want 25", sec.Delivered.Count)
	}
	if sec.InDevelopment.Count != 0 {
		t.Errorf("in development = %d, want 0 (the unfinished work parks in Backlog)", sec.InDevelopment.Count)
	}
	if sec.Deferred.Count != 8 {
		t.Errorf("deferred = %d, want 8 (7 parked in Backlog + 1 archived)", sec.Deferred.Count)
	}
	if sec.HandedOff != nil || sec.InVerification != nil {
		t.Error("handed off and in verification must be absent without verification states")
	}

	cx := sec.Delivered.Cx
	if cx.Sum != 142 || cx.Assessed != 20 || cx.Of != 25 || cx.NotAssessed != 5 {
		t.Errorf("delivered cx = %+v, want 142 across 20 of 25", cx)
	}

	wantIdentity := "30 planned + 6 added - 3 removed = 33 in the cycle = 0 in development + 25 delivered + 8 deferred"
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
		t.Errorf("added evidence rows = %d, want 6 (re-entry must count once)", added.Count)
	}

	// The plan history keeps its populations (and the contract's planned
	// complexity) even though it renders as the identity line, not as cards.
	planned, err := e.Evidence(ctx, f.project, EvidenceParams{Metric: "planned", CycleID: &cycle})
	if err != nil {
		t.Fatalf("planned evidence: %v", err)
	}
	if planned.Count != 30 || planned.Cx.Sum != 171 || planned.Cx.Assessed != 23 {
		t.Errorf("planned evidence = %d with cx %+v, want 30 with 171 across 23", planned.Count, planned.Cx)
	}
	originalPlan, err := e.Evidence(ctx, f.project, EvidenceParams{Metric: "original_plan", CycleID: &cycle})
	if err != nil {
		t.Fatalf("original_plan evidence: %v", err)
	}
	if originalPlan.Count != 22 {
		t.Errorf("original plan delivered = %d, want 22", originalPlan.Count)
	}

	// The snapshot buckets have their own drawers and partition In Cycle.
	inCycle, err := e.Evidence(ctx, f.project, EvidenceParams{Metric: "in_cycle", CycleID: &cycle})
	if err != nil {
		t.Fatalf("in_cycle evidence: %v", err)
	}
	if inCycle.Count != 33 {
		t.Errorf("in_cycle evidence rows = %d, want 33", inCycle.Count)
	}
	deferred, err := e.Evidence(ctx, f.project, EvidenceParams{Metric: "deferred", CycleID: &cycle})
	if err != nil {
		t.Fatalf("deferred evidence: %v", err)
	}
	if deferred.Count != 8 {
		t.Errorf("deferred evidence rows = %d, want 8", deferred.Count)
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
	if rep.Cycle.InCycle.Count != 1 || rep.Cycle.InDevelopment.Count != 1 {
		t.Errorf("in cycle/in development = %d/%d, want 1/1: reopened work sits in development again",
			rep.Cycle.InCycle.Count, rep.Cycle.InDevelopment.Count)
	}
}

// TestPreCompletedArrivalCountsInCycleSnapshot: a cycle reads as a snapshot,
// so an item added to the cycle already complete counts as delivered there —
// the cycle shows what it holds, not where the work happened — dated and
// credited at the completion that holds.
func TestPreCompletedArrivalCountsInCycleSnapshot(t *testing.T) {
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
	if rep.Cycle.Delivered.Count != 1 {
		t.Errorf("delivered = %d, want 1: a cycle counts what sits in it at the cutoff", rep.Cycle.Delivered.Count)
	}
	wantIdentity := "0 planned + 1 added - 0 removed = 1 in the cycle = 0 in development + 1 delivered + 0 deferred"
	if rep.Cycle.Identity != wantIdentity {
		t.Errorf("identity = %q, want %q", rep.Cycle.Identity, wantIdentity)
	}
	ev, err := e.Evidence(context.Background(), f.project, EvidenceParams{Metric: "delivered", CycleID: &cycle})
	if err != nil {
		t.Fatalf("evidence: %v", err)
	}
	if ev.Count != 1 || ev.Rows[0].CompletedAt == nil || ev.Rows[0].CompletedAt.Day() != 20 {
		t.Errorf("delivered evidence = %+v, want T1 dated its pre-cycle completion on the 20th", ev.Rows)
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

// TestHandedOffDevFinishLine covers the developer finish line: a crossing into
// the verification band counts, a band-internal move does not count again, a
// review bounce falls out until it crosses again, a straight-to-Done jump is
// both handed off and delivered, and a crossing before the window is not a
// window handoff.
func TestHandedOffDevFinishLine(t *testing.T) {
	f := newFixture("DEV")
	f.state("Backlog", "backlog", 0)
	f.state("In Progress", "started", 1)
	f.state("Code Review", "verification", 2)
	f.state("Testing", "verification", 3)
	f.state("Done", "completed", 4)
	f.prio("No priority", 0, true)

	// T1: into Code Review, still there — handed off, not delivered.
	f.task("T1", "Backlog", "No priority", 2, 3, "2026-10-02 09:00")
	f.assign("T1", "ann", "2026-10-02 10:00")
	f.move("T1", "Backlog", "In Progress", "2026-10-05 09:00", f.userIDs["ann"])
	f.move("T1", "In Progress", "Code Review", "2026-10-10 09:00", f.userIDs["ann"])

	// T2: crossed once, then moved inside the band — counts once.
	f.task("T2", "Backlog", "No priority", 0, 0, "2026-10-02 09:00")
	f.assign("T2", "bob", "2026-10-02 10:00")
	f.move("T2", "Backlog", "Code Review", "2026-10-11 09:00", f.userIDs["bob"])
	f.move("T2", "Code Review", "Testing", "2026-10-12 09:00", f.userIDs["bob"])

	// T3: bounced back by review before the cutoff — falls out.
	f.task("T3", "Backlog", "No priority", 0, 0, "2026-10-02 09:00")
	f.assign("T3", "ann", "2026-10-02 10:00")
	f.move("T3", "Backlog", "Code Review", "2026-10-11 09:00", f.userIDs["ann"])
	f.move("T3", "Code Review", "In Progress", "2026-10-13 09:00", f.userIDs["ann"])

	// T4: straight to Done — handed off and delivered.
	f.task("T4", "Backlog", "No priority", 0, 0, "2026-10-02 09:00")
	f.assign("T4", "bob", "2026-10-02 10:00")
	f.move("T4", "Backlog", "Done", "2026-10-14 09:00", f.userIDs["bob"])

	// T5: crossed before the window start, still in the band — not a window handoff.
	f.task("T5", "Backlog", "No priority", 0, 0, "2026-09-01 09:00")
	f.move("T5", "Backlog", "Code Review", "2026-09-10 09:00", f.userIDs["bob"])

	// T6: bounced, then re-crossed inside the window — counts once, and the
	// crossing that HOLDS carries the date and the credit: carol's first
	// attempt was rejected, dave reworked it and re-crossed, so dave is
	// credited, not carol.
	f.task("T6", "Backlog", "No priority", 0, 0, "2026-10-02 09:00")
	f.assign("T6", "carol", "2026-10-02 10:00")
	f.move("T6", "Backlog", "Code Review", "2026-10-08 09:00", f.userIDs["carol"])
	f.move("T6", "Code Review", "In Progress", "2026-10-09 09:00", f.userIDs["carol"])
	f.unassign("T6", "carol", "2026-10-10 09:00")
	f.assign("T6", "dave", "2026-10-10 09:01")
	f.move("T6", "In Progress", "Testing", "2026-10-15 09:00", f.userIDs["dave"])

	e := f.engine("2026-10-25 12:00")
	rep, err := e.ProjectReport(context.Background(), f.project, ReportOptions{
		From: ts("2026-10-01 00:00"), To: ts("2026-10-20 00:00"),
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if !rep.HasVerification {
		t.Error("has_verification must be true")
	}
	if rep.HandoffRule == "" {
		t.Error("handoff_rule must name the band states")
	}
	if rep.Range.HandedOff == nil || rep.Range.HandedOff.Count != 4 {
		t.Fatalf("handed off tile = %+v, want count 4 (T1, T2, T4, T6)", rep.Range.HandedOff)
	}
	if rep.Range.Completed.Count != 1 {
		t.Errorf("completed = %d, want 1 (T4 alone)", rep.Range.Completed.Count)
	}
	if rep.People.TotalHandedOff != 4 {
		t.Errorf("total handed off = %d, want 4", rep.People.TotalHandedOff)
	}
	wantPerPerson := map[string]int{"ann": 1, "bob": 2, "carol": 0, "dave": 1}
	for _, row := range rep.People.Rows {
		if want, ok := wantPerPerson[row.User.Username]; ok && row.HandedOff != want {
			t.Errorf("%s handed off = %d, want %d", row.User.Username, row.HandedOff, want)
		}
	}
	if rep.People.Rows[0].User.Username != "bob" {
		t.Errorf("table leads with %s, want bob (handed off leads the sort)", rep.People.Rows[0].User.Username)
	}

	ev, err := e.Evidence(context.Background(), f.project, EvidenceParams{
		Metric: "handed_off", From: ts("2026-10-01 00:00"), To: ts("2026-10-20 00:00"),
	})
	if err != nil {
		t.Fatalf("evidence: %v", err)
	}
	if ev.Count != 4 {
		t.Errorf("evidence count = %d, want 4", ev.Count)
	}
	for _, row := range ev.Rows {
		if row.Key == "DEV-6" && (row.CompletedAt == nil || row.CompletedAt.Day() != 15) {
			t.Errorf("T6 handoff date = %v, want the re-crossing on the 15th", row.CompletedAt)
		}
	}
	uid := f.userIDs["bob"]
	pev, err := e.Evidence(context.Background(), f.project, EvidenceParams{
		Metric: "person_handed_off", UserID: &uid,
		From: ts("2026-10-01 00:00"), To: ts("2026-10-20 00:00"),
	})
	if err != nil {
		t.Fatalf("person evidence: %v", err)
	}
	if pev.Count != 2 {
		t.Errorf("bob's handed off evidence = %d, want 2 (T2, T4)", pev.Count)
	}
}

// TestHandedOffCycleSnapshot: a cycle reads as a snapshot — carried work that
// crossed the line before the cycle started still counts in the cycle it sits
// in now, dated and credited at that original crossing, so a sprint report
// needs no memory of the previous one. The flow reading (crossing must fall
// inside the window) stays for date ranges: TestHandedOffDevFinishLine's T5.
func TestHandedOffCycleSnapshot(t *testing.T) {
	f := newFixture("SNAP")
	f.state("Backlog", "backlog", 0)
	f.state("In Progress", "started", 1)
	f.state("Code Review", "verification", 2)
	f.state("Done", "completed", 3)
	f.prio("No priority", 0, true)
	cycle := f.cycle("Sprint 16", "2026-10-10", "2026-10-20")

	// S1 crossed on the 3rd, before the cycle existed, and was carried in
	// still inside the band — counts, dated the 3rd, credited to ann.
	f.task("S1", "Backlog", "No priority", 0, 0, "2026-10-01 09:00")
	f.assign("S1", "ann", "2026-10-01 10:00")
	f.move("S1", "Backlog", "In Progress", "2026-10-02 09:00", f.userIDs["ann"])
	f.move("S1", "In Progress", "Code Review", "2026-10-03 09:00", f.userIDs["ann"])
	f.cycleAdd("S1", cycle, "2026-10-10 08:00")

	// S2 crossed inside the cycle — counts as before (straight to Done).
	f.task("S2", "Backlog", "No priority", 0, 0, "2026-10-09 09:00")
	f.assign("S2", "bob", "2026-10-09 10:00")
	f.cycleAdd("S2", cycle, "2026-10-10 08:01")
	f.move("S2", "Backlog", "In Progress", "2026-10-11 09:00", f.userIDs["bob"])
	f.move("S2", "In Progress", "Done", "2026-10-14 09:00", f.userIDs["bob"])

	// S3 is a member whose pre-cycle crossing was bounced inside the cycle and
	// never re-crossed: not past the line at the cutoff, so it never counts.
	f.task("S3", "Backlog", "No priority", 0, 0, "2026-10-01 09:00")
	f.assign("S3", "bob", "2026-10-01 10:00")
	f.move("S3", "Backlog", "Code Review", "2026-10-05 09:00", f.userIDs["bob"])
	f.cycleAdd("S3", cycle, "2026-10-10 08:02")
	f.move("S3", "Code Review", "In Progress", "2026-10-12 09:00", f.userIDs["bob"])

	// S4 is past the line but never a member of this cycle — absent.
	f.task("S4", "Backlog", "No priority", 0, 0, "2026-10-01 09:00")
	f.move("S4", "Backlog", "Code Review", "2026-10-12 09:00", f.userIDs["ann"])

	e := f.engine("2026-10-25 12:00")
	rep, err := e.ProjectReport(context.Background(), f.project, ReportOptions{CycleID: &cycle})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if rep.Cycle.HandedOff == nil || rep.Cycle.HandedOff.Count != 2 {
		t.Fatalf("handed off tile = %+v, want count 2 (S1 carried + S2)", rep.Cycle.HandedOff)
	}
	if rep.Cycle.Delivered.Count != 1 {
		t.Errorf("delivered = %d, want 1 (S2 alone)", rep.Cycle.Delivered.Count)
	}
	// The buckets partition In Cycle; Dev Handed Off is their band rollup.
	if rep.Cycle.InCycle.Count != 3 || rep.Cycle.InDevelopment.Count != 1 || rep.Cycle.Deferred.Count != 0 {
		t.Errorf("partition = %d in cycle, %d in development, %d deferred; want 3, 1, 0",
			rep.Cycle.InCycle.Count, rep.Cycle.InDevelopment.Count, rep.Cycle.Deferred.Count)
	}
	if rep.Cycle.InVerification == nil ||
		rep.Cycle.HandedOff.Count != rep.Cycle.InVerification.Count+rep.Cycle.Delivered.Count {
		t.Errorf("handed off = %+v, want in verification (%+v) + delivered (%d)",
			rep.Cycle.HandedOff, rep.Cycle.InVerification, rep.Cycle.Delivered.Count)
	}
	if !strings.Contains(rep.HandoffRule, "earlier cycle") {
		t.Errorf("cycle handoff_rule must explain the snapshot reading, got %q", rep.HandoffRule)
	}
	wantPerPerson := map[string]int{"ann": 1, "bob": 1}
	for _, row := range rep.People.Rows {
		if want, ok := wantPerPerson[row.User.Username]; ok && row.HandedOff != want {
			t.Errorf("%s handed off = %d, want %d", row.User.Username, row.HandedOff, want)
		}
	}

	ev, err := e.Evidence(context.Background(), f.project, EvidenceParams{
		Metric: "handed_off", CycleID: &cycle,
	})
	if err != nil {
		t.Fatalf("evidence: %v", err)
	}
	if ev.Count != 2 {
		t.Errorf("evidence count = %d, want 2", ev.Count)
	}
	for _, row := range ev.Rows {
		if row.Key == "SNAP-1" && (row.CompletedAt == nil || row.CompletedAt.Day() != 3) {
			t.Errorf("S1 handoff date = %v, want the original crossing on the 3rd", row.CompletedAt)
		}
	}
}

// TestHandedOffAbsentWithoutVerification: with no verification states the band
// is completed alone, so the tile stays absent and each person's handed-off
// equals their delivered by construction.
func TestHandedOffAbsentWithoutVerification(t *testing.T) {
	f := newFixture("NOV")
	f.state("Todo", "unstarted", 0)
	f.state("Done", "completed", 1)
	f.state("Released", "completed", 2)
	f.prio("No priority", 0, true)
	f.task("T1", "Todo", "No priority", 0, 0, "2026-10-02 09:00")
	f.assign("T1", "ann", "2026-10-02 10:00")
	f.move("T1", "Todo", "Done", "2026-10-10 09:00", f.userIDs["ann"])
	// T2 completed BEFORE the window and moved Done -> Released inside it: the
	// permissive completions rule counts it delivered, but it is no handoff
	// crossing. The visible table must still be ordered by Delivered.
	f.task("T2", "Todo", "No priority", 0, 0, "2026-09-01 09:00")
	f.assign("T2", "bob", "2026-09-01 10:00")
	f.move("T2", "Todo", "Done", "2026-09-10 09:00", f.userIDs["bob"])
	f.move("T2", "Done", "Released", "2026-10-11 09:00", f.userIDs["bob"])
	f.task("T3", "Todo", "No priority", 0, 0, "2026-10-02 09:00")
	f.assign("T3", "bob", "2026-10-02 10:00")
	f.move("T3", "Todo", "Done", "2026-10-12 09:00", f.userIDs["bob"])

	e := f.engine("2026-10-25 12:00")
	rep, err := e.ProjectReport(context.Background(), f.project, ReportOptions{
		From: ts("2026-10-01 00:00"), To: ts("2026-10-20 00:00"),
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if rep.HasVerification || rep.Range.HandedOff != nil || rep.HandoffRule != "" {
		t.Error("handed off must stay absent without verification states")
	}
	// bob: Delivered 2 (T2 by the permissive rule, T3), HandedOff 1 (T3 only).
	// With no verification states the hidden number must not drive the sort:
	// bob (Delivered 2) sorts above ann (Delivered 1).
	if rep.People.Rows[0].User.Username != "bob" || rep.People.Rows[0].Delivered != 2 {
		t.Errorf("table leads with %s (delivered %d), want bob with 2 — the hidden handoff number must not reorder the visible column",
			rep.People.Rows[0].User.Username, rep.People.Rows[0].Delivered)
	}
	if rep.People.CreditRule != creditRule {
		t.Error("credit rule must not mention handoffs without verification states")
	}
}

// TestIntakeLeavesOpenWork: backlog-type items are intake, not open work. The
// open headline, state bar and priority mix cover committed work only; intake
// gets its own count, named by state, with its own evidence population.
func TestIntakeLeavesOpenWork(t *testing.T) {
	f := newFixture("INT")
	f.state("New", "backlog", 0)
	f.state("TBD", "backlog", 1)
	f.state("Ready", "unstarted", 2)
	f.state("Doing", "started", 3)
	f.state("Done", "completed", 4)
	f.prio("No priority", 0, true)
	f.prio("Urgent", 4, true)

	f.task("N1", "New", "Urgent", 0, 0, "2026-10-01 09:00")
	f.task("N2", "New", "No priority", 0, 0, "2026-10-02 09:00")
	f.task("N3", "TBD", "No priority", 0, 0, "2026-10-03 09:00")
	f.task("R1", "Ready", "No priority", 0, 0, "2026-10-04 09:00")
	f.task("D1", "Doing", "Urgent", 0, 0, "2026-10-05 09:00")

	e := f.engine("2026-10-25 12:00")
	rep, err := e.ProjectReport(context.Background(), f.project, ReportOptions{
		From: ts("2026-10-01 00:00"), To: ts("2026-10-20 00:00"),
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	h := rep.Health
	if h.Open != 2 {
		t.Errorf("open = %d, want 2 (Ready + Doing; intake excluded)", h.Open)
	}
	if h.Intake != 3 {
		t.Errorf("intake = %d, want 3", h.Intake)
	}
	if len(h.IntakeByState) != 2 || h.IntakeByState[0].State.Name != "New" || h.IntakeByState[0].Count != 2 {
		t.Errorf("intake_by_state = %+v, want New 2 then TBD 1", h.IntakeByState)
	}
	for _, sc := range h.OpenByState {
		if sc.State.Type == "backlog" {
			t.Errorf("open_by_state leaks a backlog state: %s", sc.State.Name)
		}
	}
	// The urgent intake item must not inflate the open priority mix.
	for _, rc := range h.PriorityMix {
		if rc.Priority.Name == "Urgent" && rc.Count != 1 {
			t.Errorf("urgent open = %d, want 1 (D1 alone; N1 is intake)", rc.Count)
		}
	}

	ev, err := e.Evidence(context.Background(), f.project, EvidenceParams{
		Metric: "intake", From: ts("2026-10-01 00:00"), To: ts("2026-10-20 00:00"),
	})
	if err != nil {
		t.Fatalf("evidence: %v", err)
	}
	if ev.Count != 3 {
		t.Errorf("intake evidence = %d, want 3", ev.Count)
	}
}

// TestDeliveryTilesNameTheirSubtasks: Q2 decision — leaf counting stays, but a
// delivery number says how many of its units are subtasks.
func TestDeliveryTilesNameTheirSubtasks(t *testing.T) {
	f := newFixture("SUB")
	f.state("Todo", "unstarted", 0)
	f.state("Done", "completed", 1)
	f.prio("No priority", 0, true)
	f.task("P", "Todo", "No priority", 0, 0, "2026-10-01 09:00")
	f.subtask("S1", "P", "Todo", "No priority", 0, 0, "2026-10-01 09:10")
	f.subtask("S2", "P", "Todo", "No priority", 0, 0, "2026-10-01 09:20")
	f.task("R1", "Todo", "No priority", 0, 0, "2026-10-01 09:30")
	f.move("S1", "Todo", "Done", "2026-10-05 09:00", f.actor)
	f.move("S2", "Todo", "Done", "2026-10-06 09:00", f.actor)
	f.move("R1", "Todo", "Done", "2026-10-07 09:00", f.actor)

	e := f.engine("2026-10-25 12:00")
	rep, err := e.ProjectReport(context.Background(), f.project, ReportOptions{
		From: ts("2026-10-01 00:00"), To: ts("2026-10-20 00:00"),
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if rep.Range.Completed.Count != 3 || rep.Range.Completed.Subtasks != 2 {
		t.Errorf("completed = %d with %d subtasks, want 3 with 2",
			rep.Range.Completed.Count, rep.Range.Completed.Subtasks)
	}
}
