export interface Workspace {
  id: string;
  workspace_key: string;
  name: string;
  description?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface PaginatedWorkspacesResponse {
  workspaces: Workspace[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

// Workspace membership carries no role — it governs visibility only.
export interface WorkspaceMember {
  id: string;
  user_id: string;
  username: string;
  email: string;
  first_name: string;
  last_name: string;
  avatar_url?: string;
  joined_at: string;
}

export interface CreateWorkspaceRequest {
  workspace_key: string;
  name: string;
  description?: string;
}

export interface UpdateWorkspaceRequest {
  name?: string;
  description?: string;
}

export interface AddWorkspaceMemberRequest {
  user_id: string;
}
