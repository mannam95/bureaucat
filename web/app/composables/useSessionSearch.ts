import type { WritableComputedRef } from "vue";

// Free-text search text, kept session-only per (user, surface, project).
//
// Search is deliberately never written to the durable preference store. It is
// backed by sessionStorage, so it survives a reload in the same browser tab but
// not a new tab or a new browser session — the safer default for a transient,
// per-context search. Values are namespaced by user id so a shared browser
// never leaks one account's search into another's, and cleared on logout.

const SEARCH_PREFIX = "bc:search:v1";

export type SearchSurface = "tasks" | "board";

function storageKey(userId: string, surface: SearchSurface, projectKey: string): string {
  return `${SEARCH_PREFIX}:${userId}:${surface}:${projectKey}`;
}

// In-tab reactive mirror of sessionStorage, so components sharing a
// (surface, project) stay in sync without re-reading storage on every keystroke.
const cache = reactive<Record<string, string>>({});

export function useSessionSearch() {
  const { user } = useAuth();

  function keyFor(surface: SearchSurface, getProjectKey: () => string): string | null {
    const uid = user.value?.id;
    const pk = getProjectKey();
    if (!uid || !pk) return null;
    return storageKey(uid, surface, pk);
  }

  // A v-model-friendly ref bound to the search text for (surface, project).
  // getProjectKey is a getter so the ref follows the active project reactively.
  function searchRef(
    surface: SearchSurface,
    getProjectKey: () => string,
  ): WritableComputedRef<string> {
    return computed<string>({
      get: () => {
        const key = keyFor(surface, getProjectKey);
        if (!key) return "";
        // Reading cache[key] registers the reactive dependency; fall back to
        // sessionStorage for the initial value after a reload (pure read).
        const cached = cache[key];
        if (cached !== undefined) return cached;
        if (import.meta.client) {
          try {
            return sessionStorage.getItem(key) ?? "";
          } catch {
            return "";
          }
        }
        return "";
      },
      set: (v) => {
        const key = keyFor(surface, getProjectKey);
        if (!key) return;
        cache[key] = v;
        if (import.meta.client) {
          try {
            if (v) sessionStorage.setItem(key, v);
            else sessionStorage.removeItem(key);
          } catch {
            // storage unavailable — the in-memory cache still works this session.
          }
        }
      },
    });
  }

  // Drop every stored search for the current browser session. Called on logout
  // and account switch so a signed-out account's search never lingers.
  function clearAll() {
    for (const k of Object.keys(cache)) delete cache[k];
    if (!import.meta.client) return;
    try {
      const toRemove: string[] = [];
      for (let i = 0; i < sessionStorage.length; i++) {
        const k = sessionStorage.key(i);
        if (k && k.startsWith(`${SEARCH_PREFIX}:`)) toRemove.push(k);
      }
      for (const k of toRemove) sessionStorage.removeItem(k);
    } catch {
      // ignore
    }
  }

  return { searchRef, clearAll };
}
