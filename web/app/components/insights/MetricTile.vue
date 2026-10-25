<script setup lang="ts">
import type { ComplexitySum } from "~/types";

// One scope/outcome tile. Every tile is an evidence entry point: clicking it
// opens the exact population behind the number.
const props = defineProps<{
  label: string;
  count: number;
  metric?: string;
  cx?: ComplexitySum;
  sub?: string;
  highlight?: boolean;
  sign?: "plus" | "minus";
}>();

const emit = defineEmits<{ open: [metric: string] }>();

const display = computed(() => {
  if (props.sign === "plus") return `+${props.count}`;
  if (props.sign === "minus") return `−${props.count}`;
  return String(props.count);
});

const cxLine = computed(() => {
  if (!props.cx || props.cx.of === 0) return "";
  if (props.cx.assessed === 0) return "not assessed";
  let line = `cx ${props.cx.sum}`;
  if (props.cx.not_assessed > 0) line += ` · ${props.cx.not_assessed} not assessed`;
  return line;
});
</script>

<template>
  <component
    :is="metric ? 'button' : 'div'"
    class="flex min-w-28 flex-col rounded-lg border p-3 text-left transition-colors"
    :class="[
      highlight ? 'border-emerald-600/40 bg-emerald-500/10' : 'bg-card',
      metric && 'cursor-pointer hover:border-amber-500/60 focus-visible:ring-2 focus-visible:ring-ring outline-none',
    ]"
    :title="metric ? 'Open the exact population behind this number' : undefined"
    @click="metric && emit('open', metric)"
  >
    <span class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{{ label }}</span>
    <span class="mt-1 text-2xl font-semibold tabular-nums">{{ display }}</span>
    <span v-if="cxLine" class="mt-0.5 text-[11px] text-muted-foreground">{{ cxLine }}</span>
    <span v-if="sub" class="mt-0.5 text-[11px] text-muted-foreground">{{ sub }}</span>
  </component>
</template>
