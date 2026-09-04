/**
 * useFilterTree owns the current view state (filter tree, sort, group-by, and
 * the active saved view) for a project's Tasks or Board surface.
 *
 * It is backed by the durable preference store at project scope, NOT the URL.
 * Tasks and Board have separate stored view states:
 *   tasks.list.view_state   { filter, sortBy, sortDir, groupBy, viewSlug }
 *   board.view_state        { filter, sortBy, sortDir, groupBy, viewSlug }
 * The surface is passed as a getter so a single instance can follow the active
 * tab: switching tabs swaps which stored state is read and written.
 *
 * Free-text search is deliberately NOT part of the stored view state — it is
 * session-only and kept in memory here, per surface (durable session storage is
 * layered on in a later step).
 */

import type {
  FilterTree,
  FilterNode,
  Predicate,
  SortKey,
  SortDir,
  ViewGroupBy,
  ProjectView,
} from "~/types";

const DEFAULT_SORT_BY: SortKey = "created_at";
const DEFAULT_SORT_DIR: SortDir = "desc";
const DEFAULT_GROUP_BY: ViewGroupBy = "state";

export type FilterSurface = "tasks" | "board";

// The JSON shape stored per surface. Mirrors the backend registry default.
interface ViewState {
  filter: FilterTree | null;
  sortBy: SortKey;
  sortDir: SortDir;
  groupBy: ViewGroupBy;
  viewSlug: string | null;
}

function defaultViewState(): ViewState {
  return {
    filter: null,
    sortBy: DEFAULT_SORT_BY,
    sortDir: DEFAULT_SORT_DIR,
    groupBy: DEFAULT_GROUP_BY,
    viewSlug: null,
  };
}

function emptyTree(): FilterTree {
  return { children: [] };
}

function surfaceKey(surface: FilterSurface): string {
  return surface === "board" ? "board.view_state" : "tasks.list.view_state";
}

export function useFilterTree(getSurface: () => FilterSurface) {
  const route = useRoute();
  const prefs = usePreferences();

  const projectKey = () => (typeof route.params.key === "string" ? route.params.key : "");
  const currentKey = () => surfaceKey(getSurface());

  // Free-text search: session-only (survives a same-tab reload, not a new tab or
  // session), stored per surface so Tasks and Board don't share it. Never
  // written to the durable view state.
  const sessionSearch = useSessionSearch();
  const tasksSearch = sessionSearch.searchRef("tasks", projectKey);
  const boardSearch = sessionSearch.searchRef("board", projectKey);
  const searchQuery = computed<string>({
    get: () => (getSurface() === "board" ? boardSearch.value : tasksSearch.value),
    set: (v) => {
      if (getSurface() === "board") boardSearch.value = v;
      else tasksSearch.value = v;
    },
  });

  // The live view state for the active surface, read reactively from the store.
  const vs = computed<ViewState>(() => {
    const pk = projectKey();
    if (!pk) return defaultViewState();
    return prefs.getProject<ViewState>(pk, currentKey(), defaultViewState());
  });

  // Merge a partial change into the active surface's stored view state.
  function writeVS(patch: Partial<ViewState>) {
    const pk = projectKey();
    if (!pk) return;
    const cur = prefs.getProject<ViewState>(pk, currentKey(), defaultViewState());
    prefs.setProject(pk, currentKey(), { ...cur, ...patch });
  }

  const tree = computed<FilterTree>(() => vs.value.filter ?? emptyTree());

  function setTree(next: FilterTree) {
    writeVS({ filter: next.children.length > 0 ? next : null });
  }
  function clearTree() {
    setTree(emptyTree());
  }
  function addPredicate(p: Predicate) {
    setTree({ children: [...tree.value.children, { predicate: p }] });
  }
  function replacePredicateAt(index: number, p: Predicate) {
    const next = [...tree.value.children];
    next[index] = { predicate: p };
    setTree({ children: next });
  }
  function removeNodeAt(index: number) {
    setTree({ children: tree.value.children.filter((_, i) => i !== index) });
  }

  const sortBy = computed<SortKey>({
    get: () => vs.value.sortBy ?? DEFAULT_SORT_BY,
    set: (v) => writeVS({ sortBy: v }),
  });
  const sortDir = computed<SortDir>({
    get: () => vs.value.sortDir ?? DEFAULT_SORT_DIR,
    set: (v) => writeVS({ sortDir: v }),
  });
  const groupBy = computed<ViewGroupBy>({
    get: () => vs.value.groupBy ?? DEFAULT_GROUP_BY,
    set: (v) => writeVS({ groupBy: v }),
  });
  function resetSort() {
    writeVS({ sortBy: DEFAULT_SORT_BY, sortDir: DEFAULT_SORT_DIR });
  }

  const activeViewSlug = computed(() => vs.value.viewSlug ?? null);
  function setActiveView(slug: string | null) {
    writeVS({ viewSlug: slug });
  }

  // Empty the filter AND drop the active view association (search preserved).
  function clearTreeAndView() {
    writeVS({ filter: null, viewSlug: null });
  }

  // Clear everything that scopes the list — filter, active view, and search.
  function clearAll() {
    writeVS({ filter: null, viewSlug: null });
    searchQuery.value = "";
  }

  // Reset everything the toolbar owns — filter, view, sort, and search.
  function resetAll() {
    writeVS({
      filter: null,
      viewSlug: null,
      sortBy: DEFAULT_SORT_BY,
      sortDir: DEFAULT_SORT_DIR,
    });
    searchQuery.value = "";
  }

  /**
   * effectiveTree is what the API actually receives. The free-text search box is
   * emitted as a single `search contains X` predicate — the server expands that
   * to match title and description.
   *
   * Default view: with no explicit filter, no search, and no active saved view,
   * completed/cancelled tasks are hidden via an implicit `state_type not_in`
   * predicate. Any filter (or an applied view) drops that implicit exclusion.
   */
  const effectiveTree = computed<FilterTree>(() => {
    const children: FilterNode[] = [];
    if (searchQuery.value) {
      children.push({
        predicate: { field: "search", op: "contains", value: searchQuery.value },
      });
    }
    children.push(...tree.value.children);

    if (children.length === 0 && !activeViewSlug.value) {
      return {
        children: [
          {
            predicate: {
              field: "state_type",
              op: "not_in",
              value: ["completed", "cancelled"],
            },
          },
        ],
      };
    }
    return { children };
  });

  /**
   * Apply a saved view: writes its filter/sort/group/slug into the stored view
   * state of the tab the view targets, and returns that tab so the caller can
   * navigate there. Writing the target surface directly (rather than the active
   * one) avoids a race with the tab switch.
   */
  function applyView(v: ProjectView): FilterSurface {
    const targetTab: FilterSurface = v.default_tab === "board" ? "board" : "tasks";
    const pk = projectKey();
    if (pk) {
      const next: ViewState = {
        filter: v.filter_tree && v.filter_tree.children.length > 0 ? v.filter_tree : null,
        sortBy: v.sort_by,
        sortDir: v.sort_dir,
        groupBy: v.group_by,
        viewSlug: v.slug,
      };
      prefs.setProject(pk, surfaceKey(targetTab), next);
    }
    return targetTab;
  }

  /** Ensure this project's preferences are loaded before reading view state. */
  async function hydrate() {
    const pk = projectKey();
    if (pk) await prefs.hydrateProject(pk);
  }

  return {
    tree: computed(() => tree.value),
    setTree,
    clearTree,
    clearTreeAndView,
    clearAll,
    addPredicate,
    replacePredicateAt,
    removeNodeAt,
    sortBy,
    sortDir,
    resetSort,
    resetAll,
    groupBy,
    activeViewSlug,
    setActiveView,
    applyView,
    searchQuery,
    effectiveTree,
    hydrate,
  };
}
