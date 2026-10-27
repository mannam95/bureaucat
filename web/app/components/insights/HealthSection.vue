<script setup lang="ts">
import type { InsightsHealth } from "~/types";

// Question 3: is the project healthy right now? A snapshot at generation
// time, independent of the selected window. Aging rows always name the state,
// so a deliberately parked item reads as parked, not stuck.
const props = defineProps<{ health: InsightsHealth; projectName: string; projectKey: string }>();

// Task keys open in a new tab: the report stays where the reader left it.
function taskHref(key: string): string {
  return `/projects/${props.projectKey}/tasks/${key.split("-").pop()}`;
}

const emit = defineEmits<{
  open: [metric: string, params?: { state_id?: string; rank?: number }];
}>();

const narrative = computed(() => {
  const h = props.health;
  let s = `${h.open} open items, ${h.in_started} in a started state`;
  if (h.in_verification > 0) s += `, ${h.in_verification} in verification`;
  if (h.blocked > 0) s += `, ${h.blocked} blocked`;
  if (h.overdue > 0) s += `, ${h.overdue} overdue`;
  const oldest = h.aging[0];
  if (oldest) s += `. The oldest unchanged item has sat ${oldest.days} days in ${oldest.state.name}`;
  return s + ".";
});

const totalOpen = computed(() => Math.max(1, props.health.open));
const maxMix = computed(() => Math.max(1, ...props.health.priority_mix.map((m) => m.count)));
</script>

<template>
  <section>
    <h2 class="text-lg font-semibold">3 · What's open right now?</h2>
    <p class="mt-1 text-sm text-muted-foreground">{{ narrative }}</p>

    <div class="mt-4 grid gap-6 lg:grid-cols-2">
      <!-- Open work by state: the project's own vocabulary, lifecycle order. -->
      <div class="rounded-lg border p-4">
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          Open work by state · {{ health.open }} items
        </h3>
        <div class="mt-3 flex h-3 w-full overflow-hidden rounded-full bg-muted">
          <button
            v-for="sc in health.open_by_state"
            :key="sc.state.id"
            :style="{ width: (sc.count / totalOpen) * 100 + '%', backgroundColor: sc.state.color }"
            :title="`${sc.state.name}: ${sc.count} — open the list`"
            class="h-full transition-opacity hover:opacity-80"
            @click="emit('open', 'open_state', { state_id: sc.state.id })"
          />
        </div>
        <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs">
          <button
            v-for="sc in health.open_by_state"
            :key="sc.state.id"
            class="flex items-center gap-1.5 rounded px-1 outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
            @click="emit('open', 'open_state', { state_id: sc.state.id })"
          >
            <span class="size-2 rounded-full" :style="{ backgroundColor: sc.state.color }" />
            {{ sc.state.name }} <span class="tabular-nums text-muted-foreground">{{ sc.count }}</span>
          </button>
        </div>
        <p class="mt-2 text-[11px] text-muted-foreground">
          States and colors are the project's own, grouped by lifecycle type.
        </p>
      </div>

      <!-- Priority mix by rank with the project's own labels. -->
      <div class="rounded-lg border p-4">
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Priority mix of open work</h3>
        <div class="mt-3 space-y-1.5">
          <button
            v-for="m in health.priority_mix"
            :key="m.priority.id"
            class="flex w-full items-center gap-2 rounded px-1 text-left outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
            @click="emit('open', 'priority_rank', { rank: m.priority.rank })"
          >
            <span class="w-36 shrink-0 truncate text-xs text-muted-foreground">
              {{ m.priority.name }}<span v-if="!m.priority.active"> (inactive)</span>
            </span>
            <div class="h-3 rounded-sm" :style="{ width: Math.max(2, (m.count / maxMix) * 160) + 'px', backgroundColor: m.priority.color || '#9ca3af' }" />
            <span class="text-xs tabular-nums">{{ m.count }}</span>
          </button>
        </div>
      </div>

      <!-- Aging -->
      <div class="rounded-lg border p-4">
        <div class="flex items-baseline justify-between">
          <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            Aging · longest unchanged in flight
          </h3>
          <button class="text-[11px] text-muted-foreground underline-offset-2 hover:underline" @click="emit('open', 'aging')">
            all
          </button>
        </div>
        <div class="mt-2 space-y-1.5">
          <div v-for="a in health.aging.slice(0, 5)" :key="a.key" class="flex items-center gap-2 text-sm">
            <a :href="taskHref(a.key)" target="_blank" rel="noopener" class="shrink-0 font-mono text-xs text-muted-foreground underline-offset-2 hover:underline" title="Opens in a new tab">{{ a.key }}</a>
            <span class="min-w-0 flex-1 truncate">{{ a.title }}</span>
            <span class="shrink-0 rounded-full border px-1.5 py-0.5 text-[10px]" :style="{ borderColor: a.state.color, color: a.state.color }">
              {{ a.state.name }}
            </span>
            <span class="shrink-0 tabular-nums text-xs" :class="a.days >= 14 ? 'text-red-700 dark:text-red-400' : 'text-muted-foreground'">
              {{ a.days }} d
            </span>
          </div>
          <p v-if="health.aging.length === 0" class="text-xs text-muted-foreground">Nothing is sitting in a started or verification state.</p>
        </div>
      </div>

      <!-- Overdue / due soon -->
      <div class="rounded-lg border p-4">
        <div class="flex items-baseline justify-between">
          <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Overdue and due within 7 days</h3>
          <button class="text-[11px] text-muted-foreground underline-offset-2 hover:underline" @click="emit('open', 'overdue')">
            all overdue
          </button>
        </div>
        <div class="mt-2 space-y-1.5">
          <div v-for="dr in health.due.slice(0, 5)" :key="dr.key" class="flex items-center gap-2 text-sm">
            <a :href="taskHref(dr.key)" target="_blank" rel="noopener" class="shrink-0 font-mono text-xs text-muted-foreground underline-offset-2 hover:underline" title="Opens in a new tab">{{ dr.key }}</a>
            <span class="min-w-0 flex-1 truncate">{{ dr.title }}</span>
            <span class="shrink-0 text-xs tabular-nums" :class="dr.late_days > 0 ? 'font-medium text-red-700 dark:text-red-400' : 'text-muted-foreground'">
              {{ dr.late_days > 0 ? `${dr.late_days} d late` : `due in ${-dr.late_days} d` }}
            </span>
          </div>
          <p v-if="health.due.length === 0" class="text-xs text-muted-foreground">Nothing overdue or due soon.</p>
        </div>
      </div>
    </div>

    <div class="mt-6">
      <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Created vs completed · per week, last 4 weeks</h3>
      <WeeklyBars class="mt-2" :weeks="health.weekly" />
    </div>
  </section>
</template>
