// Shared "All workspaces" preference for the dashboard's "Assigned to You"
// section. Exposed as a composable (mirroring useAuth/useProjects) so the global
// Create Task dialog opened via Shift+C honors the same toggle while the user is
// on the dashboard. Backed by the durable preference store at global scope;
// defaults to true (all workspaces) until the user turns it off.
export function useDashboardScope() {
  const showAllWorkspaces = usePreferences().globalRef<boolean>(
    "dashboard.show_all_workspaces",
    true,
  );
  return { showAllWorkspaces };
}
