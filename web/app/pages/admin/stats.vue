<script setup lang="ts">
import { BarChart3, Loader2, Users, LayoutGrid, ListTodo, GitBranch, FileText, Boxes } from "lucide-vue-next";
import { toast } from "vue-sonner";
import { VisAxis, VisXYContainer, VisGroupedBar } from "@unovis/vue";
import {
  ChartContainer,
  ChartCrosshair,
  ChartTooltip,
  ChartTooltipContent,
  componentToString,
  type ChartConfig,
} from "~/components/ui/chart";
import type { AdminStats } from "~/composables/useAdmin";

definePageMeta({
  middleware: ["admin"],
});

useSeoMeta({ title: "Stats" });

const { getStats } = useAdmin();

const loading = ref(true);
const stats = ref<AdminStats | null>(null);
const days = ref(30);

const RANGE_OPTIONS = [14, 30, 90];

async function loadStats() {
  loading.value = true;
  const result = await getStats(days.value);
  loading.value = false;

  if (result.success && result.data) {
    stats.value = result.data;
  } else {
    toast.error(result.error || "Failed to load stats");
  }
}

// Reload the per-day series when the range changes.
watch(days, loadStats);

onMounted(loadStats);

const totalCards = computed(() => {
  const t = stats.value?.totals;
  if (!t) return [];
  return [
    { label: "Workspaces", value: t.workspaces, icon: Boxes, color: "text-violet-400" },
    { label: "Projects", value: t.projects, icon: LayoutGrid, color: "text-blue-400" },
    { label: "Tasks", value: t.tasks, icon: ListTodo, color: "text-emerald-400" },
    { label: "Subtasks", value: t.subtasks, icon: GitBranch, color: "text-teal-400" },
    { label: "Pages", value: t.pages, icon: FileText, color: "text-amber-400" },
    { label: "Users", value: t.users, icon: Users, color: "text-rose-400" },
  ];
});

// --- Charts ---------------------------------------------------------------

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// Parse "YYYY-MM-DD" without timezone drift and render a short label.
function shortDay(day: string | undefined): string {
  if (!day) return "";
  const [, m, d] = day.split("-").map(Number);
  return `${MONTHS[(m ?? 1) - 1]} ${d}`;
}

function titleCase(value: string): string {
  return value.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

// Compact axis numbers (1200 -> "1.2k") and integer-only y ticks so unovis
// doesn't render duplicate fractional labels for small ranges.
function compactNum(v: number): string {
  if (Math.abs(v) >= 1000) {
    const n = v / 1000;
    return `${Number.isInteger(n) ? n : n.toFixed(1)}k`;
  }
  return v.toString();
}

function yTick(v: number): string {
  return Number.isInteger(v) ? compactNum(v) : "";
}

interface SeriesPoint {
  day: string;
  count: number;
}

const seriesConfig: ChartConfig = {
  count: { label: "Created" },
};

const trendCharts = computed(() => {
  const s = stats.value?.series;
  return [
    { key: "tasks", title: "Tasks created", color: "#6EE7B7", data: s?.tasks ?? [] },
    { key: "subtasks", title: "Subtasks created", color: "#93C5FD", data: s?.subtasks ?? [] },
    { key: "pages", title: "Pages created", color: "#C4B5FD", data: s?.pages ?? [] },
  ];
});

interface BarPoint {
  label: string;
  count: number;
  color: string;
}

const barConfig: ChartConfig = {
  count: { label: "Tasks" },
};

// Light/pastel palette (Tailwind -300 shades) matching the app's soft chips.
const STATE_LIGHT: Record<string, string> = {
  backlog: "#D1D5DB", // gray-300
  unstarted: "#93C5FD", // blue-300
  started: "#6EE7B7", // emerald-300
  completed: "#86EFAC", // green-300
  cancelled: "#E5E7EB", // gray-200
};

const PRIORITY_LIGHT: Record<string, string> = {
  "No priority": "#D1D5DB", // gray-300
  "Low": "#93C5FD", // blue-300
  "Medium": "#FCD34D", // amber-300
  "High": "#FDBA74", // orange-300
  "Urgent": "#FCA5A5", // red-300
};

const stateBars = computed<BarPoint[]>(() =>
  (stats.value?.tasks_by_state ?? []).map((s) => ({
    label: titleCase(s.label),
    count: s.count,
    color: STATE_LIGHT[s.label] ?? "#D1D5DB",
  }))
);

const priorityBars = computed<BarPoint[]>(() =>
  (stats.value?.tasks_by_priority ?? []).map((p) => ({
    label: p.label,
    count: p.count,
    color: PRIORITY_LIGHT[p.label] ?? "#D1D5DB",
  }))
);

const maxWorkspaceProjects = computed(() =>
  Math.max(1, ...(stats.value?.projects_per_workspace ?? []).map((w) => w.project_count))
);

const maxProjectTasks = computed(() =>
  Math.max(1, ...(stats.value?.top_projects ?? []).map((p) => p.task_count))
);
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <Navbar />

    <main id="main-content" class="flex-1">
      <div class="mx-auto max-w-6xl px-6 py-8">
        <!-- Header -->
        <div class="mb-6 flex flex-wrap items-center justify-between gap-4">
          <div>
            <div class="mb-1 flex items-center gap-2.5">
              <div class="flex size-8 items-center justify-center rounded-lg bg-foreground">
                <BarChart3 class="size-4 text-background" />
              </div>
              <h1 class="text-2xl font-bold tracking-tight">Stats</h1>
            </div>
            <p class="text-sm text-muted-foreground">
              System-wide metrics and activity trends
            </p>
          </div>

          <div class="flex items-center gap-2">
            <Label for="range" class="text-sm text-muted-foreground">Range</Label>
            <NativeSelect
              id="range"
              :model-value="String(days)"
              class="w-36"
              @update:model-value="days = Number($event)"
            >
              <option v-for="opt in RANGE_OPTIONS" :key="opt" :value="String(opt)">
                Last {{ opt }} days
              </option>
            </NativeSelect>
          </div>
        </div>

        <!-- Loading -->
        <div v-if="loading && !stats" class="flex items-center justify-center py-24">
          <Loader2 class="size-6 animate-spin text-muted-foreground" />
        </div>

        <template v-else-if="stats">
          <!-- Totals -->
          <div class="grid gap-3 sm:grid-cols-3 lg:grid-cols-6">
            <Card v-for="card in totalCards" :key="card.label" class="gap-0 py-4">
              <CardHeader class="flex flex-row items-center justify-between space-y-0 px-4 pb-1.5">
                <CardDescription class="text-xs">{{ card.label }}</CardDescription>
                <component :is="card.icon" :class="['size-4', card.color]" />
              </CardHeader>
              <CardContent class="px-4">
                <div class="text-2xl font-bold tracking-tight">{{ card.value.toLocaleString() }}</div>
              </CardContent>
            </Card>
          </div>

          <!-- Trends -->
          <div class="mt-8">
            <div class="mb-3">
              <h2 class="text-lg font-semibold">Activity trends</h2>
              <p class="text-sm text-muted-foreground">
                Items created per day over the last {{ stats.series.days }} days
              </p>
            </div>

            <div class="grid gap-3 lg:grid-cols-3">
              <Card v-for="chart in trendCharts" :key="chart.key" class="gap-3 py-4">
                <CardHeader class="px-4 pb-1">
                  <CardTitle class="text-base">{{ chart.title }}</CardTitle>
                </CardHeader>
                <CardContent class="px-4">
                  <ChartContainer :config="seriesConfig" class="h-44 w-full">
                    <VisXYContainer :data="chart.data" :margin="{ top: 8, right: 8, bottom: 4, left: 4 }">
                      <VisGroupedBar
                        :x="(_d: SeriesPoint, i: number) => i"
                        :y="(d: SeriesPoint) => d.count"
                        :color="chart.color"
                        :rounded-corners="2"
                        :bar-padding="0.15"
                      />
                      <VisAxis
                        type="x"
                        :x="(_d: SeriesPoint, i: number) => i"
                        :tick-format="(i: number) => shortDay(chart.data[i]?.day)"
                        :num-ticks="5"
                        :grid-line="false"
                        :domain-line="false"
                        :tick-line="false"
                      />
                      <VisAxis
                        type="y"
                        :tick-format="yTick"
                        :num-ticks="4"
                        :grid-line="false"
                        :domain-line="false"
                        :tick-line="false"
                      />
                      <ChartTooltip />
                      <ChartCrosshair :template="componentToString(seriesConfig, ChartTooltipContent)" />
                    </VisXYContainer>
                  </ChartContainer>
                </CardContent>
              </Card>
            </div>
          </div>

          <!-- Breakdowns -->
          <div class="mt-8">
            <div class="mb-3">
              <h2 class="text-lg font-semibold">Task breakdowns</h2>
              <p class="text-sm text-muted-foreground">
                Distribution of top-level tasks by state and priority
              </p>
            </div>

            <div class="grid gap-3 lg:grid-cols-2">
              <Card class="gap-3 py-4">
                <CardHeader class="px-4 pb-1">
                  <CardTitle class="text-base">By state</CardTitle>
                </CardHeader>
                <CardContent class="px-4">
                  <ChartContainer v-if="stateBars.length" :config="barConfig" class="h-48 w-full">
                    <VisXYContainer :data="stateBars" :margin="{ top: 8, right: 8, bottom: 4, left: 4 }">
                      <VisGroupedBar
                        :x="(_d: BarPoint, i: number) => i"
                        :y="(d: BarPoint) => d.count"
                        :color="(d: BarPoint) => d.color"
                        :rounded-corners="6"
                        :bar-padding="0.35"
                      />
                      <VisAxis
                        type="x"
                        :x="(_d: BarPoint, i: number) => i"
                        :tick-format="(i: number) => stateBars[i]?.label ?? ''"
                        :grid-line="false"
                        :domain-line="false"
                        :tick-line="false"
                      />
                      <VisAxis
                        type="y"
                        :tick-format="yTick"
                        :num-ticks="4"
                        :grid-line="false"
                        :domain-line="false"
                        :tick-line="false"
                      />
                      <ChartTooltip />
                      <ChartCrosshair :template="componentToString(barConfig, ChartTooltipContent)" />
                    </VisXYContainer>
                  </ChartContainer>
                  <p v-else class="py-12 text-center text-sm text-muted-foreground">No tasks yet</p>
                </CardContent>
              </Card>

              <Card class="gap-3 py-4">
                <CardHeader class="px-4 pb-1">
                  <CardTitle class="text-base">By priority</CardTitle>
                </CardHeader>
                <CardContent class="px-4">
                  <ChartContainer v-if="priorityBars.length" :config="barConfig" class="h-48 w-full">
                    <VisXYContainer :data="priorityBars" :margin="{ top: 8, right: 8, bottom: 4, left: 4 }">
                      <VisGroupedBar
                        :x="(_d: BarPoint, i: number) => i"
                        :y="(d: BarPoint) => d.count"
                        :color="(d: BarPoint) => d.color"
                        :rounded-corners="6"
                        :bar-padding="0.35"
                      />
                      <VisAxis
                        type="x"
                        :x="(_d: BarPoint, i: number) => i"
                        :tick-format="(i: number) => priorityBars[i]?.label ?? ''"
                        :grid-line="false"
                        :domain-line="false"
                        :tick-line="false"
                      />
                      <VisAxis
                        type="y"
                        :tick-format="yTick"
                        :num-ticks="4"
                        :grid-line="false"
                        :domain-line="false"
                        :tick-line="false"
                      />
                      <ChartTooltip />
                      <ChartCrosshair :template="componentToString(barConfig, ChartTooltipContent)" />
                    </VisXYContainer>
                  </ChartContainer>
                  <p v-else class="py-12 text-center text-sm text-muted-foreground">No tasks yet</p>
                </CardContent>
              </Card>
            </div>
          </div>

          <!-- Projects -->
          <div class="mt-8">
            <div class="mb-3">
              <h2 class="text-lg font-semibold">Projects</h2>
              <p class="text-sm text-muted-foreground">
                Most active projects and distribution across workspaces
              </p>
            </div>

            <div class="grid gap-3 lg:grid-cols-2">
              <Card class="gap-3 py-4">
                <CardHeader class="px-4 pb-1">
                  <CardTitle class="text-base">Top projects by tasks</CardTitle>
                </CardHeader>
                <CardContent class="px-4">
                  <div v-if="stats.top_projects.length" class="space-y-3">
                    <div v-for="p in stats.top_projects" :key="p.project_key" class="space-y-1">
                      <div class="flex items-center justify-between text-sm">
                        <span class="truncate">
                          <span class="text-muted-foreground">{{ p.project_key }}</span>
                          <span class="ml-2 font-medium">{{ p.name }}</span>
                        </span>
                        <span class="ml-2 font-medium tabular-nums">{{ p.task_count }}</span>
                      </div>
                      <div class="h-1.5 w-full rounded-full bg-muted">
                        <div
                          class="h-1.5 rounded-full bg-blue-300"
                          :style="{ width: `${(p.task_count / maxProjectTasks) * 100}%` }"
                        />
                      </div>
                    </div>
                  </div>
                  <p v-else class="py-12 text-center text-sm text-muted-foreground">No projects yet</p>
                </CardContent>
              </Card>

              <Card class="gap-3 py-4">
                <CardHeader class="px-4 pb-1">
                  <CardTitle class="text-base">Projects per workspace</CardTitle>
                </CardHeader>
                <CardContent class="px-4">
                  <div v-if="stats.projects_per_workspace.length" class="space-y-3">
                    <div v-for="w in stats.projects_per_workspace" :key="w.workspace_key" class="space-y-1">
                      <div class="flex items-center justify-between text-sm">
                        <span class="truncate">
                          <span class="text-muted-foreground">{{ w.workspace_key }}</span>
                          <span class="ml-2 font-medium">{{ w.name }}</span>
                        </span>
                        <span class="ml-2 font-medium tabular-nums">{{ w.project_count }}</span>
                      </div>
                      <div class="h-1.5 w-full rounded-full bg-muted">
                        <div
                          class="h-1.5 rounded-full bg-blue-300"
                          :style="{ width: `${(w.project_count / maxWorkspaceProjects) * 100}%` }"
                        />
                      </div>
                    </div>
                  </div>
                  <p v-else class="py-12 text-center text-sm text-muted-foreground">No workspaces yet</p>
                </CardContent>
              </Card>
            </div>
          </div>
        </template>
      </div>
    </main>
  </div>
</template>
