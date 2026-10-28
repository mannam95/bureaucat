<script setup lang="ts">
// Flat list view of the cycles overview, harmonised with the task tables.
// Latest cycle first (the list is already ordered that way server-side).
import type { Cycle } from "~/types";

defineProps<{
  cycles: Cycle[];
  projectKey: string;
}>();

const statusStyles: Record<string, string> = {
  upcoming: "border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-300",
  active: "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  completed: "border-muted-foreground/30 bg-muted text-muted-foreground",
};

// Archived counts as complete and cancelled leaves the denominator,
// matching the detail Progress card. Todo falls back to the remainder so a
// payload without the per-type counts still renders a sensible bar.
function counts(c: Cycle) {
  const done = c.completed_tasks;
  const archived = c.archived_tasks ?? 0;
  const verification = c.in_verification_tasks ?? 0;
  const inProgress = c.in_progress_tasks ?? 0;
  const cancelled = c.cancelled_tasks ?? 0;
  const todo =
    c.todo_tasks ??
    Math.max(0, c.total_tasks - done - archived - verification - inProgress - cancelled);
  return { done, archived, verification, inProgress, todo, cancelled };
}

function pct(c: Cycle): number {
  const n = counts(c);
  const countable = c.total_tasks - n.cancelled;
  if (countable <= 0) return 0;
  return Math.round(((n.done + n.archived) / countable) * 100);
}

function range(c: Cycle): string {
  if (!c.start_date && !c.end_date) return "—";
  return `${c.start_date || "…"} → ${c.end_date || "…"}`;
}

const cols = "grid-template-columns: minmax(0,1fr) 8rem 14rem 9rem;";
</script>

<template>
  <div class="overflow-hidden rounded-lg border bg-background">
    <div
      class="grid items-center gap-3 border-b bg-muted/40 px-4 py-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground"
      :style="cols"
    >
      <span>Title</span>
      <span>Status</span>
      <span>Dates</span>
      <span>Progress</span>
    </div>

    <div class="max-h-[70vh] overflow-y-auto [scrollbar-gutter:stable]">
      <NuxtLink
        v-for="c in cycles"
        :key="c.id"
        :to="`/projects/${projectKey}/cycles/${c.id}`"
        class="grid items-center gap-3 border-b border-border/40 px-4 py-2.5 text-sm transition-colors last:border-0 hover:bg-muted/40"
        :style="cols"
      >
        <span class="min-w-0 truncate font-medium text-foreground">{{ c.title }}</span>
        <span
          class="inline-flex w-fit items-center rounded-md border px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide"
          :class="statusStyles[c.status] || statusStyles.upcoming"
        >
          {{ c.status }}
        </span>
        <span class="truncate text-xs text-muted-foreground">{{ range(c) }}</span>
        <div class="flex items-center gap-2">
          <LifecycleBar
            class="flex-1"
            :done="counts(c).done"
            :archived="counts(c).archived"
            :verification="counts(c).verification"
            :in-progress="counts(c).inProgress"
            :todo="counts(c).todo"
          />
          <span class="w-8 text-right text-xs tabular-nums text-muted-foreground">{{ pct(c) }}%</span>
        </div>
      </NuxtLink>
    </div>
  </div>
</template>
