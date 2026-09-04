// Show/hide state for the right-hand Progress / by-State panel on a detail page.
// Backed by the durable preference store at global scope, and kept separate for
// cycles and modules so hiding it on a cycle doesn't hide it on a module (the
// choice still applies across every cycle, and across every module).
export function useDetailPanel(entity: "cycle" | "module") {
  const key =
    entity === "cycle" ? "cycles.detail.panel_visible" : "modules.detail.panel_visible";
  const showDetailPanel = usePreferences().globalRef<boolean>(key, true);
  return { showDetailPanel };
}
