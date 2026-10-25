<script setup lang="ts">
import type { PersonInsights } from "~/types";

// Question 5: what did one person deliver? Every member of the project sees
// the same numbers — this is a shared slice, not a private score.
const props = defineProps<{ person: PersonInsights }>();

const emit = defineEmits<{ open: [metric: string, userId: string] }>();

const narrative = computed(() => {
  const p = props.person;
  let s = `${p.delivered.count} items completed`;
  if (p.delivered.cx && p.delivered.cx.assessed > 0) {
    s += ` (complexity ${p.delivered.cx.sum}`;
    if (p.delivered.cx.not_assessed > 0) s += `, ${p.delivered.cx.not_assessed} not assessed`;
    s += ")";
  }
  if (p.top_rank_handled > 0) s += `; ${p.top_rank_handled} at the project's top priority ranks`;
  s += `. ${p.comments} comments written, ${p.created} tasks created.`;
  return s;
});
</script>

<template>
  <section>
    <h2 class="text-lg font-semibold">
      What did {{ person.user.first_name }} {{ person.user.last_name }} deliver?
    </h2>
    <p class="mt-1 text-sm text-muted-foreground">{{ narrative }}</p>
    <p class="mt-1 text-[11px] text-muted-foreground">
      Everyone in the project sees this slice; the numbers are identical for every viewer.
    </p>

    <div class="mt-4 flex flex-wrap gap-3">
      <MetricTile
        label="Completed"
        :count="person.delivered.count"
        metric="person_delivered"
        :cx="person.delivered.cx"
        highlight
        @open="emit('open', 'person_delivered', person.user.id)"
      />
      <MetricTile label="Top priority handled" :count="person.top_rank_handled" />
      <MetricTile label="Open assigned now" :count="person.open_now" />
      <MetricTile label="Comments" :count="person.comments" />
      <MetricTile label="Created" :count="person.created" />
    </div>

    <div class="mt-6 grid gap-6 lg:grid-cols-2">
      <div>
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Completions per week</h3>
        <WeeklyBars class="mt-2" :weeks="(person.weekly ?? []).map((w) => ({ ...w, created: 0 }))" />
      </div>
      <div v-if="(person.by_cycle ?? []).length > 0">
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Completions by cycle</h3>
        <div class="mt-2 space-y-1.5">
          <div v-for="bc in person.by_cycle ?? []" :key="bc.cycle.id" class="flex items-center gap-2 text-sm">
            <span class="w-40 shrink-0 truncate text-xs text-muted-foreground">{{ bc.cycle.title }}</span>
            <div class="h-3 rounded-sm bg-emerald-600/80 dark:bg-emerald-400/70" :style="{ width: Math.max(2, bc.count * 24) + 'px' }" />
            <span class="text-xs tabular-nums">{{ bc.count }}</span>
          </div>
        </div>
        <p class="mt-2 text-[11px] text-muted-foreground">A subtask counts in its parent's cycle.</p>
      </div>
    </div>

    <p class="mt-4 text-[11px] text-muted-foreground">{{ person.credit_rule }}</p>
  </section>
</template>
