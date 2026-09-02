<script setup lang="ts">
/**
 * Body of a backlog detail page: the project's top-level tasks that aren't in
 * any cycle (or any module), with search, multi-select and a bulk "add to
 * <target>" action. The parent page supplies the data loader, the list of
 * targets (cycles/modules) and the add action, so one component serves both.
 */
import { Loader2, Search } from "lucide-vue-next";
import { toast } from "vue-sonner";

interface BacklogTask {
  id: string;
  title: string;
  task_id: string;
  task_number: number;
  state_name: string;
  state_color: string;
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
    // Lowercase noun for buttons/hints, e.g. "cycle" or "epic".
    targetNoun: string;
    targets: Target[];
    // Adding is admin-gated; a non-admin viewer gets a read-only list.
    canAdd?: boolean;
    loadTasks: (search: string, limit: number) => Promise<LoadResult>;
    addTasks: (targetId: string, ids: string[]) => Promise<AckResult>;
  }>(),
  { canAdd: true }
);

const emit = defineEmits<{ added: [] }>();

const tasks = ref<BacklogTask[]>([]);
const loading = ref(true);
const search = ref("");
const selectedIds = ref<Set<string>>(new Set());
const targetId = ref("");
const adding = ref(false);
let searchDebounce: ReturnType<typeof setTimeout> | null = null;

const searchActive = computed(() => search.value.trim() !== "");

const gridCols = computed(() =>
  props.canAdd
    ? "grid-template-columns: 28px 150px minmax(0, 1fr) 90px;"
    : "grid-template-columns: 150px minmax(0, 1fr) 90px;"
);

const allSelected = computed(
  () => tasks.value.length > 0 && tasks.value.every((t) => selectedIds.value.has(t.id))
);
const selectAllModel = computed<boolean | "indeterminate">(() =>
  allSelected.value ? true : selectedIds.value.size > 0 ? "indeterminate" : false
);

async function reload() {
  loading.value = true;
  const res = await props.loadTasks(search.value.trim(), 100);
  loading.value = false;
  if (res.success) {
    tasks.value = res.data || [];
    const ids = new Set(tasks.value.map((t) => t.id));
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
  selectedIds.value = allSelected.value
    ? new Set()
    : new Set(tasks.value.map((t) => t.id));
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

watch(search, () => {
  if (searchDebounce) clearTimeout(searchDebounce);
  searchDebounce = setTimeout(reload, 250);
});

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
    <!-- Toolbar: search + (admin) target picker + add -->
    <div class="flex flex-wrap items-center gap-2">
      <div class="relative min-w-[9rem] flex-1 sm:max-w-xs">
        <Search class="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input v-model="search" placeholder="Search tasks…" class="h-9 pl-8" />
      </div>

      <template v-if="canAdd">
        <select
          v-model="targetId"
          class="ml-auto h-9 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="targets.length === 0"
        >
          <option value="" disabled>
            {{ targets.length ? `Add to ${targetNoun}…` : `No ${targetNoun} yet` }}
          </option>
          <option v-for="t in targets" :key="t.id" :value="t.id">
            {{ t.title }}
          </option>
        </select>
        <Button
          size="sm"
          class="h-9"
          :disabled="adding || !targetId || selectedIds.size === 0"
          @click="add"
        >
          <Loader2 v-if="adding" class="mr-1.5 size-4 animate-spin" />
          Add {{ selectedIds.size || "" }}
        </Button>
      </template>
    </div>

    <div class="overflow-hidden rounded-lg border bg-background">
      <div
        class="grid items-center gap-3 border-b bg-muted/40 px-4 py-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground"
        :style="gridCols"
      >
        <span v-if="canAdd" class="flex items-center">
          <Checkbox
            :model-value="selectAllModel"
            aria-label="Select all backlog tasks"
            @update:model-value="toggleSelectAll"
          />
        </span>
        <span>State</span>
        <span>Title</span>
        <span>ID</span>
      </div>

      <div class="max-h-[70vh] overflow-y-auto [scrollbar-gutter:stable]">
        <div
          v-if="loading"
          class="flex items-center justify-center py-12 text-sm text-muted-foreground"
        >
          <Loader2 class="mr-2 size-4 animate-spin" /> Loading…
        </div>
        <div
          v-else-if="tasks.length === 0"
          class="py-12 text-center text-sm text-muted-foreground"
        >
          {{ searchActive ? "No matching tasks." : `Every task is in a ${targetNoun}.` }}
        </div>
        <template v-else>
          <!-- Admins get selectable rows; viewers get a plain list. -->
          <label
            v-for="task in tasks"
            :key="task.id"
            class="grid items-center gap-3 border-b border-border/40 px-4 py-2.5 last:border-0 hover:bg-muted/40"
            :class="[canAdd ? 'cursor-pointer' : '', selectedIds.has(task.id) ? 'bg-amber-500/5' : '']"
            :style="gridCols"
          >
            <Checkbox
              v-if="canAdd"
              :model-value="selectedIds.has(task.id)"
              :aria-label="`Select ${task.title}`"
              @update:model-value="toggleSelect(task.id)"
            />
            <span
              class="inline-flex w-fit max-w-full items-center truncate rounded px-1.5 py-0.5 font-mono text-[10px] font-medium uppercase tracking-wider"
              :style="{
                backgroundColor: (task.state_color || '#6B7280') + '22',
                color: task.state_color || '#6B7280',
              }"
            >
              {{ task.state_name }}
            </span>
            <NuxtLink
              :to="`/projects/${projectKey}/tasks/${task.task_number}`"
              class="min-w-0 truncate text-sm font-medium hover:text-amber-600 hover:underline dark:hover:text-amber-500"
              @click.stop
            >
              {{ task.title }}
            </NuxtLink>
            <span class="font-mono text-[11px] text-muted-foreground">{{ task.task_id }}</span>
          </label>
        </template>
      </div>
    </div>
  </div>
</template>
