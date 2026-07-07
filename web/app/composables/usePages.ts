import type {
  Page,
  PageListItem,
  CreatePageRequest,
  UpdatePageRequest,
} from "~/types";

interface PagesState {
  pages: PageListItem[];
  currentPage: Page | null;
  loading: boolean;
}

const state = reactive<PagesState>({
  pages: [],
  currentPage: null,
  loading: false,
});

export function usePages() {
  const { getAuthHeader } = useAuth();

  async function listPages(
    projectKey: string,
    search = ""
  ): Promise<{ success: boolean; data?: PageListItem[]; error?: string }> {
    try {
      state.loading = true;
      const qs = search.trim() ? `?q=${encodeURIComponent(search.trim())}` : "";
      const response = await fetch(`/api/v1/projects/${projectKey}/pages${qs}`, {
        headers: getAuthHeader(),
      });
      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to fetch pages" };
      }
      const pages: PageListItem[] = await response.json();
      state.pages = pages || [];
      return { success: true, data: state.pages };
    } catch {
      return { success: false, error: "Network error" };
    } finally {
      state.loading = false;
    }
  }

  async function createPage(
    projectKey: string,
    data: CreatePageRequest
  ): Promise<{ success: boolean; data?: Page; error?: string }> {
    try {
      const response = await fetch(`/api/v1/projects/${projectKey}/pages`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...getAuthHeader() },
        body: JSON.stringify(data),
      });
      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to create page" };
      }
      return { success: true, data: await response.json() };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  async function getPage(
    projectKey: string,
    pageNum: number
  ): Promise<{ success: boolean; data?: Page; error?: string }> {
    try {
      const response = await fetch(`/api/v1/projects/${projectKey}/pages/${pageNum}`, {
        headers: getAuthHeader(),
      });
      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to fetch page" };
      }
      const page: Page = await response.json();
      state.currentPage = page;
      return { success: true, data: page };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  async function updatePage(
    projectKey: string,
    pageNum: number,
    data: UpdatePageRequest
  ): Promise<{ success: boolean; data?: Page; error?: string }> {
    try {
      const response = await fetch(`/api/v1/projects/${projectKey}/pages/${pageNum}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json", ...getAuthHeader() },
        body: JSON.stringify(data),
      });
      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to update page" };
      }
      const page: Page = await response.json();
      state.currentPage = page;
      return { success: true, data: page };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  async function deletePage(
    projectKey: string,
    pageNum: number
  ): Promise<{ success: boolean; error?: string }> {
    try {
      const response = await fetch(`/api/v1/projects/${projectKey}/pages/${pageNum}`, {
        method: "DELETE",
        headers: getAuthHeader(),
      });
      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        return { success: false, error: error.message || "Failed to delete page" };
      }
      return { success: true };
    } catch {
      return { success: false, error: "Network error" };
    }
  }

  function clearCurrent() {
    state.currentPage = null;
  }

  return {
    pages: computed(() => state.pages),
    currentPage: computed(() => state.currentPage),
    loading: computed(() => state.loading),

    listPages,
    createPage,
    getPage,
    updatePage,
    deletePage,
    clearCurrent,
  };
}
