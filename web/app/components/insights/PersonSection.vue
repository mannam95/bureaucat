<script setup lang="ts">
import type { InsightsEvidence, PersonInsights } from "~/types";
import type { InsightsWindow } from "~/composables/useInsights";

// Question 4: what did one person deliver? The page shows the work itself, not
// only the counts: the delivered items and the person's current plate render
// inline from the same evidence populations the drawer uses, so the lists and
// the tiles can never disagree. Every member of the project sees the same
// numbers — this is a shared slice, not a private score.
const props = defineProps<{
  person: PersonInsights;
  projectKey: string;
  win: InsightsWindow;
}>();

const emit = defineEmits<{ open: [metric: string, userId: string] }>();

const { fetchEvidence } = useInsights();
const delivered = ref<InsightsEvidence | null>(null);
const openNow = ref<InsightsEvidence | null>(null);

const LIST_LIMIT = 8;

watch(
  () => [props.person.user.id, props.projectKey, props.win.cycleId, props.win.from, props.win.to],
  async () => {
    const uid = props.person.user.id;
    const [del, opn] = await Promise.all([
      fetchEvidence(props.projectKey, props.win, { metric: "person_delivered", user_id: uid }),
      fetchEvidence(props.projectKey, props.win, { metric: "person_open", user_id: uid }),
    ]);
    delivered.value = del.success ? del.data! : null;
    openNow.value = opn.success ? opn.data! : null;
  },
  { immediate: true }
);

function taskHref(key: string): string {
  return `/projects/${props.projectKey}/tasks/${key.split("-").pop()}`;
}

function fmtDate(iso?: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", { day: "numeric", month: "short" });
}

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

const bandLabel: Record<string, string> = {
  "not_assessed": "Not assessed",
};
const maxBand = computed(() => Math.max(1, ...props.person.bands.map((b) => b.count)));
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
      <MetricTile
        label="Open assigned now"
        :count="person.open_now"
        metric="person_open"
        @open="emit('open', 'person_open', person.user.id)"
      />
      <MetricTile label="Comments" :count="person.comments" />
      <MetricTile label="Created" :count="person.created" />
    </div>

    <div class="mt-6 grid gap-6 lg:grid-cols-2">
      <!-- The delivered items themselves: the work, not an abstraction of it. -->
      <div class="rounded-lg border p-4">
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          Delivered in this window<template v-if="delivered"> · {{ delivered.count }} items</template>
        </h3>
        <div class="mt-2 space-y-1.5">
          <div v-for="row in (delivered?.rows ?? []).slice(0, LIST_LIMIT)" :key="row.key" class="flex items-center gap-2 text-sm">
            <a :href="taskHref(row.key)" target="_blank" rel="noopener" class="shrink-0 font-mono text-xs text-muted-foreground underline-offset-2 hover:underline" title="Opens in a new tab">{{ row.key }}</a>
            <span class="min-w-0 flex-1 truncate">{{ row.title }}</span>
            <span class="shrink-0 text-[11px] text-muted-foreground">{{ row.assessed ? `cx ${row.complexity}` : "not assessed" }}</span>
            <span v-if="row.completed_at" class="shrink-0 text-[11px] tabular-nums text-muted-foreground">{{ fmtDate(row.completed_at) }}</span>
          </div>
          <p v-if="delivered && delivered.rows.length === 0" class="text-xs text-muted-foreground">
            Nothing delivered in this window.
          </p>
        </div>
        <button
          v-if="delivered && delivered.rows.length > LIST_LIMIT"
          class="mt-2 text-[11px] text-muted-foreground underline-offset-2 hover:underline"
          @click="emit('open', 'person_delivered', person.user.id)"
        >
          all {{ delivered.count }} with evidence
        </button>
      </div>

      <!-- Current load: a snapshot, independent of the window. -->
      <div class="rounded-lg border p-4">
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          On their plate right now<template v-if="openNow"> · {{ openNow.count }} items</template>
        </h3>
        <div class="mt-2 space-y-1.5">
          <div v-for="row in (openNow?.rows ?? []).slice(0, LIST_LIMIT)" :key="row.key" class="flex items-center gap-2 text-sm">
            <a :href="taskHref(row.key)" target="_blank" rel="noopener" class="shrink-0 font-mono text-xs text-muted-foreground underline-offset-2 hover:underline" title="Opens in a new tab">{{ row.key }}</a>
            <span class="min-w-0 flex-1 truncate">{{ row.title }}</span>
            <span class="shrink-0 rounded-full border px-1.5 py-0.5 text-[10px]" :style="{ borderColor: row.state.color, color: row.state.color }">
              {{ row.state.name }}
            </span>
            <span v-if="row.age_days" class="shrink-0 text-[11px] tabular-nums text-muted-foreground">{{ row.age_days }} d</span>
          </div>
          <p v-if="openNow && openNow.rows.length === 0" class="text-xs text-muted-foreground">
            Nothing assigned and open right now.
          </p>
        </div>
        <button
          v-if="openNow && openNow.rows.length > LIST_LIMIT"
          class="mt-2 text-[11px] text-muted-foreground underline-offset-2 hover:underline"
          @click="emit('open', 'person_open', person.user.id)"
        >
          all {{ openNow.count }} with evidence
        </button>
        <p class="mt-2 text-[11px] text-muted-foreground">A snapshot at generation time, independent of the window.</p>
      </div>

      <!-- Rhythm: when the completions happened. -->
      <div class="rounded-lg border p-4">
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Completions per week</h3>
        <WeeklyBars class="mt-2" :weeks="(person.weekly ?? []).map((w) => ({ ...w, created: 0 }))" />
        <template v-if="(person.by_cycle ?? []).length > 0">
          <h3 class="mt-4 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Completions by cycle</h3>
          <div class="mt-2 space-y-1.5">
            <div v-for="bc in person.by_cycle ?? []" :key="bc.cycle.id" class="flex items-center gap-2 text-sm">
              <span class="w-40 shrink-0 truncate text-xs text-muted-foreground">{{ bc.cycle.title }}</span>
              <div class="h-3 rounded-sm bg-emerald-600/80 dark:bg-emerald-400/70" :style="{ width: Math.max(2, bc.count * 24) + 'px' }" />
              <span class="text-xs tabular-nums">{{ bc.count }}</span>
            </div>
          </div>
          <p class="mt-2 text-[11px] text-muted-foreground">A subtask counts in its parent's cycle.</p>
        </template>
      </div>

      <!-- Weight and company: how heavy the items were, who shared the credit. -->
      <div class="rounded-lg border p-4">
        <h3 class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Delivered by complexity band</h3>
        <div class="mt-2 space-y-1.5">
          <div v-for="b in person.bands ?? []" :key="b.band" class="flex items-center gap-2 text-sm">
            <span class="w-24 shrink-0 text-xs text-muted-foreground">{{ bandLabel[b.band] ?? b.band }}</span>
            <div class="h-3 rounded-sm bg-emerald-600/60 dark:bg-emerald-400/50" :style="{ width: Math.max(2, (b.count / maxBand) * 140) + 'px' }" />
            <span class="text-xs tabular-nums">{{ b.count }}</span>
          </div>
        </div>
        <template v-if="(person.worked_with ?? []).length > 0">
          <h3 class="mt-4 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Worked with</h3>
          <div class="mt-2 flex flex-wrap gap-1.5">
            <span v-for="ww in person.worked_with ?? []" :key="ww.user.id" class="rounded-full border bg-muted/40 px-2 py-0.5 text-[11px] text-muted-foreground">
              {{ ww.user.first_name }} {{ ww.user.last_name }} · {{ ww.count }} shared
            </span>
          </div>
        </template>
      </div>
    </div>

    <p class="mt-4 text-[11px] text-muted-foreground">{{ person.credit_rule }}</p>
  </section>
</template>
