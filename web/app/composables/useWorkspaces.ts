import type {
  Workspace,
  WorkspaceMember,
  PaginatedWorkspacesResponse,
  CreateWorkspaceRequest,
  UpdateWorkspaceRequest,
  AddWorkspaceMemberRequest,
} from "~/types";

interface WorkspacesState {
  workspaces: Workspace[];
  currentWorkspace: Workspace | null;
  members: WorkspaceMember[];
  loading: boolean;
}

const STORAGE_KEY = "bureaucat.currentWorkspaceId";

// Singleton state, mirroring useAuth/useProjects.
const state = reactive<WorkspacesState>({
  workspaces: [],
  currentWorkspace: null,
  members: [],
  loading: false,
});

function persistCurrent(id: string | null) {
  if (typeof window === "undefined") return;
  if (id) {
    localStorage.setItem(STORAGE_KEY, id);
  } else {
    localStorage.removeItem(STORAGE_KEY);
  }
}

function readPersistedId(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(STORAGE_KEY);
}

export function useWorkspaces() {
  const { getAuthHeader } = useAuth();

  // Fetch all workspaces the user can see and (re)select the current one:
  // the persisted choice if still valid, otherwise the first workspace.
  async function listWorkspaces(): Promise<{ success: boolean; error?: string }> {
    try {
      state.loading = true;
      const response = await fetch("/api/v1/workspaces?page=1&per_page=100", {
        headers: getAuthHeader(),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to fetch workspaces" };
      }

      const data: PaginatedWorkspacesResponse = await response.json();
      state.workspaces = data.workspaces || [];

      const persistedId = readPersistedId();
      const match = state.workspaces.find((w) => w.id === persistedId);
      const next = match ?? state.workspaces[0] ?? null;
      setCurrentWorkspace(next);

      return { success: true };
    } catch {
      return { success: false, error: "Network error" };
    } finally {
      state.loading = false;
    }
  }

  function setCurrentWorkspace(workspace: Workspace | null) {
    state.currentWorkspace = workspace;
    persistCurrent(workspace?.id ?? null);
  }

  async function createWorkspace(
    data: CreateWorkspaceRequest
  ): Promise<{ success: boolean; data?: Workspace; error?: string }> {
    try {
      const response = await fetch("/api/v1/workspaces", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...getAuthHeader() },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to create workspace" };
      }

      const workspace: Workspace = await response.json();
      state.workspaces = [...state.workspaces, workspace].sort((a, b) =>
        a.name.localeCompare(b.name)
      );
      return { success: true, data: workspace };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  async function updateWorkspace(
    workspaceKey: string,
    data: UpdateWorkspaceRequest
  ): Promise<{ success: boolean; data?: Workspace; error?: string }> {
    try {
      const response = await fetch(`/api/v1/workspaces/${workspaceKey}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json", ...getAuthHeader() },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to update workspace" };
      }

      const workspace: Workspace = await response.json();
      state.workspaces = state.workspaces.map((w) => (w.id === workspace.id ? workspace : w));
      if (state.currentWorkspace?.id === workspace.id) {
        state.currentWorkspace = workspace;
      }
      return { success: true, data: workspace };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  async function deleteWorkspace(
    workspaceKey: string
  ): Promise<{ success: boolean; error?: string }> {
    try {
      const response = await fetch(`/api/v1/workspaces/${workspaceKey}`, {
        method: "DELETE",
        headers: getAuthHeader(),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to delete workspace" };
      }

      const deleted = state.workspaces.find((w) => w.workspace_key === workspaceKey);
      state.workspaces = state.workspaces.filter((w) => w.workspace_key !== workspaceKey);
      if (deleted && state.currentWorkspace?.id === deleted.id) {
        setCurrentWorkspace(state.workspaces[0] ?? null);
      }
      return { success: true };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  // Members
  async function listMembers(
    workspaceKey: string
  ): Promise<{ success: boolean; data?: WorkspaceMember[]; error?: string }> {
    try {
      const response = await fetch(`/api/v1/workspaces/${workspaceKey}/members`, {
        headers: getAuthHeader(),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to fetch members" };
      }

      const members: WorkspaceMember[] = await response.json();
      state.members = members;
      return { success: true, data: members };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  async function addMember(
    workspaceKey: string,
    data: AddWorkspaceMemberRequest
  ): Promise<{ success: boolean; data?: WorkspaceMember; error?: string }> {
    try {
      const response = await fetch(`/api/v1/workspaces/${workspaceKey}/members`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...getAuthHeader() },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to add member" };
      }

      const member: WorkspaceMember = await response.json();
      return { success: true, data: member };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  async function removeMember(
    workspaceKey: string,
    userId: string
  ): Promise<{ success: boolean; error?: string }> {
    try {
      const response = await fetch(`/api/v1/workspaces/${workspaceKey}/members/${userId}`, {
        method: "DELETE",
        headers: getAuthHeader(),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to remove member" };
      }

      return { success: true };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  // Search the user directory to pick someone to add as a workspace member.
  async function searchUsers(
    workspaceKey: string,
    query: string
  ): Promise<{
    success: boolean;
    data?: Array<{
      id: string;
      username: string;
      email: string;
      first_name: string;
      last_name: string;
    }>;
    error?: string;
  }> {
    try {
      const params = new URLSearchParams({ q: query });
      const response = await fetch(
        `/api/v1/workspaces/${workspaceKey}/members/search?${params}`,
        { headers: getAuthHeader() }
      );

      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to search users" };
      }

      const data = await response.json();
      return { success: true, data: data.users ?? [] };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  function clearWorkspaces() {
    state.workspaces = [];
    state.currentWorkspace = null;
    state.members = [];
  }

  return {
    // State (readonly)
    workspaces: computed(() => state.workspaces),
    currentWorkspace: computed(() => state.currentWorkspace),
    members: computed(() => state.members),
    loading: computed(() => state.loading),

    // Methods
    listWorkspaces,
    setCurrentWorkspace,
    createWorkspace,
    updateWorkspace,
    deleteWorkspace,
    listMembers,
    addMember,
    removeMember,
    searchUsers,
    clearWorkspaces,
  };
}
