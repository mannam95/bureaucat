<script setup lang="ts">
interface ProgressMetrics {
  total: number;
  completed: number;
  in_progress: number;
  todo: number;
  cancelled: number;
  archived?: number;
}

const props = defineProps<{
  metrics: ProgressMetrics | null;
}>();

const donePct = computed(() => {
  const m = props.metrics;
  if (!m || m.total === 0) return 0;
  return (m.completed / m.total) * 100;
});
const archivedPct = computed(() => {
  const m = props.metrics;
  if (!m || m.total === 0) return 0;
  return ((m.archived ?? 0) / m.total) * 100;
});
// Archived counts as complete: a cycle that is all done-or-archived reads 100%.
const progressPct = computed(() => Math.round(donePct.value + archivedPct.value));
</script>

<template>
  <section class="rounded-lg border p-4">
    <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
      Progress
    </h3>
    <div class="flex items-baseline gap-2">
      <span class="text-3xl font-bold tabular-nums">{{ progressPct }}%</span>
      <span class="text-sm text-muted-foreground">
        {{ metrics?.total ?? 0 }} task{{ (metrics?.total ?? 0) === 1 ? "" : "s" }}
      </span>
    </div>
    <!-- Done (solid) + archived (muted) fill the bar; together they reach 100%. -->
    <div class="mt-3 flex h-2 w-full overflow-hidden rounded-full bg-muted">
      <div class="h-full bg-amber-500 transition-all" :style="{ width: donePct + '%' }" />
      <div class="h-full bg-amber-500/40 transition-all" :style="{ width: archivedPct + '%' }" />
    </div>
    <dl class="mt-4 grid grid-cols-2 gap-2 text-xs">
      <div class="flex justify-between">
        <dt class="text-muted-foreground">Todo</dt>
        <dd class="font-medium tabular-nums">{{ metrics?.todo ?? 0 }}</dd>
      </div>
      <div class="flex justify-between">
        <dt class="text-muted-foreground">In progress</dt>
        <dd class="font-medium tabular-nums">{{ metrics?.in_progress ?? 0 }}</dd>
      </div>
      <div class="flex justify-between">
        <dt class="text-muted-foreground">Done</dt>
        <dd class="font-medium tabular-nums">{{ metrics?.completed ?? 0 }}</dd>
      </div>
      <div class="flex justify-between">
        <dt class="text-muted-foreground">Cancelled</dt>
        <dd class="font-medium tabular-nums">{{ metrics?.cancelled ?? 0 }}</dd>
      </div>
      <div v-if="(metrics?.archived ?? 0) > 0" class="flex justify-between">
        <dt class="text-muted-foreground">Archived</dt>
        <dd class="font-medium tabular-nums">{{ metrics?.archived }}</dd>
      </div>
    </dl>
  </section>
</template>
