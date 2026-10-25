<script setup lang="ts">
import type { ProjectInsights } from "~/types";

// Question 1: how did the cycle (or window) land? Tiles, the reconciliation
// identity, the daily chart and the complexity bands. Tiles for state types
// the project does not define are absent, not zero.
const props = defineProps<{ report: ProjectInsights }>();

const emit = defineEmits<{
  open: [metric: string];
  day: [date: string];
}>();

const cycle = computed(() => props.report.cycle);
const range = computed(() => props.report.range);

const planPercent = computed(() => {
  if (!cycle.value || cycle.value.planned_at_start.count === 0) return null;
  return Math.round((cycle.value.original_plan_delivered.count / cycle.value.planned_at_start.count) * 100);
});

const endScopePercent = computed(() => {
  if (!cycle.value || cycle.value.at_close === 0) return null;
  return Math.round((cycle.value.delivered.count / cycle.value.at_close) * 100);
});

const narrative = computed(() => {
  if (cycle.value) {
    const c = cycle.value;
    let s = `${c.delivered.count} work items completed`;
    if (planPercent.value !== null) {
      s += `; ${c.original_plan_delivered.count} of ${c.planned_at_start.count} planned items delivered (${planPercent.value}% of the original plan)`;
    }
    s += `. The cycle started with ${c.planned_at_start.count} items`;
    if (c.added.count > 0) s += `, ${c.added.count} were added`;
    if (c.removed.count > 0) s += ` and ${c.removed.count} removed`;
    s += `. ${c.carried_over.count} carried over`;
    if (c.cancelled && c.cancelled.count > 0) s += `, ${c.cancelled.count} cancelled`;
    if (c.archived && c.archived.count > 0) s += `, ${c.archived.count} archived`;
    return s + ".";
  }
  if (range.value) {
    const r = range.value;
    let s = `${r.completed.count} work items completed and ${r.created.count} created in the window`;
    if (r.cancelled && r.cancelled.count > 0) s += `; ${r.cancelled.count} cancelled (never counted as delivery)`;
    return s + ".";
  }
  return "";
});

const bandLabels: Record<string, string> = {
  "1-4": "Low · 1-4",
  "5-9": "Medium · 5-9",
  "10-16": "High · 10-16",
  "17+": "Very high · 17+",
  not_assessed: "Not assessed",
};

const bands = computed(() => (cycle.value ?? range.value)?.bands ?? []);
const maxBand = computed(() => Math.max(1, ...bands.value.map((b) => b.count)));
const days = computed(() => (cycle.value ?? range.value)?.days ?? []);
</script>

<template>
  <section>
    <h2 class="text-lg font-semibold">
      1 · How did {{ report.scope.cycle_title ? report.scope.cycle_title + " land" : "the window land" }}?
    </h2>
    <p class="mt-1 text-sm text-muted-foreground">{{ narrative }}</p>

    <!-- Cycle tiles -->
    <div v-if="cycle" class="mt-4 flex flex-wrap gap-3">
      <MetricTile
        label="Delivered"
        :count="cycle.delivered.count"
        metric="delivered"
        :cx="cycle.delivered.cx"
        highlight
        :sub="endScopePercent !== null ? `${endScopePercent}% of end scope (${cycle.at_close})` : undefined"
        @open="emit('open', $event)"
      />
      <MetricTile
        label="Original plan"
        :count="cycle.original_plan_delivered.count"
        metric="original_plan"
        :sub="planPercent !== null ? `of ${cycle.planned_at_start.count} planned · ${planPercent}%` : undefined"
        @open="emit('open', $event)"
      />
      <MetricTile
        label="Planned at start"
        :count="cycle.planned_at_start.count"
        metric="planned"
        :cx="cycle.planned_at_start.cx"
        @open="emit('open', $event)"
      />
      <MetricTile label="Added" :count="cycle.added.count" metric="added" sign="plus" @open="emit('open', $event)" />
      <MetricTile label="Descoped" :count="cycle.removed.count" metric="descoped" sign="minus" @open="emit('open', $event)" />
      <MetricTile
        label="Carried over"
        :count="cycle.carried_over.count"
        metric="carried"
        :cx="cycle.carried_over.cx"
        @open="emit('open', $event)"
      />
      <MetricTile
        v-if="cycle.cancelled"
        label="Cancelled"
        :count="cycle.cancelled.count"
        metric="cancelled"
        sub="not delivery"
        @open="emit('open', $event)"
      />
      <MetricTile
        v-if="cycle.archived"
        label="Archived"
        :count="cycle.archived.count"
        metric="archived"
        @open="emit('open', $event)"
      />
    </div>

    <!-- Range tiles -->
    <div v-else-if="range" class="mt-4 flex flex-wrap gap-3">
      <MetricTile label="Completed" :count="range.completed.count" metric="completed" :cx="range.completed.cx" highlight @open="emit('open', $event)" />
      <MetricTile label="Created" :count="range.created.count" metric="created" @open="emit('open', $event)" />
      <MetricTile
        v-if="range.cancelled"
        label="Cancelled"
        :count="range.cancelled.count"
        metric="cancelled"
        sub="not delivery"
        @open="emit('open', $event)"
      />
    </div>

    <!-- The reconciliation identity: the whole report visibly adds up. -->
    <p v-if="cycle" class="mt-3 rounded-md border bg-muted/30 px-3 py-2 font-mono text-xs text-muted-foreground">
      {{ cycle.identity }}
    </p>

    <!-- No-cycles hint: the date-range report is the report, not a fallback. -->
    <div v-if="range && !range.has_cycles" class="mt-3 rounded-md border border-dashed px-3 py-2 text-xs text-muted-foreground">
      This project runs without cycles, so the report covers the selected date range. Project admins can create a cycle to add
      planned-vs-delivered reporting.
    </div>

    <div class="mt-6 grid gap-6 lg:grid-cols-[1fr_280px]">
      <div>
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          Scope vs completed · daily, reconstructed from the activity log
        </h3>
        <ScopeChart class="mt-2" :days="days" caption="click a marked day for its events" @day="emit('day', $event)" />
      </div>
      <div>
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Delivered by complexity band</h3>
        <div class="mt-2 space-y-1.5">
          <div v-for="b in bands" :key="b.band" class="flex items-center gap-2">
            <span class="w-24 shrink-0 text-xs text-muted-foreground">{{ bandLabels[b.band] || b.band }}</span>
            <div
              class="h-3 rounded-sm"
              :class="b.band === 'not_assessed' ? 'bg-muted-foreground/30 [background-image:repeating-linear-gradient(45deg,transparent,transparent_3px,rgba(0,0,0,0.15)_3px,rgba(0,0,0,0.15)_6px)]' : 'bg-sky-600/80 dark:bg-sky-400/70'"
              :style="{ width: Math.max(2, (b.count / maxBand) * 140) + 'px' }"
            />
            <span class="text-xs tabular-nums">{{ b.count }}</span>
          </div>
        </div>
        <p class="mt-2 text-[11px] text-muted-foreground">
          Complexity = difficulty x effort, as set on each task. Items with either value unset are "not assessed", never dropped.
        </p>
      </div>
    </div>
  </section>
</template>
