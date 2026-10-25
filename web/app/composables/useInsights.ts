import type {
  EvidenceQuery,
  InsightsEvidence,
  InsightsPortfolio,
  PersonInsights,
  ProjectInsights,
} from "~/types";

// InsightsWindow narrows every call to a cycle or a date range; empty fields
// mean the backend default (last 30 days).
export interface InsightsWindow {
  cycleId?: string;
  from?: string;
  to?: string;
}

function windowParams(w: InsightsWindow): URLSearchParams {
  const p = new URLSearchParams();
  if (w.cycleId) p.set("cycle_id", w.cycleId);
  if (w.from) p.set("from", w.from);
  if (w.to) p.set("to", w.to);
  return p;
}

export function useInsights() {
  const { getAuthHeader } = useAuth();

  async function get<T>(url: string): Promise<{ success: boolean; data?: T; error?: string }> {
    try {
      const response = await fetch(url, { headers: getAuthHeader() });
      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || `Request failed (${response.status})` };
      }
      return { success: true, data: (await response.json()) as T };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  function fetchPortfolio(w: InsightsWindow, workspaceId?: string) {
    const p = windowParams(w);
    if (workspaceId) p.set("workspace_id", workspaceId);
    return get<InsightsPortfolio>(`/api/v1/insights/portfolio?${p}`);
  }

  function fetchProject(projectKey: string, w: InsightsWindow) {
    const p = windowParams(w);
    return get<ProjectInsights>(`/api/v1/projects/${projectKey}/insights?${p}`);
  }

  function fetchPerson(projectKey: string, userId: string, w: InsightsWindow) {
    const p = windowParams(w);
    return get<PersonInsights>(`/api/v1/projects/${projectKey}/insights/person/${userId}?${p}`);
  }

  function evidenceParams(w: InsightsWindow, q: EvidenceQuery): URLSearchParams {
    const p = windowParams(w);
    p.set("metric", q.metric);
    if (q.user_id) p.set("user_id", q.user_id);
    if (q.state_id) p.set("state_id", q.state_id);
    if (q.rank !== undefined) p.set("rank", String(q.rank));
    if (q.day) p.set("day", q.day);
    return p;
  }

  function fetchEvidence(projectKey: string, w: InsightsWindow, q: EvidenceQuery) {
    const p = evidenceParams(w, q);
    return get<InsightsEvidence>(`/api/v1/projects/${projectKey}/insights/evidence?${p}`);
  }

  // The CSV export carries exactly the drawer's rows; the auth header cannot
  // ride on a plain link, so the file is fetched and saved client-side.
  async function downloadEvidenceCSV(projectKey: string, w: InsightsWindow, q: EvidenceQuery): Promise<boolean> {
    const p = evidenceParams(w, q);
    p.set("format", "csv");
    try {
      const response = await fetch(`/api/v1/projects/${projectKey}/insights/evidence?${p}`, {
        headers: getAuthHeader(),
      });
      if (!response.ok) return false;
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `insights-${q.metric}.csv`;
      a.click();
      URL.revokeObjectURL(url);
      return true;
    } catch {
      return false;
    }
  }

  return { fetchPortfolio, fetchProject, fetchPerson, fetchEvidence, downloadEvidenceCSV };
}
