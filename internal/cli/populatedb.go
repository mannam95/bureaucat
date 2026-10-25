package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"bereaucat/internal/auth"
	"bereaucat/internal/server"
	"bereaucat/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/urfave/cli/v3"
)

// apiClient is a thin wrapper around net/http.Client that carries a base URL
// and an optional Bearer token for authenticated requests.
type apiClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func (c *apiClient) do(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}

func (c *apiClient) post(path string, body interface{}) (*http.Response, error) {
	return c.do("POST", path, body)
}

func (c *apiClient) patch(path string, body interface{}) (*http.Response, error) {
	return c.do("PATCH", path, body)
}

func (c *apiClient) get(path string) (*http.Response, error) {
	return c.do("GET", path, nil)
}

func (c *apiClient) delete(path string) (*http.Response, error) {
	return c.do("DELETE", path, nil)
}

// decodeJSON reads the response body and decodes it into dst.
func decodeJSON(resp *http.Response, dst interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// discardBody reads and closes the response body, returning an error if the
// status code indicates failure.
func discardBody(resp *http.Response) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// --- Response types (only the fields we need) ---

type signinResponse struct {
	User        userResponse `json:"user"`
	AccessToken string       `json:"access_token"`
}

type userResponse struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
}

type projectResponse struct {
	ID         uuid.UUID `json:"id"`
	ProjectKey string    `json:"project_key"`
}

type stateResponse struct {
	ID        uuid.UUID `json:"id"`
	StateType string    `json:"state_type"`
	Name      string    `json:"name"`
}

type labelResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type areaResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type priorityResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Rank int       `json:"rank"`
}

type cycleResponse struct {
	ID uuid.UUID `json:"id"`
}

type taskResponse struct {
	ID         uuid.UUID `json:"id"`
	TaskNumber int       `json:"task_number"`
}

// PopulateDBCommand seeds a fresh database with demo data whose HISTORY is
// realistic: every change goes through the real API (so the activity log, the
// contributor auto-rule and notifications all fire exactly like production),
// and each logical event is then retimed to a planned moment in the past.
// Insights reconstructs its reporting from that log, so the seed deliberately
// exercises every counting rule: cycles with planned/added/descoped/carried
// work, delivery via subtasks, the build-then-verify hand-over (auto
// contributors), shared credit, unassessed estimates, a cancelled and an
// archived outcome, a no-cycles project, blockers, and a near-dormant project.
func PopulateDBCommand() *cli.Command {
	return &cli.Command{
		Name:  "populatedb",
		Usage: "Populate the database with sample demo data",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "database-url",
				Usage:   "Database connection URL",
				Sources: cli.EnvVars("DATABASE_URL"),
			},
			&cli.StringFlag{
				Name:    "demo-user-email",
				Usage:   "Email for an additional demo admin user",
				Sources: cli.EnvVars("DEMO_USER_EMAIL"),
			},
			&cli.StringFlag{
				Name:    "demo-user-password",
				Usage:   "Password for the additional demo admin user",
				Sources: cli.EnvVars("DEMO_USER_PASSWORD"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dbURL := cmd.String("database-url")
			if dbURL == "" {
				return fmt.Errorf("database-url is required (use --database-url or DATABASE_URL env var)")
			}

			// Boot the real server with all handlers and middleware.
			srv, err := server.New(true, dbURL, server.AuthConfig{
				JWTSecret:              "populatedb-dummy-jwt-secret-key-32chars!",
				AccessTokenExpiryMins:  60, // generous expiry for seeding
				RefreshTokenExpiryDays: 1,
			}, nil)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}
			defer srv.Close()

			ts := httptest.NewServer(srv.Echo())
			defer ts.Close()

			client := &apiClient{baseURL: ts.URL, http: ts.Client()}

			pool, err := pgxpool.New(ctx, dbURL)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer pool.Close()
			q := store.New(pool)

			fmt.Println("Populating database with demo data...")

			// -----------------------------------------------------------------
			// Time machinery. Every seeded change happens "now" through the API
			// and is immediately retimed to its planned past moment: rows the
			// call just wrote (activity, comments, assignments, contributors,
			// cycle membership) are found by a DB-clock marker taken right
			// before the call, so nothing else is touched. Checksums are
			// recomputed once at the end.
			// -----------------------------------------------------------------
			now0 := time.Now().UTC()
			day0 := time.Date(now0.Year(), now0.Month(), now0.Day(), 0, 0, 0, 0, time.UTC)

			// at returns a past moment: daysAgo days before today, at hh:mm UTC.
			at := func(daysAgo int, hhmm string) time.Time {
				t, err := time.Parse("15:04", hhmm)
				if err != nil {
					panic(fmt.Sprintf("bad hhmm %q", hhmm))
				}
				return day0.AddDate(0, 0, -daysAgo).Add(
					time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute)
			}
			date := func(daysAgo int) string { return day0.AddDate(0, 0, -daysAgo).Format("2006-01-02") }

			dbNow := func() (time.Time, error) {
				var t time.Time
				err := pool.QueryRow(ctx, `SELECT now()`).Scan(&t)
				return t, err
			}

			retimeSQL := []string{
				`UPDATE activity_log SET created_at=$1 WHERE task_id=$2 AND created_at>=$3`,
				`UPDATE comments SET created_at=$1, updated_at=$1 WHERE task_id=$2 AND created_at>=$3`,
				`UPDATE task_assignees SET assigned_at=$1 WHERE task_id=$2 AND assigned_at>=$3`,
				`UPDATE task_contributors SET added_at=$1 WHERE task_id=$2 AND added_at>=$3`,
				`UPDATE cycle_tasks SET added_at=$1 WHERE task_id=$2 AND added_at>=$3`,
			}
			// act performs one API action about taskID and stamps everything it
			// wrote with the planned time t.
			act := func(taskID uuid.UUID, t time.Time, fn func() error) error {
				marker, err := dbNow()
				if err != nil {
					return fmt.Errorf("db clock: %w", err)
				}
				if err := fn(); err != nil {
					return err
				}
				for _, sql := range retimeSQL {
					if _, err := pool.Exec(ctx, sql, t, taskID, marker); err != nil {
						return fmt.Errorf("retime: %w", err)
					}
				}
				return nil
			}

			// -----------------------------------------------------------------
			// 1. Users. The admin goes in via store (there is no one to sign in
			//    as yet); everyone else through the admin API.
			// -----------------------------------------------------------------
			fmt.Println("Creating users...")

			adminHash, err := auth.HashPassword("admin123")
			if err != nil {
				return fmt.Errorf("failed to hash password: %w", err)
			}
			if _, err := q.CreateUser(ctx, store.CreateUserParams{
				Username:     "admin",
				Email:        "admin@example.com",
				PasswordHash: pgtype.Text{String: adminHash, Valid: true},
				FirstName:    "Ada",
				LastName:     "Lovelace",
				UserType:     "admin",
			}); err != nil {
				return fmt.Errorf("failed to create admin user: %w", err)
			}

			resp, err := client.post("/api/v1/signin", map[string]string{
				"identifier": "admin@example.com",
				"password":   "admin123",
			})
			if err != nil {
				return fmt.Errorf("failed to sign in: %w", err)
			}
			var adminAuth signinResponse
			if err := decodeJSON(resp, &adminAuth); err != nil {
				return fmt.Errorf("failed to decode signin response: %w", err)
			}
			client.token = adminAuth.AccessToken

			userIDs := map[string]uuid.UUID{"admin": adminAuth.User.ID}
			passwords := map[string]string{"admin": "admin123"}

			type userSpec struct {
				username, email, first, last, password, userType string
			}
			demoUsers := []userSpec{
				{"alice", "alice@example.com", "Alice", "Zhang", "user123", "user"},
				{"bob", "bob@example.com", "Bob", "Patel", "user123", "user"},
				{"charlie", "charlie@example.com", "Charlie", "Okonkwo", "user123", "user"},
				{"diana", "diana@example.com", "Diana", "Müller", "user123", "user"},
				{"erik", "erik@example.com", "Erik", "Lindqvist", "user123", "user"},
			}
			demoEmail := cmd.String("demo-user-email")
			demoPassword := cmd.String("demo-user-password")
			// The "star" is the person whose insights slice must look rich: the
			// env-configured demo account when given, the admin otherwise.
			star := "admin"
			if demoEmail != "" && demoPassword != "" {
				demoUsers = append(demoUsers, userSpec{"demo", demoEmail, "Demo", "Admin", demoPassword, "admin"})
				star = "demo"
			}
			for _, u := range demoUsers {
				resp, err := client.post("/api/v1/admin/users", map[string]string{
					"username": u.username, "email": u.email, "password": u.password,
					"first_name": u.first, "last_name": u.last, "user_type": u.userType,
				})
				if err != nil {
					return fmt.Errorf("failed to create user %s: %w", u.username, err)
				}
				var created userResponse
				if err := decodeJSON(resp, &created); err != nil {
					return fmt.Errorf("failed to decode create user response for %s: %w", u.username, err)
				}
				userIDs[u.username] = created.ID
				passwords[u.username] = u.password
				fmt.Printf("  %s\n", u.username)
			}

			// Per-user API clients, signed in lazily.
			clients := map[string]*apiClient{"admin": client}
			as := func(username string) (*apiClient, error) {
				if c, ok := clients[username]; ok {
					return c, nil
				}
				resp, err := client.post("/api/v1/signin", map[string]string{
					"identifier": username, "password": passwords[username],
				})
				if err != nil {
					return nil, fmt.Errorf("signin %s: %w", username, err)
				}
				var a signinResponse
				if err := decodeJSON(resp, &a); err != nil {
					return nil, fmt.Errorf("signin %s: %w", username, err)
				}
				c := &apiClient{baseURL: ts.URL, http: ts.Client(), token: a.AccessToken}
				clients[username] = c
				return c, nil
			}

			// -----------------------------------------------------------------
			// 2. Projects and members.
			//    APP runs sprints; OPS works without cycles; LAB is dormant.
			// -----------------------------------------------------------------
			fmt.Println("Creating projects...")

			type projSpec struct {
				key, name, desc string
				members         []string
			}
			projectSpecs := []projSpec{
				{"APP", "Nova Mobile", "The mobile application: features, fixes and releases.",
					[]string{"alice", "bob", "charlie", "diana", "erik", star}},
				{"OPS", "Operations", "Platform operations: infrastructure, upgrades and incidents.",
					[]string{"alice", "charlie", "erik", star}},
				{"LAB", "Research Lab", "Prototypes and explorations, picked up when time allows.",
					[]string{"diana"}},
			}
			projectIDs := map[string]uuid.UUID{}
			for _, p := range projectSpecs {
				resp, err := client.post("/api/v1/projects", map[string]string{
					"project_key": p.key, "name": p.name, "description": p.desc,
				})
				if err != nil {
					return fmt.Errorf("failed to create project %s: %w", p.key, err)
				}
				var created projectResponse
				if err := decodeJSON(resp, &created); err != nil {
					return fmt.Errorf("failed to decode project %s: %w", p.key, err)
				}
				projectIDs[p.key] = created.ID
				for _, m := range p.members {
					if m == "admin" {
						continue // the creator is already an admin member
					}
					resp, err := client.post(fmt.Sprintf("/api/v1/projects/%s/members", p.key),
						map[string]string{"user_id": userIDs[m].String(), "role": "member"})
					if err != nil {
						return fmt.Errorf("failed to add %s to %s: %w", m, p.key, err)
					}
					if err := discardBody(resp); err != nil {
						return fmt.Errorf("failed to add %s to %s: %w", m, p.key, err)
					}
				}
				fmt.Printf("  %s — %s (%d members)\n", p.key, p.name, len(p.members)+1)
			}

			// -----------------------------------------------------------------
			// 3. Vocabularies. APP gets an archived-type state and areas; OPS
			//    renames its top priority (per-project vocabularies are the
			//    whole point of the agnostic insights rules). Labels for both.
			// -----------------------------------------------------------------
			fmt.Println("Shaping project vocabularies...")

			resp, err = client.post("/api/v1/projects/APP/states",
				map[string]string{"state_type": "archived", "name": "Archived", "color": "#A9AABC"})
			if err != nil {
				return fmt.Errorf("failed to create Archived state: %w", err)
			}
			if err := discardBody(resp); err != nil {
				return fmt.Errorf("failed to create Archived state: %w", err)
			}

			areaIDs := map[string]uuid.UUID{}
			for _, a := range []struct{ name, color string }{
				{"Platform", "#3B82F6"}, {"Billing", "#F97316"}, {"UX", "#8B5CF6"},
			} {
				resp, err := client.post("/api/v1/projects/APP/areas",
					map[string]string{"name": a.name, "color": a.color})
				if err != nil {
					return fmt.Errorf("failed to create area %s: %w", a.name, err)
				}
				var created areaResponse
				if err := decodeJSON(resp, &created); err != nil {
					return fmt.Errorf("failed to decode area %s: %w", a.name, err)
				}
				areaIDs[a.name] = created.ID
			}

			labelIDs := map[string]map[string]uuid.UUID{}
			for projKey, labels := range map[string][]struct{ name, color string }{
				"APP": {{"bug", "#EF4444"}, {"feature", "#3B82F6"}, {"tech-debt", "#F59E0B"}},
				"OPS": {{"incident", "#EF4444"}, {"maintenance", "#6B7280"}},
			} {
				labelIDs[projKey] = map[string]uuid.UUID{}
				for _, l := range labels {
					resp, err := client.post(fmt.Sprintf("/api/v1/projects/%s/labels", projKey),
						map[string]string{"name": l.name, "color": l.color})
					if err != nil {
						return fmt.Errorf("failed to create label %s: %w", l.name, err)
					}
					var created labelResponse
					if err := decodeJSON(resp, &created); err != nil {
						return fmt.Errorf("failed to decode label %s: %w", l.name, err)
					}
					labelIDs[projKey][l.name] = created.ID
				}
			}

			// States and priorities per project, by name and by rank.
			stateIDs := map[string]map[string]uuid.UUID{}
			prioByRank := map[string]map[int]uuid.UUID{}
			loadVocab := func(projKey string) error {
				var states []stateResponse
				resp, err := client.get(fmt.Sprintf("/api/v1/projects/%s/states", projKey))
				if err != nil {
					return err
				}
				if err := decodeJSON(resp, &states); err != nil {
					return err
				}
				stateIDs[projKey] = map[string]uuid.UUID{}
				for _, s := range states {
					stateIDs[projKey][s.Name] = s.ID
				}
				var prios []priorityResponse
				resp, err = client.get(fmt.Sprintf("/api/v1/projects/%s/priorities", projKey))
				if err != nil {
					return err
				}
				if err := decodeJSON(resp, &prios); err != nil {
					return err
				}
				prioByRank[projKey] = map[int]uuid.UUID{}
				for _, p := range prios {
					prioByRank[projKey][p.Rank] = p.ID
				}
				return nil
			}
			for _, p := range projectSpecs {
				if err := loadVocab(p.key); err != nil {
					return fmt.Errorf("failed to load vocabulary for %s: %w", p.key, err)
				}
			}

			// OPS speaks severity, not urgency: rename the top rank.
			resp, err = client.patch(
				fmt.Sprintf("/api/v1/projects/OPS/priorities/%s", prioByRank["OPS"][4]),
				map[string]string{"name": "Critical / Blocking"})
			if err != nil {
				return fmt.Errorf("failed to rename OPS priority: %w", err)
			}
			if err := discardBody(resp); err != nil {
				return fmt.Errorf("failed to rename OPS priority: %w", err)
			}

			// -----------------------------------------------------------------
			// 4. Cycles in APP: two finished sprints and an active one.
			// -----------------------------------------------------------------
			fmt.Println("Creating cycles...")

			cycleIDs := map[string]uuid.UUID{}
			for _, c := range []struct {
				name             string
				startAgo, endAgo int
			}{
				{"Sprint 1", 35, 22},
				{"Sprint 2", 21, 8},
				{"Sprint 3", 4, -10}, // active: started 4 days ago, ends in 10
			} {
				resp, err := client.post("/api/v1/projects/APP/cycles", map[string]string{
					"title": c.name, "start_date": date(c.startAgo), "end_date": date(c.endAgo),
				})
				if err != nil {
					return fmt.Errorf("failed to create %s: %w", c.name, err)
				}
				var created cycleResponse
				if err := decodeJSON(resp, &created); err != nil {
					return fmt.Errorf("failed to decode %s: %w", c.name, err)
				}
				cycleIDs[c.name] = created.ID
				fmt.Printf("  %s: %s → %s\n", c.name, date(c.startAgo), date(c.endAgo))
			}

			// -----------------------------------------------------------------
			// 5. Task helpers. Every helper acts through the API as a specific
			//    user and stamps the planned moment.
			// -----------------------------------------------------------------
			type seededTask struct {
				id     uuid.UUID
				number int
				proj   string
			}
			tasks := map[string]seededTask{} // seed-local handle -> task

			createTask := func(handle, projKey, actor, title string, body map[string]interface{}, t time.Time) error {
				c, err := as(actor)
				if err != nil {
					return err
				}
				body["title"] = title
				var created taskResponse
				// The creation cannot be wrapped in act() before the task id
				// exists, so it takes its own marker.
				marker, err := dbNow()
				if err != nil {
					return err
				}
				resp, err := c.post(fmt.Sprintf("/api/v1/projects/%s/tasks", projKey), body)
				if err != nil {
					return fmt.Errorf("create %s: %w", handle, err)
				}
				if err := decodeJSON(resp, &created); err != nil {
					return fmt.Errorf("create %s: %w", handle, err)
				}
				for _, sql := range retimeSQL {
					if _, err := pool.Exec(ctx, sql, t, created.ID, marker); err != nil {
						return fmt.Errorf("retime create %s: %w", handle, err)
					}
				}
				if _, err := pool.Exec(ctx,
					`UPDATE tasks SET created_at=$1, updated_at=$1 WHERE id=$2`, t, created.ID); err != nil {
					return fmt.Errorf("backdate create %s: %w", handle, err)
				}
				tasks[handle] = seededTask{id: created.ID, number: created.TaskNumber, proj: projKey}
				return nil
			}

			move := func(handle, stateName, actor string, t time.Time) error {
				c, err := as(actor)
				if err != nil {
					return err
				}
				tk := tasks[handle]
				sid, ok := stateIDs[tk.proj][stateName]
				if !ok {
					return fmt.Errorf("move %s: unknown state %q in %s", handle, stateName, tk.proj)
				}
				return act(tk.id, t, func() error {
					resp, err := c.patch(fmt.Sprintf("/api/v1/projects/%s/tasks/%d", tk.proj, tk.number),
						map[string]string{"state_id": sid.String()})
					if err != nil {
						return fmt.Errorf("move %s: %w", handle, err)
					}
					return discardBody(resp)
				})
			}

			assign := func(handle, user, actor string, t time.Time) error {
				c, err := as(actor)
				if err != nil {
					return err
				}
				tk := tasks[handle]
				return act(tk.id, t, func() error {
					resp, err := c.post(fmt.Sprintf("/api/v1/projects/%s/tasks/%d/assignees", tk.proj, tk.number),
						map[string]string{"user_id": userIDs[user].String()})
					if err != nil {
						return fmt.Errorf("assign %s: %w", handle, err)
					}
					return discardBody(resp)
				})
			}

			unassign := func(handle, user, actor string, t time.Time) error {
				c, err := as(actor)
				if err != nil {
					return err
				}
				tk := tasks[handle]
				return act(tk.id, t, func() error {
					resp, err := c.delete(fmt.Sprintf("/api/v1/projects/%s/tasks/%d/assignees/%s",
						tk.proj, tk.number, userIDs[user]))
					if err != nil {
						return fmt.Errorf("unassign %s: %w", handle, err)
					}
					return discardBody(resp)
				})
			}

			comment := func(handle, actor, content string, t time.Time) error {
				c, err := as(actor)
				if err != nil {
					return err
				}
				tk := tasks[handle]
				return act(tk.id, t, func() error {
					resp, err := c.post(fmt.Sprintf("/api/v1/projects/%s/tasks/%d/comments", tk.proj, tk.number),
						map[string]string{"content": "<p>" + content + "</p>"})
					if err != nil {
						return fmt.Errorf("comment %s: %w", handle, err)
					}
					return discardBody(resp)
				})
			}

			toCycle := func(handle, cycleName string, t time.Time) error {
				tk := tasks[handle]
				return act(tk.id, t, func() error {
					resp, err := client.post(
						fmt.Sprintf("/api/v1/projects/%s/cycles/%s/tasks", tk.proj, cycleIDs[cycleName]),
						map[string][]string{"task_ids": {tk.id.String()}})
					if err != nil {
						return fmt.Errorf("cycle add %s: %w", handle, err)
					}
					return discardBody(resp)
				})
			}

			fromCycle := func(handle, cycleName string, t time.Time) error {
				tk := tasks[handle]
				return act(tk.id, t, func() error {
					resp, err := client.delete(
						fmt.Sprintf("/api/v1/projects/%s/cycles/%s/tasks/%s", tk.proj, cycleIDs[cycleName], tk.id))
					if err != nil {
						return fmt.Errorf("cycle remove %s: %w", handle, err)
					}
					return discardBody(resp)
				})
			}

			block := func(handle, blockerHandle string, t time.Time) error {
				tk := tasks[handle]
				return act(tk.id, t, func() error {
					resp, err := client.post(
						fmt.Sprintf("/api/v1/projects/%s/tasks/%d/blockers", tk.proj, tk.number),
						map[string]interface{}{"task_ids": []string{tasks[blockerHandle].id.String()}, "relation": "blocked_by"})
					if err != nil {
						return fmt.Errorf("block %s: %w", handle, err)
					}
					return discardBody(resp)
				})
			}

			// step runs a named seeding step and fails loudly.
			type stepFn func() error
			run := func(steps ...stepFn) error {
				for _, s := range steps {
					if err := s(); err != nil {
						return err
					}
				}
				return nil
			}
			s := func(fn func() error) stepFn { return fn }

			// Shorthand builders for createTask bodies.
			ids := func(names ...string) []string {
				out := make([]string, len(names))
				for i, n := range names {
					out[i] = userIDs[n].String()
				}
				return out
			}
			prio := func(projKey string, rank int) string { return prioByRank[projKey][rank].String() }

			// -----------------------------------------------------------------
			// 6. APP: Sprint 1 (finished, light), Sprint 2 (the showcase),
			//    Sprint 3 (active). Times are spread across real days so the
			//    daily charts and weekly bars have shape.
			// -----------------------------------------------------------------
			fmt.Println("Seeding APP (sprints)...")

			err = run(
				// --- Sprint 1: four tasks, three delivered, one still open.
				s(func() error {
					return createTask("s1a", "APP", "alice", "Set up crash reporting", map[string]interface{}{
						"description": "Wire the crash reporter into release builds.",
						"priority_id": prio("APP", 3), "difficulty": 3, "effort": 2,
						"assignees": ids("alice"), "labels": []string{labelIDs["APP"]["feature"].String()},
					}, at(37, "09:10"))
				}),
				s(func() error {
					return createTask("s1b", "APP", "bob", "Fix onboarding typo flood", map[string]interface{}{
						"priority_id": prio("APP", 2), "difficulty": 1, "effort": 1,
						"assignees": ids("bob"), "labels": []string{labelIDs["APP"]["bug"].String()},
					}, at(37, "09:40"))
				}),
				s(func() error {
					return createTask("s1c", "APP", star, "Spike: background sync", map[string]interface{}{
						"priority_id": prio("APP", 2), "assignees": ids(star),
					}, at(36, "11:00"))
				}),
				s(func() error {
					return createTask("s1d", "APP", "charlie", "Harden deep-link parsing", map[string]interface{}{
						"priority_id": prio("APP", 3), "difficulty": 3, "effort": 3,
						"assignees": ids("charlie"),
					}, at(36, "11:30"))
				}),
				s(func() error { return toCycle("s1a", "Sprint 1", at(36, "12:00")) }),
				s(func() error { return toCycle("s1b", "Sprint 1", at(36, "12:01")) }),
				s(func() error { return toCycle("s1c", "Sprint 1", at(36, "12:02")) }),
				s(func() error { return toCycle("s1d", "Sprint 1", at(36, "12:03")) }),
				s(func() error { return move("s1a", "In Progress", "alice", at(33, "10:00")) }),
				s(func() error { return move("s1a", "Done", "alice", at(30, "16:00")) }),
				s(func() error { return move("s1b", "Done", "bob", at(28, "14:30")) }),
				s(func() error { return move("s1c", "In Progress", star, at(29, "09:30")) }),
				s(func() error { return move("s1c", "Done", star, at(24, "17:15")) }),
				// s1d never moved: carried inside Sprint 1 forever.

				// --- Sprint 2 planned work: a parent with three subtasks plus
				//     ten standalone tasks, all in the cycle before it starts.
				s(func() error {
					return createTask("p1", "APP", "alice", "Implement checkout flow", map[string]interface{}{
						"description": "Checkout end to end: UI, payment call, analytics.",
						"priority_id": prio("APP", 3), "assignees": ids("alice"),
						"labels":   []string{labelIDs["APP"]["feature"].String()},
						"areas":    []string{areaIDs["Billing"].String()},
						"watchers": ids("diana"),
					}, at(25, "09:00"))
				}),
				s(func() error {
					return createTask("p1a", "APP", "alice", "Checkout UI", map[string]interface{}{
						"parent_task_number": tasks["p1"].number,
						"difficulty":         3, "effort": 3, "assignees": ids("alice"),
					}, at(25, "09:05"))
				}),
				s(func() error {
					return createTask("p1b", "APP", "alice", "Payment API integration", map[string]interface{}{
						"parent_task_number": tasks["p1"].number,
						"difficulty":         4, "effort": 4, "assignees": ids("bob"),
					}, at(25, "09:06"))
				}),
				s(func() error {
					return createTask("p1c", "APP", "alice", "Checkout analytics events", map[string]interface{}{
						"parent_task_number": tasks["p1"].number,
						"difficulty":         2, "effort": 2,
					}, at(25, "09:07"))
				}),
				s(func() error {
					return createTask("p2", "APP", star, "Fix login crash on Android 15", map[string]interface{}{
						"priority_id": prio("APP", 4), "priority_rating": 9,
						"difficulty": 2, "effort": 3, "assignees": ids(star),
						"labels": []string{labelIDs["APP"]["bug"].String()},
					}, at(24, "10:00"))
				}),
				s(func() error {
					return createTask("p3", "APP", "bob", "Rate limiting for API calls", map[string]interface{}{
						"priority_id": prio("APP", 3), "difficulty": 3, "effort": 4,
						"assignees": ids("bob"), "areas": []string{areaIDs["Platform"].String()},
						"watchers": ids("admin"),
					}, at(24, "10:30"))
				}),
				s(func() error {
					return createTask("p4", "APP", "alice", "Dark mode theming", map[string]interface{}{
						"priority_id": prio("APP", 2), "difficulty": 3, "effort": 3,
						"assignees": ids("alice", star), "areas": []string{areaIDs["UX"].String()},
					}, at(24, "11:00"))
				}),
				s(func() error {
					return createTask("p5", "APP", "charlie", "Migrate image cache", map[string]interface{}{
						"priority_id": prio("APP", 2), "assignees": ids("charlie"),
						"labels": []string{labelIDs["APP"]["tech-debt"].String()},
					}, at(24, "11:30"))
				}),
				s(func() error {
					return createTask("p6", "APP", "diana", "Push notification preferences", map[string]interface{}{
						"priority_id": prio("APP", 2), "difficulty": 2, "effort": 2,
						"assignees": ids("diana"),
					}, at(23, "09:15"))
				}),
				s(func() error {
					return createTask("p7", "APP", "charlie", "Refactor session storage", map[string]interface{}{
						"priority_id": prio("APP", 3), "difficulty": 4, "effort": 2,
						"assignees": ids("charlie"), "areas": []string{areaIDs["Platform"].String()},
						"due_date": at(1, "17:00").Format(time.RFC3339),
					}, at(23, "09:45"))
				}),
				s(func() error {
					return createTask("p8", "APP", "bob", "Upgrade build pipeline", map[string]interface{}{
						"priority_id": prio("APP", 1), "difficulty": 2, "effort": 2,
						"assignees": ids("bob"), "labels": []string{labelIDs["APP"]["tech-debt"].String()},
					}, at(23, "10:15"))
				}),
				s(func() error {
					return createTask("p9", "APP", "admin", "Legacy export cleanup", map[string]interface{}{
						"priority_id": prio("APP", 1), "difficulty": 1, "effort": 2,
					}, at(23, "10:45"))
				}),
				s(func() error {
					return createTask("p10", "APP", "diana", "Beta feedback triage", map[string]interface{}{
						"priority_id": prio("APP", 1), "assignees": ids("diana"),
					}, at(23, "11:15"))
				}),
				s(func() error {
					return createTask("p11", "APP", "charlie", "Offline sync spike", map[string]interface{}{
						"priority_id": prio("APP", 2), "difficulty": 5, "effort": 4,
						"assignees": ids("charlie"),
					}, at(23, "11:45"))
				}),
				// Into Sprint 2 the day before it starts. Subtasks follow their
				// parent, so only the parent joins.
				s(func() error { return toCycle("p1", "Sprint 2", at(22, "15:00")) }),
				s(func() error { return toCycle("p2", "Sprint 2", at(22, "15:01")) }),
				s(func() error { return toCycle("p3", "Sprint 2", at(22, "15:02")) }),
				s(func() error { return toCycle("p4", "Sprint 2", at(22, "15:03")) }),
				s(func() error { return toCycle("p5", "Sprint 2", at(22, "15:04")) }),
				s(func() error { return toCycle("p6", "Sprint 2", at(22, "15:05")) }),
				s(func() error { return toCycle("p7", "Sprint 2", at(22, "15:06")) }),
				s(func() error { return toCycle("p8", "Sprint 2", at(22, "15:07")) }),
				s(func() error { return toCycle("p9", "Sprint 2", at(22, "15:08")) }),
				s(func() error { return toCycle("p10", "Sprint 2", at(22, "15:09")) }),
				s(func() error { return toCycle("p11", "Sprint 2", at(22, "15:10")) }),

				// --- Sprint 2 unfolds, day by day.
				s(func() error { return move("p2", "In Progress", star, at(20, "09:30")) }),
				s(func() error { return move("p1a", "In Progress", "alice", at(20, "10:00")) }),
				s(func() error { return move("p3", "In Progress", "bob", at(19, "09:20")) }),
				s(func() error {
					return comment("p3", "bob", "Sliding window per client; draft in review.", at(19, "15:40"))
				}),
				s(func() error { return move("p2", "Done", star, at(18, "16:45")) }),
				s(func() error { return move("p1b", "In Progress", "bob", at(17, "09:10")) }),
				s(func() error { return move("p6", "In Progress", "diana", at(17, "11:00")) }),
				s(func() error { return move("p7", "In Progress", "charlie", at(16, "09:35")) }),
				// Descoped: the spike leaves the sprint mid-way.
				s(func() error { return fromCycle("p11", "Sprint 2", at(15, "10:00")) }),
				s(func() error {
					return comment("p11", "admin", "Descoped from Sprint 2; revisit after the checkout flow lands.", at(15, "10:01"))
				}),
				s(func() error { return move("p1a", "Done", "alice", at(15, "17:20")) }),
				s(func() error { return move("p6", "Done", "diana", at(14, "15:10")) }),
				// Added mid-sprint, each delivered or carried later.
				s(func() error {
					return createTask("a1", "APP", star, "Hotfix: payment double-charge", map[string]interface{}{
						"priority_id": prio("APP", 4), "priority_rating": 10,
						"difficulty": 2, "effort": 2, "assignees": ids(star),
						"labels": []string{labelIDs["APP"]["bug"].String()},
						"areas":  []string{areaIDs["Billing"].String()},
					}, at(16, "08:30"))
				}),
				s(func() error { return toCycle("a1", "Sprint 2", at(16, "08:35")) }),
				s(func() error { return move("a1", "In Progress", star, at(16, "09:00")) }),
				s(func() error { return move("a1", "Done", star, at(15, "12:40")) }),
				s(func() error {
					return createTask("a2", "APP", "alice", "App store review fixes", map[string]interface{}{
						"priority_id": prio("APP", 3), "difficulty": 1, "effort": 3,
						"assignees": ids("alice"),
					}, at(13, "09:50"))
				}),
				s(func() error { return toCycle("a2", "Sprint 2", at(13, "09:55")) }),
				s(func() error {
					return createTask("a3", "APP", "bob", "Crash reporting noise", map[string]interface{}{
						"priority_id": prio("APP", 2), "assignees": ids("bob"),
						"labels": []string{labelIDs["APP"]["bug"].String()},
					}, at(12, "10:20"))
				}),
				s(func() error { return toCycle("a3", "Sprint 2", at(12, "10:25")) }),
				// The hand-over: bob builds, hands to Erik for verification, and
				// the unassignment while the task is started records bob as a
				// contributor automatically — delivery credit survives.
				s(func() error { return move("p3", "Testing", "bob", at(13, "11:00")) }),
				s(func() error { return comment("p3", "bob", "Ready for verification on staging.", at(13, "11:01")) }),
				s(func() error { return unassign("p3", "bob", "admin", at(13, "11:05")) }),
				s(func() error { return assign("p3", "erik", "admin", at(13, "11:06")) }),
				s(func() error { return move("p3", "Done", "erik", at(12, "14:50")) }),
				s(func() error { return move("p1b", "Done", "bob", at(12, "16:30")) }),
				s(func() error {
					return comment("p1", "alice", "Two of three checkout subtasks done; analytics events remain.", at(12, "17:00"))
				}),
				s(func() error { return move("p1", "In Progress", "alice", at(20, "10:01")) }),
				// Cancelled and archived outcomes stay distinct from delivery.
				s(func() error { return move("p9", "Cancelled", "admin", at(11, "10:30")) }),
				s(func() error { return move("p10", "Archived", "admin", at(9, "15:00")) }),
				s(func() error { return move("p4", "In Progress", "alice", at(11, "09:25")) }),
				s(func() error { return move("p4", "Done", "alice", at(10, "15:45")) }),
				s(func() error { return move("p5", "In Progress", "charlie", at(10, "09:40")) }),
				s(func() error { return move("p5", "Done", "charlie", at(9, "11:20")) }),
				s(func() error { return move("a2", "In Progress", "alice", at(10, "10:10")) }),
				s(func() error { return move("a2", "Done", "alice", at(9, "16:05")) }),

				// --- Between sprints: carry the open work over to Sprint 3.
				s(func() error { return fromCycle("p1", "Sprint 2", at(7, "09:00")) }),
				s(func() error { return toCycle("p1", "Sprint 3", at(4, "09:00")) }),
				s(func() error { return fromCycle("p7", "Sprint 2", at(7, "09:01")) }),
				s(func() error { return toCycle("p7", "Sprint 3", at(4, "09:01")) }),
				s(func() error { return fromCycle("p8", "Sprint 2", at(7, "09:02")) }),
				s(func() error { return toCycle("p8", "Sprint 3", at(4, "09:02")) }),
				s(func() error { return fromCycle("a3", "Sprint 2", at(7, "09:03")) }),
				s(func() error { return toCycle("a3", "Sprint 3", at(4, "09:03")) }),

				// --- Sprint 3, active: fresh work plus the carried items.
				s(func() error {
					return createTask("n1", "APP", "alice", "Onboarding revamp", map[string]interface{}{
						"priority_id": prio("APP", 3), "difficulty": 3, "effort": 3,
						"assignees": ids("alice"), "areas": []string{areaIDs["UX"].String()},
						"due_date": day0.AddDate(0, 0, 3).Add(17 * time.Hour).Format(time.RFC3339),
					}, at(4, "10:00"))
				}),
				s(func() error {
					return createTask("n2", "APP", star, "Settings search", map[string]interface{}{
						"priority_id": prio("APP", 2), "difficulty": 2, "effort": 2,
						"assignees": ids(star),
					}, at(4, "10:30"))
				}),
				s(func() error {
					return createTask("n3", "APP", "bob", "Perf: cold start under two seconds", map[string]interface{}{
						"priority_id": prio("APP", 3), "difficulty": 4, "effort": 4,
						"assignees": ids("bob"), "areas": []string{areaIDs["Platform"].String()},
					}, at(4, "11:00"))
				}),
				s(func() error { return toCycle("n1", "Sprint 3", at(4, "11:30")) }),
				s(func() error { return toCycle("n2", "Sprint 3", at(4, "11:31")) }),
				s(func() error { return toCycle("n3", "Sprint 3", at(4, "11:32")) }),
				s(func() error { return move("n1", "In Progress", "alice", at(2, "09:30")) }),
				s(func() error { return move("n2", "In Progress", star, at(3, "09:45")) }),
				s(func() error { return move("n2", "Done", star, at(1, "14:20")) }),
				s(func() error { return move("p8", "In Progress", "bob", at(3, "10:15")) }),
				s(func() error { return move("p8", "Done", "bob", at(2, "16:40")) }),
				// Added yesterday, delivered today: visible in "last 7 days".
				s(func() error {
					return createTask("n4", "APP", "erik", "Hotfix: widget crash on resize", map[string]interface{}{
						"priority_id": prio("APP", 4), "difficulty": 1, "effort": 2,
						"assignees": ids("erik"), "labels": []string{labelIDs["APP"]["bug"].String()},
					}, at(1, "08:40"))
				}),
				s(func() error { return toCycle("n4", "Sprint 3", at(1, "08:45")) }),
				s(func() error { return move("n4", "In Progress", "erik", at(1, "09:00")) }),
				s(func() error { return move("n4", "Done", "erik", at(0, "10:30")) }),
				// A live blocker: the cold-start work waits on the revamp.
				s(func() error { return block("n3", "n1", at(2, "10:00")) }),
				s(func() error { return move("p1c", "Todo", "alice", at(3, "09:00")) }),
			)
			if err != nil {
				return err
			}

			// -----------------------------------------------------------------
			// 7. OPS: a no-cycles project with work spread over the month, an
			//    aging in-progress item, a blocked one and open top-severity
			//    work (so the portfolio attention rule has something to say).
			// -----------------------------------------------------------------
			fmt.Println("Seeding OPS (no cycles)...")

			err = run(
				s(func() error {
					return createTask("o1", "OPS", "admin", "Rotate TLS certificates", map[string]interface{}{
						"priority_id": prio("OPS", 3), "difficulty": 2, "effort": 2,
						"assignees": ids("admin"), "labels": []string{labelIDs["OPS"]["maintenance"].String()},
					}, at(26, "09:00"))
				}),
				s(func() error { return move("o1", "In Progress", "admin", at(21, "10:00")) }),
				s(func() error { return move("o1", "Done", "admin", at(20, "15:30")) }),
				s(func() error {
					return createTask("o2", "OPS", "charlie", "Postgres minor upgrade", map[string]interface{}{
						"priority_id": prio("OPS", 3), "difficulty": 3, "effort": 2,
						"assignees": ids("charlie"), "labels": []string{labelIDs["OPS"]["maintenance"].String()},
					}, at(19, "09:30"))
				}),
				s(func() error { return move("o2", "In Progress", "charlie", at(16, "09:00")) }),
				s(func() error { return move("o2", "Done", "charlie", at(14, "17:00")) }),
				s(func() error {
					return createTask("o3", "OPS", star, "On-call runbook update", map[string]interface{}{
						"priority_id": prio("OPS", 2), "assignees": ids(star),
					}, at(10, "11:00"))
				}),
				s(func() error { return move("o3", "Done", star, at(6, "16:20")) }),
				s(func() error {
					return createTask("o4", "OPS", "alice", "Disk alerts misfire at 80%", map[string]interface{}{
						"priority_id": prio("OPS", 2), "difficulty": 2, "effort": 1,
						"assignees": ids("alice"), "labels": []string{labelIDs["OPS"]["incident"].String()},
					}, at(7, "08:50"))
				}),
				s(func() error { return move("o4", "In Progress", "alice", at(5, "09:10")) }),
				s(func() error { return move("o4", "Done", "alice", at(3, "13:40")) }),
				s(func() error {
					return createTask("o5", "OPS", star, "Zero-downtime deploy script", map[string]interface{}{
						"priority_id": prio("OPS", 3), "difficulty": 4, "effort": 3,
						"assignees": ids(star),
					}, at(12, "10:30"))
				}),
				s(func() error { return move("o5", "In Progress", star, at(9, "09:20")) }),
				s(func() error {
					return comment("o5", star, "Draining connections cleanly; rollback path still to prove.", at(8, "17:30"))
				}),
				s(func() error {
					return createTask("o6", "OPS", "charlie", "Backup restore drill", map[string]interface{}{
						"priority_id": prio("OPS", 2), "difficulty": 2, "effort": 3,
						"assignees": ids("charlie"),
						"due_date":  day0.AddDate(0, 0, 2).Add(17 * time.Hour).Format(time.RFC3339),
					}, at(6, "10:00"))
				}),
				s(func() error { return move("o6", "Todo", "charlie", at(6, "10:05")) }),
				s(func() error {
					return createTask("o7", "OPS", "admin", "CDN migration plan", map[string]interface{}{
						"priority_id": prio("OPS", 4), "assignees": ids("admin"),
					}, at(5, "09:40"))
				}),
				s(func() error {
					return createTask("o8", "OPS", "erik", "Log retention policy", map[string]interface{}{
						"priority_id": prio("OPS", 1),
					}, at(2, "11:10"))
				}),
				s(func() error {
					return createTask("o9", "OPS", "erik", "SSO session bug", map[string]interface{}{
						"priority_id": prio("OPS", 3), "difficulty": 2, "effort": 2,
						"assignees": ids("erik"), "labels": []string{labelIDs["OPS"]["incident"].String()},
					}, at(8, "09:00"))
				}),
				s(func() error { return move("o9", "In Progress", "erik", at(7, "10:30")) }),
				s(func() error { return block("o9", "o5", at(7, "10:35")) }),
			)
			if err != nil {
				return err
			}

			// -----------------------------------------------------------------
			// 8. LAB: nearly dormant — too little recent history to trend.
			// -----------------------------------------------------------------
			fmt.Println("Seeding LAB (dormant)...")

			err = run(
				s(func() error {
					return createTask("l1", "LAB", "diana", "Prototype voice notes", map[string]interface{}{
						"priority_id": prio("LAB", 2), "assignees": ids("diana"),
					}, at(41, "10:00"))
				}),
				s(func() error {
					return createTask("l2", "LAB", "admin", "Evaluate vector database", map[string]interface{}{
						"priority_id": prio("LAB", 1),
					}, at(39, "11:30"))
				}),
				s(func() error {
					return createTask("l3", "LAB", "diana", "Curate research reading list", map[string]interface{}{
						"priority_id": prio("LAB", 1), "assignees": ids("diana"),
					}, at(38, "09:45"))
				}),
				s(func() error { return move("l3", "Done", "diana", at(33, "15:00")) }),
			)
			if err != nil {
				return err
			}

			// -----------------------------------------------------------------
			// 9. Recompute every task's activity checksum chain: retiming
			//    changed created_at, which is part of the chained hash.
			// -----------------------------------------------------------------
			fmt.Println("Recomputing activity checksums...")

			taskRows, err := pool.Query(ctx, `SELECT id FROM tasks`)
			if err != nil {
				return fmt.Errorf("failed to list tasks: %w", err)
			}
			var allTaskIDs []uuid.UUID
			for taskRows.Next() {
				var id uuid.UUID
				if err := taskRows.Scan(&id); err != nil {
					taskRows.Close()
					return err
				}
				allTaskIDs = append(allTaskIDs, id)
			}
			taskRows.Close()

			for _, taskID := range allTaskIDs {
				rows, err := pool.Query(ctx,
					`SELECT id, task_id, activity_type, actor_id, field_name, old_value, new_value, created_at
					 FROM activity_log WHERE task_id = $1 ORDER BY created_at ASC, id ASC`, taskID)
				if err != nil {
					return fmt.Errorf("failed to fetch activity for recompute: %w", err)
				}
				type activityRow struct {
					id           uuid.UUID
					taskID       uuid.UUID
					activityType string
					actorID      uuid.UUID
					fieldName    pgtype.Text
					oldValue     []byte
					newValue     []byte
					createdAt    pgtype.Timestamptz
				}
				var activities []activityRow
				for rows.Next() {
					var a activityRow
					if err := rows.Scan(&a.id, &a.taskID, &a.activityType, &a.actorID,
						&a.fieldName, &a.oldValue, &a.newValue, &a.createdAt); err != nil {
						rows.Close()
						return fmt.Errorf("failed to scan activity row: %w", err)
					}
					activities = append(activities, a)
				}
				rows.Close()

				previousChecksum := "0000000000000000000000000000000000000000000000000000000000000000"
				for _, a := range activities {
					var fieldName *string
					if a.fieldName.Valid {
						fieldName = &a.fieldName.String
					}
					checksum := computeChecksum(previousChecksum, a.taskID, a.activityType, a.actorID,
						fieldName, a.oldValue, a.newValue, a.createdAt.Time)
					if _, err := pool.Exec(ctx,
						`UPDATE activity_log SET checksum = $1 WHERE id = $2`, checksum, a.id); err != nil {
						return fmt.Errorf("failed to update checksum: %w", err)
					}
					previousChecksum = checksum
				}
			}

			// Entity timestamps that are display-only: tasks follow their last
			// event; projects look suitably old.
			if _, err := pool.Exec(ctx,
				`UPDATE tasks SET updated_at = GREATEST(created_at,
				   COALESCE((SELECT MAX(a.created_at) FROM activity_log a WHERE a.task_id = tasks.id), created_at))`); err != nil {
				return fmt.Errorf("failed to normalize task updated_at: %w", err)
			}
			if _, err := pool.Exec(ctx,
				`UPDATE projects SET created_at = $1, updated_at = $1`, now0.Add(-45*24*time.Hour)); err != nil {
				return fmt.Errorf("failed to backdate projects: %w", err)
			}

			fmt.Println("\nDemo data populated successfully!")
			fmt.Println("\nWhat to look at:")
			fmt.Println("  /insights                 portfolio: OPS should fire the attention rule")
			fmt.Println("  /insights APP + Sprint 2  the showcase: planned/added/descoped/carried,")
			fmt.Println("                            delivery via subtasks, a hand-over credit (bob),")
			fmt.Println("                            shared credit, a cancelled and an archived outcome")
			fmt.Println("  /insights OPS             the no-cycles date-range report")
			fmt.Println("\nCredentials:")
			fmt.Println("  Admin:  admin@example.com / admin123")
			if demoEmail != "" && demoPassword != "" {
				fmt.Printf("  Demo:   %s / %s\n", demoEmail, demoPassword)
			}
			fmt.Println("  Users:  alice@example.com / user123 (bob, charlie, diana, erik alike)")

			return nil
		},
	}
}

// computeChecksum replicates the checksum calculation from
// internal/activity/activity.go so we can recompute after backdating.
func computeChecksum(
	previousChecksum string,
	taskID uuid.UUID,
	activityType string,
	actorID uuid.UUID,
	fieldName *string,
	oldValue []byte,
	newValue []byte,
	timestamp time.Time,
) string {
	fieldNameStr := ""
	if fieldName != nil {
		fieldNameStr = *fieldName
	}

	// Normalize JSON values the same way activity.go does during verification.
	oldValue = normalizeJSON(oldValue)
	newValue = normalizeJSON(newValue)

	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		previousChecksum,
		taskID.String(),
		activityType,
		actorID.String(),
		fieldNameStr,
		string(oldValue),
		string(newValue),
		timestamp.Format(time.RFC3339Nano),
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// normalizeJSON unmarshals and re-marshals JSON to ensure consistent key ordering.
func normalizeJSON(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	normalized, err := json.Marshal(v)
	if err != nil {
		return data
	}
	return normalized
}
