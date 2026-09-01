<script setup lang="ts" generic="T extends TaskRow">
import { X, ChevronUp, ChevronDown, ChevronsUpDown } from "lucide-vue-next";
import type { TaskAssignee } from "~/types";

// Minimum shape the table needs from each task row.
interface TaskRow {
  id: string;
  task_number: number;
  task_id: string;
  title: string;
  state_name: string;
  state_color: string;
  priority_rating?: number;
  cycle_title?: string;
  assignees?: TaskAssignee[];
}

const props = withDefaults(
  defineProps<{
    tasks: T[];
    projectKey: string;
    isAdmin: boolean;
    removeLabel?: string;
    // Optional multi-select mode: adds a leading checkbox column. Off by default,
    // so callers that don't need it (e.g. the module page) are unaffected.
    selectable?: boolean;
    selected?: Set<string>;
    // Show a "Sprint" column with each task's cycle. Used on the module page,
    // where a task's sprint isn't otherwise visible; redundant on a cycle page.
    showCycle?: boolean;
  }>(),
  { selectable: false, showCycle: false }
);

const emit = defineEmits<{
  remove: [taskId: string];
  toggleSelect: [taskId: string];
  toggleSelectAll: [];
}>();

// ---- Client-side sorting (the collection's tasks are all loaded already) ----
type SortKey = "state_name" | "priority_rating" | "title" | "assignee";
const sortKey = ref<SortKey | null>(null);
const sortDir = ref<"asc" | "desc">("asc");

function toggleSort(key: SortKey) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === "asc" ? "desc" : "asc";
  } else {
    sortKey.value = key;
    // Rating is most useful highest-first; text/state read better A->Z.
    sortDir.value = key === "priority_rating" ? "desc" : "asc";
  }
}

function assigneeName(t: TaskRow): string {
  const a = t.assignees?.[0];
  if (!a) return "";
  return (`${a.first_name} ${a.last_name}`.trim() || a.username).toLowerCase();
}

const sortedTasks = computed<T[]>(() => {
  const key = sortKey.value;
  if (!key) return props.tasks;
  const dir = sortDir.value === "asc" ? 1 : -1;
  return [...props.tasks].sort((a, b) => {
    let cmp = 0;
    if (key === "priority_rating") {
      cmp = (a.priority_rating ?? 0) - (b.priority_rating ?? 0);
    } else if (key === "title") {
      cmp = a.title.localeCompare(b.title);
    } else if (key === "state_name") {
      cmp = a.state_name.localeCompare(b.state_name);
    } else {
      cmp = assigneeName(a).localeCompare(assigneeName(b));
    }
    // Stable, deterministic tiebreak so equal rows don't jitter between sorts.
    if (cmp === 0) return a.task_number - b.task_number;
    return cmp * dir;
  });
});

const allSelected = computed(
  () => props.tasks.length > 0 && props.tasks.every((t) => props.selected?.has(t.id))
);
// reka-ui accepts the string "indeterminate" for the partial state.
const selectAllModel = computed<boolean | "indeterminate">(() =>
  allSelected.value ? true : (props.selected?.size ?? 0) > 0 ? "indeterminate" : false
);

const gridStyle = computed(() => {
  const cols: string[] = [];
  if (props.selectable) cols.push("28px");
  cols.push("120px"); // State
  cols.push("64px"); // Priority (stars)
  cols.push("minmax(0, 1fr)"); // Title
  cols.push("84px"); // ID
  if (props.showCycle) cols.push("130px"); // Sprint
  cols.push("84px"); // Assigned
  if (props.isAdmin) cols.push("28px"); // remove
  return `grid-template-columns: ${cols.join(" ")};`;
});
</script>

<template>
  <div class="overflow-hidden rounded-lg border bg-background">
    <div
      class="grid items-center gap-3 border-b bg-muted/40 px-4 py-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground"
      :style="gridStyle"
    >
      <span v-if="selectable" class="flex items-center">
        <Checkbox
          :model-value="selectAllModel"
          aria-label="Select all tasks"
          @update:model-value="emit('toggleSelectAll')"
        />
      </span>

      <button type="button" class="flex items-center gap-1 text-left uppercase hover:text-foreground" @click="toggleSort('state_name')">
        State
        <component :is="sortKey === 'state_name' ? (sortDir === 'asc' ? ChevronUp : ChevronDown) : ChevronsUpDown" class="size-3" :class="sortKey === 'state_name' ? 'opacity-100' : 'opacity-40'" />
      </button>

      <button type="button" class="flex items-center gap-1 text-left uppercase hover:text-foreground" @click="toggleSort('priority_rating')">
        <span title="Priority rating">★</span>
        <component :is="sortKey === 'priority_rating' ? (sortDir === 'asc' ? ChevronUp : ChevronDown) : ChevronsUpDown" class="size-3" :class="sortKey === 'priority_rating' ? 'opacity-100' : 'opacity-40'" />
      </button>

      <button type="button" class="flex items-center gap-1 text-left uppercase hover:text-foreground" @click="toggleSort('title')">
        Title
        <component :is="sortKey === 'title' ? (sortDir === 'asc' ? ChevronUp : ChevronDown) : ChevronsUpDown" class="size-3" :class="sortKey === 'title' ? 'opacity-100' : 'opacity-40'" />
      </button>

      <span>ID</span>
      <span v-if="showCycle">Sprint</span>

      <button type="button" class="flex items-center gap-1 text-left uppercase hover:text-foreground" @click="toggleSort('assignee')">
        Assigned
        <component :is="sortKey === 'assignee' ? (sortDir === 'asc' ? ChevronUp : ChevronDown) : ChevronsUpDown" class="size-3" :class="sortKey === 'assignee' ? 'opacity-100' : 'opacity-40'" />
      </button>

      <span v-if="isAdmin"></span>
    </div>

    <div class="max-h-[70vh] overflow-y-auto [scrollbar-gutter:stable]">
      <div
        v-for="task in sortedTasks"
        :key="task.id"
        class="group grid items-center gap-3 border-b border-border/40 px-4 py-2.5 text-sm transition-colors last:border-0 hover:bg-muted/40"
        :class="{ 'bg-amber-500/5': selectable && selected?.has(task.id) }"
        :style="gridStyle"
      >
        <span v-if="selectable" class="flex items-center">
          <Checkbox
            :model-value="selected?.has(task.id) ?? false"
            :aria-label="`Select ${task.title}`"
            @update:model-value="emit('toggleSelect', task.id)"
          />
        </span>

        <span
          class="inline-flex w-fit max-w-full items-center truncate rounded px-1.5 py-0.5 font-mono text-[10px] font-medium uppercase tracking-wider"
          :style="{
            backgroundColor: (task.state_color || '#6B7280') + '22',
            color: task.state_color || '#6B7280',
          }"
        >
          {{ task.state_name }}
        </span>

        <span class="flex items-center">
          <PriorityRating :model-value="task.priority_rating ?? 0" />
        </span>

        <NuxtLink
          :to="`/projects/${projectKey}/tasks/${task.task_number}`"
          class="min-w-0 truncate font-medium text-foreground hover:text-amber-600 hover:underline dark:hover:text-amber-500"
        >
          {{ task.title }}
        </NuxtLink>

        <span class="font-mono text-[11px] text-muted-foreground">
          {{ task.task_id }}
        </span>

        <!-- Sprint / cycle (module page only) -->
        <span v-if="showCycle" class="truncate text-xs text-muted-foreground" :title="task.cycle_title || 'No sprint'">
          {{ task.cycle_title || "—" }}
        </span>

        <!-- Assigned: a dash makes unassigned rows obvious, which is what the
             "Unassigned" filter above is for. -->
        <span class="flex items-center">
          <span v-if="(task.assignees?.length ?? 0) > 0" class="flex -space-x-1.5">
            <NuxtLink
              v-for="person in (task.assignees ?? []).slice(0, 3)"
              :key="person.user_id"
              :to="`/profile/${person.user_id}`"
              :title="`${person.first_name} ${person.last_name}`.trim() || person.username"
              class="hover:z-10"
              @click.stop
            >
              <Avatar class="size-6 border-2 border-background transition-transform hover:scale-110">
                <AvatarImage v-if="person.avatar_url" :src="person.avatar_url" />
                <AvatarFallback class="text-[10px]" :seed="person.user_id">
                  {{ (person.first_name?.[0] || "") + (person.last_name?.[0] || "") }}
                </AvatarFallback>
              </Avatar>
            </NuxtLink>
            <Avatar
              v-if="(task.assignees?.length ?? 0) > 3"
              class="size-6 border-2 border-background"
              :title="`${(task.assignees?.length ?? 0) - 3} more`"
            >
              <AvatarFallback class="text-[10px] bg-muted">
                +{{ (task.assignees?.length ?? 0) - 3 }}
              </AvatarFallback>
            </Avatar>
          </span>
          <span v-else class="text-xs text-muted-foreground">—</span>
        </span>

        <button
          v-if="isAdmin"
          class="rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-muted hover:text-destructive focus-visible:opacity-100 group-hover:opacity-100"
          :aria-label="`${removeLabel || 'Remove'} ${task.title}`"
          @click.prevent="emit('remove', task.id)"
        >
          <X class="size-3.5" />
        </button>
      </div>
    </div>
  </div>
</template>
