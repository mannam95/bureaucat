<script setup lang="ts">
/**
 * Harmonised toolbar for a collection's task list (cycle or module), matching
 * the project Tasks/Board toolbar: Search first, then a "+ Filter" popover
 * (State + Assignee, including "Unassigned") and a "Sort" menu (State, Priority
 * rating, Title, Assignee). Everything is applied client-side over the already
 * loaded tasks, and the component emits the final filtered + sorted list, so the
 * table stays a plain renderer with no controls of its own.
 *
 * Options are derived from the loaded tasks (modules carry no assignee summary),
 * which is also what makes the "Unassigned" option possible.
 */
import { Search, Plus, ArrowUpDown, ArrowUp, ArrowDown, Check, UserX, X } from "lucide-vue-next";
import EntityMultiSelect from "~/components/shared/EntityMultiSelect.vue";

interface FilterAssignee {
  user_id: string;
  username: string;
  first_name: string;
  last_name: string;
  avatar_url?: string;
}

interface FilterableTask {
  task_number: number;
  title: string;
  state_id: string;
  state_name: string;
  state_color: string;
  priority_rating?: number;
  assignees?: FilterAssignee[];
}

interface StateOption {
  state_id: string;
  state_name: string;
  state_color: string;
}

type SortKey = "state_name" | "priority_rating" | "title" | "assignee";

/** Sentinel id for "no one is assigned". Never collides with a user UUID. */
const UNASSIGNED = "__unassigned__";

const props = withDefaults(
  defineProps<{
    tasks: FilterableTask[];
    /** Preferred state ordering (a cycle/module state breakdown). */
    stateBuckets?: StateOption[];
  }>(),
  { stateBuckets: undefined }
);

const emit = defineEmits<{
  "update:filtered": [tasks: FilterableTask[]];
  "update:active": [active: boolean];
  // Current sort, so the table can show a read-only indicator on the column.
  "update:sort": [value: { key: SortKey | null; dir: "asc" | "desc" }];
}>();

const filterOpen = ref(false);

const stateFilter = ref<Set<string>>(new Set());
const assigneeFilter = ref<Set<string>>(new Set());
const searchQuery = ref("");

const sortKey = ref<SortKey | null>(null);
const sortDir = ref<"asc" | "desc">("asc");

const SORT_OPTIONS: { key: SortKey; label: string }[] = [
  { key: "state_name", label: "State" },
  { key: "priority_rating", label: "Priority rating" },
  { key: "title", label: "Title" },
  { key: "assignee", label: "Assignee" },
];

// ---- options derived from the loaded tasks ----
const stateOptions = computed<StateOption[]>(() => {
  if (props.stateBuckets?.length) return props.stateBuckets;
  const seen = new Map<string, StateOption>();
  for (const t of props.tasks) {
    if (!seen.has(t.state_id)) {
      seen.set(t.state_id, {
        state_id: t.state_id,
        state_name: t.state_name,
        state_color: t.state_color,
      });
    }
  }
  return [...seen.values()];
});

const assigneeOptions = computed<FilterAssignee[]>(() => {
  const seen = new Map<string, FilterAssignee>();
  let anyUnassigned = false;
  for (const t of props.tasks) {
    const people = t.assignees ?? [];
    if (people.length === 0) anyUnassigned = true;
    for (const a of people) {
      if (!seen.has(a.user_id)) seen.set(a.user_id, a);
    }
  }
  const people = [...seen.values()].sort((a, b) =>
    displayName(a).localeCompare(displayName(b))
  );
  return anyUnassigned
    ? [{ user_id: UNASSIGNED, username: "Unassigned", first_name: "Unassigned", last_name: "" }, ...people]
    : people;
});

function displayName(a: FilterAssignee): string {
  return `${a.first_name} ${a.last_name}`.trim() || a.username;
}

const filterCount = computed(() => stateFilter.value.size + assigneeFilter.value.size);
const anyFilterActive = computed(
  () => filterCount.value > 0 || searchQuery.value.trim() !== "" || sortKey.value !== null
);

// ---- filter, then sort ----
const filtered = computed(() => {
  let list = props.tasks;
  if (stateFilter.value.size > 0) {
    list = list.filter((t) => stateFilter.value.has(t.state_id));
  }
  if (assigneeFilter.value.size > 0) {
    const wantUnassigned = assigneeFilter.value.has(UNASSIGNED);
    list = list.filter((t) => {
      const people = t.assignees ?? [];
      if (wantUnassigned && people.length === 0) return true;
      return people.some((a) => assigneeFilter.value.has(a.user_id));
    });
  }
  const q = searchQuery.value.trim().toLowerCase();
  if (q) list = list.filter((t) => t.title.toLowerCase().includes(q));
  return list;
});

function assigneeName(t: FilterableTask): string {
  const a = t.assignees?.[0];
  if (!a) return "";
  return displayName(a).toLowerCase();
}

const filteredSorted = computed(() => {
  const key = sortKey.value;
  if (!key) return filtered.value;
  const dir = sortDir.value === "asc" ? 1 : -1;
  return [...filtered.value].sort((a, b) => {
    let cmp = 0;
    if (key === "priority_rating") cmp = (a.priority_rating ?? 0) - (b.priority_rating ?? 0);
    else if (key === "title") cmp = a.title.localeCompare(b.title);
    else if (key === "state_name") cmp = a.state_name.localeCompare(b.state_name);
    else cmp = assigneeName(a).localeCompare(assigneeName(b));
    if (cmp === 0) return a.task_number - b.task_number;
    return cmp * dir;
  });
});

// Drop filter entries whose option disappeared (task moved out, member removed).
watch([stateOptions, assigneeOptions], ([states, people]) => {
  const stateIds = new Set(states.map((s) => s.state_id));
  const userIds = new Set(people.map((p) => p.user_id));
  const nextStates = new Set([...stateFilter.value].filter((id) => stateIds.has(id)));
  const nextUsers = new Set([...assigneeFilter.value].filter((id) => userIds.has(id)));
  if (nextStates.size !== stateFilter.value.size) stateFilter.value = nextStates;
  if (nextUsers.size !== assigneeFilter.value.size) assigneeFilter.value = nextUsers;
});

watch(filteredSorted, (v) => emit("update:filtered", v), { immediate: true });
watch(anyFilterActive, (v) => emit("update:active", v), { immediate: true });

function setStateFilter(ids: string[]) {
  stateFilter.value = new Set(ids);
}
function setAssigneeFilter(ids: string[]) {
  assigneeFilter.value = new Set(ids);
}

// Key and direction are set independently (like the Tasks Sort menu): picking a
// field sets what to sort by, and the explicit Ascending/Descending items set
// the direction.
function setSortKey(key: SortKey) {
  sortKey.value = key;
}
function setSortDir(dir: "asc" | "desc") {
  sortDir.value = dir;
}
function resetSort() {
  sortKey.value = null;
  sortDir.value = "asc";
}

watch(
  [sortKey, sortDir],
  () => emit("update:sort", { key: sortKey.value, dir: sortDir.value }),
  { immediate: true }
);

function clear() {
  stateFilter.value = new Set();
  assigneeFilter.value = new Set();
  searchQuery.value = "";
  resetSort();
}

defineExpose({ clear });
</script>

<template>
  <!-- `contents` so the controls join the parent toolbar's flex row directly. -->
  <div class="contents">
    <template v-if="tasks.length > 0">
      <!-- Search (leads, like the project Tasks toolbar) -->
      <div class="relative min-w-[9rem] flex-1 sm:max-w-[18rem]">
        <Search class="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input v-model="searchQuery" placeholder="Search tasks…" class="h-9 pl-8" />
      </div>

      <!-- Filter (State + Assignee) -->
      <Popover v-model:open="filterOpen">
        <PopoverTrigger as-child>
          <Button variant="outline" size="sm" class="h-9 gap-1.5">
            <Plus class="size-3.5" />
            Filter
            <span
              v-if="filterCount"
              class="rounded bg-primary/10 px-1 text-xs font-medium text-primary"
            >
              {{ filterCount }}
            </span>
          </Button>
        </PopoverTrigger>
        <PopoverContent class="w-64 p-0" align="start">
          <div class="border-b px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
            Assignee
          </div>
          <EntityMultiSelect
            :items="assigneeOptions"
            item-key="user_id"
            :model-value="[...assigneeFilter]"
            placeholder="Find member…"
            empty-message="No assignees"
            max-height-class="max-h-40"
            @update:model-value="setAssigneeFilter"
          >
            <template #option="{ item }">
              <template v-if="(item as FilterAssignee).user_id === UNASSIGNED">
                <span class="flex size-5 shrink-0 items-center justify-center rounded-full border border-dashed text-muted-foreground">
                  <UserX class="size-3" />
                </span>
                <span class="truncate text-muted-foreground">Unassigned</span>
              </template>
              <template v-else>
                <Avatar class="size-5">
                  <AvatarImage v-if="(item as FilterAssignee).avatar_url" :src="(item as FilterAssignee).avatar_url!" />
                  <AvatarFallback class="text-[9px]" :seed="(item as FilterAssignee).user_id">
                    {{ ((item as FilterAssignee).first_name[0] || "") + ((item as FilterAssignee).last_name[0] || "") }}
                  </AvatarFallback>
                </Avatar>
                <span class="truncate">{{ displayName(item as FilterAssignee) }}</span>
              </template>
            </template>
          </EntityMultiSelect>

          <div class="border-y px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
            State
          </div>
          <EntityMultiSelect
            :items="stateOptions"
            item-key="state_id"
            :model-value="[...stateFilter]"
            placeholder="Find state…"
            empty-message="No states"
            max-height-class="max-h-40"
            @update:model-value="setStateFilter"
          >
            <template #option="{ item }">
              <span class="size-2 shrink-0 rounded-full" :style="{ backgroundColor: (item as StateOption).state_color || '#6B7280' }" />
              <span class="truncate">{{ (item as StateOption).state_name }}</span>
            </template>
          </EntityMultiSelect>
        </PopoverContent>
      </Popover>

      <!-- Sort -->
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm" class="h-9 gap-1.5">
            <ArrowUpDown class="size-3.5" />
            Sort
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-52">
          <DropdownMenuLabel class="text-xs uppercase tracking-wider text-muted-foreground">
            Sort by
          </DropdownMenuLabel>
          <DropdownMenuItem
            v-for="opt in SORT_OPTIONS"
            :key="opt.key"
            class="flex items-center justify-between"
            @click="setSortKey(opt.key)"
          >
            <span>{{ opt.label }}</span>
            <Check v-if="sortKey === opt.key" class="size-3.5 text-primary" />
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem class="flex items-center justify-between" @click="setSortDir('asc')">
            <span class="flex items-center gap-2"><ArrowUp class="size-3.5" /> Ascending</span>
            <Check v-if="sortKey && sortDir === 'asc'" class="size-3.5 text-primary" />
          </DropdownMenuItem>
          <DropdownMenuItem class="flex items-center justify-between" @click="setSortDir('desc')">
            <span class="flex items-center gap-2"><ArrowDown class="size-3.5" /> Descending</span>
            <Check v-if="sortKey && sortDir === 'desc'" class="size-3.5 text-primary" />
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Button
        v-if="anyFilterActive"
        variant="ghost"
        size="sm"
        class="h-9 text-muted-foreground"
        @click="clear"
      >
        <X class="mr-1 size-3.5" /> Reset
      </Button>
    </template>
  </div>
</template>
