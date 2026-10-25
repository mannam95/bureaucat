<script setup lang="ts">
import type { DayPoint } from "~/types";

// The daily delivery chart, reconstructed from the activity log. Readable
// first: labeled axes, weekly date ticks, gridlines, and a legend that
// explains every mark. In cycle mode a grey line shows the scope; in range
// mode there is no scope concept, so only completion is drawn.
// Hand-rolled SVG: the data is small and this control is the point.
const props = defineProps<{
  days: DayPoint[];
  mode: "cycle" | "range";
}>();

const emit = defineEmits<{ day: [date: string] }>();

const W = 720;
const H = 200;
// Room on the right for the direct line labels ("15 in the cycle").
const PAD = { left: 46, right: 128, top: 12, bottom: 40 };

const maxY = computed(() => {
  const values = props.days.map((d) => (props.mode === "cycle" ? Math.max(d.scope, d.completed_cum) : d.completed_cum));
  return Math.max(1, ...values);
});

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

// Horizontal gridlines at round steps.
const yTicks = computed(() => {
  const max = maxY.value;
  const step = max <= 6 ? 2 : max <= 12 ? 4 : max <= 30 ? Math.ceil(max / 4 / 5) * 5 : Math.ceil(max / 4 / 10) * 10;
  const ticks: number[] = [];
  for (let v = 0; v <= max; v += step) ticks.push(v);
  if (ticks[ticks.length - 1] !== max) ticks.push(max);
  return ticks;
});

// One date tick per Monday (plus the first and last day).
const xTicks = computed(() => {
  const out: { i: number; label: string }[] = [];
  props.days.forEach((d, i) => {
    const day = new Date(d.date + "T00:00:00");
    const isEdge = i === 0 || i === props.days.length - 1;
    if (isEdge || day.getDay() === 1) out.push({ i, label: dayLabel(d.date) });
  });
  // Thin out when edges and a Monday sit on top of each other.
  return out.filter((t, idx) => idx === 0 || t.i - out[idx - 1]!.i > 2 || idx === out.length - 1);
});

// Days with completions get a clickable dot on the green line; days where the
// scope changed (cycle mode) get an amber marker on the grey line.
const doneDays = computed(() => props.days.map((d, i) => ({ d, i })).filter(({ d }) => d.completed > 0));
const scopeDays = computed(() =>
  props.mode === "cycle"
    ? props.days.map((d, i) => ({ d, i })).filter(({ d }) => d.added > 0 || d.removed > 0)
    : []
);

// Working days = the window minus weekends; shown under the x axis so the
// pace is judged against real days, simply.
const workingDays = computed(() => props.days.filter((d) => !d.weekend).length);

function dayLabel(date: string): string {
  return new Date(date + "T00:00:00").toLocaleDateString("en-US", { day: "numeric", month: "short" });
}

function doneTitle(d: DayPoint): string {
  return `${dayLabel(d.date)}: ${d.completed} completed (${d.completed_cum} in total by this day). Click for the day's events.`;
}

function scopeTitle(d: DayPoint): string {
  const parts = [];
  if (d.added > 0) parts.push(`${d.added} task${d.added > 1 ? "s" : ""} added to the cycle`);
  if (d.removed > 0) parts.push(`${d.removed} removed from the cycle`);
  return `${dayLabel(d.date)}: ${parts.join(", ")}. Click to see which.`;
}

// Direct labels at the line ends: the lines say what they are where they end.
const last = computed(() => props.days[props.days.length - 1]);
const doneEnd = computed(() => (last.value ? { y: y(last.value.completed_cum), n: last.value.completed_cum } : null));
const scopeEnd = computed(() => (last.value && props.mode === "cycle" ? { y: y(last.value.scope), n: last.value.scope } : null));
// Nudge the labels apart when the lines end close together.
const labelYs = computed(() => {
  let d = doneEnd.value ? doneEnd.value.y : 0;
  let sc = scopeEnd.value ? scopeEnd.value.y : -999;
  if (scopeEnd.value && doneEnd.value && Math.abs(d - sc) < 12) {
    if (sc <= d) { sc -= (12 - (d - sc)); } else { d -= (12 - (sc - d)); }
  }
  return { done: d, scope: sc };
});
</script>

<template>
  <div>
    <svg :viewBox="`0 0 ${W} ${H}`" class="w-full" role="img" aria-label="Completed work per day">
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

      <!-- y gridlines with values -->
      <template v-for="t in yTicks" :key="'y' + t">
        <line :x1="PAD.left" :y1="y(t)" :x2="W - PAD.right" :y2="y(t)"
          class="stroke-border" stroke-width="1" :stroke-dasharray="t === 0 ? undefined : '3,3'" />
        <text :x="PAD.left - 6" :y="y(t) + 3.5" text-anchor="end" class="fill-muted-foreground text-[10px]">{{ t }}</text>
      </template>

      <!-- axis titles -->
      <text
        :x="12" :y="(PAD.top + H - PAD.bottom) / 2"
        :transform="`rotate(-90 12 ${(PAD.top + H - PAD.bottom) / 2})`"
        text-anchor="middle"
        class="fill-muted-foreground text-[10px] font-medium"
      >number of tasks</text>
      <text :x="(PAD.left + W - PAD.right) / 2" :y="H - 4" text-anchor="middle" class="fill-muted-foreground text-[10px] font-medium">
        {{ mode === 'cycle' ? 'days of the cycle' : 'days of the selected window' }}
        · {{ days.length }} days, {{ workingDays }} working days
      </text>

      <!-- direct line labels -->
      <text v-if="doneEnd" :x="x(days.length - 1) + 7" :y="labelYs.done + 3.5"
        class="fill-emerald-700 text-[11px] font-medium dark:fill-emerald-400">
        {{ doneEnd.n }} tasks finished
      </text>
      <text v-if="scopeEnd" :x="x(days.length - 1) + 7" :y="labelYs.scope + 3.5"
        class="fill-muted-foreground text-[11px] font-medium">
        {{ scopeEnd.n }} tasks in the cycle
      </text>

      <!-- x date ticks -->
      <template v-for="t in xTicks" :key="'x' + t.i">
        <line :x1="x(t.i)" :y1="H - PAD.bottom" :x2="x(t.i)" :y2="H - PAD.bottom + 4" class="stroke-border" stroke-width="1" />
        <text :x="x(t.i)" :y="H - PAD.bottom + 15" text-anchor="middle" class="fill-muted-foreground text-[10px]">{{ t.label }}</text>
      </template>

      <!-- series -->
      <path v-if="mode === 'cycle'" :d="scopePath" fill="none" class="stroke-muted-foreground/70" stroke-width="1.5" />
      <path :d="donePath" fill="none" class="stroke-emerald-600 dark:stroke-emerald-400" stroke-width="2" />

      <!-- scope-change markers (cycle mode): amber squares on the grey line -->
      <rect
        v-for="{ d, i } in scopeDays"
        :key="'s' + d.date"
        :x="x(i) - 4" :y="y(d.scope) - 4" width="8" height="8" rx="1.5"
        class="cursor-pointer fill-amber-500/90 hover:fill-amber-500"
        tabindex="0" role="button" :aria-label="scopeTitle(d)"
        @click="emit('day', d.date)" @keydown.enter="emit('day', d.date)"
      >
        <title>{{ scopeTitle(d) }}</title>
      </rect>

      <!-- completion markers: green dots on the green line -->
      <circle
        v-for="{ d, i } in doneDays"
        :key="'c' + d.date"
        :cx="x(i)" :cy="y(d.completed_cum)" r="4"
        class="cursor-pointer fill-emerald-600 stroke-background hover:fill-emerald-500 dark:fill-emerald-400"
        stroke-width="1.5"
        tabindex="0" role="button" :aria-label="doneTitle(d)"
        @click="emit('day', d.date)" @keydown.enter="emit('day', d.date)"
      >
        <title>{{ doneTitle(d) }}</title>
      </circle>
    </svg>

    <!-- Every mark on the chart gets its own row: glyph | name | meaning. -->
    <div class="mt-2 grid grid-cols-[1.5rem_6.5rem_1fr] items-center gap-x-2 gap-y-1 text-[11px] text-muted-foreground">
      <span class="inline-block h-0.5 w-5 bg-emerald-600 dark:bg-emerald-400" />
      <span class="font-medium text-foreground">Green line</span>
      <span>How many tasks were finished in total, day by day.</span>

      <span class="inline-block size-2 justify-self-start rounded-full bg-emerald-600 dark:bg-emerald-400" />
      <span class="font-medium text-foreground">Green dot</span>
      <span>A day on which tasks were finished. Click it to see which ones.</span>

      <template v-if="mode === 'cycle'">
        <span class="inline-block h-0.5 w-5 bg-muted-foreground/70" />
        <span class="font-medium text-foreground">Grey line</span>
        <span>How many tasks the cycle contained on that day (the plan).</span>

        <span class="inline-block size-2 justify-self-start rounded-[2px] bg-amber-500" />
        <span class="font-medium text-foreground">Orange square</span>
        <span>A day on which tasks were added to or removed from the cycle. Click it to see which ones.</span>
      </template>

      <span class="inline-block h-3 w-5 bg-muted/80" />
      <span class="font-medium text-foreground">Shaded column</span>
      <span>A weekend.</span>
    </div>
  </div>
</template>
