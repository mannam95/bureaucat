#!/usr/bin/env python3
"""Seed a local Bureaucat instance with rich, realistic demo data.

Creates: an admin (demo@gmail.com / password), a handful of users (plus one
deactivated account), a workspace, a couple of projects, and — per project —
labels, task templates, pages, sprints (cycles spanning completed/active/
upcoming), epics (modules across every status so both Active and Completed
tabs have content), and meaningful tasks with sub-tasks, assignees,
originators, watchers, labels, dates, priorities and workflow states.
Tasks get comments written by different users, every project gets a set of
saved views (shared and private, list and board), the first project gets a
pinned default view, and the projects get a custom global display order.

Idempotent: it RESETS all app data first (TRUNCATE via the postgres container),
then rebuilds, so `make seed` always yields the same clean dataset. The very first
account is created via /signup, which the app promotes to admin automatically.

Stdlib only. Configure via env: BUREAUCAT_URL, PG_CONTAINER, SEED, NUM_USERS,
NUM_PROJECTS, CYCLES_PER_PROJECT, MODULES_PER_PROJECT, TASKS_PER_PROJECT.
"""
import json, os, random, subprocess, urllib.request, urllib.error
from datetime import date, timedelta

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
NUM_PROJECTS = int(os.environ.get("NUM_PROJECTS", "2"))
# 4 sprints centred on today: two finished, one active, one upcoming — so the
# cycles overview has content on both the Active and the Completed tab.
CYCLES_PER_PROJECT = int(os.environ.get("CYCLES_PER_PROJECT", "4"))
MODULES_PER_PROJECT = int(os.environ.get("MODULES_PER_PROJECT", "5"))
TASKS_PER_PROJECT = int(os.environ.get("TASKS_PER_PROJECT", "15"))

# ---------------------------------------------------------------- data pools
FIRST = ["Alice", "Bob", "Carla", "Diego", "Ellen", "Farah", "Grace", "Hiro",
         "Ivan", "Julia", "Kwame", "Lena", "Mateo", "Nina", "Omar", "Priya"]
LAST = ["Zhang", "Okafor", "Rossi", "Nguyen", "Schmidt", "Haddad", "Kim",
        "Silva", "Ivanov", "Brown", "Mensah", "Novak", "Garcia", "Patel"]
WORKSPACES = ["Northwind", "Acme", "Globex", "Umbrella", "Initech"]
PROJECTS = [("Web Platform", "WEB"), ("Mobile App", "MOBILE"),
            ("Data Pipeline", "DATA"), ("Billing Service", "BILLING")]
EPICS = ["Authentication & SSO", "Billing & Invoicing", "Search & Discovery",
         "Notifications", "Dashboard & Analytics", "Onboarding", "Public API",
         "Performance", "Admin Console", "Reporting"]
# Modules are spread across the whole status enum so the overview's
# Active/Completed tabs both have content (completed + cancelled land on
# the Completed tab).
MODULE_STATUSES = ["planned", "in_progress", "ongoing", "completed", "cancelled",
                   "backlog", "paused"]
STORIES = [
    "Add password reset flow", "Implement OAuth login", "Design settings page",
    "Optimize task list query", "Add CSV export", "Fix timezone handling in reports",
    "Build notification preferences", "Add dark mode", "Implement bulk actions",
    "Paginate the activity feed", "Add rate limiting", "Write E2E tests for checkout",
    "Migrate to the new table component", "Add keyboard shortcuts", "Improve empty states",
    "Add an audit-log viewer", "Support markdown in comments", "Add file attachments",
    "Implement webhooks", "Cache dashboard metrics", "Add SSO group mapping",
    "Redesign the onboarding wizard", "Add per-project roles", "Ship the mobile nav",
]
GITHUB_REPO = "northwind/platform"

LABELS = [("bug", "#EF4444"), ("feature", "#3B82F6"), ("tech-debt", "#F97316"),
          ("design", "#8B5CF6"), ("docs", "#10B981"), ("urgent", "#DC2626")]

COMMENTS = [
    "Picking this up today.",
    "Blocked on the API change — see the linked PR.",
    "I pushed a first draft, feedback welcome.",
    "Can we split this into two tasks? The scope grew a bit.",
    "Tested on staging, looks good so far.",
    "The design file was updated, please re-check the spacing.",
    "This also fixes the issue reported by support last week.",
    "Rebased on main and resolved the conflicts.",
    "Adding a feature flag so we can roll this out gradually.",
    "QA found one edge case with empty states, fixing now.",
    "Let's demo this in the next sprint review.",
    "Done from my side, over to review.",
]

PAGES = [
    ("Team Handbook",
     "# Team Handbook\n\n"
     "How we work in this project.\n\n"
     "## Rituals\n\n"
     "- Standup: daily, async in the channel\n"
     "- Planning: first Monday of the sprint\n"
     "- Review & retro: last Friday of the sprint\n\n"
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
    ("Bug report", "Bug: <summary>",
     "Steps to reproduce:\n1. \n2. \n\nExpected:\n\nActual:\n\nEnvironment:"),
    ("Feature request", "Feature: <summary>",
     "Problem:\n\nProposal:\n\nAcceptance criteria:\n- [ ] "),
]

def _slug(title):
    s = "".join(c if c.isalnum() else "-" for c in title.lower())
    while "--" in s:
        s = s.replace("--", "-")
    return s.strip("-")[:32]

def custom_fields(title):
    """Random Figma / branch / pull request links. All three are links so the
    demo shows them rendered as links."""
    slug = _slug(title)
    token = "".join(random.choices("abcdef0123456789", k=12))
    branch = f"{random.choice(['feat', 'fix', 'chore'])}/{slug}"
    return {
        "figma_link": f"https://www.figma.com/design/{token}/{slug}?node-id={random.randint(1000, 9999)}-{random.randint(100, 999)}",
        "branch": f"https://github.com/{GITHUB_REPO}/tree/{branch}",
        "pull_request": f"https://github.com/{GITHUB_REPO}/pull/{random.randint(100, 999)}",
    }

def task_dates():
    """A mix of task schedules: ~30% undated, the rest spread around today so
    the list shows overdue, due-soon and future dates."""
    if random.random() < 0.3:
        return {}
    start = date.today() - timedelta(days=random.randint(0, 20))
    due = start + timedelta(days=random.randint(3, 25))
    return {"start_date": f"{start.isoformat()}T00:00:00Z",
            "due_date": f"{due.isoformat()}T00:00:00Z"}

SUBTASKS = ["Write unit tests", "Update API docs", "Add DB migration",
            "Address code-review feedback", "Handle error states", "Add loading skeleton",
            "Wire up the endpoint", "Emit analytics event", "QA on staging",
            "Accessibility pass", "Add feature flag", "Backfill existing rows"]
# (state name, relative weight) over the project's auto-created default states
STATE_WEIGHTS = [("Backlog", 3), ("Todo", 3), ("Approval Pending", 1),
                 ("In Progress", 4), ("Blocked", 1), ("Testing", 2),
                 ("Done", 4), ("Cancelled", 1)]

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
    """Access token for a seeded user, so comments carry different authors."""
    if email not in USER_TOKENS:
        USER_TOKENS[email] = api("POST", "/signin",
                                 {"identifier": email, "password": USER_PW},
                                 auth=False)["access_token"]
    return USER_TOKENS[email]

def reset():
    sql = "TRUNCATE users, workspaces RESTART IDENTITY CASCADE;"
    p = subprocess.run(["docker", "exec", PG_CONTAINER, "psql", "-U", PG_USER, "-d", PG_DB, "-c", sql],
                       capture_output=True, text=True)
    if p.returncode != 0:
        raise SystemExit(f"reset failed (is the stack up & migrated?):\n{p.stderr.strip()}")

# ---------------------------------------------------------------- views
def pred(field, op, value=None):
    p = {"field": field, "op": op}
    if value is not None:
        p["value"] = value
    return {"predicate": p}

def seed_views(key, make_default):
    """A spread of saved views per project: shared and private, list and board,
    including one built on the watcher filter. Returns (created, default_slug)."""
    views = [
        # (payload, set_as_default)
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
          "filter_tree": {"children": [pred("priority", "in", [3, 4])]}},
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

    print(f"→ creating {NUM_USERS} users")
    users = []                       # list of {id, name, email}
    used = set()
    for _ in range(NUM_USERS):
        while True:
            fn, ln = random.choice(FIRST), random.choice(LAST)
            uname = (fn + ln).lower()
            if uname not in used:
                used.add(uname); break
        r = api("POST", "/admin/users", {"username": uname, "email": f"{uname}@demo.local",
                "password": USER_PW, "first_name": fn, "last_name": ln, "user_type": "user"},
                ignore=(409,))
        if r.get("_status") == 409:
            continue
        users.append({"id": r["id"], "name": f"{fn} {ln}", "email": f"{uname}@demo.local"})
    user_ids = [u["id"] for u in users]

    # One extra account that is deactivated, to demo the account states in the
    # admin console and member lists. Never assigned to any work.
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

    total = {"projects": 0, "cycles": 0, "modules": 0, "tasks": 0, "subtasks": 0,
             "views": 0, "labels": 0, "comments": 0, "pages": 0, "watchers": 0}
    project_ids = []
    default_view_note = ""
    for pi, (name, key) in enumerate(PROJECTS[:NUM_PROJECTS]):
        print(f"→ project {key} ({name})")
        total["projects"] += 1
        proj = api("POST", "/projects", {"project_key": key, "name": name, "workspace_id": ws["id"],
                                         "description": f"{name} — demo project"})
        project_ids.append(proj["id"])
        for uid in user_ids:
            api("POST", f"/projects/{key}/members", {"user_id": uid, "role": "member"}, ignore=(409,))
        # One member is a project admin, so the role-gated UI (default view pin,
        # cycle/module management) can be tried with a non-global-admin login.
        api("PATCH", f"/projects/{key}/members/{user_ids[0]}", {"role": "admin"})
        if "id" in inactive and pi == 0:
            api("POST", f"/projects/{key}/members", {"user_id": inactive["id"], "role": "member"},
                ignore=(409,))

        states = {s["name"]: s["id"] for s in api("GET", f"/projects/{key}/states")}
        state_pool = [states[n] for n, w in STATE_WEIGHTS for _ in range(w) if n in states]

        # labels
        label_ids = []
        for lname, color in LABELS:
            l = api("POST", f"/projects/{key}/labels", {"name": lname, "color": color})
            label_ids.append(l["id"]); total["labels"] += 1

        # task templates + documentation pages
        for tname, ttitle, tdesc in TEMPLATES:
            api("POST", f"/projects/{key}/templates",
                {"name": tname, "title": ttitle, "description": tdesc})
        for ptitle, pcontent in PAGES:
            api("POST", f"/projects/{key}/pages", {"title": ptitle, "content": pcontent})
            total["pages"] += 1

        # sprints (cycles): consecutive, non-overlapping, centred on today —
        # with 4 per project that is two completed, one active, one upcoming.
        cycles = []
        base = date.today() - timedelta(days=14 * (CYCLES_PER_PROJECT - 2))
        for i in range(CYCLES_PER_PROJECT):
            s = base + timedelta(days=14 * i)
            c = api("POST", f"/projects/{key}/cycles", {"title": f"Sprint {i + 1}",
                    "start_date": s.isoformat(), "end_date": (s + timedelta(days=13)).isoformat()})
            cycles.append(c["id"]); total["cycles"] += 1

        # epics (modules): spread over the status enum, with leads, dates,
        # members and priority ratings, so list/tile views and both overview
        # tabs have something to show.
        modules = []
        for mi, title in enumerate(random.sample(EPICS, MODULES_PER_PROJECT)):
            status = MODULE_STATUSES[mi % len(MODULE_STATUSES)]
            if status in ("completed", "cancelled"):
                m_start = date.today() - timedelta(days=random.randint(40, 70))
                m_end = m_start + timedelta(days=random.randint(20, 30))
            else:
                m_start = date.today() - timedelta(days=random.randint(0, 20))
                m_end = m_start + timedelta(days=random.randint(20, 45))
            m = api("POST", f"/projects/{key}/modules",
                    {"title": title, "status": status, "description": f"Epic: {title}",
                     "start_date": m_start.isoformat(), "end_date": m_end.isoformat(),
                     "lead_id": random.choice(user_ids),
                     "member_ids": random.sample(user_ids, k=random.choice([2, 3])),
                     "priority_rating": random.choice([0, 0, 3, 5, 7, 9])})
            modules.append(m["id"]); total["modules"] += 1

        # tasks (+ sub-tasks) with assignees, originators, watchers, labels,
        # dates and star ratings
        cycle_task_ids = {c: [] for c in cycles}
        module_task_ids = {m: [] for m in modules}
        for title in random.sample(STORIES, min(TASKS_PER_PROJECT, len(STORIES))):
            assignees = random.sample(user_ids, k=random.choice([0, 1, 1, 2]))
            watchers = random.sample(user_ids, k=random.choice([0, 0, 1, 2, 3]))
            total["watchers"] += len(watchers)
            body = {"title": title, "description": f"{title}.",
                    "state_id": random.choice(state_pool), "priority": random.choice([0, 1, 2, 2, 3, 4]),
                    "assignees": assignees, "watchers": watchers,
                    "labels": random.sample(label_ids, k=random.choice([0, 0, 1, 1, 2])),
                    **task_dates(), **custom_fields(title)}
            if random.random() < 0.4:
                body["originators"] = random.sample(user_ids, k=random.choice([1, 1, 2]))
            if random.random() < 0.5:
                body["priority_rating"] = random.randint(1, 10)
            t = api("POST", f"/projects/{key}/tasks", body)
            total["tasks"] += 1
            cyc = random.choice(cycles); cycle_task_ids[cyc].append(t["id"])
            if random.random() < 0.7:
                mod = random.choice(modules); module_task_ids[mod].append(t["id"])
            for st in random.sample(SUBTASKS, k=random.choice([0, 1, 2, 2, 3])):
                api("POST", f"/projects/{key}/tasks", {"title": st,
                    "state_id": random.choice(state_pool), "priority": random.choice([0, 1, 2]),
                    "assignees": random.sample(user_ids, k=random.choice([0, 1])),
                    "watchers": random.sample(user_ids, k=random.choice([0, 0, 1])),
                    "parent_task_number": t["task_number"], **custom_fields(st)})
                total["subtasks"] += 1
                # Sub-tasks inherit their parent's cycle/epic, so they are never
                # placed in a cycle on their own.
            # comments by different authors, so the activity feed looks lived-in
            if random.random() < 0.6:
                for text in random.sample(COMMENTS, k=random.choice([1, 2, 3])):
                    author = random.choice(users)
                    api("POST", f"/projects/{key}/tasks/{t['task_number']}/comments",
                        {"content": text}, token=user_token(author["email"]))
                    total["comments"] += 1

        for cid, ids in cycle_task_ids.items():
            if ids:
                api("POST", f"/projects/{key}/cycles/{cid}/tasks", {"task_ids": ids}, ignore=(409,))
        for mid, ids in module_task_ids.items():
            if ids:
                api("POST", f"/projects/{key}/modules/{mid}/tasks", {"task_ids": ids}, ignore=(409,))

        # saved views — the first project also pins one as the project default,
        # the second stays without one so both behaviours can be compared.
        created, default_slug = seed_views(key, make_default=(pi == 0))
        total["views"] += created
        if default_slug:
            default_view_note = f"{key} default view: '{default_slug}' (auto-applies for untouched members)"

    # deactivate the demo account only after memberships are in place
    if "id" in inactive:
        api("PUT", f"/admin/users/{inactive['id']}/active", {"active": False})

    # a custom global project order (reverse of creation), so the site-admin
    # drag-and-drop ordering has visible effect on /projects and the dashboard
    if len(project_ids) > 1:
        items = [{"id": pid, "position": i + 1}
                 for i, pid in enumerate(reversed(project_ids))]
        api("PATCH", "/projects/reorder", {"items": items})
        print("→ projects reordered (reverse of name order) via site-admin ordering")

    print("\n✅ Seed complete")
    print(f"   workspace: {ws_key}   projects: {total['projects']}   "
          f"cycles: {total['cycles']}   modules: {total['modules']}   "
          f"tasks: {total['tasks']}   sub-tasks: {total['subtasks']}   users: {len(users)}")
    print(f"   views: {total['views']}   labels: {total['labels']}   "
          f"comments: {total['comments']}   pages: {total['pages']}   "
          f"watcher links: {total['watchers']}")
    if default_view_note:
        print(f"   {default_view_note}")
    print(f"   project admin (non-global): {users[0]['name']}   "
          f"deactivated account: inactivedemo@demo.local")
    print(f"   admin login: {ADMIN_EMAIL} / {ADMIN_PW}")
    print(f"   users (log in to see per-person views): password '{USER_PW}', e.g. "
          + ", ".join(u["name"] for u in users[:3]) + ", …")

if __name__ == "__main__":
    main()
