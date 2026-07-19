<script setup lang="ts">
import type { Task, Subtask, ProjectState } from "~/types";

const props = withDefaults(
  defineProps<{
    tasks: Task[];
    // Single-project usage (project page): all tasks share one project key,
    // one set of states, and one membership flag.
    projectKey?: string;
    states?: ProjectState[];
    isMember?: boolean;
    // Multi-project usage (dashboard): tasks span projects, so states and
    // membership are resolved per task via its project_key. These take
    // precedence over the single-value props above when provided.
    statesByProject?: Record<string, ProjectState[]>;
    isMemberByProject?: Record<string, boolean>;
    // Bulk-selection mode (single-project project page only).
    selectable?: boolean;
    selected?: Set<number>;
    // Multi-project usage (dashboard): show each task's workspace as a leading
    // column, resolved per task via its project_key.
    showWorkspace?: boolean;
    workspaceByProject?: Record<string, string>;
    // Allow expanding a parent task's sub-tasks inline in the list.
    expandable?: boolean;
  }>(),
  { states: () => [], isMember: false, selectable: false, showWorkspace: false, expandable: true }
);

const emit = defineEmits<{
  updated: [];
  toggleSelect: [taskNumber: number];
}>();

const { listSubtasks } = useTasks();

// Inline sub-task expansion state, keyed by the parent task_number.
const expanded = ref<Set<number>>(new Set());
const loadingSubtasks = ref<Set<number>>(new Set());
const subtasksByTask = reactive<Record<number, Subtask[]>>({});

function keyFor(task: Task): string {
  return props.projectKey ?? task.project_key;
}

async function loadSubtasks(task: Task) {
  loadingSubtasks.value.add(task.task_number);
  const res = await listSubtasks(keyFor(task), task.task_number);
  if (res.success && res.data) subtasksByTask[task.task_number] = res.data;
  loadingSubtasks.value.delete(task.task_number);
}

async function toggleExpand(task: Task) {
  if (expanded.value.has(task.task_number)) {
    expanded.value.delete(task.task_number);
    return;
  }
  expanded.value.add(task.task_number);
  if (!(task.task_number in subtasksByTask)) await loadSubtasks(task);
}

async function onSubtaskUpdated(task: Task) {
  await loadSubtasks(task);
  emit("updated");
}

function workspaceFor(task: Task): string {
  return props.workspaceByProject?.[task.project_key] ?? "";
}

function statesFor(task: Task): ProjectState[] {
  return props.statesByProject?.[task.project_key] ?? props.states;
}

function isMemberFor(task: Task): boolean {
  if (props.isMemberByProject) return props.isMemberByProject[task.project_key] ?? false;
  return props.isMember;
}

// Adapt a Subtask into the shape TaskCard renders. Indented rows hide the comment
// badge, so the sub-task's missing comment_count is never shown.
function subtaskAsTask(sub: Subtask): Task {
  return { ...sub, comment_count: 0, subtask_count: 0 } as unknown as Task;
}
</script>

<template>
  <div class="overflow-hidden rounded-lg border border-border/50 divide-y divide-border/50">
    <template v-for="task in tasks" :key="task.id">
      <TaskCard
        :task="task"
        :project-key="projectKey ?? task.project_key"
        :states="statesFor(task)"
        :is-member="isMemberFor(task)"
        :selectable="selectable"
        :selected="selected?.has(task.task_number) ?? false"
        :show-workspace="showWorkspace"
        :workspace-name="workspaceFor(task)"
        :expandable="expandable"
        :expanded="expanded.has(task.task_number)"
        @updated="emit('updated')"
        @toggle-select="emit('toggleSelect', task.task_number)"
        @toggle-expand="toggleExpand(task)"
      />

      <!-- Inline sub-task rows: same list, same columns, lightly highlighted -->
      <template v-if="expanded.has(task.task_number)">
        <p
          v-if="loadingSubtasks.has(task.task_number)"
          class="bg-muted/40 py-2 pl-12 pr-3 text-xs text-muted-foreground"
        >
          Loading sub-tasks…
        </p>
        <template v-else>
          <TaskCard
            v-for="sub in subtasksByTask[task.task_number] ?? []"
            :key="sub.id"
            :task="subtaskAsTask(sub)"
            :project-key="projectKey ?? task.project_key"
            :states="statesFor(task)"
            :is-member="isMemberFor(task)"
            :selectable="selectable"
            :show-workspace="showWorkspace"
            :workspace-name="workspaceFor(task)"
            indented
            @updated="onSubtaskUpdated(task)"
          />
          <p
            v-if="(subtasksByTask[task.task_number]?.length ?? 0) === 0"
            class="bg-muted/40 py-2 pl-12 pr-3 text-xs text-muted-foreground"
          >
            No sub-tasks.
          </p>
        </template>
      </template>
    </template>
  </div>
</template>
