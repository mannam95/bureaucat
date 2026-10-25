<script setup lang="ts">
import type { PersonTable } from "~/types";

// Question 2: what did each person deliver? Credit goes to Contributors,
// falling back to assignees at completion; shared items count once per person
// and the total de-duplicates. The sentences state facts the event log can
// prove — never interpretations.
const props = defineProps<{ people: PersonTable }>();

const emit = defineEmits<{
  person: [userId: string];
  open: [metric: string, userId: string];
}>();

const activeRows = computed(() => props.people.rows.filter((r) => r.has_activity || r.delivered > 0));
const quietRows = computed(() => props.people.rows.filter((r) => !r.has_activity && r.delivered === 0));

const creditSum = computed(() => props.people.rows.reduce((n, r) => n + r.delivered, 0));

const narrative = computed(() => {
  const n = activeRows.value.filter((r) => r.delivered > 0).length;
  let s = `${n} of ${props.people.rows.length} members delivered completed work in this window.`;
  if (creditSum.value > props.people.total_delivered) {
    s += ` Person rows sum to ${creditSum.value} because shared items count once per credited person; the total counts ${props.people.total_delivered} distinct items.`;
  }
  return s;
});
</script>

<template>
  <section>
    <h2 class="text-lg font-semibold">2 · What did each person deliver?</h2>
    <p class="mt-1 text-sm text-muted-foreground">{{ narrative }}</p>

    <div class="mt-4 overflow-x-auto rounded-lg border">
      <table class="w-full min-w-[680px] text-sm">
        <thead>
          <tr class="border-b bg-muted/40 text-left text-[11px] uppercase tracking-wide text-muted-foreground">
            <th class="px-3 py-2 font-medium">Member</th>
            <th class="px-3 py-2 text-right font-medium">Delivered</th>
            <th class="px-3 py-2 text-right font-medium">Complexity</th>
            <th class="px-3 py-2 text-right font-medium">Top priority handled</th>
            <th class="px-3 py-2 text-right font-medium">Open at end</th>
            <th class="px-3 py-2 text-right font-medium">Comments</th>
            <th class="px-3 py-2 text-right font-medium">Created</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="r in activeRows"
            :key="r.user.id"
            class="cursor-pointer border-b transition-colors last:border-0 hover:bg-muted/40"
            title="Open this person's view"
            @click="emit('person', r.user.id)"
          >
            <td class="px-3 py-2.5">
              <div class="flex items-center gap-2">
                <Avatar class="size-6">
                  <AvatarImage v-if="r.user.avatar_url" :src="r.user.avatar_url" />
                  <AvatarFallback class="text-[10px]" :seed="r.user.id">
                    {{ r.user.first_name[0] }}{{ r.user.last_name[0] }}
                  </AvatarFallback>
                </Avatar>
                <span class="font-medium">{{ r.user.first_name }} {{ r.user.last_name }}</span>
              </div>
            </td>
            <td class="px-3 py-2.5 text-right tabular-nums">
              <button
                class="rounded px-1 underline-offset-2 hover:underline focus-visible:ring-2 focus-visible:ring-ring outline-none"
                title="Open the exact items behind this number"
                @click.stop="emit('open', 'person_delivered', r.user.id)"
              >
                {{ r.delivered }}<span v-if="r.shared > 0" class="text-muted-foreground"> · {{ r.shared }} shared</span>
              </button>
            </td>
            <td class="px-3 py-2.5 text-right tabular-nums">
              <template v-if="r.cx && r.cx.assessed > 0">
                {{ r.cx.sum }}<span v-if="r.cx.not_assessed > 0" class="text-muted-foreground"> · {{ r.cx.not_assessed }} n/a</span>
              </template>
              <span v-else class="text-muted-foreground">—</span>
            </td>
            <td class="px-3 py-2.5 text-right tabular-nums">{{ r.top_rank_handled }}</td>
            <td class="px-3 py-2.5 text-right tabular-nums">{{ r.open_at_end }}</td>
            <td class="px-3 py-2.5 text-right tabular-nums">{{ r.comments }}</td>
            <td class="px-3 py-2.5 text-right tabular-nums">{{ r.created }}</td>
          </tr>
          <tr v-if="quietRows.length > 0" class="border-b last:border-0">
            <td colspan="7" class="px-3 py-2.5 text-xs text-muted-foreground">
              No recorded activity in this project during the window:
              <span v-for="(r, i) in quietRows" :key="r.user.id">{{ i > 0 ? ", " : "" }}{{ r.user.first_name }} {{ r.user.last_name }}</span>
            </td>
          </tr>
          <tr class="bg-muted/30 font-medium">
            <td class="px-3 py-2.5">Total · shared counted once</td>
            <td class="px-3 py-2.5 text-right tabular-nums">{{ people.total_delivered }}</td>
            <td class="px-3 py-2.5 text-right tabular-nums">
              <template v-if="people.total_cx && people.total_cx.assessed > 0">{{ people.total_cx.sum }}</template>
              <span v-else>—</span>
            </td>
            <td colspan="4" />
          </tr>
        </tbody>
      </table>
    </div>
    <p class="mt-2 text-[11px] text-muted-foreground">{{ people.credit_rule }}</p>
  </section>
</template>
