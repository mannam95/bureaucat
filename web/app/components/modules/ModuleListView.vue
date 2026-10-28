<script setup lang="ts">
// Flat list view of an epic/module overview, harmonised with the task tables:
// a bordered grid with a header row and clickable rows. Lets you scan and
// reprioritise many epics at once (sort is driven by the shared ModuleFiltersBar).
import type { Module } from "~/types";

defineProps<{
  modules: Module[];
  projectKey: string;
}>();

const statusStyles: Record<string, string> = {
  backlog: "border-muted-foreground/30 bg-muted text-muted-foreground",
  planned: "border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-300",
  ongoing: "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300",
  in_progress: "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300",
  completed: "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  paused: "border-muted-foreground/30 bg-muted text-muted-foreground",
};

// Archived counts as complete and cancelled leaves the denominator,
// matching the detail Progress card. Todo falls back to the remainder so a
// payload without the per-type counts still renders a sensible bar.
function counts(m: Module) {
  const done = m.completed_tasks;
  const archived = m.archived_tasks ?? 0;
  const verification = m.in_verification_tasks ?? 0;
  const inProgress = m.in_progress_tasks ?? 0;
  const cancelled = m.cancelled_tasks ?? 0;
  const todo =
    m.todo_tasks ??
    Math.max(0, m.total_tasks - done - archived - verification - inProgress - cancelled);
  return { done, archived, verification, inProgress, todo, cancelled };
}

function pct(m: Module): number {
  const n = counts(m);
  const countable = m.total_tasks - n.cancelled;
  if (countable <= 0) return 0;
  return Math.round(((n.done + n.archived) / countable) * 100);
}

const cols = "grid-template-columns: minmax(0,1fr) 8rem 6rem 9rem 8rem 6rem;";
</script>

<template>
  <div class="overflow-hidden rounded-lg border bg-background">
    <div
      class="grid items-center gap-3 border-b bg-muted/40 px-4 py-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground"
      :style="cols"
    >
      <span>Title</span>
      <span>Status</span>
      <span title="Priority rating">Priority ★</span>
      <span>Progress</span>
      <span>Lead</span>
      <span class="justify-self-end">Due</span>
    </div>

    <div class="max-h-[70vh] overflow-y-auto [scrollbar-gutter:stable]">
      <NuxtLink
        v-for="m in modules"
        :key="m.id"
        :to="`/projects/${projectKey}/modules/${m.id}`"
        class="grid items-center gap-3 border-b border-border/40 px-4 py-2.5 text-sm transition-colors last:border-0 hover:bg-muted/40"
        :style="cols"
      >
        <span class="min-w-0 truncate font-medium text-foreground">{{ m.title }}</span>
        <span
          class="inline-flex w-fit items-center rounded-md border px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide"
          :class="statusStyles[m.status] || statusStyles.backlog"
        >
          {{ m.status.replace("_", " ") }}
        </span>
        <span class="flex items-center">
          <PriorityRating :model-value="m.priority_rating ?? 0" />
        </span>
        <div class="flex items-center gap-2">
          <LifecycleBar
            class="flex-1"
            :done="counts(m).done"
            :archived="counts(m).archived"
            :verification="counts(m).verification"
            :in-progress="counts(m).inProgress"
            :todo="counts(m).todo"
          />
          <span class="w-8 text-right text-xs tabular-nums text-muted-foreground">{{ pct(m) }}%</span>
        </div>
        <span class="min-w-0 truncate text-xs text-muted-foreground">
          {{ m.lead ? `${m.lead.first_name} ${m.lead.last_name}`.trim() : "—" }}
        </span>
        <span class="justify-self-end text-xs text-muted-foreground">
          {{ m.end_date || "—" }}
        </span>
      </NuxtLink>
    </div>
  </div>
</template>
