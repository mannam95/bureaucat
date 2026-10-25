<script setup lang="ts">
import { ChevronRight, ArrowRight } from "lucide-vue-next";
import { getAvatarColor } from "~/utils/avatarColor";
import type { Task, Cycle, ProjectState } from "~/types";

// "Assigned to You", grouped by project (Brief C): collapsible headers carry
// the project identity once — icon or initial chip, full name, key, assigned
// count, the active cycle with its day count — so the rows stay dense. A child
// whose parent is also assigned nests under it; an orphan subtask row carries
// a one-line parent crumb instead.
const props = defineProps<{
  tasks: Task[];
  // task.id -> parent task id / parent crumb ("WEB-39 · Payment flow").
  parentIdByTask: Record<string, string>;
  parentCrumbByTask: Record<string, string>;
  projectMeta: Record<string, { name: string; iconUrl?: string | null }>;
  activeCycleByKey: Record<string, Cycle>;
  statesByProject: Record<string, ProjectState[]>;
  isMemberByProject: Record<string, boolean>;
  // With the all-workspaces scope on, the header names the workspace too.
  showWorkspace: boolean;
  workspaceByProject: Record<string, string>;
}>();

const emit = defineEmits<{ updated: [] }>();

interface GroupRow {
  task: Task;
  indented: boolean;
  crumb?: string;
}

interface Group {
  key: string;
  name: string;
  iconUrl?: string | null;
  rows: GroupRow[];
  count: number;
}

const groups = computed<Group[]>(() => {
  const byKey = new Map<string, Task[]>();
  for (const t of props.tasks) {
    const list = byKey.get(t.project_key) ?? [];
    list.push(t);
    byKey.set(t.project_key, list);
  }
  const out: Group[] = [];
  for (const [key, list] of byKey) {
    const ids = new Set(list.map((t) => t.id));
    const childrenOf = new Map<string, Task[]>();
    const top: Task[] = [];
    for (const t of list) {
      const pid = props.parentIdByTask[t.id];
      if (pid && ids.has(pid)) {
        const kids = childrenOf.get(pid) ?? [];
        kids.push(t);
        childrenOf.set(pid, kids);
      } else {
        top.push(t);
      }
    }
    const rows: GroupRow[] = [];
    for (const t of top) {
      // A subtask whose parent is not in the list stays a top-level row with
      // its parent named in a crumb.
      rows.push({
        task: t,
        indented: false,
        crumb: props.parentIdByTask[t.id] ? props.parentCrumbByTask[t.id] : undefined,
      });
      for (const child of childrenOf.get(t.id) ?? []) {
        rows.push({ task: child, indented: true });
      }
    }
    const meta = props.projectMeta[key];
    out.push({ key, name: meta?.name ?? key, iconUrl: meta?.iconUrl, rows, count: list.length });
  }
  out.sort((a, b) => a.name.localeCompare(b.name));
  return out;
});

const collapsed = ref(new Set<string>());

function toggle(key: string) {
  const next = new Set(collapsed.value);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  collapsed.value = next;
}

// "Sprint 2 · day 10 of 14" — dates are date-only, both ends inclusive.
function cycleLabel(c: Cycle): string {
  const start = new Date(c.start_date + "T00:00:00").getTime();
  const end = new Date(c.end_date + "T00:00:00").getTime();
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const total = Math.round((end - start) / 86400000) + 1;
  const day = Math.min(Math.max(1, Math.round((today - start) / 86400000) + 1), total);
  return `${c.title} · day ${day} of ${total}`;
}
</script>

<template>
  <div class="space-y-4">
    <section v-for="g in groups" :key="g.key" class="overflow-hidden rounded-lg border border-border/50">
      <div class="flex items-center gap-2 bg-muted/40 px-3 py-2">
        <button
          type="button"
          class="flex min-w-0 flex-1 items-center gap-2.5 rounded text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
          :aria-expanded="!collapsed.has(g.key)"
          @click="toggle(g.key)"
        >
          <ChevronRight
            class="size-4 shrink-0 text-muted-foreground transition-transform"
            :class="{ 'rotate-90': !collapsed.has(g.key) }"
          />
          <span
            class="flex size-5 shrink-0 items-center justify-center overflow-hidden rounded text-[10px] font-semibold"
            :class="!g.iconUrl && getAvatarColor(g.key)"
          >
            <img v-if="g.iconUrl" :src="g.iconUrl" :alt="g.name" class="size-full object-cover" />
            <template v-else>{{ g.name[0] }}</template>
          </span>
          <span class="truncate text-sm font-semibold">{{ g.name }}</span>
          <span class="shrink-0 font-mono text-xs text-muted-foreground">{{ g.key }}</span>
          <span class="shrink-0 text-xs text-muted-foreground">· {{ g.count }} assigned</span>
          <span
            v-if="showWorkspace && workspaceByProject[g.key]"
            class="hidden shrink-0 rounded-md border bg-muted/50 px-1.5 py-0.5 text-[11px] text-muted-foreground sm:inline"
          >
            {{ workspaceByProject[g.key] }}
          </span>
          <span
            v-if="activeCycleByKey[g.key]"
            class="hidden shrink-0 rounded-full border border-emerald-600/40 bg-emerald-500/10 px-2 py-0.5 text-[11px] text-emerald-700 sm:inline dark:text-emerald-400"
          >
            {{ cycleLabel(activeCycleByKey[g.key]!) }}
          </span>
        </button>
        <NuxtLink
          :to="`/projects/${g.key}`"
          class="flex shrink-0 items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
        >
          Open project
          <ArrowRight class="size-3" />
        </NuxtLink>
      </div>

      <div v-show="!collapsed.has(g.key)" class="divide-y divide-border/50 border-t border-border/50">
        <TaskCard
          v-for="row in g.rows"
          :key="row.task.id"
          :task="row.task"
          :project-key="g.key"
          :states="statesByProject[g.key] ?? []"
          :is-member="isMemberByProject[g.key] ?? false"
          :indented="row.indented"
          :parent-crumb="row.crumb"
          @updated="emit('updated')"
        />
      </div>
    </section>
  </div>
</template>
