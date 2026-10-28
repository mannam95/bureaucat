<script setup lang="ts">
// One visual language for cycle and module progress: a stacked lifecycle bar.
// Segments run done -> in verification -> in progress from the left, so the
// delivered percentage still maps onto the left edge like a classic progress
// bar; todo is the muted track itself, so an untouched cycle reads as the
// familiar empty bar. Cancelled is excluded from the denominator entirely:
// it is never progress and never outstanding work. Hovering a segment names
// its bucket and count.
const props = withDefaults(
  defineProps<{
    done: number;
    archived?: number;
    verification?: number;
    inProgress: number;
    todo: number;
    /** Bar height utility class; callers pick their scale. */
    heightClass?: string;
  }>(),
  { archived: 0, verification: 0, heightClass: "h-1.5" }
);

const denom = computed(
  () => props.done + props.archived + props.verification + props.inProgress + props.todo
);

function width(n: number): string {
  return denom.value === 0 ? "0%" : (n / denom.value) * 100 + "%";
}

// Archived rides as a faded tail on done, matching the Progress card's rule
// that archived counts as complete.
const segments = computed(() =>
  [
    { key: "done", label: "Done", count: props.done, cls: "bg-emerald-500 dark:bg-emerald-400" },
    { key: "archived", label: "Archived", count: props.archived, cls: "bg-emerald-500/40 dark:bg-emerald-400/40" },
    { key: "verification", label: "In verification", count: props.verification, cls: "bg-violet-500 dark:bg-violet-400" },
    { key: "in_progress", label: "In progress", count: props.inProgress, cls: "bg-amber-500" },
  ].filter((s) => s.count > 0)
);
</script>

<template>
  <div class="flex w-full overflow-hidden rounded-full bg-muted" :class="heightClass">
    <div
      v-for="s in segments"
      :key="s.key"
      class="h-full transition-all"
      :class="s.cls"
      :style="{ width: width(s.count) }"
      :title="`${s.label} · ${s.count}`"
    />
  </div>
</template>
