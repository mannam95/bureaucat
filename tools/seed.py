#!/usr/bin/env python3
"""Seed a local Bureaucat instance with rich, realistic demo data — including
a real HISTORY, so the /insights reporting has something true to reconstruct.

Creates: an admin (demo@gmail.com / password), a handful of users (plus one
deactivated account), a workspace, three projects, and — per project — labels,
task templates, pages, epics (modules), saved views and a pinned default view.

What is new versus the old seed: tasks are no longer conjured directly into
their final states. Every task is created through the API and then WALKED
through its lifecycle (state changes, assignments, hand-overs, cycle
membership, comments) exactly like production traffic, which exercises the
activity log, the contributor auto-rule and notifications. Each logical event
is then retimed to a planned moment in the past (insights replays the activity
log, so history must actually be historical), the per-task checksum chains are
recomputed, and the seed verifies a sample of chains through the API before
declaring success.

The scripted story covers, per sprint project: two finished sprints and an
active one; planned/added/descoped/carried scope; delivery through subtasks; a
build-then-verify hand-over (auto contributor credit); a shared-credit item;
unassessed estimates; a cancelled outcome; blockers; overdue and due-soon
dates; and completions spread over many days so the daily and weekly charts
have shape. The third project runs without cycles (the date-range report).

Idempotent: it RESETS all app data first (TRUNCATE via the postgres container),
then rebuilds, so `make seed` always yields the same clean dataset.

Stdlib only. Configure via env: BUREAUCAT_URL, PG_CONTAINER, SEED, NUM_USERS.
"""
import json, os, random, subprocess, urllib.request, urllib.error
from datetime import date, datetime, timedelta

BASE = os.environ.get("BUREAUCAT_URL", "http://localhost:1341") + "/api/v1"
PG_CONTAINER = os.environ.get("PG_CONTAINER", "bureaucat-postgres-1")
PG_USER = os.environ.get("PG_USER", "bureaucat")
PG_DB = os.environ.get("PG_DB", "bureaucat")

STRONG_PW = "Passw0rd!"  # prod-safe default (upper + number + special char)

def _demo_password():
    """Single demo password: env override -> DEMO_USER_PASSWORD in .env -> strong
    default. One strong password works in both dev and prod (prod enforces a policy)."""
    v = os.environ.get("DEMO_USER_PASSWORD") or os.environ.get("DEMO_PASSWORD")
    if v:
        return v
    try:
        for line in open(os.environ.get("ENV_FILE", ".env")):
            s = line.strip()
            if s.startswith("DEMO_USER_PASSWORD="):
                return s.split("=", 1)[1].strip().strip('"').strip("'")
    except OSError:
        pass
    return STRONG_PW

ADMIN_EMAIL = os.environ.get("DEMO_EMAIL", "demo@gmail.com")
ADMIN_PW = _demo_password()
USER_PW = ADMIN_PW

random.seed(int(os.environ.get("SEED", "7")))
NUM_USERS = int(os.environ.get("NUM_USERS", "8"))

# ---------------------------------------------------------------- data pools
FIRST = ["Alice", "Bob", "Carla", "Diego", "Ellen", "Farah", "Grace", "Hiro",
         "Ivan", "Julia", "Kwame", "Lena", "Mateo", "Nina", "Omar", "Priya"]
LAST = ["Zhang", "Okafor", "Rossi", "Nguyen", "Schmidt", "Haddad", "Kim",
        "Silva", "Ivanov", "Brown", "Mensah", "Novak", "Garcia", "Patel"]
WORKSPACES = ["Northwind", "Acme", "Globex", "Umbrella", "Initech"]
EPICS = ["Authentication and SSO", "Billing and Invoicing", "Search and Discovery",
         "Notifications", "Dashboard and Analytics", "Onboarding", "Public API",
         "Performance", "Admin Console", "Reporting"]
MODULE_STATUSES = ["planned", "in_progress", "ongoing", "completed", "cancelled",
                   "backlog", "paused"]
LABELS = [("bug", "#EF4444"), ("feature", "#3B82F6"), ("tech-debt", "#F97316"),
          ("design", "#8B5CF6"), ("docs", "#10B981")]

COMMENTS = [
    "Picking this up today.",
    "I pushed a first draft, feedback welcome.",
    "Tested on staging, looks good so far.",
    "The design file was updated, please re-check the spacing.",
    "Rebased on main and resolved the conflicts.",
    "Adding a feature flag so we can roll this out gradually.",
    "QA found one edge case with empty states, fixing now.",
    "Done from my side, over to review.",
]

PAGES = [
    ("Team Handbook",
     "# Team Handbook\n\n"
     "How we work in this project.\n\n"
     "## Rituals\n\n"
     "- Standup: daily, async in the channel\n"
     "- Planning: first Monday of the sprint\n"
     "- Review and retro: last Friday of the sprint\n\n"
     "## Definitions\n\n"
     "**Done** means merged, deployed to staging, and verified by QA.\n"),
    ("Release Checklist",
     "# Release Checklist\n\n"
     "1. All sprint tasks in **Done** or moved out\n"
     "2. Changelog written and reviewed\n"
     "3. Migrations tested against a staging dump\n"
     "4. Feature flags default states confirmed\n"
     "5. Tag, deploy, and watch the dashboards for 30 minutes\n"),
]

TEMPLATES = [
    ("Bug report", "Bug: summary",
     "Steps to reproduce:\n1. \n2. \n\nExpected:\n\nActual:\n\nEnvironment:"),
    ("Feature request", "Feature: summary",
     "Problem:\n\nProposal:\n\nAcceptance criteria:\n- [ ] "),
]

# ---------------------------------------------------------------- http
TOKEN = [None]          # admin access token
USER_TOKENS = {}        # email -> access token (for acting as other users)

def api(method, path, body=None, auth=True, ignore=(), token=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(BASE + path, data=data, method=method)
    if body is not None:
        r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    elif auth and TOKEN[0]:
        r.add_header("Authorization", "Bearer " + TOKEN[0])
    try:
        with urllib.request.urlopen(r) as resp:
            b = resp.read()
            return json.loads(b) if b else {}
    except urllib.error.HTTPError as e:
        if e.code == 401 and auth and not token:
            signin()
            return api(method, path, body, auth, ignore)
        if e.code in ignore:
            return {"_status": e.code}
        raise RuntimeError(f"{method} {path} -> {e.code}: {e.read().decode('utf-8','replace')}")

def signin():
    TOKEN[0] = api("POST", "/signin",
                   {"identifier": ADMIN_EMAIL, "password": ADMIN_PW}, auth=False)["access_token"]

def user_token(email):
    """Access token for a seeded user, so actions carry different actors."""
    if email not in USER_TOKENS:
        USER_TOKENS[email] = api("POST", "/signin",
                                 {"identifier": email, "password": USER_PW},
                                 auth=False)["access_token"]
    return USER_TOKENS[email]

# ---------------------------------------------------------------- postgres
def psql(sql, quiet=True):
    """Run SQL in the postgres container; sql is piped via stdin so it can be big."""
    p = subprocess.run(
        ["docker", "exec", "-i", PG_CONTAINER, "psql", "-U", PG_USER, "-d", PG_DB,
         "-v", "ON_ERROR_STOP=1", "-q" if quiet else "-a", "-f", "-"],
        input=sql, capture_output=True, text=True)
    if p.returncode != 0:
        raise SystemExit(f"psql failed:\n{p.stderr.strip()}\n--- sql head ---\n{sql[:800]}")
    return p.stdout

def psql_rows(sql):
    """Run a SELECT and return rows split on the unit separator."""
    p = subprocess.run(
        ["docker", "exec", "-i", PG_CONTAINER, "psql", "-U", PG_USER, "-d", PG_DB,
         "-v", "ON_ERROR_STOP=1", "-At", "-F", "\x1f", "-f", "-"],
        input=sql, capture_output=True, text=True)
    if p.returncode != 0:
        raise SystemExit(f"psql query failed:\n{p.stderr.strip()}")
    return [line.split("\x1f") for line in p.stdout.splitlines() if line]

def reset():
    sql = "TRUNCATE users, workspaces RESTART IDENTITY CASCADE;"
    p = subprocess.run(["docker", "exec", PG_CONTAINER, "psql", "-U", PG_USER, "-d", PG_DB, "-c", sql],
                       capture_output=True, text=True)
    if p.returncode != 0:
        raise SystemExit(f"reset failed (is the stack up & migrated?):\n{p.stderr.strip()}")

# ---------------------------------------------------------------- retiming
# Every story action records how many activity rows (and comment rows) it
# wrote and the planned past moment they belong to. Rows are matched back by
# their per-task insertion order, retimed in one SQL pass, and the checksum
# chains recomputed afterwards (created_at is part of the chained hash).
PLAN_ROWS = {}      # task uuid -> [iso ts, ...] one per activity row, in order
PLAN_COMMENTS = {}  # task uuid -> [iso ts, ...] one per comment, in order
PLAN_CREATED = {}   # task uuid -> iso ts of task creation
PLAN_CYCLE_AT = {}  # task uuid -> iso ts of its last cycle entry

def T(days_ago, hhmm):
    return f"{(date.today() - timedelta(days=days_ago)).isoformat()}T{hhmm}:00+00:00"

def log_rows(tid, ts, n=1):
    # Every row gets a UNIQUE timestamp (one second apart within a call): the
    # verify endpoint walks the chain ordered by created_at alone, so ties
    # would make its order unspecified and verification nondeterministic.
    base = datetime.fromisoformat(ts)
    lst = PLAN_ROWS.setdefault(tid, [])
    for i in range(n):
        lst.append((base + timedelta(seconds=i)).isoformat())

def apply_plan():
    """Rewrite created_at for every planned activity/comment row, matched by
    per-task insertion order, plus the tasks' own timestamps."""
    # Safety: the plan must account for every activity row written so far. A
    # mismatch means some handler logged more (or fewer) rows than the story
    # recorded, and silent drift here would corrupt the timeline.
    actual = {tid: int(n) for tid, n in
              psql_rows("SELECT task_id::text, count(*) FROM activity_log GROUP BY task_id;")}
    bad = [f"{tid}: planned {len(stamps)} rows, found {actual.get(tid, 0)}"
           for tid, stamps in PLAN_ROWS.items() if actual.get(tid, 0) != len(stamps)]
    if bad:
        raise SystemExit("retiming plan mismatch:\n  " + "\n  ".join(bad[:10]))
    parts = []
    for tid, stamps in PLAN_ROWS.items():
        values = ",".join(f"({i + 1},'{ts}'::timestamptz)" for i, ts in enumerate(stamps))
        parts.append(f"""
WITH ordered AS (
  SELECT id, row_number() OVER (ORDER BY created_at ASC, id ASC) AS rn
  FROM activity_log WHERE task_id = '{tid}'
)
UPDATE activity_log a SET created_at = v.ts
FROM ordered o JOIN (VALUES {values}) AS v(rn, ts) ON v.rn = o.rn
WHERE a.id = o.id;""")
    for tid, stamps in PLAN_COMMENTS.items():
        values = ",".join(f"({i + 1},'{ts}'::timestamptz)" for i, ts in enumerate(stamps))
        parts.append(f"""
WITH ordered AS (
  SELECT id, row_number() OVER (ORDER BY created_at ASC, id ASC) AS rn
  FROM comments WHERE task_id = '{tid}'
)
UPDATE comments c SET created_at = v.ts, updated_at = v.ts
FROM ordered o JOIN (VALUES {values}) AS v(rn, ts) ON v.rn = o.rn
WHERE c.id = o.id;""")
    for tid, ts in PLAN_CREATED.items():
        parts.append(f"UPDATE tasks SET created_at='{ts}'::timestamptz WHERE id='{tid}';")
    for tid, ts in PLAN_CYCLE_AT.items():
        parts.append(f"UPDATE cycle_tasks SET added_at='{ts}'::timestamptz WHERE task_id='{tid}';")
    psql("BEGIN;" + "".join(parts) + "COMMIT;")

def go_json(value):
    """Byte-parity with Go's json.Marshal of an unmarshaled value: sorted keys,
    no spaces, HTML-escaped angle brackets and ampersands, raw UTF-8."""
    s = json.dumps(value, separators=(",", ":"), sort_keys=True, ensure_ascii=False)
    return s.replace("&", "\\u0026").replace("<", "\\u003c").replace(">", "\\u003e")

def rechecksum():
    """Recompute every task's activity checksum chain exactly like
    internal/activity does, then write the new sums back in one pass."""
    import hashlib
    rows = psql_rows("""
SELECT a.id, a.task_id, a.activity_type::text, a.actor_id, coalesce(a.field_name,''),
       encode(convert_to(coalesce(a.old_value::text,''),'UTF8'),'hex'),
       encode(convert_to(coalesce(a.new_value::text,''),'UTF8'),'hex'),
       to_char(a.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS'),
       to_char(a.created_at AT TIME ZONE 'UTC', 'US')
FROM activity_log a ORDER BY a.task_id, a.created_at ASC, a.id ASC;""")
    updates, prev_task, prev_sum = [], None, None
    seed_sum = "0" * 64
    for rid, task_id, typ, actor, field, old_hex, new_hex, ts_sec, ts_us in rows:
        if task_id != prev_task:
            prev_task, prev_sum = task_id, seed_sum
        def norm(hx):
            raw = bytes.fromhex(hx).decode("utf-8")
            if raw == "":
                return ""
            return go_json(json.loads(raw))
        frac = ts_us.rstrip("0")
        stamp = ts_sec + (("." + frac) if frac else "") + "Z"
        data = f"{prev_sum}|{task_id}|{typ}|{actor}|{field}|{norm(old_hex)}|{norm(new_hex)}|{stamp}"
        prev_sum = hashlib.sha256(data.encode("utf-8")).hexdigest()
        updates.append(f"UPDATE activity_log SET checksum='{prev_sum}' WHERE id='{rid}';")
    psql("BEGIN;" + "\n".join(updates) + "COMMIT;")
    return len(updates)

# ---------------------------------------------------------------- views
def pred(field, op, value=None):
    p = {"field": field, "op": op}
    if value is not None:
        p["value"] = value
    return {"predicate": p}

def seed_views(key, prio_by_rank, make_default):
    """A spread of saved views per project: shared and private, list and board,
    including one built on the watcher filter. Returns (created, default_slug)."""
    hot = [prio_by_rank[r] for r in (3, 4) if r in prio_by_rank]
    views = [
        ({"name": "Active work",
          "description": "Everything not finished, grouped by state.",
          "visibility": "shared", "group_by": "state", "sort_by": "updated_at",
          "sort_dir": "desc", "default_tab": "tasks",
          "filter_tree": {"children": [pred("state_type", "in", ["backlog", "unstarted", "started"])]}},
         True),
        ({"name": "Urgent and high priority",
          "description": "Only urgent/high tasks, hottest first.",
          "visibility": "shared", "group_by": "priority", "sort_by": "priority",
          "sort_dir": "desc", "default_tab": "tasks",
          "filter_tree": {"children": [pred("priority", "in", hot)]}},
         False),
        ({"name": "Due soon",
          "description": "Tasks due between today and the end of next week.",
          "visibility": "shared", "group_by": "state", "sort_by": "due_date",
          "sort_dir": "asc", "default_tab": "tasks",
          "filter_tree": {"children": [pred("due_date", "between", {"from": "today", "to": "next_week"})]}},
         False),
        ({"name": "Watched by me",
          "description": "Tasks you follow as a watcher.",
          "visibility": "shared", "group_by": "state", "sort_by": "updated_at",
          "sort_dir": "desc", "default_tab": "tasks",
          "filter_tree": {"children": [pred("watchers", "has_any", ["@me"])]}},
         False),
        ({"name": "Sprint board",
          "description": "Open work on the board.",
          "visibility": "shared", "group_by": "state", "sort_by": "priority",
          "sort_dir": "desc", "default_tab": "board",
          "filter_tree": {"children": [pred("state_type", "in", ["unstarted", "started"])]}},
         False),
        ({"name": "My open tasks",
          "description": "Assigned to me and not finished (private).",
          "visibility": "private", "group_by": "state", "sort_by": "due_date",
          "sort_dir": "asc", "default_tab": "tasks",
          "filter_tree": {"children": [pred("assignees", "has_any", ["@me"]),
                                       pred("state_type", "not_in", ["completed", "cancelled"])]}},
         False),
    ]
    default_slug = None
    for payload, is_default in views:
        v = api("POST", f"/projects/{key}/views", payload)
        if is_default and make_default:
            api("PUT", f"/projects/{key}/views/{v['slug']}/default")
            default_slug = v["slug"]
    return len(views), default_slug

# ---------------------------------------------------------------- the story
class Project:
    """Seeding handle for one project: vocabulary lookups plus the action
    wrappers that write history and record the retiming plan."""

    def __init__(self, key, team):
        self.key = key
        self.team = team  # list of {id, email, name}; [0] is the admin
        self.states = {s["name"]: s["id"] for s in api("GET", f"/projects/{key}/states")}
        self.prio = {p["rank"]: p["id"] for p in api("GET", f"/projects/{key}/priorities")}
        self.labels = {}
        self.cycles = {}
        self.tasks = {}     # handle -> {id, num}
        self.counts = {"tasks": 0, "subtasks": 0, "comments": 0, "delivered": 0}

    def member(self, i):
        return self.team[i % len(self.team)]

    def tok(self, member):
        if member["email"] == ADMIN_EMAIL:
            return TOKEN[0]
        return user_token(member["email"])

    def label(self, name):
        if name not in self.labels:
            raise RuntimeError(f"unknown label {name}")
        return self.labels[name]

    def cycle(self, name, start_ago, end_ago):
        c = api("POST", f"/projects/{self.key}/cycles", {
            "title": name,
            "start_date": (date.today() - timedelta(days=start_ago)).isoformat(),
            "end_date": (date.today() - timedelta(days=end_ago)).isoformat(),
        })
        self.cycles[name] = c["id"]

    def mk(self, handle, title, ts, assignees=(), labels=(), watchers=(),
           rank=2, rating=None, dif=0, eff=0, due_in=None, parent=None,
           creator=None, originators=()):
        creator = creator or (assignees[0] if assignees else self.team[0])
        body = {"title": title,
                "description": f"{title}.",
                "priority_id": self.prio[rank],
                "assignees": [m["id"] for m in assignees],
                "watchers": [m["id"] for m in watchers],
                "labels": [self.label(l) for l in labels]}
        if originators:
            body["originators"] = [m["id"] for m in originators]
        if rating is not None:
            body["priority_rating"] = rating
        if dif:
            body["difficulty"] = dif
        if eff:
            body["effort"] = eff
        if due_in is not None:
            body["due_date"] = f"{(date.today() + timedelta(days=due_in)).isoformat()}T17:00:00Z"
        if parent is not None:
            body["parent_task_number"] = self.tasks[parent]["num"]
        t = api("POST", f"/projects/{self.key}/tasks", body, token=self.tok(creator))
        tid = t["id"]
        self.tasks[handle] = {"id": tid, "num": t["task_number"]}
        rows = 1 + len(assignees) + len(watchers) + len(labels) + (len(originators) or 1)
        log_rows(tid, ts, rows)
        PLAN_CREATED[tid] = ts
        self.counts["subtasks" if parent else "tasks"] += 1
        return handle

    def mv(self, handle, state_name, actor, ts):
        tk = self.tasks[handle]
        api("PATCH", f"/projects/{self.key}/tasks/{tk['num']}",
            {"state_id": self.states[state_name]}, token=self.tok(actor))
        log_rows(tk["id"], ts)
        if state_name in ("Done",):
            self.counts["delivered"] += 1

    def asg(self, handle, member, ts):
        tk = self.tasks[handle]
        api("POST", f"/projects/{self.key}/tasks/{tk['num']}/assignees",
            {"user_id": member["id"]})
        log_rows(tk["id"], ts)

    def unasg(self, handle, member, ts, auto_contrib=False):
        tk = self.tasks[handle]
        api("DELETE", f"/projects/{self.key}/tasks/{tk['num']}/assignees/{member['id']}")
        # An unassignment during a started-type state also writes the
        # contributor_added entry (the hand-over rule).
        log_rows(tk["id"], ts, 2 if auto_contrib else 1)

    def cmt(self, handle, member, text, ts):
        tk = self.tasks[handle]
        api("POST", f"/projects/{self.key}/tasks/{tk['num']}/comments",
            {"content": f"<p>{text}</p>"}, token=self.tok(member))
        log_rows(tk["id"], ts)
        PLAN_COMMENTS.setdefault(tk["id"], []).append(ts)
        self.counts["comments"] += 1

    def cyc_add(self, handle, cycle_name, ts):
        tk = self.tasks[handle]
        api("POST", f"/projects/{self.key}/cycles/{self.cycles[cycle_name]}/tasks",
            {"task_ids": [tk["id"]]})
        log_rows(tk["id"], ts)
        PLAN_CYCLE_AT[tk["id"]] = ts

    def cyc_rm(self, handle, cycle_name, ts):
        tk = self.tasks[handle]
        api("DELETE", f"/projects/{self.key}/cycles/{self.cycles[cycle_name]}/tasks/{tk['id']}")
        log_rows(tk["id"], ts)

def seed_sprint_project(p: Project):
    """Two finished sprints and an active one, with every insights shape:
    Sprint 2 is the showcase. Work is spread across the whole team."""
    m = p.member  # m(0) = admin, m(1..) = users

    p.cycle("Sprint 1", 31, 18)
    p.cycle("Sprint 2", 17, 4)
    p.cycle("Sprint 3", 3, -10)
    p.cycle("Sprint 4", -11, -24)

    # ---- Sprint 1: six planned, five delivered, one still open (old carry).
    s1 = [
        ("s1a", "Set up error budgets", m(1), 3, 2),
        ("s1b", "Fix onboarding typo flood", m(2), 1, 1),
        ("s1c", "Spike: background sync", m(3), 0, 0),
        ("s1d", "Harden deep-link parsing", m(4), 3, 3),
        ("s1e", "Add CSV export", m(5), 2, 2),
        ("s1f", "Paginate the activity feed", m(6), 2, 3),
    ]
    for i, (h, title, who, dif, eff) in enumerate(s1):
        p.mk(h, title, T(33, f"09:{10 + i:02d}"), assignees=[who], rank=2, dif=dif, eff=eff)
        p.cyc_add(h, "Sprint 1", T(32, f"15:{i:02d}"))
    for i, (h, _, who, _, _) in enumerate(s1[:5]):
        p.mv(h, "In Progress", who, T(30 - i, "09:30"))
        p.mv(h, "Done", who, T(28 - (i * 2), "16:00"))
    # s1f never finished inside Sprint 1.

    # ---- Sprint 2, the showcase.
    p.mk("p1", "Implement checkout flow", T(21, "09:00"), assignees=[m(1)],
         labels=["feature"], watchers=[m(4)], rank=3)
    p.mk("p1a", "Checkout UI", T(21, "09:05"), parent="p1", assignees=[m(1)], dif=3, eff=3)
    p.mk("p1b", "Payment API integration", T(21, "09:06"), parent="p1", assignees=[m(2)], dif=4, eff=4)
    p.mk("p1c", "Checkout analytics events", T(21, "09:07"), parent="p1", dif=2, eff=2)
    p.mk("p2", "Fix login crash on cold start", T(20, "10:00"), assignees=[m(0)],
         labels=["bug"], rank=4, rating=9, dif=2, eff=3)
    p.mk("p3", "Add rate limiting", T(20, "10:30"), assignees=[m(2)],
         watchers=[m(0)], rank=3, dif=3, eff=4)
    p.mk("p4", "Add dark mode", T(20, "11:00"), assignees=[m(1), m(0)],
         labels=["design"], rank=2, dif=3, eff=3)
    p.mk("p5", "Migrate image cache", T(20, "11:30"), assignees=[m(3)],
         labels=["tech-debt"], rank=2)
    p.mk("p6", "Build notification preferences", T(19, "09:15"), assignees=[m(4)],
         rank=2, dif=2, eff=2)
    p.mk("p7", "Refactor session storage", T(19, "09:45"), assignees=[m(5)],
         rank=3, dif=4, eff=2, due_in=-1)
    p.mk("p8", "Upgrade build pipeline", T(19, "10:15"), assignees=[m(6)],
         labels=["tech-debt"], rank=1, dif=2, eff=2)
    p.mk("p9", "Legacy export cleanup", T(19, "10:45"), assignees=[m(7)], rank=1, dif=1, eff=2)
    p.mk("p10", "Improve empty states", T(19, "11:15"), assignees=[m(8)],
         labels=["design"], rank=2, dif=2, eff=2)
    p.mk("p11", "Offline sync spike", T(19, "11:45"), assignees=[m(3)], rank=2, dif=5, eff=4)
    for i, h in enumerate(["p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10", "p11"]):
        p.cyc_add(h, "Sprint 2", T(18, f"15:{i:02d}"))

    p.mv("p2", "In Progress", m(0), T(16, "09:30"))
    p.mv("p1a", "In Progress", m(1), T(16, "10:00"))
    p.mv("p1", "In Progress", m(1), T(16, "10:01"))
    p.mv("p3", "In Progress", m(2), T(15, "09:20"))
    p.cmt("p3", m(2), "Sliding window per client; draft in review.", T(15, "15:40"))
    p.mv("p2", "Done", m(0), T(14, "16:45"))
    p.mv("p1b", "In Progress", m(2), T(14, "09:10"))
    p.mv("p6", "In Progress", m(4), T(14, "11:00"))
    p.mv("p7", "In Progress", m(5), T(13, "09:35"))
    p.cyc_rm("p11", "Sprint 2", T(12, "10:00"))
    p.cmt("p11", m(0), "Descoped from Sprint 2; revisit after checkout lands.", T(12, "10:01"))
    p.mv("p1a", "Done", m(1), T(12, "17:20"))
    p.mv("p6", "Done", m(4), T(11, "15:10"))
    p.mv("p10", "In Progress", m(8), T(11, "09:30"))
    if "Archived" in p.states:
        p.mv("p10", "Archived", m(0), T(9, "12:10"))
    else:
        p.mv("p10", "Done", m(8), T(9, "12:10"))

    # Added mid-sprint.
    p.mk("a1", "Hotfix: payment double-charge", T(13, "08:30"), assignees=[m(0)],
         labels=["bug"], rank=4, rating=10, dif=2, eff=2)
    p.cyc_add("a1", "Sprint 2", T(13, "08:35"))
    p.mv("a1", "In Progress", m(0), T(13, "09:00"))
    p.mv("a1", "Done", m(0), T(12, "12:40"))
    p.mk("a2", "App store review fixes", T(11, "09:50"), assignees=[m(7)], rank=3, dif=1, eff=3)
    p.cyc_add("a2", "Sprint 2", T(11, "09:55"))
    p.mv("a2", "In Progress", m(7), T(10, "10:10"))
    p.mv("a2", "Done", m(7), T(8, "16:05"))
    p.mk("a3", "Crash reporting noise", T(10, "10:20"), assignees=[m(2)], labels=["bug"], rank=2)
    p.cyc_add("a3", "Sprint 2", T(10, "10:25"))

    # The hand-over: m(2) builds, hands to m(6) for verification; unassigning
    # during Testing records the builder as a contributor automatically.
    p.mv("p3", "Testing", m(2), T(10, "11:00"))
    p.cmt("p3", m(2), "Ready for verification on staging.", T(10, "11:01"))
    p.unasg("p3", m(2), T(10, "11:05"), auto_contrib=True)
    p.asg("p3", m(6), T(10, "11:06"))
    p.mv("p3", "Done", m(6), T(9, "14:50"))
    p.mv("p1b", "Done", m(2), T(9, "16:30"))
    p.cmt("p1", m(1), "Two of three checkout subtasks done; analytics remain.", T(9, "17:00"))

    # Distinct outcomes.
    p.mv("p9", "Cancelled", m(0), T(8, "10:30"))
    p.mv("p4", "In Progress", m(1), T(8, "09:25"))
    p.mv("p4", "Done", m(1), T(7, "15:45"))
    p.mv("p5", "In Progress", m(3), T(7, "09:40"))
    p.mv("p5", "Done", m(3), T(6, "11:20"))

    # ---- Between sprints: carry the open work over.
    for i, h in enumerate(["p1", "p7", "p8", "a3"]):
        p.cyc_rm(h, "Sprint 2", T(3, f"08:{30 + i:02d}"))
        p.cyc_add(h, "Sprint 3", T(3, f"09:{i:02d}"))

    # ---- Sprint 3, active.
    p.mk("n1", "Redesign the onboarding wizard", T(3, "10:00"), assignees=[m(4)],
         labels=["design"], rank=3, dif=3, eff=3, due_in=3)
    p.mk("n2", "Add keyboard shortcuts", T(3, "10:30"), assignees=[m(0)], rank=2, dif=2, eff=2)
    p.mk("n3", "Perf: cold start under two seconds", T(3, "11:00"), assignees=[m(5)],
         rank=3, dif=4, eff=4)
    p.mk("n4", "Support markdown in comments", T(3, "11:15"), assignees=[m(8)], rank=2, dif=2, eff=3)
    for i, h in enumerate(["n1", "n2", "n3", "n4"]):
        p.cyc_add(h, "Sprint 3", T(3, f"11:{30 + i:02d}"))
    p.mv("n1", "In Progress", m(4), T(2, "09:30"))
    p.mv("n2", "In Progress", m(0), T(2, "09:45"))
    p.mv("n2", "Done", m(0), T(1, "14:20"))
    p.mv("p8", "In Progress", m(6), T(2, "10:15"))
    p.mv("p8", "Done", m(6), T(1, "16:40"))
    p.mv("n4", "In Progress", m(8), T(1, "10:05"))
    p.mk("n5", "Hotfix: widget crash on resize", T(1, "08:40"), assignees=[m(6)],
         labels=["bug"], rank=4, dif=1, eff=2)
    p.cyc_add("n5", "Sprint 3", T(1, "08:45"))
    p.mv("n5", "In Progress", m(6), T(1, "09:00"))
    p.mv("n5", "Done", m(6), T(0, "10:30"))

    # ---- Ambient, cycle-less work for the health panels.
    p.mk("x1", "Write E2E tests for checkout", T(12, "14:00"), assignees=[m(7)],
         rank=2, dif=3, eff=3)
    p.mv("x1", "In Progress", m(7), T(12, "15:00"))
    p.mk("x2", "Add an audit-log viewer", T(26, "09:30"), assignees=[m(5)], rank=1)
    p.mk("x3", "Implement webhooks", T(9, "13:20"), assignees=[m(3)],
         rank=3, dif=3, eff=4, due_in=2)
    p.mv("x3", "In Progress", m(3), T(9, "14:00"))

    # ---- The admin's own plate, so the grouped dashboard has a story to tell
    # for the login account: a parent with subtasks in flight (nesting), an
    # orphan subtask on someone else's parent (the crumb), and planned work in
    # mixed states. The full plate lives in WEB; the sibling sprint project
    # gets a lighter touch so the two groups read differently.
    if p.key == "WEB":
        p.mk("n6", "Ship the insights report", T(3, "13:00"), assignees=[m(0)],
             labels=["feature"], rank=3, dif=4, eff=3)
        p.mk("n6a", "Report API endpoints", T(3, "13:05"), parent="n6",
             assignees=[m(0)], dif=3, eff=3)
        p.mk("n6b", "Evidence drawer UI", T(3, "13:06"), parent="n6",
             assignees=[m(0)], dif=3, eff=2)
        p.mk("n6c", "Report print styles", T(3, "13:07"), parent="n6",
             assignees=[m(2)], dif=1, eff=2)
        p.mk("n7", "Review the audit-log spec", T(2, "09:10"), assignees=[m(0)],
             rank=2, dif=2, eff=2, due_in=4)
        p.mk("n8", "Quarterly dependency audit", T(2, "09:40"), assignees=[m(0)],
             rank=1, dif=2, eff=3)
        for i, h in enumerate(["n6", "n7", "n8"]):
            p.cyc_add(h, "Sprint 3", T(2, f"10:{i:02d}"))
        p.mv("n6", "In Progress", m(0), T(2, "11:00"))
        p.mv("n6a", "In Progress", m(0), T(2, "11:05"))
        p.mv("n6b", "In Progress", m(0), T(1, "09:30"))
        p.mv("n6b", "Testing", m(0), T(0, "09:40"))
        # The crumb case: the admin picks up a subtask on m(1)'s open parent.
        p.asg("p1c", m(0), T(2, "14:00"))
    else:
        p.asg("a3", m(0), T(1, "09:15"))
        p.asg("x3", m(0), T(1, "09:20"))

def seed_flow_project(p: Project):
    """A project without cycles: steady intake, completions spread over two
    months, an aging item, open top-severity work. The date-range report."""
    m = p.member
    flow = [
        # (handle, title, member idx, dif, eff, created, started, done)
        ("f1", "Rotate TLS certificates", 1, 2, 2, 55, 52, 50),
        ("f2", "Postgres minor upgrade", 2, 3, 2, 48, 44, 41),
        ("f3", "Automate dependency updates", 3, 2, 2, 40, 37, 33),
        ("f4", "On-call runbook update", 0, 0, 0, 28, 26, 24),
        ("f5", "Disk alerts misfire at 80 percent", 4, 2, 1, 18, 16, 14),
        ("f6", "Cache dashboard metrics", 5, 3, 3, 15, 12, 9),
        ("f7", "Add SSO group mapping", 6, 3, 4, 12, 8, 5),
        ("f8", "Fix timezone handling in reports", 7, 2, 2, 7, 5, 3),
    ]
    for h, title, who, dif, eff, created, started, done in flow:
        p.mk(h, title, T(created, "09:20"), assignees=[m(who)], rank=2, dif=dif, eff=eff)
        p.mv(h, "In Progress", m(who), T(started, "10:00"))
        p.mv(h, "Done", m(who), T(done, "16:10"))
    p.cmt("f6", m(5), "Tested on staging, looks good so far.", T(10, "11:30"))
    p.cmt("f7", m(6), "Adding a feature flag so we can roll this out gradually.", T(7, "09:40"))

    # Open work: an aging item, a blocked pair, top severity, due soon.
    p.mk("f9", "Zero-downtime deploy script", T(14, "10:30"), assignees=[m(0)], rank=3, dif=4, eff=3)
    p.mv("f9", "In Progress", m(0), T(11, "09:20"))
    p.cmt("f9", m(0), "Draining connections cleanly; rollback path still to prove.", T(10, "17:30"))
    p.mk("f10", "Backup restore drill", T(6, "10:00"), assignees=[m(2)], rank=2, dif=2, eff=3, due_in=2)
    p.mk("f11", "CDN migration plan", T(5, "09:40"), assignees=[m(1)], rank=4)
    p.mk("f12", "Log retention policy", T(2, "11:10"), rank=1)
    p.mk("f13", "SSO session bug", T(8, "09:00"), assignees=[m(6)], labels=["bug"], rank=3, dif=2, eff=2)
    p.mv("f13", "In Progress", m(6), T(7, "10:30"))

    # The admin's share of the ops queue, so the dashboard's OPS group holds
    # more than one row: planned, in progress, and shared work.
    p.mk("f14", "Incident postmortem template", T(4, "10:20"), assignees=[m(0)],
         rank=2, dif=1, eff=2, due_in=5)
    p.mk("f15", "Review access policies", T(3, "09:30"), assignees=[m(0), m(4)],
         rank=3, dif=2, eff=2)
    p.mv("f15", "In Progress", m(0), T(1, "11:00"))

# ---------------------------------------------------------------- seed
def main():
    global ADMIN_PW, USER_PW
    print("→ resetting app data")
    reset()

    print(f"→ creating admin {ADMIN_EMAIL} (first signup is promoted to admin)")
    signup = {"username": "demo", "email": ADMIN_EMAIL, "first_name": "Demo", "last_name": "Admin"}
    try:
        api("POST", "/signup", {**signup, "password": ADMIN_PW}, auth=False)
    except RuntimeError as e:
        if "requirements" not in str(e):
            raise
        ADMIN_PW = USER_PW = STRONG_PW  # prod enforces strong passwords
        api("POST", "/signup", {**signup, "password": STRONG_PW}, auth=False)
        print(f"   (prod password policy → using '{STRONG_PW}')")
    signin()
    admin_id = api("GET", "/me")["id"]

    print(f"→ creating {NUM_USERS} users")
    users = []
    used = set()
    for _ in range(NUM_USERS):
        while True:
            fn, ln = random.choice(FIRST), random.choice(LAST)
            uname = (fn + ln).lower()
            if uname not in used:
                used.add(uname)
                break
        r = api("POST", "/admin/users", {"username": uname, "email": f"{uname}@demo.local",
                "password": USER_PW, "first_name": fn, "last_name": ln, "user_type": "user"},
                ignore=(409,))
        if r.get("_status") == 409:
            continue
        users.append({"id": r["id"], "name": f"{fn} {ln}", "email": f"{uname}@demo.local"})
    user_ids = [u["id"] for u in users]

    print("→ creating a deactivated demo account")
    inactive = api("POST", "/admin/users", {"username": "inactivedemo",
                   "email": "inactivedemo@demo.local", "password": USER_PW,
                   "first_name": "Ines", "last_name": "Active", "user_type": "user"},
                   ignore=(409,))

    ws_key = random.choice(WORKSPACES).upper()[:10]
    print(f"→ workspace {ws_key}")
    ws = api("POST", "/workspaces", {"workspace_key": ws_key, "name": random.choice(WORKSPACES) + " HQ",
                                     "description": "Demo workspace"})
    for uid in user_ids + ([inactive["id"]] if "id" in inactive else []):
        api("POST", f"/workspaces/{ws_key}/members", {"user_id": uid}, ignore=(409,))

    # The story team: the admin plus every user, in a stable order.
    team = [{"id": admin_id, "email": ADMIN_EMAIL, "name": "Demo Admin"}] + users

    total = {"projects": 0, "cycles": 0, "modules": 0, "tasks": 0, "subtasks": 0,
             "views": 0, "labels": 0, "comments": 0, "pages": 0, "delivered": 0}
    project_ids = []
    default_view_note = ""
    projects = [("Web Platform", "WEB", "sprints"),
                ("Mobile App", "MOBILE", "sprints"),
                ("Operations", "OPS", "flow")]
    story_projects = []
    for pi, (name, key, kind) in enumerate(projects):
        print(f"→ project {key} ({name})")
        total["projects"] += 1
        proj = api("POST", "/projects", {"project_key": key, "name": name, "workspace_id": ws["id"],
                                         "description": f"{name} — demo project"})
        project_ids.append(proj["id"])
        for uid in user_ids:
            api("POST", f"/projects/{key}/members", {"user_id": uid, "role": "member"}, ignore=(409,))
        api("PATCH", f"/projects/{key}/members/{user_ids[0]}", {"role": "admin"})
        if "id" in inactive and pi == 0:
            api("POST", f"/projects/{key}/members", {"user_id": inactive["id"], "role": "member"},
                ignore=(409,))

        if kind == "sprints" and pi == 0:
            # One project gets an archived-type state so the archived outcome
            # bucket (its own bucket, never merged) shows up in insights.
            api("POST", f"/projects/{key}/states",
                {"state_type": "archived", "name": "Archived", "color": "#A9AABC"})
        p = Project(key, team)
        for lname, color in LABELS:
            l = api("POST", f"/projects/{key}/labels", {"name": lname, "color": color})
            p.labels[lname] = l["id"]
            total["labels"] += 1

        for tname, ttitle, tdesc in TEMPLATES:
            api("POST", f"/projects/{key}/templates",
                {"name": tname, "title": ttitle, "description": tdesc})
        for ptitle, pcontent in PAGES:
            api("POST", f"/projects/{key}/pages", {"title": ptitle, "content": pcontent})
            total["pages"] += 1

        print(f"   writing the {kind} story...")
        if kind == "sprints":
            seed_sprint_project(p)
        else:
            seed_flow_project(p)
        total["cycles"] += len(p.cycles)
        total["tasks"] += p.counts["tasks"]
        total["subtasks"] += p.counts["subtasks"]
        total["comments"] += p.counts["comments"]
        total["delivered"] += p.counts["delivered"]
        story_projects.append(p)

        created, default_slug = seed_views(key, p.prio, make_default=(pi == 0))
        total["views"] += created
        if default_slug:
            default_view_note = f"{key} default view: '{default_slug}' (auto-applies for untouched members)"

    # -------------------------------------------------- retime the history
    print("→ retiming history to its planned days")
    apply_plan()

    # ------------------------------------- uncounted extras (real-time now)
    # Blockers and epics come after the retime pass: their rows keep "now"
    # timestamps, which is fine — they are not insights inputs.
    print("→ blockers and epics")
    for p in story_projects:
        if "n3" in p.tasks and "n1" in p.tasks:
            api("POST", f"/projects/{p.key}/tasks/{p.tasks['n3']['num']}/blockers",
                {"task_ids": [p.tasks["n1"]["id"]], "relation": "blocked_by"})
        if "f13" in p.tasks and "f9" in p.tasks:
            api("POST", f"/projects/{p.key}/tasks/{p.tasks['f13']['num']}/blockers",
                {"task_ids": [p.tasks["f9"]["id"]], "relation": "blocked_by"})
        story_ids = [t["id"] for t in p.tasks.values()]
        for mi, title in enumerate(random.sample(EPICS, 5)):
            status = MODULE_STATUSES[mi % len(MODULE_STATUSES)]
            m_start = date.today() - timedelta(days=random.randint(20, 50))
            m = api("POST", f"/projects/{p.key}/modules",
                    {"title": title, "status": status, "description": f"Epic: {title}",
                     "start_date": m_start.isoformat(),
                     "end_date": (m_start + timedelta(days=random.randint(20, 45))).isoformat(),
                     "lead_id": random.choice(user_ids),
                     "member_ids": random.sample(user_ids, k=2),
                     "priority_rating": random.choice([0, 3, 5, 7, 9])})
            total["modules"] += 1
            picks = random.sample(story_ids, k=min(4, len(story_ids)))
            api("POST", f"/projects/{p.key}/modules/{m['id']}/tasks", {"task_ids": picks},
                ignore=(409,))

    if "id" in inactive:
        api("PUT", f"/admin/users/{inactive['id']}/active", {"active": False})

    if len(project_ids) > 1:
        items = [{"id": pid, "position": i + 1}
                 for i, pid in enumerate(reversed(project_ids))]
        api("PATCH", "/projects/reorder", {"items": items})

    # Display timestamps follow the history.
    psql("""UPDATE tasks SET updated_at = GREATEST(created_at,
             COALESCE((SELECT MAX(a.created_at) FROM activity_log a WHERE a.task_id = tasks.id),
                      created_at));
            UPDATE projects SET created_at = now() - interval '60 days',
                                updated_at = now() - interval '60 days';""")

    # ------------------------------------------------- re-chain + verify
    print("→ recomputing activity checksum chains")
    n = rechecksum()
    print(f"   {n} activity rows re-chained")

    print("→ verifying checksum chains through the API")
    bad = []
    for p in story_projects:
        for handle, tk in p.tasks.items():
            v = api("GET", f"/projects/{p.key}/tasks/{tk['num']}/activity/verify")
            if not v.get("valid"):
                bad.append(f"{p.key}-{tk['num']} ({handle}): {v.get('message')}")
    if bad:
        raise SystemExit("checksum verification FAILED for:\n  " + "\n  ".join(bad[:10]))
    print("   all chains valid")

    print("\n✅ Seed complete")
    print(f"   workspace: {ws_key}   projects: {total['projects']}   cycles: {total['cycles']}   "
          f"modules: {total['modules']}   tasks: {total['tasks']}   sub-tasks: {total['subtasks']}")
    print(f"   delivered (history!): {total['delivered']}   comments: {total['comments']}   "
          f"views: {total['views']}   labels: {total['labels']}   pages: {total['pages']}")
    if default_view_note:
        print(f"   {default_view_note}")
    print("   insights: /insights (portfolio) → WEB or MOBILE + Sprint 2 (the showcase),")
    print("             OPS as the no-cycles date-range report")
    print("   dashboard: the admin's plate spans WEB (parent + subtasks, a crumb),")
    print("              MOBILE and OPS — the grouped Assigned-to-You demo")
    print(f"   admin login: {ADMIN_EMAIL} / {ADMIN_PW}")
    print(f"   users: password '{USER_PW}', e.g. " + ", ".join(u["name"] for u in users[:3]) + ", …")

if __name__ == "__main__":
    main()
