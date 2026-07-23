<script setup lang="ts" generic="T extends TaskRow">
import { X } from "lucide-vue-next";
import type { TaskAssignee } from "~/types";

// Minimum shape the table needs from each task row.
interface TaskRow {
  id: string;
  task_number: number;
  task_id: string;
  title: string;
  state_name: string;
  state_color: string;
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
  }>(),
  { selectable: false }
);

const emit = defineEmits<{
  remove: [taskId: string];
  toggleSelect: [taskId: string];
  toggleSelectAll: [];
}>();

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
  cols.push("140px", "minmax(0, 1fr)", "90px", "84px");
  if (props.isAdmin) cols.push("28px");
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
      <span>State</span>
      <span>Title</span>
      <span>ID</span>
      <span>Assigned</span>
      <span v-if="isAdmin"></span>
    </div>

    <div class="max-h-[70vh] overflow-y-auto [scrollbar-gutter:stable]">
      <div
        v-for="task in tasks"
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

        <NuxtLink
          :to="`/projects/${projectKey}/tasks/${task.task_number}`"
          class="min-w-0 truncate font-medium text-foreground hover:text-amber-600 hover:underline dark:hover:text-amber-500"
        >
          {{ task.title }}
        </NuxtLink>

        <span class="font-mono text-[11px] text-muted-foreground">
          {{ task.task_id }}
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
