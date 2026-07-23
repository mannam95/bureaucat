<script setup lang="ts">
import { Circle, CircleDot, CheckCircle2, XCircle, Clock, MessageSquare, Building2, ChevronRight, CornerDownRight } from "lucide-vue-next";
import type { Task, ProjectState } from "~/types";
import { PRIORITY_LABELS } from "~/types";

const props = withDefaults(
  defineProps<{
    task: Task;
    projectKey?: string;
    states?: ProjectState[];
    isMember?: boolean;
    // When true, a leading checkbox is shown and the row toggles selection
    // instead of navigating on the checkbox itself.
    selectable?: boolean;
    selected?: boolean;
    // Multi-project usage (dashboard): show the task's workspace as a leading
    // column. Redundant inside a single project, so off by default.
    showWorkspace?: boolean;
    workspaceName?: string;
    // When expandable and the task has sub-tasks, a leading chevron toggles the
    // parent's sub-tasks inline (expansion is managed by the parent TaskList).
    expandable?: boolean;
    expanded?: boolean;
    // Renders this row as a nested sub-task: light background, indented ID with a
    // corner marker, and no expand chevron / comment badge.
    indented?: boolean;
  }>(),
  {
    states: () => [],
    isMember: false,
    selectable: false,
    selected: false,
    showWorkspace: false,
    workspaceName: "",
    expandable: false,
    expanded: false,
    indented: false,
  }
);

const emit = defineEmits<{
  updated: [];
  toggleSelect: [];
  toggleExpand: [];
}>();

// Fall back to the task's own project_key (e.g. on the dashboard, where tasks
// span multiple projects) when no explicit key is passed.
const resolvedKey = computed(() => props.projectKey ?? props.task.project_key);

const { updateTask } = useTasks();
const updatingState = ref(false);

// Inline state editing is only offered to members who have states to pick from.
const canEditState = computed(() => props.isMember && props.states.length > 0);

async function changeState(stateId: string) {
  if (stateId === props.task.state_id || updatingState.value) return;
  updatingState.value = true;
  const res = await updateTask(resolvedKey.value, props.task.task_number, { state_id: stateId });
  updatingState.value = false;
  if (res.success) emit("updated");
}

const stateIcon = computed(() => {
  switch (props.task.state_type) {
    case "backlog":
      return Clock;
    case "unstarted":
      return Circle;
    case "started":
      return CircleDot;
    case "completed":
      return CheckCircle2;
    case "cancelled":
      return XCircle;
    default:
      return Circle;
  }
});

const priorityInfo = computed(() => PRIORITY_LABELS[props.task.priority] || PRIORITY_LABELS[0]);

interface Person {
  id: string;
  firstName: string;
  lastName: string;
  avatarUrl?: string;
}

// Creator and assignees are separate columns: merging them made it impossible
// to tell who raised a task from who is doing it. Empty on the dashboard, whose
// API doesn't return creator fields.
const creator = computed<Person | null>(() =>
  props.task.created_by
    ? {
        id: props.task.created_by,
        firstName: props.task.creator_first_name || "",
        lastName: props.task.creator_last_name || "",
        avatarUrl: props.task.creator_avatar_url,
      }
    : null
);

const assignedTo = computed<Person[]>(() =>
  (props.task.assignees ?? []).map((a) => ({
    id: a.user_id,
    firstName: a.first_name,
    lastName: a.last_name,
    avatarUrl: a.avatar_url,
  }))
);
</script>

<template>
  <NuxtLink :to="`/projects/${resolvedKey}/tasks/${task.task_number}`" class="block">
    <div
      class="task-row group grid items-center bg-background/50 px-3 py-2.5 transition-colors hover:bg-muted/50"
      :class="{ selectable, 'has-workspace': showWorkspace, 'bg-accent/40': selectable && selected, 'bg-muted/40 hover:bg-muted/60': indented }"
    >
      <!-- Col 0: Selection checkbox (column reserved but empty for nested sub-tasks) -->
      <div v-if="selectable" class="justify-self-center" @click.stop.prevent>
        <Checkbox v-if="!indented" :model-value="selected" @update:model-value="emit('toggleSelect')" />
      </div>

      <!-- Col: Workspace (multi-project lists only; hidden for nested sub-tasks) -->
      <div v-if="showWorkspace" class="min-w-0">
        <span
          v-if="!indented"
          class="inline-flex max-w-full items-center gap-1 rounded-md border bg-muted/50 px-1.5 py-0.5"
          :title="workspaceName"
        >
          <Building2 class="size-3 shrink-0 text-muted-foreground" />
          <span class="truncate text-xs text-muted-foreground">{{ workspaceName || "—" }}</span>
        </span>
      </div>

      <!-- Col 1: Task ID (chevron for parents with sub-tasks; corner marker for sub-tasks) -->
      <span
        class="flex min-w-0 items-center gap-1 font-mono text-sm text-muted-foreground"
        :class="{ 'pl-5': indented }"
      >
        <CornerDownRight v-if="indented" class="size-3.5 shrink-0 text-muted-foreground/50" />
        <button
          v-else-if="expandable && (task.subtask_count ?? 0) > 0"
          type="button"
          class="-ml-1 flex size-4 shrink-0 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          :aria-label="expanded ? 'Collapse sub-tasks' : 'Expand sub-tasks'"
          @click.stop.prevent="emit('toggleExpand')"
        >
          <ChevronRight class="size-3.5 transition-transform" :class="{ 'rotate-90': expanded }" />
        </button>
        <span class="truncate">{{ task.task_id }}</span>
      </span>

      <!-- Col 2: Title -->
      <span class="truncate text-sm font-medium min-w-0">{{ task.title }}</span>

      <!-- Col 3: State badge (editable for members) -->
      <div class="justify-self-end" @click.stop.prevent>
        <TaskStateSelector
          v-if="canEditState"
          :states="states"
          :model-value="task.state_id"
          :disabled="updatingState"
          compact
          dense
          @update:model-value="changeState"
        />
        <div
          v-else
          class="flex items-center gap-1 rounded-md border bg-muted/50 px-1.5 py-0.5 w-fit"
        >
          <component
            :is="stateIcon"
            class="size-3.5 shrink-0 stroke-[2.5]"
            :style="{ color: task.state_color }"
          />
          <span class="text-xs text-muted-foreground whitespace-nowrap">{{ task.state_name }}</span>
        </div>
      </div>

      <!-- Col 4: Priority badge -->
      <div class="flex items-center gap-1 rounded-md border bg-muted/50 px-1.5 py-0.5 w-fit justify-self-end">
        <span
          class="size-2.5 shrink-0 rounded-full ring-1.5 ring-offset-1 ring-offset-background"
          :style="{ backgroundColor: priorityInfo.color, '--tw-ring-color': priorityInfo.color }"
        />
        <span class="text-xs text-muted-foreground whitespace-nowrap">{{ priorityInfo.label }}</span>
      </div>

      <!-- Col 5: Created by (always a single person) -->
      <div class="flex items-center justify-end">
        <NuxtLink
          v-if="creator"
          :to="`/profile/${creator.id}`"
          :title="`Created by ${creator.firstName} ${creator.lastName}`.trim()"
          @click.stop
        >
          <Avatar class="size-6 border-2 border-background transition-transform hover:scale-110">
            <AvatarImage
              v-if="creator.avatarUrl"
              :src="creator.avatarUrl"
              :alt="`${creator.firstName} ${creator.lastName}`"
            />
            <AvatarFallback class="text-[10px]" :seed="creator.id">
              {{ creator.firstName?.[0] || "" }}{{ creator.lastName?.[0] || "" }}
            </AvatarFallback>
          </Avatar>
        </NuxtLink>
        <span v-else class="text-xs text-muted-foreground">—</span>
      </div>

      <!-- Col 6: Assigned to (stacked; a dash makes unassigned tasks obvious) -->
      <div class="flex items-center justify-end">
        <div v-if="assignedTo.length > 0" class="flex -space-x-1.5">
          <NuxtLink
            v-for="person in assignedTo.slice(0, 3)"
            :key="person.id"
            :to="`/profile/${person.id}`"
            :title="`Assigned to ${person.firstName} ${person.lastName}`.trim()"
            class="hover:z-10"
            @click.stop
          >
            <Avatar class="size-6 border-2 border-background transition-transform hover:scale-110">
              <AvatarImage
                v-if="person.avatarUrl"
                :src="person.avatarUrl"
                :alt="`${person.firstName} ${person.lastName}`"
              />
              <AvatarFallback class="text-[10px]" :seed="person.id">
                {{ person.firstName?.[0] || "" }}{{ person.lastName?.[0] || "" }}
              </AvatarFallback>
            </Avatar>
          </NuxtLink>
          <Avatar
            v-if="assignedTo.length > 3"
            class="size-6 border-2 border-background"
            :title="`${assignedTo.length - 3} more`"
          >
            <AvatarFallback class="text-[10px] bg-muted">
              +{{ assignedTo.length - 3 }}
            </AvatarFallback>
          </Avatar>
        </div>
        <span v-else class="text-xs text-muted-foreground">—</span>
      </div>

      <!-- Col 7: Comment count (hidden for nested sub-task rows) -->
      <div class="flex items-center justify-end">
        <div
          v-if="!indented"
          class="flex items-center gap-1 rounded-full bg-muted px-1.5 py-0.5"
          :title="`${task.comment_count} comment${task.comment_count !== 1 ? 's' : ''}`"
        >
          <MessageSquare class="size-3 text-muted-foreground" />
          <span class="font-mono text-xs font-medium text-muted-foreground">{{ task.comment_count }}</span>
        </div>
      </div>
    </div>
  </NuxtLink>
</template>

<!-- The .task-row grid templates live in assets/css/tailwind.css so TaskList's
     header row can share them. -->
