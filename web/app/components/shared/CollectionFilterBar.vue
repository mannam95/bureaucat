<script setup lang="ts">
/**
 * State + assignee + search filters for a collection's task list
 * (cycle or module). The component owns the filter state and emits the
 * filtered list, so both pages stay identical without duplicating the toolbar.
 *
 * Assignee options are derived from the tasks actually loaded rather than from
 * collection metrics: modules carry no assignee summary, and deriving is what
 * makes the "Unassigned" option possible on both pages.
 */
import { ChevronsUpDown, Search, UserX, X } from "lucide-vue-next";
import EntityMultiSelect from "~/components/shared/EntityMultiSelect.vue";

interface FilterAssignee {
  user_id: string;
  username: string;
  first_name: string;
  last_name: string;
  avatar_url?: string;
}

interface FilterableTask {
  title: string;
  state_id: string;
  state_name: string;
  state_color: string;
  assignees?: FilterAssignee[];
}

interface StateOption {
  state_id: string;
  state_name: string;
  state_color: string;
}

/** Sentinel id for "no one is assigned". Never collides with a user UUID. */
const UNASSIGNED = "__unassigned__";

const props = withDefaults(
  defineProps<{
    tasks: FilterableTask[];
    /**
     * Preferred state ordering (a cycle/module state breakdown). When omitted,
     * states are derived from the tasks in first-seen order.
     */
    stateBuckets?: StateOption[];
  }>(),
  { stateBuckets: undefined }
);

const emit = defineEmits<{
  "update:filtered": [tasks: FilterableTask[]];
  "update:active": [active: boolean];
}>();

const stateOpen = ref(false);
const assigneeOpen = ref(false);

const stateFilter = ref<Set<string>>(new Set());
const assigneeFilter = ref<Set<string>>(new Set());
const searchQuery = ref("");

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
  // Offered only when it would actually match something.
  return anyUnassigned
    ? [
        {
          user_id: UNASSIGNED,
          username: "Unassigned",
          first_name: "Unassigned",
          last_name: "",
        },
        ...people,
      ]
    : people;
});

function displayName(a: FilterAssignee): string {
  return `${a.first_name} ${a.last_name}`.trim() || a.username;
}

const anyFilterActive = computed(
  () =>
    stateFilter.value.size > 0 ||
    assigneeFilter.value.size > 0 ||
    searchQuery.value.trim() !== ""
);

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
  if (q) {
    list = list.filter((t) => t.title.toLowerCase().includes(q));
  }
  return list;
});

// Drop filter entries whose option disappeared (task moved out, member removed),
// otherwise a stale selection silently hides everything.
watch(
  [stateOptions, assigneeOptions],
  ([states, people]) => {
    const stateIds = new Set(states.map((s) => s.state_id));
    const userIds = new Set(people.map((p) => p.user_id));
    const nextStates = new Set(
      [...stateFilter.value].filter((id) => stateIds.has(id))
    );
    const nextUsers = new Set(
      [...assigneeFilter.value].filter((id) => userIds.has(id))
    );
    if (nextStates.size !== stateFilter.value.size) stateFilter.value = nextStates;
    if (nextUsers.size !== assigneeFilter.value.size) assigneeFilter.value = nextUsers;
  }
);

watch(filtered, (v) => emit("update:filtered", v), { immediate: true });
watch(anyFilterActive, (v) => emit("update:active", v), { immediate: true });

function setStateFilter(ids: string[]) {
  stateFilter.value = new Set(ids);
}

function setAssigneeFilter(ids: string[]) {
  assigneeFilter.value = new Set(ids);
}

function clear() {
  stateFilter.value = new Set();
  assigneeFilter.value = new Set();
  searchQuery.value = "";
}

defineExpose({ clear });
</script>

<template>
  <!-- `contents` so the controls join the parent toolbar's flex row directly.
       The root stays mounted even with no tasks (the controls hide instead), so
       the filtered list keeps being emitted and never goes stale. -->
  <div class="contents">
    <template v-if="tasks.length > 0">
    <!-- State (multi-select) -->
    <Popover v-model:open="stateOpen">
      <PopoverTrigger as-child>
        <Button variant="outline" size="sm" class="h-9 gap-1.5">
          State
          <span
            v-if="stateFilter.size"
            class="rounded bg-primary/10 px-1 text-xs font-medium text-primary"
          >
            {{ stateFilter.size }}
          </span>
          <ChevronsUpDown class="size-3.5 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent class="w-56 p-0" align="start">
        <EntityMultiSelect
          :items="stateOptions"
          item-key="state_id"
          :model-value="[...stateFilter]"
          placeholder="Find state…"
          empty-message="No states"
          @update:model-value="setStateFilter"
        >
          <template #option="{ item }">
            <span
              class="size-2 shrink-0 rounded-full"
              :style="{ backgroundColor: (item as StateOption).state_color || '#6B7280' }"
            />
            <span class="truncate">{{ (item as StateOption).state_name }}</span>
          </template>
        </EntityMultiSelect>
      </PopoverContent>
    </Popover>

    <!-- Assignee (multi-select, including "Unassigned") -->
    <Popover v-model:open="assigneeOpen">
      <PopoverTrigger as-child>
        <Button variant="outline" size="sm" class="h-9 gap-1.5">
          Assignee
          <span
            v-if="assigneeFilter.size"
            class="rounded bg-primary/10 px-1 text-xs font-medium text-primary"
          >
            {{ assigneeFilter.size }}
          </span>
          <ChevronsUpDown class="size-3.5 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent class="w-60 p-0" align="start">
        <EntityMultiSelect
          :items="assigneeOptions"
          item-key="user_id"
          :model-value="[...assigneeFilter]"
          placeholder="Find member…"
          empty-message="No assignees"
          @update:model-value="setAssigneeFilter"
        >
          <template #option="{ item }">
            <template v-if="(item as FilterAssignee).user_id === UNASSIGNED">
              <span
                class="flex size-5 shrink-0 items-center justify-center rounded-full border border-dashed text-muted-foreground"
              >
                <UserX class="size-3" />
              </span>
              <span class="truncate text-muted-foreground">Unassigned</span>
            </template>
            <template v-else>
              <Avatar class="size-5">
                <AvatarImage
                  v-if="(item as FilterAssignee).avatar_url"
                  :src="(item as FilterAssignee).avatar_url!"
                />
                <AvatarFallback class="text-[9px]" :seed="(item as FilterAssignee).user_id">
                  {{ ((item as FilterAssignee).first_name[0] || "") + ((item as FilterAssignee).last_name[0] || "") }}
                </AvatarFallback>
              </Avatar>
              <span class="truncate">{{ displayName(item as FilterAssignee) }}</span>
            </template>
          </template>
        </EntityMultiSelect>
      </PopoverContent>
    </Popover>

    <!-- Search -->
    <div class="relative min-w-[9rem] flex-1 sm:max-w-[16rem]">
      <Search class="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
      <Input
        v-model="searchQuery"
        placeholder="Search tasks…"
        class="h-9 pl-8"
      />
    </div>

    <Button
      v-if="anyFilterActive"
      variant="ghost"
      size="sm"
      class="h-9 text-muted-foreground"
      @click="clear"
    >
      <X class="mr-1 size-3.5" /> Clear
    </Button>
    </template>
  </div>
</template>
