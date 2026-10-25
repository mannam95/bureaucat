<script setup lang="ts">
import { Maximize2, ChevronRight } from "lucide-vue-next";
import type { Task, Subtask, ProjectState, ProjectMember, ProjectLabel } from "~/types";

const props = withDefaults(
  defineProps<{
    task: Task;
    projectKey: string;
    isMember: boolean;
    /** Dragging is disabled under groupings where a move is ambiguous. */
    canDrag?: boolean;
    /** The id of the column this card is currently rendered inside. */
    columnId?: string;
    states?: ProjectState[];
    members?: ProjectMember[];
    labels?: ProjectLabel[];
  }>(),
  { canDrag: undefined, states: () => [], members: () => [], labels: () => [] }
);

const draggable = computed(() => props.canDrag ?? props.isMember);

const emit = defineEmits<{ refresh: [] }>();

const isDragging = ref(false);
const detailOpen = ref(false);

function handleDragStart(event: DragEvent) {
  if (!draggable.value) {
    event.preventDefault();
    return;
  }
  isDragging.value = true;
  event.dataTransfer!.effectAllowed = "move";
  event.dataTransfer!.setData(
    "application/json",
    JSON.stringify({ task: props.task, fromColumnId: props.columnId ?? "" })
  );
}

function handleDragEnd() {
  isDragging.value = false;
}

function toggleDetail(open: boolean) {
  // A drag ends with a click on some browsers; never treat that as a card tap.
  if (isDragging.value) return;
  detailOpen.value = open;
}

// --- Subtask stack (parent cards only) ---
// The card stays in the PARENT's state column; children never move the parent.
// Collapsed: a fraction + segmented bar. Expanded in place: child mini-rows
// with each child's real state and owner. A cancelled child never counts as
// done and never shrinks the open work.
const { listSubtasks } = useTasks();

const subTotal = computed(() => props.task.subtask_count ?? 0);
const subDone = computed(() => props.task.subtask_done_count ?? 0);
const subCancelled = computed(() => props.task.subtask_cancelled_count ?? 0);
// The denominator excludes cancelled children, so the fraction can reach n/n.
const subCountable = computed(() => subTotal.value - subCancelled.value);

const stackLabel = computed(() => {
  if (subCountable.value === 0) {
    return `${subCancelled.value} subtask${subCancelled.value === 1 ? "" : "s"} cancelled`;
  }
  let s = `${subDone.value}/${subCountable.value} subtask${subCountable.value === 1 ? "" : "s"} done`;
  if (subCancelled.value > 0) s += ` · ${subCancelled.value} cancelled`;
  return s;
});

const stackTooltip = computed(() => {
  const open = subCountable.value - subDone.value;
  let s = `${subDone.value} done · ${open} open · ${subCancelled.value} cancelled`;
  if (subCancelled.value > 0) s += " — a cancelled subtask never counts as done";
  return s;
});

function pct(n: number): string {
  return subTotal.value > 0 ? `${(n / subTotal.value) * 100}%` : "0%";
}

const stackOpen = ref(false);
const stackLoading = ref(false);
const stack = ref<Subtask[] | null>(null);

async function toggleStack() {
  stackOpen.value = !stackOpen.value;
  if (stackOpen.value && stack.value === null) {
    stackLoading.value = true;
    const res = await listSubtasks(props.projectKey, props.task.task_number);
    if (res.success && res.data) stack.value = res.data;
    stackLoading.value = false;
  }
}
</script>

<template>
  <Popover :open="detailOpen" @update:open="toggleDetail">
    <PopoverTrigger as-child>
      <div
        :draggable="draggable"
        role="button"
        tabindex="0"
        :aria-label="`Task ${task.task_id}: ${task.title}`"
        class="group cursor-pointer rounded-lg border bg-background p-3 text-left shadow-sm transition-all hover:border-amber-500/30 hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 outline-none"
        :class="{ 'cursor-grab active:cursor-grabbing': draggable }"
        @dragstart="handleDragStart"
        @dragend="handleDragEnd"
        @keydown.enter="toggleDetail(true)"
        @keydown.space.prevent="toggleDetail(true)"
      >
        <!-- Task ID and priority -->
        <div class="mb-2 flex items-center justify-between">
          <NuxtLink
            :to="`/projects/${projectKey}/tasks/${task.task_number}`"
            :title="`Open ${task.task_id}`"
            class="-mx-1 -my-0.5 flex items-center gap-1.5 rounded-md px-1 py-0.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            @click.stop
          >
            <Maximize2 class="size-3 shrink-0 opacity-50" />
            <span class="font-mono text-xs">{{ task.task_id }}</span>
          </NuxtLink>
          <PriorityRating
            v-if="(task.priority_rating ?? 0) > 0"
            :model-value="task.priority_rating"
          />
        </div>

        <!-- Title -->
        <p class="line-clamp-2 text-sm font-medium">{{ task.title }}</p>

        <!-- Labels -->
        <div
          v-if="task.labels && task.labels.length > 0"
          class="mt-2 flex flex-wrap gap-1"
        >
          <span
            v-for="label in task.labels.slice(0, 3)"
            :key="label.id"
            class="rounded px-1.5 py-0.5 text-xs"
            :style="{
              backgroundColor: label.color + '20',
              color: label.color,
            }"
          >
            {{ label.name }}
          </span>
          <span
            v-if="task.labels.length > 3"
            class="text-xs text-muted-foreground"
          >
            +{{ task.labels.length - 3 }}
          </span>
        </div>

        <!-- Assignees -->
        <div
          v-if="task.assignees && task.assignees.length > 0"
          class="mt-2 flex items-center justify-end"
        >
          <div class="flex -space-x-1.5">
            <NuxtLink
              v-for="assignee in task.assignees.slice(0, 3)"
              :key="assignee.id"
              :to="`/profile/${assignee.user_id}`"
              :title="`${assignee.first_name} ${assignee.last_name}`"
              class="hover:z-10"
              @click.stop
            >
              <Avatar class="size-5 border border-background transition-transform hover:scale-110">
                <AvatarFallback class="text-[10px]" :seed="assignee.user_id">
                  {{ assignee.first_name[0] }}{{ assignee.last_name[0] }}
                </AvatarFallback>
              </Avatar>
            </NuxtLink>
            <div
              v-if="task.assignees.length > 3"
              class="flex size-5 items-center justify-center rounded-full border border-background bg-muted text-[10px]"
            >
              +{{ task.assignees.length - 3 }}
            </div>
          </div>
        </div>

        <!-- Subtask stack: the card lives in the PARENT's own state column; the
             line expands in place to child mini-rows with their real states. -->
        <div v-if="subTotal > 0" class="mt-2 border-t pt-2">
          <button
            type="button"
            class="flex w-full items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
            :title="stackTooltip"
            :aria-label="stackOpen ? 'Collapse subtasks' : 'Expand subtasks'"
            @click.stop.prevent="toggleStack"
          >
            <ChevronRight class="size-3 shrink-0 transition-transform" :class="{ 'rotate-90': stackOpen }" />
            <span class="truncate">{{ stackLabel }}</span>
            <span class="ml-auto flex h-1 w-14 shrink-0 overflow-hidden rounded-full bg-muted">
              <span class="bg-emerald-500" :style="{ width: pct(subDone) }" />
              <span class="ml-auto bg-muted-foreground/30" :style="{ width: pct(subCancelled) }" />
            </span>
          </button>

          <div v-if="stackOpen" class="mt-1.5 space-y-1" @click.stop>
            <p v-if="stackLoading" class="px-1.5 text-xs text-muted-foreground">Loading subtasks…</p>
            <template v-else>
              <NuxtLink
                v-for="sub in stack ?? []"
                :key="sub.id"
                :to="`/projects/${projectKey}/tasks/${sub.task_number}`"
                :title="`${sub.task_id} · ${sub.title} — ${sub.state_name}`"
                class="flex items-center gap-1.5 rounded-md border bg-muted/30 px-1.5 py-1 transition-colors hover:bg-muted/60"
                @click.stop
              >
                <span class="size-2 shrink-0 rounded-full" :style="{ backgroundColor: sub.state_color }" />
                <span class="shrink-0 font-mono text-[10px] text-muted-foreground">{{ sub.task_id }}</span>
                <span class="min-w-0 truncate text-xs">{{ sub.title }}</span>
                <span class="ml-auto flex shrink-0 items-center gap-1">
                  <span class="whitespace-nowrap text-[10px] text-muted-foreground">{{ sub.state_name }}</span>
                  <span v-if="sub.assignees?.length" class="flex -space-x-1">
                    <Avatar
                      v-for="a in sub.assignees.slice(0, 2)"
                      :key="a.user_id"
                      class="size-4 border border-background"
                      :title="`${a.first_name} ${a.last_name}`"
                    >
                      <AvatarFallback class="text-[8px]" :seed="a.user_id">
                        {{ a.first_name?.[0] || "" }}{{ a.last_name?.[0] || "" }}
                      </AvatarFallback>
                    </Avatar>
                    <span
                      v-if="sub.assignees.length > 2"
                      class="flex size-4 items-center justify-center rounded-full border border-background bg-muted text-[8px]"
                    >
                      +{{ sub.assignees.length - 2 }}
                    </span>
                  </span>
                </span>
              </NuxtLink>
              <p v-if="(stack?.length ?? 0) === 0" class="px-1.5 text-xs text-muted-foreground">No subtasks.</p>
            </template>
          </div>
        </div>
      </div>
    </PopoverTrigger>

    <PopoverContent
      align="start"
      :collision-padding="16"
      class="w-[48rem] max-w-[calc(100vw-2rem)] max-h-[min(75vh,var(--reka-popover-content-available-height))] overflow-y-auto shadow-lg"
    >
      <TaskDetailCard
        v-if="detailOpen"
        :project-key="projectKey"
        :task-number="task.task_number"
        :states="states"
        :members="members"
        :project-labels="labels"
        :is-member="isMember"
        @updated="emit('refresh')"
      />
    </PopoverContent>
  </Popover>
</template>
