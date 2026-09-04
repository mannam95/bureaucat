export type ViewMode = "card" | "list";

// Persisted card/list view preference for an overview (modules, cycles). Keyed
// so each overview remembers its own choice across visits.
export function useViewMode(key: string, initial: ViewMode = "card") {
  const mode = ref<ViewMode>(initial);
  if (import.meta.client) {
    const saved = localStorage.getItem(key);
    if (saved === "card" || saved === "list") mode.value = saved;
    watch(mode, (v) => localStorage.setItem(key, v));
  }
  return mode;
}
