<script setup lang="ts">
import { STATE_TYPE_COLORS } from "~/types/task";
import { stateTypeLabel } from "~/components/filters/filterCatalog";

interface StateBucket {
  state_id: string;
  state_name: string;
  state_color: string;
  state_type: string;
  count: number;
}

const props = withDefaults(
  defineProps<{
    buckets: StateBucket[];
    // When interactive, each state row becomes a clickable filter toggle.
    interactive?: boolean;
    activeStateId?: string | null;
  }>(),
  { interactive: false, activeStateId: null }
);

const emit = defineEmits<{ select: [stateId: string] }>();

// One container per lifecycle type, so the banding is visible instead of
// implied. The server already delivers the buckets banded (type, then board
// position), so grouping consecutive rows preserves that order with no list
// of types hardcoded here.
const groups = computed(() => {
  const out: { type: string; buckets: StateBucket[] }[] = [];
  for (const b of props.buckets) {
    const last = out[out.length - 1];
    if (last && last.type === b.state_type) last.buckets.push(b);
    else out.push({ type: b.state_type, buckets: [b] });
  }
  return out;
});

// A faint tint of the type color: barely-there on white, a subtle lift in
// dark mode ("14" is ~8% alpha).
function tint(type: string): string {
  return (STATE_TYPE_COLORS[type] ?? "#6B7280") + "14";
}
</script>

<template>
  <section v-if="buckets.length" class="rounded-lg border p-4">
    <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
      By state
    </h3>
    <div class="space-y-2">
      <div
        v-for="g in groups"
        :key="g.type"
        class="rounded-md px-2 py-1.5"
        :style="{ backgroundColor: tint(g.type) }"
      >
        <div class="mb-1 px-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
          {{ stateTypeLabel(g.type) }}
        </div>
        <ul class="space-y-1">
          <template v-for="b in g.buckets" :key="b.state_id">
            <!-- Static (default) -->
            <li
              v-if="!interactive"
              class="flex items-center justify-between gap-2 px-1 text-sm"
            >
              <span class="flex items-center gap-2 truncate">
                <span
                  class="size-2 rounded-full"
                  :style="{ backgroundColor: b.state_color || '#6B7280' }"
                />
                <span class="truncate">{{ b.state_name }}</span>
              </span>
              <span class="font-medium tabular-nums text-muted-foreground">
                {{ b.count }}
              </span>
            </li>

            <!-- Interactive: click to filter the task list by this state -->
            <li v-else>
              <button
                type="button"
                class="flex w-full items-center justify-between gap-2 rounded-md px-1 py-1 text-left text-sm transition-colors"
                :class="
                  activeStateId === b.state_id
                    ? 'bg-amber-500/10 text-amber-700 dark:text-amber-400'
                    : 'hover:bg-muted'
                "
                @click="emit('select', b.state_id)"
              >
                <span class="flex min-w-0 items-center gap-2 truncate">
                  <span
                    class="size-2 shrink-0 rounded-full"
                    :style="{ backgroundColor: b.state_color || '#6B7280' }"
                  />
                  <span class="truncate">{{ b.state_name }}</span>
                </span>
                <span class="font-medium tabular-nums text-muted-foreground">
                  {{ b.count }}
                </span>
              </button>
            </li>
          </template>
        </ul>
      </div>
    </div>
  </section>
</template>
