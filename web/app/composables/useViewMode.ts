export type ViewMode = "card" | "list";

// Card/list view preference for an overview (cycles, modules). Backed by the
// durable preference store at global scope, so the choice follows the user
// across devices and reloads. `key` is the canonical preference key, e.g.
// "cycles.overview.view_mode". Returns a v-model-friendly ref: reading gives the
// effective value, writing persists in the background.
export function useViewMode(key: string, fallback: ViewMode = "card") {
  return usePreferences().globalRef<ViewMode>(key, fallback);
}
