<script setup lang="ts">
// Renders a task's resolved priority (per-project entity): colored dot + name.
const props = withDefaults(
  defineProps<{
    name?: string;
    color?: string;
    showLabel?: boolean;
  }>(),
  { name: "", color: "#6B7280", showLabel: false }
);
void props;
</script>

<template>
  <div class="flex items-center gap-1.5">
    <div class="size-2.5 rounded-full" :style="{ backgroundColor: color }" />
    <span v-if="showLabel" class="text-sm">{{ name }}</span>
  </div>
</template>
