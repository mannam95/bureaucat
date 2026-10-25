// Package insights reconstructs delivery reporting from the activity log.
//
// Everything is computed on state TYPES and priority RANKS, never on names:
// states, labels, areas and priorities are defined per project, so names and
// colors are presentation that rides along with each number. Counting follows
// the consolidated direction: leaf items (a parent with subtasks counts
// through its subtasks), cancelled is never delivery, archived is its own
// bucket, unknown estimates are never zero, and a completed item credits its
// Contributors, falling back to its assignees at completion time.
package insights

import (
	"time"

	"github.com/google/uuid"
)

// Scope is repeated on every payload (and in the evidence drawer) so each
// number states what it was computed over.
type Scope struct {
	ProjectKey  string     `json:"project_key,omitempty"`
	ProjectName string     `json:"project_name,omitempty"`
	CycleID     *uuid.UUID `json:"cycle_id,omitempty"`
	CycleTitle  string     `json:"cycle_title,omitempty"`
	From        time.Time  `json:"from"`
	To          time.Time  `json:"to"` // inclusive end of the window's last day
	Cutoff      time.Time  `json:"cutoff"`
	Timezone    string     `json:"timezone"`
	GeneratedAt time.Time  `json:"generated_at"`
	// CountingRule is the plain-language line shown under the scope.
	CountingRule string `json:"counting_rule"`
}

// UserRef identifies a person on rows and tables.
type UserRef struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
}

// StateRef carries a project's own state vocabulary next to a type-computed
// number.
type StateRef struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Type  string    `json:"type"`
	Color string    `json:"color"`
}

// PriorityRef mirrors StateRef for the rank ladder.
type PriorityRef struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Rank   int       `json:"rank"`
	Color  string    `json:"color"`
	Active bool      `json:"active"`
}

// ComplexitySum is a sum that always carries its coverage; the two are
// meaningless apart ("142 across 20 of 25 assessed").
type ComplexitySum struct {
	Sum         int `json:"sum"`
	Assessed    int `json:"assessed"`
	Of          int `json:"of"`
	NotAssessed int `json:"not_assessed"`
}

// Tile is one scope/outcome number with its evidence route. Metric plus the
// scope parameters reproduce its exact population via the evidence endpoint.
type Tile struct {
	Metric string         `json:"metric"`
	Count  int            `json:"count"`
	Cx     *ComplexitySum `json:"cx,omitempty"`
}

// DayPoint is one day of the scope-vs-completed series plus the day strip.
type DayPoint struct {
	Date         string `json:"date"` // YYYY-MM-DD in the reporting timezone
	Scope        int    `json:"scope"`
	CompletedCum int    `json:"completed_cum"`
	Completed    int    `json:"completed"`
	Added        int    `json:"added"`
	Removed      int    `json:"removed"`
	Cancelled    int    `json:"cancelled"`
	Weekend      bool   `json:"weekend"`
}

// BandCount is one row of the delivered-by-complexity-band panel.
type BandCount struct {
	Band  string `json:"band"` // "1-4", "5-9", "10-16", "17+", "not_assessed"
	Count int    `json:"count"`
}

// PersonRow is one member in "what did each person deliver".
type PersonRow struct {
	User        UserRef        `json:"user"`
	Delivered   int            `json:"delivered"`
	Shared      int            `json:"shared"` // of Delivered, items credited to >1 person
	Cx          *ComplexitySum `json:"cx,omitempty"`
	TopRank     int            `json:"top_rank_handled"` // delivered at the project's top two ranks
	OpenAtEnd   int            `json:"open_at_end"`
	Comments    int            `json:"comments"`
	Created     int            `json:"created"`
	HasActivity bool           `json:"has_activity"`
}

// PersonTable adds the de-duplicated totals and the explanation the footnote
// needs (rows may sum above the total because of shared items).
type PersonTable struct {
	Rows           []PersonRow    `json:"rows"`
	TotalDelivered int            `json:"total_delivered"` // distinct items
	TotalCx        *ComplexitySum `json:"total_cx,omitempty"`
	CreditRule     string         `json:"credit_rule"`
}

// AgingRow is one long-unchanged started-type item; the state name travels
// with it so "41 d in On hold" reads as parked, not stuck.
type AgingRow struct {
	Key      string   `json:"key"`
	Title    string   `json:"title"`
	State    StateRef `json:"state"`
	Days     int      `json:"days"`
	Assignee *UserRef `json:"assignee,omitempty"`
}

// DueRow is one overdue or due-soon item.
type DueRow struct {
	Key      string    `json:"key"`
	Title    string    `json:"title"`
	Due      time.Time `json:"due"`
	LateDays int       `json:"late_days"` // negative = due in N days
	Assignee *UserRef  `json:"assignee,omitempty"`
}

// StateCount pairs a project state with how many open leaf items sit in it,
// ordered type -> position -> name.
type StateCount struct {
	State StateRef `json:"state"`
	Count int      `json:"count"`
}

// RankCount is the priority mix of open work by rank, with the project's own
// label. Rank 0 is the explicit unranked bucket.
type RankCount struct {
	Priority PriorityRef `json:"priority"`
	Count    int         `json:"count"`
}

// WeekPoint is one week of created vs completed.
type WeekPoint struct {
	WeekStart string `json:"week_start"` // YYYY-MM-DD (Monday, reporting timezone)
	Created   int    `json:"created"`
	Completed int    `json:"completed"`
}

// Health is "is the project healthy right now" — a snapshot at generation
// time, independent of the selected window.
type Health struct {
	Open        int          `json:"open"`
	InStarted   int          `json:"in_started"`
	Blocked     int          `json:"blocked"`
	Overdue     int          `json:"overdue"`
	OpenByState []StateCount `json:"open_by_state"`
	PriorityMix []RankCount  `json:"priority_mix"`
	Aging       []AgingRow   `json:"aging"`
	Due         []DueRow     `json:"due"`
	Weekly      []WeekPoint  `json:"weekly"`
}

// CycleSection is question 1 when the report is cycle-scoped.
type CycleSection struct {
	Delivered      Tile `json:"delivered"`
	OriginalPlan   Tile `json:"original_plan_delivered"` // delivered items that were planned at start
	PlannedAtStart Tile `json:"planned_at_start"`
	Added          Tile `json:"added"`
	Removed        Tile `json:"removed"`
	CarriedOver    Tile `json:"carried_over"`
	// Cancelled/Archived are nil when the project defines no state of that
	// type — the tile is absent, not zero.
	Cancelled *Tile `json:"cancelled,omitempty"`
	Archived  *Tile `json:"archived,omitempty"`
	AtClose   int   `json:"at_close"`
	// Identity is the reconciliation line: planned + added - removed = at
	// close = completed + open + cancelled + archived.
	Identity string      `json:"identity"`
	Days     []DayPoint  `json:"days"`
	Bands    []BandCount `json:"bands"`
}

// RangeSection is question 1 when the report is date-range-scoped (the
// majority case: projects without cycles).
type RangeSection struct {
	Completed Tile        `json:"completed"`
	Created   Tile        `json:"created"`
	Cancelled *Tile       `json:"cancelled,omitempty"`
	Days      []DayPoint  `json:"days"`
	Bands     []BandCount `json:"bands"`
	// HasCycles says whether the project could run cycle reports at all; false
	// renders the quiet "create a cycle" hint.
	HasCycles bool `json:"has_cycles"`
}

// CycleRef lists a selectable cycle in the filter bar.
type CycleRef struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	StartDate string    `json:"start_date"`
	EndDate   string    `json:"end_date"`
	Status    string    `json:"status"` // upcoming | active | completed
}

// ProjectReport is the full project answer: cycle or range section, people,
// health.
type ProjectReport struct {
	Scope  Scope         `json:"scope"`
	Cycle  *CycleSection `json:"cycle,omitempty"`
	Range  *RangeSection `json:"range,omitempty"`
	People PersonTable   `json:"people"`
	Health Health        `json:"health"`
	Cycles []CycleRef    `json:"cycles"`
	// Vocabularies the frontend needs for panels that are conditional on their
	// existence.
	HasAreas  bool `json:"has_areas"`
	HasLabels bool `json:"has_labels"`
}

// PortfolioRow is one project in the attention table.
type PortfolioRow struct {
	ProjectKey   string      `json:"project_key"`
	ProjectName  string      `json:"project_name"`
	IconURL      *string     `json:"icon_url,omitempty"`
	ActiveCycle  *CycleRef   `json:"active_cycle,omitempty"`
	HasCycles    bool        `json:"has_cycles"`
	Open         int         `json:"open"`
	Archived     int         `json:"archived"`
	TopRankOpen  int         `json:"top_rank_open"`
	TopRankName  string      `json:"top_rank_name"`
	Overdue      int         `json:"overdue"`
	Blocked      int         `json:"blocked"`
	Completed    int         `json:"completed"`
	Cancelled    int         `json:"cancelled"`
	Created      int         `json:"created"`
	Weekly       []WeekPoint `json:"weekly"`
	Trend        string      `json:"trend"` // filling | draining | steady | no_history
	Oldest       *AgingRow   `json:"oldest,omitempty"`
	ActivePeople int         `json:"active_people"`
	Members      int         `json:"members"`
	Attention    bool        `json:"attention"`
	AttentionWhy string      `json:"attention_why,omitempty"`
	HistoryDays  int         `json:"history_days"`
	LowHistory   bool        `json:"low_history"`
}

// Portfolio is question 0 across the viewer's projects.
type Portfolio struct {
	Scope     Scope          `json:"scope"`
	Rows      []PortfolioRow `json:"rows"`
	Rule      string         `json:"rule"` // the attention rule, shown, not hidden
	Delivered struct {
		Total     int            `json:"total"`
		Cancelled int            `json:"cancelled"`
		Cx        *ComplexitySum `json:"cx,omitempty"`
		ByProject []struct {
			ProjectKey string `json:"project_key"`
			Count      int    `json:"count"`
		} `json:"by_project"`
	} `json:"delivered"`
}

// PersonReport is one member's slice of one project.
type PersonReport struct {
	Scope     Scope       `json:"scope"`
	User      UserRef     `json:"user"`
	Delivered Tile        `json:"delivered"`
	TopRank   int         `json:"top_rank_handled"`
	OpenNow   int         `json:"open_now"`
	Comments  int         `json:"comments"`
	Created   int         `json:"created"`
	Weekly    []WeekPoint `json:"weekly"`
	ByCycle   []struct {
		Cycle CycleRef `json:"cycle"`
		Count int      `json:"count"`
	} `json:"by_cycle"`
	CreditRule string `json:"credit_rule"`
}

// EvidenceRow is one leaf item inside an evidence population. Optional fields
// render only where they apply.
type EvidenceRow struct {
	Key              string       `json:"key"`
	Title            string       `json:"title"`
	ParentKey        string       `json:"parent_key,omitempty"`
	ParentTitle      string       `json:"parent_title,omitempty"`
	State            StateRef     `json:"state"`
	Priority         *PriorityRef `json:"priority,omitempty"`
	Complexity       int          `json:"complexity"`
	Assessed         bool         `json:"assessed"`
	CompletedAt      *time.Time   `json:"completed_at,omitempty"`
	MarkedCompleteBy *UserRef     `json:"marked_complete_by,omitempty"`
	Credits          []UserRef    `json:"credits,omitempty"`
	Assignees        []UserRef    `json:"assignees,omitempty"`
	Due              *time.Time   `json:"due,omitempty"`
	EnteredCycle     string       `json:"entered_cycle,omitempty"` // "planned" or "added YYYY-MM-DD"
	AgeDays          int          `json:"age_days,omitempty"`
}

// Evidence is the drawer: scope, plain-language computation, the population
// definition as chips, and the exact rows. CSV serializes Rows verbatim.
type Evidence struct {
	Scope        Scope          `json:"scope"`
	Metric       string         `json:"metric"`
	Title        string         `json:"title"`
	Count        int            `json:"count"`
	Cx           *ComplexitySum `json:"cx,omitempty"`
	HowComputed  string         `json:"how_computed"`
	SourceEvents string         `json:"source_events"`
	Chips        []string       `json:"chips"`
	Rows         []EvidenceRow  `json:"rows"`
	// TaskListQuery is set only when the population is expressible as task
	// list filters; empty means link/CSV only. No fake equivalence.
	TaskListQuery string `json:"task_list_query,omitempty"`
}
