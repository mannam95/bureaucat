<script setup lang="ts">
interface StateBucket {
  state_id: string;
  state_name: string;
  state_color: string;
  state_type: string;
  count: number;
}

withDefaults(
  defineProps<{
    buckets: StateBucket[];
    // When interactive, each state row becomes a clickable filter toggle.
    interactive?: boolean;
    activeStateId?: string | null;
  }>(),
  { interactive: false, activeStateId: null }
);

const emit = defineEmits<{ select: [stateId: string] }>();
</script>

<template>
  <section v-if="buckets.length" class="rounded-lg border p-4">
    <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
      By state
    </h3>
    <ul class="space-y-1.5">
      <template v-for="b in buckets" :key="b.state_id">
        <!-- Static (default) -->
        <li
          v-if="!interactive"
          class="flex items-center justify-between gap-2 text-sm"
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
            class="flex w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors"
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
  </section>
</template>
