<script setup lang="ts">
interface ProgressMetrics {
  total: number;
  completed: number;
  in_progress: number;
  in_verification?: number;
  todo: number;
  cancelled: number;
  archived?: number;
}

// Per-state counts, used only to tell on hover which states feed each bucket.
interface StateBucket {
  state_name: string;
  state_type: string;
  count: number;
}

const props = defineProps<{
  metrics: ProgressMetrics | null;
  buckets?: StateBucket[];
}>();

// Cancelled is excluded from the denominator (the same rule the subtask
// rollup and Insights apply): the percentage can honestly reach 100 even
// when work was cancelled along the way.
const countable = computed(() => {
  const m = props.metrics;
  if (!m) return 0;
  return Math.max(0, m.total - m.cancelled);
});
const progressPct = computed(() => {
  const m = props.metrics;
  if (!m || countable.value === 0) return 0;
  return Math.round(((m.completed + (m.archived ?? 0)) / countable.value) * 100);
});

// A bucket is a lifecycle type; the hover names the member states, built live
// from the state breakdown so workflow edits show up with zero code changes.
function memberStates(types: string[]): string | undefined {
  const rows = (props.buckets ?? []).filter((b) => types.includes(b.state_type));
  if (rows.length === 0) return undefined;
  return rows.map((b) => `${b.state_name} ${b.count}`).join(" · ");
}

const rows = computed(() => {
  const m = props.metrics;
  if (!m) return [];
  const out = [
    { key: "done", label: "Done", count: m.completed, dot: "bg-emerald-500 dark:bg-emerald-400", title: memberStates(["completed"]) },
    { key: "verification", label: "In verification", count: m.in_verification ?? 0, dot: "bg-violet-500 dark:bg-violet-400", title: memberStates(["verification"]) },
    { key: "in_progress", label: "In progress", count: m.in_progress, dot: "bg-amber-500", title: memberStates(["started"]) },
    { key: "todo", label: "Todo", count: m.todo, dot: "bg-muted-foreground/40", title: memberStates(["backlog", "unstarted"]) },
  ];
  // In verification only renders for projects that use the type.
  return out.filter((r) => r.key !== "verification" || r.count > 0);
});
</script>

<template>
  <section class="rounded-lg border p-4">
    <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
      Progress
    </h3>
    <div class="flex items-baseline gap-2">
      <span
        class="text-3xl font-bold tabular-nums"
        :title="`${(metrics?.completed ?? 0) + (metrics?.archived ?? 0)} of ${countable} tasks delivered (cancelled excluded)`"
      >{{ progressPct }}%</span>
      <span class="text-sm text-muted-foreground">
        delivered · {{ metrics?.total ?? 0 }} task{{ (metrics?.total ?? 0) === 1 ? "" : "s" }}
      </span>
    </div>
    <LifecycleBar
      class="mt-3"
      height-class="h-2"
      :done="metrics?.completed ?? 0"
      :archived="metrics?.archived ?? 0"
      :verification="metrics?.in_verification ?? 0"
      :in-progress="metrics?.in_progress ?? 0"
      :todo="metrics?.todo ?? 0"
    />
    <dl class="mt-4 space-y-1.5 text-sm">
      <div
        v-for="r in rows"
        :key="r.key"
        class="flex items-center justify-between gap-2"
        :title="r.title"
      >
        <dt class="flex items-center gap-2 text-muted-foreground">
          <span class="size-2 rounded-full" :class="r.dot" />
          {{ r.label }}
        </dt>
        <dd class="font-medium tabular-nums">{{ r.count }}</dd>
      </div>
      <!-- Outside the bar and the percentage: cancelled is never progress,
           archived already rides on the done segment. -->
      <div
        v-if="(metrics?.cancelled ?? 0) > 0"
        class="flex items-center justify-between gap-2 pl-4 text-xs text-muted-foreground"
        :title="memberStates(['cancelled'])"
      >
        <dt>Cancelled · not counted</dt>
        <dd class="tabular-nums">{{ metrics?.cancelled }}</dd>
      </div>
      <div
        v-if="(metrics?.archived ?? 0) > 0"
        class="flex items-center justify-between gap-2 pl-4 text-xs text-muted-foreground"
        :title="memberStates(['archived'])"
      >
        <dt>Archived · counts as done</dt>
        <dd class="tabular-nums">{{ metrics?.archived }}</dd>
      </div>
    </dl>
  </section>
</template>
