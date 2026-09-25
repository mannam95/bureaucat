<script setup lang="ts">
import { Plus, Layers, Loader2, ChevronLeft, ChevronRight, Search } from "lucide-vue-next";
import type { ModuleListFilters, ModuleStatus } from "~/types";
import { MODULE_STATUSES } from "~/types";

const props = defineProps<{
  projectKey: string;
  isAdmin: boolean;
}>();

const { modules, loading, total, page, totalPages, listModules, listTasksInNoModule } =
  useModules();

const showCreate = ref(false);
const perPage = 12;
// Overview sort + status/lead filters, persisted per project through the
// preference store (the project page hydrates prefs before this tab renders).
const MODULES_OVERVIEW_DEFAULT: ModuleListFilters = { sort_by: "created_at", sort_dir: "desc" };
const prefs = usePreferences();
const filters = computed<ModuleListFilters>({
  get: () =>
    prefs.getProject<ModuleListFilters>(
      props.projectKey,
      "modules.overview.view_state",
      MODULES_OVERVIEW_DEFAULT,
    ),
  set: (v) => prefs.setProject(props.projectKey, "modules.overview.view_state", v),
});
// Card (tile) vs flat list view; persisted so it sticks across visits.
const viewMode = useViewMode("modules.overview.view_mode");

// Active/Completed tabs; each tab fetches only its own slice from the server.
// Completed covers finished work in both senses: completed and cancelled.
// Session-only, but if a completed-group status filter was persisted, start
// on the Completed tab so the stored filter still matches what's shown.
const COMPLETED_GROUP: readonly ModuleStatus[] = ["completed", "cancelled"];
const statusGroup = ref<"active" | "completed">(
  filters.value.status && COMPLETED_GROUP.includes(filters.value.status)
    ? "completed"
    : "active"
);
const STATUS_TABS = [
  { key: "active", label: "Active" },
  { key: "completed", label: "Completed" },
] as const;
// The status dropdown only offers statuses that exist on the visible tab.
const tabStatuses = computed<readonly ModuleStatus[]>(() =>
  statusGroup.value === "completed"
    ? COMPLETED_GROUP
    : MODULE_STATUSES.filter((s) => !COMPLETED_GROUP.includes(s))
);
watch(statusGroup, () => {
  const s = filters.value.status;
  if (s && !tabStatuses.value.includes(s)) {
    // Drop a status filter that can't match on this tab; the filters
    // watcher below refetches page 1 for us.
    filters.value = { ...filters.value, status: undefined };
  } else {
    fetchPage(1);
  }
});

// How many top-level tasks aren't in any module, for the backlog card that opens
// the "Tasks Without an Epic" view. Only shown to admins (adding is admin-only).
const backlogCount = ref(0);
async function loadBacklogCount() {
  if (!props.isAdmin) return;
  const r = await listTasksInNoModule(props.projectKey, "", 100);
  if (r.success && r.data) backlogCount.value = r.data.length;
}
const showBacklogCard = computed(
  () => props.isAdmin && backlogCount.value > 0 && statusGroup.value === "active"
);

// Free-text search over module titles/descriptions. Session-only on purpose
// (like the tasks search): it is merged into the fetch, never persisted.
const searchQuery = ref("");
let searchDebounce: ReturnType<typeof setTimeout> | null = null;
watch(searchQuery, () => {
  if (searchDebounce) clearTimeout(searchDebounce);
  searchDebounce = setTimeout(() => fetchPage(1), 300);
});

function fetchPage(p = 1) {
  listModules(props.projectKey, p, perPage, {
    ...filters.value,
    search: searchQuery.value.trim() || undefined,
    status_group: statusGroup.value,
  });
}

function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return;
  fetchPage(p);
}

function onSaved() {
  // New modules start in the active group; jump there so it's visible.
  if (statusGroup.value !== "active") {
    statusGroup.value = "active"; // the watcher refetches page 1
  } else {
    fetchPage(1);
  }
}

onMounted(() => {
  fetchPage(1);
  loadBacklogCount();
});

watch(
  () => props.projectKey,
  () => {
    fetchPage(1);
    loadBacklogCount();
  }
);

watch(
  filters,
  () => fetchPage(1),
  { deep: true }
);
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-lg font-semibold">Modules</h2>
        <p class="text-sm text-muted-foreground">
          Reusable sub-projects. Group tasks, assign a lead, and duplicate
          for repeating workflows.
        </p>
      </div>
      <div class="flex items-center gap-2">
        <ViewModeToggle v-model="viewMode" />
        <Button v-if="isAdmin" @click="showCreate = true">
          <Plus class="mr-2 size-4" />
          New Module
        </Button>
      </div>
    </div>

    <div class="flex w-fit items-center rounded-md border p-0.5">
      <button
        v-for="t in STATUS_TABS"
        :key="t.key"
        type="button"
        class="rounded px-3 py-1 text-sm font-medium transition-colors"
        :class="statusGroup === t.key
          ? 'bg-muted text-foreground'
          : 'text-muted-foreground hover:text-foreground'"
        @click="statusGroup = t.key"
      >
        {{ t.label }}
      </button>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <div class="relative w-full sm:max-w-xs">
        <Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input v-model="searchQuery" placeholder="Search modules…" class="h-9 pl-9" />
      </div>
      <ModuleFiltersBar v-model="filters" :project-key="projectKey" :statuses="tabStatuses" />
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <Loader2 class="size-6 animate-spin text-muted-foreground" />
    </div>

    <div
      v-else-if="modules.length === 0 && !showBacklogCard"
      class="flex flex-col items-center justify-center rounded-lg border border-dashed py-16"
    >
      <div class="flex size-16 items-center justify-center rounded-full bg-muted">
        <Layers class="size-8 text-muted-foreground" />
      </div>
      <template v-if="statusGroup === 'completed'">
        <h3 class="mt-4 text-lg font-semibold">No completed modules</h3>
        <p class="mt-2 max-w-sm text-center text-sm text-muted-foreground">
          Modules show up here once they are completed or cancelled.
        </p>
      </template>
      <template v-else>
        <h3 class="mt-4 text-lg font-semibold">No active modules</h3>
        <p class="mt-2 max-w-sm text-center text-sm text-muted-foreground">
          Create a module to group tasks into a reusable sub-project with its own
          lead and members.
        </p>
        <Button v-if="isAdmin" class="mt-4" @click="showCreate = true">
          <Plus class="mr-2 size-4" />
          New Module
        </Button>
      </template>
    </div>

    <template v-else>
      <div v-if="viewMode === 'card'" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <BacklogCard
          v-if="showBacklogCard && page === 1"
          title="Tasks Without an Epic"
          subtitle="Top-level tasks not in any epic yet."
          :count="backlogCount"
          :to="`/projects/${projectKey}/modules/backlog`"
        />
        <ModuleCard
          v-for="m in modules"
          :key="m.id"
          :module="m"
          :to="`/projects/${projectKey}/modules/${m.id}`"
        />
      </div>

      <template v-else>
        <div v-if="showBacklogCard && page === 1" class="mb-3">
          <BacklogCard
            variant="row"
            title="Tasks Without an Epic"
            subtitle="not in any epic yet"
            :count="backlogCount"
            :to="`/projects/${projectKey}/modules/backlog`"
          />
        </div>
        <ModuleListView :modules="modules" :project-key="projectKey" />
      </template>

      <div
        v-if="totalPages > 1"
        class="flex items-center justify-between border-t pt-4"
      >
        <p class="text-sm text-muted-foreground">
          {{ total }} module{{ total === 1 ? "" : "s" }}
        </p>
        <div class="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            :disabled="page <= 1"
            @click="goToPage(page - 1)"
          >
            <ChevronLeft class="size-4" />
          </Button>
          <span class="text-sm">Page {{ page }} of {{ totalPages }}</span>
          <Button
            variant="outline"
            size="sm"
            :disabled="page >= totalPages"
            @click="goToPage(page + 1)"
          >
            <ChevronRight class="size-4" />
          </Button>
        </div>
      </div>
    </template>

    <CreateModuleDialog
      v-model:open="showCreate"
      :project-key="projectKey"
      @saved="onSaved"
    />
  </div>
</template>
