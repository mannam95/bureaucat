// Mirrors internal/insights/types.go. Every number on the insights page
// carries its scope and an evidence route; unknown values are explicit, never
// zero.

export interface InsightsScope {
  project_key?: string;
  project_name?: string;
  cycle_id?: string;
  cycle_title?: string;
  from: string;
  to: string;
  cutoff: string;
  timezone: string;
  generated_at: string;
  counting_rule: string;
}

export interface InsightsUser {
  id: string;
  username: string;
  first_name: string;
  last_name: string;
  avatar_url?: string;
}

export interface InsightsState {
  id: string;
  name: string;
  type: string;
  color: string;
}

export interface InsightsPriority {
  id: string;
  name: string;
  rank: number;
  color: string;
  active: boolean;
}

export interface ComplexitySum {
  sum: number;
  assessed: number;
  of: number;
  not_assessed: number;
}

export interface MetricTile {
  metric: string;
  count: number;
  cx?: ComplexitySum;
}

export interface DayPoint {
  date: string;
  scope: number;
  completed_cum: number;
  completed: number;
  added: number;
  removed: number;
  cancelled: number;
  weekend: boolean;
}

export interface BandCount {
  band: string;
  count: number;
}

export interface PersonRow {
  user: InsightsUser;
  delivered: number;
  shared: number;
  cx?: ComplexitySum;
  top_rank_handled: number;
  open_at_end: number;
  comments: number;
  created: number;
  has_activity: boolean;
}

export interface PersonTable {
  rows: PersonRow[];
  total_delivered: number;
  total_cx?: ComplexitySum;
  credit_rule: string;
}

export interface AgingRow {
  key: string;
  title: string;
  state: InsightsState;
  days: number;
  assignee?: InsightsUser;
}

export interface DueRow {
  key: string;
  title: string;
  due: string;
  late_days: number;
  assignee?: InsightsUser;
}

export interface StateCount {
  state: InsightsState;
  count: number;
}

export interface RankCount {
  priority: InsightsPriority;
  count: number;
}

export interface WeekPoint {
  week_start: string;
  created: number;
  completed: number;
}

export interface InsightsHealth {
  open: number;
  in_started: number;
  blocked: number;
  overdue: number;
  open_by_state: StateCount[];
  priority_mix: RankCount[];
  aging: AgingRow[];
  due: DueRow[];
  weekly: WeekPoint[];
}

export interface CycleSection {
  delivered: MetricTile;
  original_plan_delivered: MetricTile;
  planned_at_start: MetricTile;
  added: MetricTile;
  removed: MetricTile;
  carried_over: MetricTile;
  cancelled?: MetricTile;
  archived?: MetricTile;
  at_close: number;
  identity: string;
  days: DayPoint[];
  bands: BandCount[];
}

export interface RangeSection {
  completed: MetricTile;
  created: MetricTile;
  cancelled?: MetricTile;
  days: DayPoint[];
  bands: BandCount[];
  has_cycles: boolean;
}

export interface InsightsCycle {
  id: string;
  title: string;
  start_date: string;
  end_date: string;
  status: "upcoming" | "active" | "completed";
}

export interface ProjectInsights {
  scope: InsightsScope;
  cycle?: CycleSection;
  range?: RangeSection;
  people: PersonTable;
  health: InsightsHealth;
  cycles: InsightsCycle[];
  has_areas: boolean;
  has_labels: boolean;
}

export interface PortfolioRow {
  project_key: string;
  project_name: string;
  icon_url?: string;
  active_cycle?: InsightsCycle;
  has_cycles: boolean;
  open: number;
  archived: number;
  top_rank_open: number;
  top_rank_name: string;
  overdue: number;
  blocked: number;
  completed: number;
  cancelled: number;
  created: number;
  weekly: WeekPoint[];
  trend: "filling" | "draining" | "steady" | "no_history";
  oldest?: AgingRow;
  active_people: number;
  members: number;
  attention: boolean;
  attention_why?: string;
  history_days: number;
  low_history: boolean;
}

export interface InsightsPortfolio {
  scope: InsightsScope;
  rows: PortfolioRow[];
  rule: string;
  delivered: {
    total: number;
    cancelled: number;
    cx?: ComplexitySum;
    by_project: { project_key: string; count: number }[];
  };
}

export interface UserCount {
  user: InsightsUser;
  count: number;
}

export interface PersonInsights {
  scope: InsightsScope;
  user: InsightsUser;
  delivered: MetricTile;
  top_rank_handled: number;
  open_now: number;
  comments: number;
  created: number;
  weekly: WeekPoint[];
  by_cycle: { cycle: InsightsCycle; count: number }[];
  bands: BandCount[];
  worked_with: UserCount[];
  credit_rule: string;
}

export interface EvidenceRow {
  key: string;
  title: string;
  parent_key?: string;
  parent_title?: string;
  state: InsightsState;
  priority?: InsightsPriority;
  complexity: number;
  assessed: boolean;
  completed_at?: string;
  marked_complete_by?: InsightsUser;
  credits?: InsightsUser[];
  assignees?: InsightsUser[];
  due?: string;
  entered_cycle?: string;
  age_days?: number;
}

export interface InsightsEvidence {
  scope: InsightsScope;
  metric: string;
  title: string;
  count: number;
  cx?: ComplexitySum;
  how_computed: string;
  source_events: string;
  chips: string[];
  rows: EvidenceRow[];
  task_list_query?: string;
}

// EvidenceQuery describes which population to open: the metric plus its
// parameters; serialized into the URL so a copied link reproduces the drawer.
export interface EvidenceQuery {
  metric: string;
  user_id?: string;
  state_id?: string;
  rank?: number;
  day?: string;
}
