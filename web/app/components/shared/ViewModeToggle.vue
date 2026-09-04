<script setup lang="ts">
import { LayoutGrid, List } from "lucide-vue-next";
import type { ViewMode } from "~/composables/useViewMode";

const model = defineModel<ViewMode>({ default: "card" });
</script>

<template>
  <div class="inline-flex items-center rounded-md border p-0.5">
    <button
      type="button"
      class="flex size-7 items-center justify-center rounded-sm transition-colors"
      :class="model === 'card' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'"
      aria-label="Card view"
      @click="model = 'card'"
    >
      <LayoutGrid class="size-4" />
    </button>
    <button
      type="button"
      class="flex size-7 items-center justify-center rounded-sm transition-colors"
      :class="model === 'list' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'"
      aria-label="List view"
      @click="model = 'list'"
    >
      <List class="size-4" />
    </button>
  </div>
</template>
