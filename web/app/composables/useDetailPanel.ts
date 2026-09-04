// Shared show/hide state for the right-hand Progress / by-State panel on the
// cycle and module detail pages. The panel is liked but shouldn't permanently
// eat table width, so it is collapsible on demand; the choice is persisted and
// shared across both detail views.
const PANEL_KEY = "bc:detail-panel";
const showDetailPanel = ref(true);
let hydrated = false;

export function useDetailPanel() {
  if (import.meta.client && !hydrated) {
    hydrated = true;
    const saved = localStorage.getItem(PANEL_KEY);
    if (saved !== null) showDetailPanel.value = saved === "1";
    watch(showDetailPanel, (v) => localStorage.setItem(PANEL_KEY, v ? "1" : "0"));
  }
  return { showDetailPanel };
}
