<script setup lang="ts">
import { ArrowRight } from "lucide-vue-next";
import type { WorkspacePersonInsights } from "~/types";

// The cross-project person report: one person summed over the projects the
// viewer may see. Evidence lives one level down — each project row opens that
// project's person view, where every number has its drawer — so this surface
// never shows a number it cannot back.
const props = defineProps<{ person: WorkspacePersonInsights }>();

const emit = defineEmits<{ project: [projectKey: string] }>();

const narrative = computed(() => {
  const p = props.person;
  const active = p.projects.filter((r) => r.delivered > 0).length;
  let s = `${p.delivered.count} items completed across ${active} of ${p.projects_considered} projects`;
  if (p.delivered.cx && p.delivered.cx.assessed > 0) {
    s += ` (complexity ${p.delivered.cx.sum}`;
    if (p.delivered.cx.not_assessed > 0) s += `, ${p.delivered.cx.not_assessed} not assessed`;
    s += ")";
  }
  if (p.top_rank_handled > 0) s += `; ${p.top_rank_handled} at top priority ranks`;
  s += `. ${p.comments} comments written, ${p.created} tasks created.`;
  return s;
});

const maxDelivered = computed(() => Math.max(1, ...props.person.projects.map((r) => r.delivered)));

function cxLabel(row: { cx?: { sum: number; assessed: number; not_assessed: number; of: number } }): string {
  if (!row.cx || row.cx.of === 0) return "";
  if (row.cx.assessed === 0) return "not assessed";
  let s = `cx ${row.cx.sum}`;
  if (row.cx.not_assessed > 0) s += ` · ${row.cx.not_assessed} n/a`;
  return s;
}
</script>

<template>
  <section>
    <h2 class="text-lg font-semibold">
      What did {{ person.user.first_name }} {{ person.user.last_name }} deliver across projects?
    </h2>
    <p class="mt-1 text-sm text-muted-foreground">{{ narrative }}</p>
    <p class="mt-1 text-[11px] text-muted-foreground">
      Summed over the {{ person.projects_considered }} projects you are a member of; a viewer with different
      project access sees different totals.
    </p>

    <div class="mt-4 flex flex-wrap gap-3">
      <MetricTile label="Completed" :count="person.delivered.count" :cx="person.delivered.cx" highlight />
      <MetricTile label="Top priority handled" :count="person.top_rank_handled" />
      <MetricTile label="Open assigned now" :count="person.open_now" />
      <MetricTile label="Comments" :count="person.comments" />
      <MetricTile label="Created" :count="person.created" />
    </div>

    <div class="mt-6 rounded-lg border p-4">
      <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">By project</h3>
      <div class="mt-2 space-y-1">
        <button
          v-for="row in person.projects"
          :key="row.project_key"
          class="group flex w-full items-center gap-3 rounded px-1.5 py-1 text-left outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
          :title="`Open ${person.user.first_name}'s report in ${row.project_name} — every number there has its evidence`"
          @click="emit('project', row.project_key)"
        >
          <span class="w-44 shrink-0 truncate text-sm">{{ row.project_name }}</span>
          <div class="h-3 shrink-0 rounded-sm bg-emerald-600/80 dark:bg-emerald-400/70" :style="{ width: Math.max(2, (row.delivered / maxDelivered) * 160) + 'px' }" />
          <span class="shrink-0 text-xs tabular-nums">{{ row.delivered }}</span>
          <span class="shrink-0 text-[11px] text-muted-foreground">{{ cxLabel(row) }}</span>
          <span class="ml-auto hidden shrink-0 flex-wrap gap-x-3 text-[11px] text-muted-foreground sm:flex">
            <span v-if="row.top_rank_handled">{{ row.top_rank_handled }} top priority</span>
            <span v-if="row.open_now">{{ row.open_now }} open now</span>
            <span v-if="row.comments">{{ row.comments }} comments</span>
          </span>
          <ArrowRight class="size-3 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
        </button>
        <p v-if="person.projects.length === 0" class="py-4 text-center text-sm text-muted-foreground">
          No shared projects hold activity by this person.
        </p>
      </div>
      <p class="mt-2 text-[11px] text-muted-foreground">
        A row opens that project's person report, where every number carries its evidence.
      </p>
    </div>

    <div class="mt-6 grid gap-6 lg:grid-cols-2">
      <div class="rounded-lg border p-4">
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Completions per week · all projects</h3>
        <WeeklyBars class="mt-2" :weeks="(person.weekly ?? []).map((w) => ({ ...w, created: 0 }))" />
      </div>
      <div class="rounded-lg border p-4">
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Worked with</h3>
        <div class="mt-2 flex flex-wrap gap-1.5">
          <span v-for="ww in person.worked_with ?? []" :key="ww.user.id" class="rounded-full border bg-muted/40 px-2 py-0.5 text-[11px] text-muted-foreground">
            {{ ww.user.first_name }} {{ ww.user.last_name }} · {{ ww.count }} shared
          </span>
          <p v-if="(person.worked_with ?? []).length === 0" class="text-xs text-muted-foreground">
            No shared credit in this window.
          </p>
        </div>
      </div>
    </div>

    <p class="mt-4 text-[11px] text-muted-foreground">{{ person.credit_rule }}</p>
  </section>
</template>
