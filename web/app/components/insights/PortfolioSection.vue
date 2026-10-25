<script setup lang="ts">
import type { InsightsPortfolio, PortfolioRow } from "~/types";

// Question 0: which projects need attention? The rule is shown, not hidden,
// so it can be challenged.
const props = defineProps<{ portfolio: InsightsPortfolio }>();

const emit = defineEmits<{ open: [projectKey: string] }>();

const narrative = computed(() => {
  const firing = props.portfolio.rows.filter((r) => r.attention);
  const total = props.portfolio.rows.length;
  if (firing.length === 0) {
    return `None of your ${total} projects meets the attention rule over this window.`;
  }
  const names = firing.map((r) => r.project_name).join(" and ");
  return `${firing.length} of ${total} projects carry open top-priority work and created more than they completed: ${names}.`;
});

function trendWord(r: PortfolioRow): string {
  if (r.trend === "no_history") return `${r.history_days} days of history · not enough to trend`;
  return r.trend;
}
</script>

<template>
  <section>
    <h2 class="text-lg font-semibold">0 · Which projects need attention?</h2>
    <p class="mt-1 text-sm text-muted-foreground">{{ narrative }}</p>

    <div class="mt-4 overflow-x-auto rounded-lg border">
      <table class="w-full min-w-[920px] text-sm">
        <thead>
          <tr class="border-b bg-muted/40 text-left text-[11px] uppercase tracking-wide text-muted-foreground">
            <th class="px-3 py-2 font-medium">Project</th>
            <th class="px-3 py-2 text-right font-medium">Open</th>
            <th class="px-3 py-2 text-right font-medium">Top priority open</th>
            <th class="px-3 py-2 text-right font-medium">Overdue</th>
            <th class="px-3 py-2 text-right font-medium">Blocked</th>
            <th class="px-3 py-2 text-right font-medium">Completed</th>
            <th class="px-3 py-2 font-medium">Intake vs completion</th>
            <th class="px-3 py-2 font-medium">Oldest unchanged</th>
            <th class="px-3 py-2 text-right font-medium">Active people</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="r in portfolio.rows"
            :key="r.project_key"
            class="cursor-pointer border-b last:border-0 transition-colors hover:bg-muted/40"
            :class="r.attention && 'bg-amber-500/10 hover:bg-amber-500/15'"
            :title="r.attention_why ? `Why: ${r.attention_why}` : undefined"
            @click="emit('open', r.project_key)"
          >
            <td class="px-3 py-2.5">
              <div class="flex items-center gap-2">
                <Avatar class="size-6 rounded-md">
                  <AvatarImage v-if="r.icon_url" :src="r.icon_url" />
                  <AvatarFallback class="rounded-md text-[10px]" :seed="r.project_key">
                    {{ r.project_key.slice(0, 2) }}
                  </AvatarFallback>
                </Avatar>
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="truncate font-medium">{{ r.project_name }}</span>
                    <Badge v-if="r.attention" variant="outline" class="border-amber-600/50 text-[10px] text-amber-700 dark:text-amber-400">
                      attention
                    </Badge>
                  </div>
                  <span class="text-[11px] text-muted-foreground">
                    {{ r.project_key }}
                    <template v-if="r.active_cycle"> · {{ r.active_cycle.title }}</template>
                    <template v-else-if="!r.has_cycles"> · no cycles</template>
                  </span>
                </div>
              </div>
            </td>
            <td class="px-3 py-2.5 text-right tabular-nums">{{ r.open }}</td>
            <td class="px-3 py-2.5 text-right tabular-nums">
              <span v-if="r.top_rank_open > 0" class="font-medium text-red-700 dark:text-red-400">
                {{ r.top_rank_open }} · {{ r.top_rank_name }}
              </span>
              <span v-else class="text-muted-foreground">0</span>
            </td>
            <td class="px-3 py-2.5 text-right tabular-nums">{{ r.overdue }}</td>
            <td class="px-3 py-2.5 text-right tabular-nums">{{ r.blocked }}</td>
            <td class="px-3 py-2.5 text-right tabular-nums">
              {{ r.completed }}<span v-if="r.cancelled" class="text-muted-foreground"> · {{ r.cancelled }} cancelled</span>
            </td>
            <td class="px-3 py-2.5">
              <div class="flex items-center gap-2">
                <WeeklyBars v-if="r.trend !== 'no_history'" :weeks="r.weekly" compact />
                <span class="text-[11px] text-muted-foreground">{{ trendWord(r) }}</span>
              </div>
            </td>
            <td class="px-3 py-2.5">
              <template v-if="r.oldest">
                <span class="tabular-nums">{{ r.oldest.days }} d</span>
                <span
                  class="ml-1.5 rounded-full border px-1.5 py-0.5 text-[10px]"
                  :style="{ borderColor: r.oldest.state.color, color: r.oldest.state.color }"
                >
                  {{ r.oldest.state.name }}
                </span>
              </template>
              <span v-else class="text-muted-foreground">—</span>
            </td>
            <td class="px-3 py-2.5 text-right tabular-nums">{{ r.active_people }} / {{ r.members }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="mt-2 text-[11px] text-muted-foreground">
      {{ portfolio.rule }} Blocked counts blocker links, not state names; blocked and overdue can overlap.
      Open and archived are snapshots; completed and created use the selected window.
    </p>

    <h2 class="mt-8 text-lg font-semibold">What did we deliver?</h2>
    <p class="mt-1 text-sm text-muted-foreground">
      <strong>{{ portfolio.delivered.total }} items completed</strong> across your projects in the window<template
        v-if="portfolio.delivered.cancelled > 0"
      >; {{ portfolio.delivered.cancelled }} were cancelled and are not counted as delivery</template>.
    </p>
    <div class="mt-3 space-y-1.5">
      <div v-for="bp in portfolio.delivered.by_project" :key="bp.project_key" class="flex items-center gap-2">
        <span class="w-20 shrink-0 text-xs text-muted-foreground">{{ bp.project_key }}</span>
        <div class="h-3 rounded-sm bg-emerald-600/80 dark:bg-emerald-400/70"
          :style="{ width: Math.max(2, (bp.count / Math.max(1, portfolio.delivered.total)) * 420) + 'px' }" />
        <span class="text-xs tabular-nums">{{ bp.count }}</span>
      </div>
    </div>
  </section>
</template>
