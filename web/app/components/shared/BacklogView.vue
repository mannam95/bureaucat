<script setup lang="ts">
/**
 * Body of a backlog detail page: the project's top-level tasks that aren't in
 * any cycle (or any module). It reuses the SAME toolbar (CollectionFilterBar)
 * and table (CollectionTaskTable) as the cycle/module detail views, so the
 * columns, filtering, sorting and reset all match. The only extra is a bulk
 * "add to <target>" action in the selection bar.
 */
import { Loader2 } from "lucide-vue-next";
import { toast } from "vue-sonner";
import CollectionFilterBar from "~/components/shared/CollectionFilterBar.vue";
import type { TaskAssignee } from "~/types";

interface BacklogTask {
  id: string;
  task_number: number;
  task_id: string;
  title: string;
  state_id: string;
  state_name: string;
  state_color: string;
  priority_rating?: number;
  cycle_title?: string;
  assignees?: TaskAssignee[];
}
interface Target {
  id: string;
  title: string;
}
type LoadResult = { success: boolean; data?: BacklogTask[]; error?: string };
type AckResult = { success: boolean; error?: string };

const props = withDefaults(
  defineProps<{
    projectKey: string;
    // Lowercase noun for buttons, e.g. "cycle" or "epic".
    targetNoun: string;
    targets: Target[];
    // Show the Sprint column (module backlog: a no-epic task can still be in a
    // cycle). Off for the cycle backlog, where tasks are in no cycle by design.
    showCycle?: boolean;
    // Adding is admin-gated; a non-admin viewer gets a read-only list.
    canAdd?: boolean;
    loadTasks: (search: string, limit: number) => Promise<LoadResult>;
    addTasks: (targetId: string, ids: string[]) => Promise<AckResult>;
  }>(),
  { showCycle: false, canAdd: true }
);

const emit = defineEmits<{ added: [] }>();

const loadedTasks = ref<BacklogTask[]>([]);
const visibleTasks = ref<BacklogTask[]>([]);
const sortState = ref<{ key: string | null; dir: "asc" | "desc" }>({ key: null, dir: "asc" });
const selectedIds = ref<Set<string>>(new Set());
const targetId = ref("");
const loading = ref(true);
const adding = ref(false);

async function reload() {
  loading.value = true;
  // Whole set (server caps at 200), then filtered/sorted client-side by the bar.
  const res = await props.loadTasks("", 200);
  loading.value = false;
  if (res.success) {
    loadedTasks.value = res.data || [];
    const ids = new Set(loadedTasks.value.map((t) => t.id));
    selectedIds.value = new Set([...selectedIds.value].filter((id) => ids.has(id)));
  }
}

function toggleSelect(id: string) {
  const next = new Set(selectedIds.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  selectedIds.value = next;
}
function toggleSelectAll() {
  const ids = visibleTasks.value.map((t) => t.id);
  const allSel = ids.length > 0 && ids.every((id) => selectedIds.value.has(id));
  selectedIds.value = allSel ? new Set() : new Set(ids);
}

async function add() {
  if (!targetId.value || selectedIds.value.size === 0) return;
  adding.value = true;
  const ids = [...selectedIds.value];
  const res = await props.addTasks(targetId.value, ids);
  adding.value = false;
  if (res.success) {
    toast.success(`Added ${ids.length} task${ids.length === 1 ? "" : "s"}`);
    selectedIds.value = new Set();
    await reload();
    emit("added");
  } else {
    toast.error(res.error || "Failed to add tasks");
  }
}

// Preselect the only target so a one-cycle / one-module project needs no pick.
watch(
  () => props.targets,
  (t) => {
    if (!targetId.value && t.length === 1) targetId.value = t[0]!.id;
  },
  { immediate: true }
);

onMounted(reload);
</script>

<template>
  <div class="space-y-4">
    <div v-if="loading" class="flex items-center justify-center py-16">
      <Loader2 class="size-6 animate-spin text-muted-foreground" />
    </div>

    <template v-else>
      <!-- Same toolbar as the cycle/module views: search, filter, sort, reset -->
      <div class="flex flex-wrap items-center gap-2">
        <CollectionFilterBar
          :tasks="loadedTasks"
          @update:filtered="(list) => (visibleTasks = list as BacklogTask[])"
          @update:sort="(s) => (sortState = s)"
        />
      </div>

      <!-- Bulk selection: pick a target and add, mirroring the detail views -->
      <div
        v-if="canAdd && selectedIds.size > 0"
        class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-sm"
      >
        <span class="font-medium">{{ selectedIds.size }} selected</span>
        <div class="flex items-center gap-2">
          <Button variant="ghost" size="sm" @click="selectedIds = new Set()">
            Clear
          </Button>
          <select
            v-model="targetId"
            class="h-9 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="targets.length === 0"
          >
            <option value="" disabled>
              {{ targets.length ? `Add to ${targetNoun}…` : `No ${targetNoun} yet` }}
            </option>
            <option v-for="t in targets" :key="t.id" :value="t.id">
              {{ t.title }}
            </option>
          </select>
          <Button size="sm" :disabled="adding || !targetId" @click="add">
            <Loader2 v-if="adding" class="mr-1.5 size-4 animate-spin" />
            Add
          </Button>
        </div>
      </div>

      <div
        v-if="visibleTasks.length === 0"
        class="rounded-lg border border-dashed py-16 text-center text-sm text-muted-foreground"
      >
        {{ loadedTasks.length === 0 ? `Every task is in a ${targetNoun}.` : "No tasks match the filters." }}
      </div>

      <CollectionTaskTable
        v-else
        :tasks="visibleTasks"
        :project-key="projectKey"
        :is-admin="false"
        :selectable="canAdd"
        :selected="selectedIds"
        :show-cycle="showCycle"
        :sort-key="sortState.key"
        :sort-dir="sortState.dir"
        @toggle-select="toggleSelect"
        @toggle-select-all="toggleSelectAll"
      />
    </template>
  </div>
</template>
