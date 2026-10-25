<script setup lang="ts">
import type { DayPoint } from "~/types";

// The scope-vs-completed daily chart, reconstructed from the activity log.
// Clicking a day opens that day's events as evidence. Hand-rolled SVG: the
// data is small and the control is worth more than a chart library.
const props = defineProps<{
  days: DayPoint[];
  caption?: string;
}>();

const emit = defineEmits<{ day: [date: string] }>();

const W = 720;
const H = 160;
const PAD = { left: 30, right: 10, top: 10, bottom: 22 };

const maxY = computed(() => Math.max(1, ...props.days.map((d) => Math.max(d.scope, d.completed_cum))));

function x(i: number): number {
  const n = Math.max(1, props.days.length - 1);
  return PAD.left + (i * (W - PAD.left - PAD.right)) / n;
}
function y(v: number): number {
  return H - PAD.bottom - (v * (H - PAD.top - PAD.bottom)) / maxY.value;
}

const scopePath = computed(() =>
  props.days.map((d, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(d.scope).toFixed(1)}`).join(" ")
);
const donePath = computed(() =>
  props.days.map((d, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(d.completed_cum).toFixed(1)}`).join(" ")
);

// Days where scope changed or work completed carry a clickable marker.
const markers = computed(() =>
  props.days
    .map((d, i) => ({ d, i }))
    .filter(({ d }) => d.added > 0 || d.removed > 0 || d.completed > 0)
);

function dayLabel(date: string): string {
  const d = new Date(date + "T00:00:00");
  return d.toLocaleDateString("en-US", { day: "numeric", month: "short" });
}

const firstLabel = computed(() => (props.days.length ? dayLabel(props.days[0]!.date) : ""));
const lastLabel = computed(() =>
  props.days.length > 1 ? dayLabel(props.days[props.days.length - 1]!.date) : ""
);
</script>

<template>
  <div>
    <svg :viewBox="`0 0 ${W} ${H}`" class="w-full" role="img" aria-label="Scope vs completed, daily">
      <!-- weekend shading -->
      <template v-for="(d, i) in days" :key="d.date + '-we'">
        <rect
          v-if="d.weekend"
          :x="x(i) - (W - PAD.left - PAD.right) / Math.max(1, days.length - 1) / 2"
          :y="PAD.top"
          :width="(W - PAD.left - PAD.right) / Math.max(1, days.length - 1)"
          :height="H - PAD.top - PAD.bottom"
          class="fill-muted/60"
        />
      </template>
      <!-- axes ticks -->
      <text :x="PAD.left - 6" :y="y(0) + 4" text-anchor="end" class="fill-muted-foreground text-[10px]">0</text>
      <text :x="PAD.left - 6" :y="y(maxY) + 4" text-anchor="end" class="fill-muted-foreground text-[10px]">{{ maxY }}</text>
      <line :x1="PAD.left" :y1="y(0)" :x2="W - PAD.right" :y2="y(0)" class="stroke-border" stroke-width="1" />
      <!-- series -->
      <path :d="scopePath" fill="none" class="stroke-muted-foreground/70" stroke-width="1.5" />
      <path :d="donePath" fill="none" class="stroke-emerald-600 dark:stroke-emerald-400" stroke-width="2" />
      <!-- clickable day markers -->
      <circle
        v-for="{ d, i } in markers"
        :key="d.date"
        :cx="x(i)"
        :cy="y(d.completed_cum)"
        r="5"
        class="cursor-pointer fill-background stroke-amber-600 hover:fill-amber-500/30"
        stroke-width="1.5"
        tabindex="0"
        role="button"
        :aria-label="`Open events on ${d.date}`"
        @click="emit('day', d.date)"
        @keydown.enter="emit('day', d.date)"
      >
        <title>{{ dayLabel(d.date) }}: {{ d.completed }} completed, +{{ d.added }} / -{{ d.removed }} scope</title>
      </circle>
      <!-- first/last date labels -->
      <text v-if="firstLabel" :x="x(0)" :y="H - 6" class="fill-muted-foreground text-[10px]">{{ firstLabel }}</text>
      <text v-if="lastLabel" :x="x(days.length - 1)" :y="H - 6" text-anchor="end" class="fill-muted-foreground text-[10px]">
        {{ lastLabel }}
      </text>
    </svg>
    <div class="mt-1 flex items-center gap-4 text-[11px] text-muted-foreground">
      <span class="flex items-center gap-1.5"><span class="inline-block h-0.5 w-4 bg-muted-foreground/70" /> In scope</span>
      <span class="flex items-center gap-1.5"><span class="inline-block h-0.5 w-4 bg-emerald-600 dark:bg-emerald-400" /> Completed</span>
      <span v-if="caption" class="ml-auto">{{ caption }}</span>
    </div>
  </div>
</template>
