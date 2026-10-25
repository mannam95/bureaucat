<script setup lang="ts">
import type { WeekPoint } from "~/types";

// Created vs completed per week, as paired bars with the numbers printed:
// honesty over prettiness, per the design direction.
const props = defineProps<{ weeks: WeekPoint[]; compact?: boolean }>();

const maxY = computed(() => Math.max(1, ...props.weeks.map((w) => Math.max(w.created, w.completed))));

function h(v: number): number {
  return Math.round((v / maxY.value) * (props.compact ? 24 : 48));
}

function weekLabel(ws: string): string {
  const d = new Date(ws + "T00:00:00");
  return d.toLocaleDateString("en-US", { day: "numeric", month: "short" });
}
</script>

<template>
  <div class="flex items-end gap-3">
    <div v-for="w in weeks" :key="w.week_start" class="flex flex-col items-center gap-0.5">
      <div class="flex items-end gap-0.5" :style="{ height: (compact ? 24 : 48) + 'px' }">
        <div
          class="w-2.5 rounded-t-sm bg-muted-foreground/40"
          :style="{ height: Math.max(2, h(w.created)) + 'px' }"
          :title="`${w.created} created`"
        />
        <div
          class="w-2.5 rounded-t-sm bg-emerald-600/80 dark:bg-emerald-400/80"
          :style="{ height: Math.max(2, h(w.completed)) + 'px' }"
          :title="`${w.completed} completed`"
        />
      </div>
      <span v-if="!compact" class="text-[10px] text-muted-foreground">{{ weekLabel(w.week_start) }}</span>
      <span class="text-[10px] tabular-nums text-muted-foreground">{{ w.created }}/{{ w.completed }}</span>
    </div>
  </div>
</template>
