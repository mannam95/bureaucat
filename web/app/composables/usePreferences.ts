import type { WritableComputedRef } from "vue";
import { toast } from "vue-sonner";

// Durable user preferences, backed by the Go preference API and PostgreSQL.
//
// Design (matches the agreed architecture):
//   - PostgreSQL is authoritative. Each authenticated session hydrates the
//     effective preference set from the API before it trusts local state.
//   - Global preferences load once per session (after auth). Project
//     preferences load the first time a project is entered.
//   - Writes are optimistic for the UI but database-first for anything durable:
//     the visible value updates immediately, and the acknowledged value/revision
//     only advance once the server confirms the write.
//   - Concurrency is handled with a per-(scope,key) revision. A stale write gets
//     a 409; we refetch and retry the newest draft once against the fresh
//     revision.
//   - This is a plain reactive-singleton composable (the pattern used across the
//     app); no external store library.

export type PreferenceScope = "global" | "project";

// One effective preference held in memory.
interface PreferenceEntry {
  // The value the UI should show. May be an un-acknowledged optimistic draft.
  value: unknown;
  // The last value the server confirmed. Used to roll back on failure.
  acknowledged: unknown;
  // Server revision for optimistic concurrency; 0 means "no stored row yet".
  revision: number;
  valueVersion: number;
  source: "default" | "user";
  status: "ready" | "saving" | "error";
  error: string | null;
}

// Wire shape returned by the API.
interface ApiEntry {
  key: string;
  scope: string;
  value: unknown;
  value_version: number;
  revision: number;
  source: "default" | "user";
}

interface ApiListResponse {
  scope: string;
  preferences: ApiEntry[];
}

// Serializes writes per (scope, key) and always sends the newest draft.
interface Writer {
  inFlight: boolean;
  queued: boolean;
  queuedValue: unknown;
}

// ---- singleton state ----
const state = reactive({
  globalLoaded: false,
  globalLoading: false,
  global: {} as Record<string, PreferenceEntry>,
  projects: {} as Record<string, Record<string, PreferenceEntry>>,
  projectLoaded: {} as Record<string, boolean>,
  projectLoading: {} as Record<string, boolean>,
});

const writers = new Map<string, Writer>();

// Tracks an in-flight global hydration so concurrent callers await the same
// request instead of racing (e.g. the auth plugin and workspace selection).
let globalHydration: Promise<void> | null = null;

// Same idea per project, so a detail page and the project page can both request
// a project's preferences without triggering a double fetch.
const projectHydration = new Map<string, Promise<void>>();

// ---- helpers ----

function sameValue(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

function toEntry(e: ApiEntry): PreferenceEntry {
  return {
    value: e.value,
    acknowledged: e.value,
    revision: e.revision,
    valueVersion: e.value_version,
    source: e.source,
    status: "ready",
    error: null,
  };
}

function getEntry(
  scope: PreferenceScope,
  projectKey: string | undefined,
  key: string,
): PreferenceEntry | undefined {
  if (scope === "global") return state.global[key];
  return projectKey ? state.projects[projectKey]?.[key] : undefined;
}

function ensureEntry(
  scope: PreferenceScope,
  projectKey: string | undefined,
  key: string,
  fallback: unknown,
): PreferenceEntry {
  const existing = getEntry(scope, projectKey, key);
  if (existing) return existing;
  const fresh: PreferenceEntry = {
    value: fallback,
    acknowledged: fallback,
    revision: 0,
    valueVersion: 1,
    source: "default",
    status: "ready",
    error: null,
  };
  if (scope === "global") {
    state.global[key] = fresh;
  } else if (projectKey) {
    if (!state.projects[projectKey]) state.projects[projectKey] = {};
    state.projects[projectKey][key] = fresh;
  }
  // Return the reactive proxy, not the raw object, so mutations are tracked.
  return getEntry(scope, projectKey, key)!;
}

function scopeUrl(scope: PreferenceScope, projectKey?: string): string {
  return scope === "global"
    ? "/api/v1/me/preferences/global"
    : `/api/v1/projects/${projectKey}/preferences`;
}

function keyUrl(scope: PreferenceScope, projectKey: string | undefined, key: string): string {
  return scope === "global"
    ? `/api/v1/me/preferences/global/${key}`
    : `/api/v1/projects/${projectKey}/preferences/${key}`;
}

async function fetchScope(scope: PreferenceScope, projectKey?: string): Promise<ApiEntry[]> {
  const { getAuthHeader } = useAuth();
  const res = await fetch(scopeUrl(scope, projectKey), {
    headers: { ...getAuthHeader() },
    credentials: "include",
  });
  if (!res.ok) throw new Error(`preferences ${scope} load failed (${res.status})`);
  const data = (await res.json()) as ApiListResponse;
  return data.preferences ?? [];
}

async function fetchServerEntry(
  scope: PreferenceScope,
  projectKey: string | undefined,
  key: string,
): Promise<ApiEntry | undefined> {
  const entries = await fetchScope(scope, projectKey);
  return entries.find((e) => e.key === key);
}

interface WriteError {
  code: number;
  message?: string;
}

async function putRequest(
  scope: PreferenceScope,
  projectKey: string | undefined,
  key: string,
  value: unknown,
  expectedRevision: number,
  valueVersion: number,
): Promise<ApiEntry> {
  const { getAuthHeader } = useAuth();
  const res = await fetch(keyUrl(scope, projectKey, key), {
    method: "PUT",
    headers: { "Content-Type": "application/json", ...getAuthHeader() },
    credentials: "include",
    body: JSON.stringify({
      value,
      value_version: valueVersion,
      expected_revision: expectedRevision,
    }),
  });
  if (!res.ok) {
    let message: string | undefined;
    if (res.status === 422) {
      const body = await res.json().catch(() => ({}));
      message = body?.message;
    }
    throw { code: res.status, message } as WriteError;
  }
  return (await res.json()) as ApiEntry;
}

function notifyError(message: string): void {
  try {
    toast.error(message);
  } catch {
    // toast host not mounted (e.g. during startup) — swallow.
  }
}

function writerId(scope: PreferenceScope, projectKey: string | undefined, key: string): string {
  return `${scope}:${projectKey ?? ""}:${key}`;
}

// Drains the write queue for one (scope, key). Only one drain runs at a time;
// while it runs, newer drafts overwrite the queued value so the latest wins.
async function flush(
  id: string,
  scope: PreferenceScope,
  projectKey: string | undefined,
  key: string,
): Promise<void> {
  const w = writers.get(id);
  if (!w || w.inFlight) return;
  w.inFlight = true;
  let conflictRetries = 0;
  try {
    while (w.queued) {
      w.queued = false;
      const valueToSend = w.queuedValue;
      const entry = getEntry(scope, projectKey, key);
      if (!entry) break;
      entry.status = "saving";
      entry.error = null;
      try {
        const acked = await putRequest(
          scope,
          projectKey,
          key,
          valueToSend,
          entry.revision,
          entry.valueVersion || 1,
        );
        entry.acknowledged = acked.value;
        entry.revision = acked.revision;
        entry.source = acked.source;
        entry.valueVersion = acked.value_version;
        // Don't clobber a newer draft the user just made.
        if (!w.queued) {
          entry.value = acked.value;
          entry.status = "ready";
        }
        conflictRetries = 0;
      } catch (e) {
        const err = e as WriteError;
        if (err?.code === 409 && conflictRetries < 2) {
          conflictRetries++;
          const server = await fetchServerEntry(scope, projectKey, key).catch(() => undefined);
          if (server) {
            entry.revision = server.revision;
            entry.acknowledged = server.value;
            entry.source = server.source;
            entry.valueVersion = server.value_version;
            if (!sameValue(entry.value, server.value)) {
              // The user still wants their draft — retry once against the fresh
              // revision.
              w.queued = true;
              w.queuedValue = entry.value;
            } else {
              entry.value = server.value;
              entry.status = "ready";
            }
          }
        } else if (err?.code === 422) {
          entry.value = entry.acknowledged;
          entry.status = "error";
          entry.error = err.message || "That value is not allowed.";
          notifyError(entry.error);
        } else {
          entry.value = entry.acknowledged;
          entry.status = "error";
          entry.error = err?.message || "Could not save.";
          notifyError("Couldn't save your preference — the previous value was restored.");
        }
      }
    }
  } finally {
    w.inFlight = false;
  }
}

function enqueue(scope: PreferenceScope, projectKey: string | undefined, key: string): void {
  const entry = getEntry(scope, projectKey, key);
  if (!entry) return;
  const id = writerId(scope, projectKey, key);
  let w = writers.get(id);
  if (!w) {
    w = { inFlight: false, queued: false, queuedValue: undefined };
    writers.set(id, w);
  }
  w.queued = true;
  w.queuedValue = entry.value;
  void flush(id, scope, projectKey, key);
}

// ---- hydration ----

async function hydrateGlobal(force = false): Promise<void> {
  if (state.globalLoaded && !force) return;
  if (globalHydration) return globalHydration;
  globalHydration = (async () => {
    state.globalLoading = true;
    try {
      const entries = await fetchScope("global");
      for (const e of entries) {
        // Never clobber an entry that has an in-flight optimistic write.
        if (state.global[e.key]?.status === "saving") continue;
        state.global[e.key] = toEntry(e);
      }
      state.globalLoaded = true;
    } catch (e) {
      // Leave globalLoaded false so a later navigation can retry.
      console.error("[preferences] global hydration failed", e);
    } finally {
      state.globalLoading = false;
      globalHydration = null;
    }
  })();
  return globalHydration;
}

async function hydrateProject(projectKey: string, force = false): Promise<void> {
  if (!projectKey) return;
  if (state.projectLoaded[projectKey] && !force) return;
  const existing = projectHydration.get(projectKey);
  if (existing) return existing;
  const p = (async () => {
    state.projectLoading[projectKey] = true;
    try {
      const entries = await fetchScope("project", projectKey);
      if (!state.projects[projectKey]) state.projects[projectKey] = {};
      for (const e of entries) {
        if (state.projects[projectKey][e.key]?.status === "saving") continue;
        state.projects[projectKey][e.key] = toEntry(e);
      }
      state.projectLoaded[projectKey] = true;
    } catch (e) {
      console.error(`[preferences] project hydration failed (${projectKey})`, e);
    } finally {
      state.projectLoading[projectKey] = false;
      projectHydration.delete(projectKey);
    }
  })();
  projectHydration.set(projectKey, p);
  return p;
}

// ---- mutations ----

function writeValue(
  scope: PreferenceScope,
  projectKey: string | undefined,
  key: string,
  value: unknown,
): void {
  const existing = getEntry(scope, projectKey, key);
  // No-op if this exact value is already the committed (acknowledged) state, so
  // re-selecting the current value never fires a redundant write.
  if (existing && sameValue(existing.value, value) && sameValue(existing.acknowledged, value)) {
    return;
  }
  const entry = existing ?? ensureEntry(scope, projectKey, key, value);
  entry.value = value; // optimistic
  enqueue(scope, projectKey, key);
}

async function resetValue(
  scope: PreferenceScope,
  projectKey: string | undefined,
  key: string,
): Promise<void> {
  const { getAuthHeader } = useAuth();
  try {
    const res = await fetch(keyUrl(scope, projectKey, key), {
      method: "DELETE",
      headers: { ...getAuthHeader() },
      credentials: "include",
    });
    if (!res.ok) throw new Error(`reset failed (${res.status})`);
    const def = (await res.json()) as ApiEntry;
    const entry = ensureEntry(scope, projectKey, key, def.value);
    entry.value = def.value;
    entry.acknowledged = def.value;
    entry.revision = def.revision;
    entry.valueVersion = def.value_version;
    entry.source = def.source;
    entry.status = "ready";
    entry.error = null;
  } catch (e) {
    console.error(`[preferences] reset failed (${key})`, e);
    notifyError("Couldn't reset that preference.");
  }
}

function clearAll(): void {
  for (const k of Object.keys(state.global)) delete state.global[k];
  for (const k of Object.keys(state.projects)) delete state.projects[k];
  for (const k of Object.keys(state.projectLoaded)) delete state.projectLoaded[k];
  for (const k of Object.keys(state.projectLoading)) delete state.projectLoading[k];
  state.globalLoaded = false;
  state.globalLoading = false;
  writers.clear();
  globalHydration = null;
  projectHydration.clear();
}

// ---- reactive accessors ----

// A v-model-friendly ref bound to a global preference. Reads the effective
// value (falling back until hydration completes); writing it updates the UI
// immediately and persists in the background.
function globalRef<T>(key: string, fallback: T): WritableComputedRef<T> {
  return computed<T>({
    get: () => (state.global[key]?.value ?? fallback) as T,
    set: (v: T) => writeValue("global", undefined, key, v),
  });
}

// A v-model-friendly ref bound to a project-scoped preference.
function projectRef<T>(projectKey: string, key: string, fallback: T): WritableComputedRef<T> {
  return computed<T>({
    get: () => (state.projects[projectKey]?.[key]?.value ?? fallback) as T,
    set: (v: T) => writeValue("project", projectKey, key, v),
  });
}

export function usePreferences() {
  return {
    globalLoaded: computed(() => state.globalLoaded),
    isProjectLoaded: (projectKey: string) => !!state.projectLoaded[projectKey],
    hydrateGlobal,
    hydrateProject,
    clearAll,
    globalRef,
    projectRef,
    // Imperative accessors for surfaces that read/write outside a v-model.
    getGlobal: <T>(key: string, fallback: T): T => (state.global[key]?.value ?? fallback) as T,
    setGlobal: (key: string, value: unknown) => writeValue("global", undefined, key, value),
    getProject: <T>(projectKey: string, key: string, fallback: T): T =>
      (state.projects[projectKey]?.[key]?.value ?? fallback) as T,
    setProject: (projectKey: string, key: string, value: unknown) =>
      writeValue("project", projectKey, key, value),
    resetGlobal: (key: string) => resetValue("global", undefined, key),
    resetProject: (projectKey: string, key: string) => resetValue("project", projectKey, key),
    // Status accessor for surfaces that want to show saving/error state.
    entry: (scope: PreferenceScope, projectKey: string | undefined, key: string) =>
      getEntry(scope, projectKey, key),
  };
}
