<script setup lang="ts">
import { toast } from "vue-sonner";
import { Copy, Download, List, X } from "lucide-vue-next";
import type { InsightsEvidence } from "~/types";

// The evidence drawer: one reusable surface behind every number. It always
// carries the scope, the plain-language computation, the population as chips,
// the exact rows, and actions that never lie — "open in task list" appears
// only when the task list can express the same population.
const props = defineProps<{
  evidence: InsightsEvidence | null;
  loading: boolean;
  projectKey: string;
  shareUrl: string;
}>();

const emit = defineEmits<{
  close: [];
  csv: [];
}>();

const open = computed(() => props.loading || props.evidence !== null);

function onOpenChange(v: boolean) {
  if (!v) emit("close");
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(props.shareUrl);
    toast.success("Link copied — it reproduces this exact view");
  } catch {
    toast.error("Could not copy the link");
  }
}

function fmtDate(iso?: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", { day: "numeric", month: "short" });
}

const scopeLine = computed(() => {
  const s = props.evidence?.scope;
  if (!s) return "";
  const parts = [s.project_name || s.project_key];
  if (s.cycle_title) parts.push(s.cycle_title);
  parts.push(`${fmtDate(s.from)} – ${fmtDate(s.to)}`);
  parts.push(s.timezone);
  return parts.filter(Boolean).join(" · ");
});

const cxLine = computed(() => {
  const cx = props.evidence?.cx;
  if (!cx || cx.of === 0 || cx.assessed === 0) return "";
  let line = `complexity ${cx.sum}`;
  if (cx.not_assessed > 0) line += ` (${cx.not_assessed} not assessed)`;
  return line;
});
</script>

<template>
  <Sheet :open="open" @update:open="onOpenChange">
    <SheetContent side="right" class="flex w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-xl">
      <template v-if="loading && !evidence">
        <div class="flex flex-1 items-center justify-center text-sm text-muted-foreground">Computing the population…</div>
      </template>
      <template v-else-if="evidence">
        <SheetHeader class="border-b px-5 py-4">
          <div class="flex items-start justify-between gap-3">
            <div>
              <p class="text-[11px] font-medium uppercase tracking-wide text-amber-700 dark:text-amber-400">Evidence</p>
              <SheetTitle class="mt-0.5 text-base">
                {{ evidence.title }} · {{ evidence.count }} items
                <span v-if="cxLine" class="font-normal text-muted-foreground"> · {{ cxLine }}</span>
              </SheetTitle>
              <SheetDescription class="mt-0.5 text-xs">{{ scopeLine }}</SheetDescription>
            </div>
            <SheetClose class="rounded-md p-1 opacity-70 outline-none hover:opacity-100 focus-visible:ring-2 focus-visible:ring-ring">
              <X class="size-4" />
            </SheetClose>
          </div>
        </SheetHeader>

        <div class="flex-1 overflow-y-auto px-5 py-4">
          <div class="rounded-md border bg-muted/30 p-3">
            <p class="text-xs font-medium">How this number is computed</p>
            <p class="mt-1 text-xs text-muted-foreground">{{ evidence.how_computed }}</p>
            <p class="mt-1 text-[11px] text-muted-foreground">Source: {{ evidence.source_events }}.</p>
          </div>
          <div class="mt-3 flex flex-wrap gap-1.5">
            <span v-for="chip in evidence.chips" :key="chip" class="rounded-full border bg-muted/40 px-2 py-0.5 text-[11px] text-muted-foreground">
              {{ chip }}
            </span>
          </div>

          <div class="mt-4 space-y-2">
            <div v-for="row in evidence.rows" :key="row.key" class="rounded-md border p-3">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="text-sm">
                    <NuxtLink
                      :to="`/projects/${projectKey}/tasks/${row.key.split('-').pop()}`"
                      class="font-mono text-xs text-muted-foreground underline-offset-2 hover:underline"
                    >{{ row.key }}</NuxtLink>
                    <span class="ml-2 font-medium">{{ row.title }}</span>
                  </p>
                  <p v-if="row.parent_key" class="mt-0.5 truncate text-[11px] text-muted-foreground">
                    ↳ {{ row.parent_key }} {{ row.parent_title }}
                  </p>
                </div>
                <span class="shrink-0 rounded-full border px-1.5 py-0.5 text-[10px]" :style="{ borderColor: row.state.color, color: row.state.color }">
                  {{ row.state.name }}
                </span>
              </div>
              <div class="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
                <span v-if="row.priority">{{ row.priority.name }}</span>
                <span>{{ row.assessed ? `cx ${row.complexity}` : "not assessed" }}</span>
                <span v-if="row.completed_at">done {{ fmtDate(row.completed_at) }}</span>
                <span v-if="row.marked_complete_by">by {{ row.marked_complete_by.username }}</span>
                <span v-if="row.credits && row.credits.length">
                  credit: {{ row.credits.map((u) => u.username).join(", ") }}
                </span>
                <span v-if="row.entered_cycle" :class="row.entered_cycle !== 'planned' && 'rounded bg-amber-500/15 px-1 text-amber-700 dark:text-amber-400'">
                  {{ row.entered_cycle }}
                </span>
                <span v-if="row.age_days" class="tabular-nums">{{ row.age_days }} d unchanged</span>
                <span v-if="row.due">due {{ fmtDate(row.due) }}</span>
              </div>
            </div>
            <p v-if="evidence.rows.length === 0" class="py-6 text-center text-sm text-muted-foreground">
              No items in this population.
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2 border-t px-5 py-3">
          <Button variant="outline" size="sm" class="gap-1.5" @click="copyLink">
            <Copy class="size-3.5" /> Copy link
          </Button>
          <Button variant="outline" size="sm" class="gap-1.5" @click="emit('csv')">
            <Download class="size-3.5" /> Export CSV
          </Button>
          <NuxtLink v-if="evidence.task_list_query" :to="`/projects/${projectKey}?${evidence.task_list_query}`">
            <Button variant="outline" size="sm" class="gap-1.5"><List class="size-3.5" /> Open in task list</Button>
          </NuxtLink>
          <span v-else class="ml-auto text-[10px] text-muted-foreground">
            Activity-derived population: link and CSV carry it exactly.
          </span>
        </div>
      </template>
    </SheetContent>
  </Sheet>
</template>
