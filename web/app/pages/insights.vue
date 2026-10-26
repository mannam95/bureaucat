<script setup lang="ts">
import { toast } from "vue-sonner";
import { Copy, RotateCcw, ArrowLeft } from "lucide-vue-next";
import type {
  EvidenceQuery,
  InsightsEvidence,
  InsightsPortfolio,
  PersonInsights,
  ProjectInsights,
  Project,
  WorkspacePersonInsights,
} from "~/types";

// The insights report: a narrative answer with a question rail, where every
// number opens its exact evidence. The whole view state lives in the URL, so
// a copied link reproduces it for anyone with access.
definePageMeta({ middleware: ["auth", "insights"] });
useSeoMeta({ title: "Insights" });

const route = useRoute();
const router = useRouter();
const { fetchPortfolio, fetchProject, fetchPerson, fetchWorkspacePerson, fetchEvidence, downloadEvidenceCSV } =
  useInsights();
const { listProjects } = useProjects();
const { currentWorkspace } = useWorkspaces();

const projects = ref<Project[]>([]);
const portfolio = ref<InsightsPortfolio | null>(null);
const report = ref<ProjectInsights | null>(null);
const person = ref<PersonInsights | null>(null);
const wsPerson = ref<WorkspacePersonInsights | null>(null);
const evidence = ref<InsightsEvidence | null>(null);
const loading = ref(false);
const evidenceLoading = ref(false);
const error = ref("");

const q = computed(() => ({
  project: (route.query.project as string) || "",
  cycleId: (route.query.cycle_id as string) || "",
  from: (route.query.from as string) || "",
  to: (route.query.to as string) || "",
  person: (route.query.person as string) || "",
  open: (route.query.open as string) || "",
}));

const win = computed(() => ({
  cycleId: q.value.cycleId || undefined,
  from: q.value.from || undefined,
  to: q.value.to || undefined,
}));

// RANGES are quick picks; a cycle selection overrides them.
const RANGES = [
  { label: "Last 30 days", days: 30 },
  { label: "Last 60 days", days: 60 },
  { label: "Last 90 days", days: 90 },
];

function setQuery(patch: Record<string, string | undefined>) {
  const next: Record<string, string> = {};
  for (const [k, v] of Object.entries({ ...route.query, ...patch })) {
    if (v !== undefined && v !== "") next[k] = String(v);
  }
  router.replace({ query: next });
}

function pickProject(key: string) {
  setQuery({
    project: key === "__all__" ? undefined : key,
    cycle_id: undefined,
    person: undefined,
    open: undefined,
    user_id: undefined,
  });
}

function pickCycle(id: string) {
  setQuery({ cycle_id: id === "__range__" ? undefined : id, open: undefined });
}

const hasFilters = computed(() => Object.keys(route.query).length > 0);

function resetView() {
  router.replace({ query: {} });
}

// Custom range inputs; applied once both ends are set.
const customFrom = ref("");
const customTo = ref("");
watch(
  () => [q.value.from, q.value.to],
  () => {
    customFrom.value = q.value.from;
    customTo.value = q.value.to;
  },
  { immediate: true }
);
function applyCustomRange() {
  if (!customFrom.value || !customTo.value) return;
  if (customTo.value < customFrom.value) return;
  setQuery({ from: customFrom.value, to: customTo.value, cycle_id: undefined, open: undefined });
}

function pickRange(days: number) {
  const to = new Date();
  const from = new Date();
  from.setDate(from.getDate() - (days - 1));
  const fmt = (d: Date) => d.toISOString().slice(0, 10);
  setQuery({ from: fmt(from), to: fmt(to), cycle_id: undefined, open: undefined });
}

function openPerson(userId: string) {
  setQuery({ person: userId, open: undefined });
}

function openEvidence(metric: string, params: { user_id?: string; state_id?: string; rank?: number; day?: string } = {}) {
  setQuery({
    open: metric,
    user_id: params.user_id,
    state_id: params.state_id,
    rank: params.rank !== undefined ? String(params.rank) : undefined,
    day: params.day,
  });
}

function closeEvidence() {
  setQuery({ open: undefined, user_id: undefined, state_id: undefined, rank: undefined, day: undefined });
}

const evidenceQuery = computed<EvidenceQuery | null>(() => {
  if (!q.value.open) return null;
  return {
    metric: q.value.open,
    user_id: (route.query.user_id as string) || undefined,
    state_id: (route.query.state_id as string) || undefined,
    rank: route.query.rank !== undefined ? Number(route.query.rank) : undefined,
    day: (route.query.day as string) || undefined,
  };
});

async function loadMain() {
  loading.value = true;
  error.value = "";
  try {
    if (!q.value.project) {
      person.value = null;
      report.value = null;
      // A person without a project is the cross-project person view.
      if (q.value.person) {
        const res = await fetchWorkspacePerson(q.value.person, win.value, currentWorkspace.value?.id);
        if (!res.success) error.value = res.error || "Failed to load";
        else wsPerson.value = res.data!;
        return;
      }
      wsPerson.value = null;
      const res = await fetchPortfolio(win.value, currentWorkspace.value?.id);
      if (!res.success) error.value = res.error || "Failed to load";
      else portfolio.value = res.data!;
      return;
    }
    wsPerson.value = null;
    const repRes = await fetchProject(q.value.project, win.value);
    if (!repRes.success) {
      error.value = repRes.error || "Failed to load";
      return;
    }
    report.value = repRes.data!;
    if (q.value.person) {
      const pr = await fetchPerson(q.value.project, q.value.person, win.value);
      person.value = pr.success ? pr.data! : null;
    } else {
      person.value = null;
    }
  } finally {
    loading.value = false;
  }
}

async function loadEvidence() {
  const eq = evidenceQuery.value;
  if (!eq || !q.value.project) {
    evidence.value = null;
    return;
  }
  evidenceLoading.value = true;
  const res = await fetchEvidence(q.value.project, win.value, eq);
  evidenceLoading.value = false;
  if (res.success) evidence.value = res.data!;
  else {
    toast.error(res.error || "Failed to load evidence");
    closeEvidence();
  }
}

watch(() => [q.value.project, q.value.cycleId, q.value.from, q.value.to, q.value.person], loadMain, { immediate: true });
watch(evidenceQuery, loadEvidence, { immediate: true });

// Insights follows the active workspace, like every other list in the app;
// within it, scope stays the viewer's memberships.
async function loadProjectList() {
  const res = await listProjects(1, 100, "");
  if (res.success && res.data) projects.value = res.data.projects;
}
onMounted(loadProjectList);
watch(currentWorkspace, () => {
  loadProjectList();
  if (!q.value.project) loadMain();
});

async function copyViewLink() {
  try {
    await navigator.clipboard.writeText(window.location.href);
    toast.success("View link copied");
  } catch {
    toast.error("Could not copy the link");
  }
}

async function exportCSV() {
  const eq = evidenceQuery.value;
  if (!eq || !q.value.project) return;
  const ok = await downloadEvidenceCSV(q.value.project, win.value, eq);
  if (!ok) toast.error("CSV export failed");
  else toast.success("CSV exported — exactly the drawer's rows");
}

const scope = computed(
  () => person.value?.scope ?? report.value?.scope ?? wsPerson.value?.scope ?? portfolio.value?.scope ?? null
);

function fmtDate(iso?: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", { day: "numeric", month: "short", year: "numeric" });
}

const questions = computed(() => {
  if (!q.value.project) {
    const list = [{ id: "q0", label: "0 · Which projects need attention?" }];
    if (q.value.person && wsPerson.value) {
      list.push({ id: "qw", label: `What did ${wsPerson.value.user.first_name} deliver across projects?` });
    }
    return list;
  }
  const list = [
    { id: "q1", label: "1 · How did it land?" },
    { id: "q2", label: "2 · What did each person deliver?" },
    { id: "q3", label: "3 · What's open right now?" },
  ];
  if (q.value.person && person.value) {
    list.push({ id: "q4", label: `4 · What did ${person.value.user.first_name} deliver?` });
  }
  return list;
});

function jumpTo(id: string) {
  // The project questions are not on screen while a person view is open:
  // leave it first, then scroll once the sections exist.
  if (q.value.person && id !== "q4" && id !== "qw") {
    setQuery({ person: undefined });
    nextTick(() => {
      setTimeout(() => document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" }), 150);
    });
    return;
  }
  document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
}

// Scroll spy: the rail shows which question is on screen. The portfolio and
// person views each render a single section, so only the project report needs
// the scroll position; "in view" means the last section whose top has passed
// the sticky filter bar.
const activeQ = ref("");
function updateActiveQ() {
  if (!q.value.project) {
    activeQ.value = q.value.person ? "qw" : "q0";
    return;
  }
  if (q.value.person) {
    activeQ.value = "q4";
    return;
  }
  let current = "q1";
  for (const id of ["q1", "q2", "q3"]) {
    const el = document.getElementById(id);
    if (el && el.getBoundingClientRect().top <= 120) current = id;
  }
  activeQ.value = current;
}
onMounted(() => window.addEventListener("scroll", updateActiveQ, { passive: true }));
onBeforeUnmount(() => window.removeEventListener("scroll", updateActiveQ));
watch([() => q.value.project, () => q.value.person, report, portfolio, person, wsPerson], () => nextTick(updateActiveQ), {
  immediate: true,
});
</script>

<template>
  <div class="min-h-screen bg-background">
    <!-- Filter bar: every number's scope is set here and travels in the URL. -->
    <div class="sticky top-0 z-30 border-b bg-background/95 backdrop-blur print:hidden">
      <div class="flex flex-wrap items-center gap-2 px-6 py-2.5">
        <Select :model-value="q.project || '__all__'" @update:model-value="pickProject($event as string)">
          <SelectTrigger class="h-8 w-52">
            <SelectValue placeholder="All projects" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All projects</SelectItem>
            <SelectItem v-for="p in projects" :key="p.id" :value="p.project_key">{{ p.name }}</SelectItem>
          </SelectContent>
        </Select>

        <Select
          v-if="report && report.cycles.length > 0"
          :model-value="q.cycleId || '__range__'"
          @update:model-value="pickCycle($event as string)"
        >
          <SelectTrigger class="h-8 w-56">
            <SelectValue placeholder="Date range" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__range__">Date range</SelectItem>
            <SelectItem v-for="c in [...report.cycles].reverse()" :key="c.id" :value="c.id">
              {{ c.title }} · {{ c.status }}
            </SelectItem>
          </SelectContent>
        </Select>

        <div v-if="!q.cycleId" class="flex items-center gap-1">
          <Button v-for="r in RANGES" :key="r.days" variant="ghost" size="sm" class="h-8 text-xs" @click="pickRange(r.days)">
            {{ r.label }}
          </Button>
          <div class="ml-1 flex items-center gap-1 rounded-md border px-1.5 py-0.5">
            <input
              v-model="customFrom"
              type="date"
              class="h-6 bg-transparent text-xs outline-none"
              aria-label="From date"
              @change="applyCustomRange"
            />
            <span class="text-xs text-muted-foreground">–</span>
            <input
              v-model="customTo"
              type="date"
              class="h-6 bg-transparent text-xs outline-none"
              aria-label="To date"
              @change="applyCustomRange"
            />
          </div>
        </div>

        <div class="ml-auto flex items-center gap-1.5">
          <Button
            v-if="hasFilters"
            variant="ghost"
            size="sm"
            class="h-8 gap-1.5 text-xs text-muted-foreground"
            title="Back to the default view"
            @click="resetView"
          >
            <RotateCcw class="size-3.5" /> Reset
          </Button>
          <Button variant="outline" size="sm" class="h-8 gap-1.5 text-xs" @click="copyViewLink">
            <Copy class="size-3.5" /> Copy link
          </Button>
        </div>
      </div>
    </div>

    <div class="flex">
      <!-- Question rail: navigation plus the scope of every number. -->
      <aside class="sticky top-[49px] hidden h-[calc(100vh-49px)] w-60 shrink-0 flex-col border-r px-4 py-5 lg:flex print:hidden">
        <p class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Questions</p>
        <nav class="mt-2 space-y-1">
          <button
            v-for="item in questions"
            :key="item.id"
            class="block w-full rounded-md px-2 py-1.5 text-left text-sm transition-colors"
            :class="
              item.id === activeQ
                ? 'bg-muted font-medium text-foreground'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground'
            "
            :aria-current="item.id === activeQ ? 'true' : undefined"
            @click="jumpTo(item.id)"
          >
            {{ item.label }}
          </button>
        </nav>

        <div v-if="scope" class="mt-auto rounded-lg border bg-muted/30 p-3">
          <p class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Scope of every number</p>
          <p class="mt-1.5 text-xs">
            {{ scope.project_name || (currentWorkspace ? `Your projects in ${currentWorkspace.name}` : "All your projects") }}
            <template v-if="scope.cycle_title"> · {{ scope.cycle_title }}</template>
          </p>
          <p class="text-xs text-muted-foreground">{{ fmtDate(scope.from) }} – {{ fmtDate(scope.to) }} · {{ scope.timezone }}</p>
          <p class="mt-1.5 text-[11px] leading-snug text-muted-foreground">{{ scope.counting_rule }}</p>
        </div>
      </aside>

      <!-- The report body -->
      <main class="min-w-0 flex-1 px-6 py-6 lg:px-10">
        <div v-if="error" class="rounded-md border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm">{{ error }}</div>
        <div v-else-if="loading && !report && !portfolio && !wsPerson" class="space-y-4">
          <Skeleton class="h-8 w-72" />
          <Skeleton class="h-40 w-full" />
          <Skeleton class="h-64 w-full" />
        </div>

        <!-- Person across projects -->
        <template v-else-if="!q.project && q.person && wsPerson">
          <div class="mb-6">
            <button
              class="mb-1 flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
              @click="setQuery({ person: undefined })"
            >
              <ArrowLeft class="size-3" /> All projects
            </button>
            <h1 class="text-2xl font-semibold">
              {{ wsPerson.user.first_name }} {{ wsPerson.user.last_name }} · across projects
            </h1>
            <p class="mt-1 text-sm text-muted-foreground">
              {{ fmtDate(wsPerson.scope.from) }} – {{ fmtDate(wsPerson.scope.to) }} · {{ wsPerson.scope.timezone }}
              · {{ wsPerson.scope.counting_rule }}
            </p>
          </div>
          <div id="qw" class="scroll-mt-16">
            <WorkspacePersonSection :person="wsPerson" @project="(key) => setQuery({ project: key })" />
          </div>
        </template>

        <!-- Portfolio (question 0) -->
        <template v-else-if="!q.project && portfolio">
          <div class="mb-6">
            <h1 class="text-2xl font-semibold">Insights · {{ currentWorkspace?.name ?? "All projects" }}</h1>
            <p class="mt-1 text-sm text-muted-foreground">
              {{ portfolio.rows.length }} projects · {{ fmtDate(portfolio.scope.from) }} – {{ fmtDate(portfolio.scope.to) }}
              · generated {{ new Date(portfolio.scope.generated_at).toLocaleString("en-US", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" }) }}
            </p>
          </div>
          <div id="q0" class="scroll-mt-16">
            <PortfolioSection :portfolio="portfolio" @open="pickProject" />
          </div>
        </template>

        <!-- Project report -->
        <template v-else-if="q.project && report">
          <div class="mb-6">
            <button
              class="mb-1 flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
              @click="pickProject('__all__')"
            >
              <ArrowLeft class="size-3" /> All projects
            </button>
            <h1 class="text-2xl font-semibold">
              {{ report.scope.project_name }}<template v-if="report.scope.cycle_title"> · {{ report.scope.cycle_title }}</template>
            </h1>
            <p class="mt-1 text-sm text-muted-foreground">
              {{ fmtDate(report.scope.from) }} – {{ fmtDate(report.scope.to) }} · {{ report.scope.timezone }}
              · {{ report.scope.counting_rule }}
            </p>
          </div>

          <template v-if="q.person && person">
            <div id="q4" class="scroll-mt-16">
              <div class="mb-2 flex items-center gap-4">
                <button class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground" @click="setQuery({ person: undefined })">
                  <ArrowLeft class="size-3" /> Back to the project report
                </button>
                <button
                  class="text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                  title="The same person, summed over every project you can see"
                  @click="setQuery({ project: undefined, cycle_id: undefined, open: undefined, user_id: undefined })"
                >
                  This person across all projects →
                </button>
              </div>
              <PersonSection :person="person" :project-key="q.project" :win="win" @open="(m, uid) => openEvidence(m, { user_id: uid })" />
            </div>
          </template>
          <template v-else>
            <div id="q1" class="scroll-mt-16 break-before-page">
              <ProjectSection :report="report" @open="openEvidence" @day="(d) => openEvidence('day', { day: d })" />
            </div>
            <div id="q2" class="mt-10 scroll-mt-16 break-before-page">
              <PeopleSection :people="report.people" @person="openPerson" @open="(m, uid) => openEvidence(m, { user_id: uid })" />
            </div>
            <div id="q3" class="mt-10 scroll-mt-16 break-before-page">
              <HealthSection :health="report.health" :project-name="report.scope.project_name || ''" :project-key="q.project" @open="(m, p) => openEvidence(m, p)" />
            </div>
          </template>

          <p class="mt-10 border-t pt-3 text-[11px] text-muted-foreground">
            Generated {{ new Date(report.scope.generated_at).toLocaleString("en-US") }} · cutoff {{ new Date(report.scope.cutoff).toLocaleString("en-US") }}
            · timezone {{ report.scope.timezone }} · reconstructed from the activity log.
          </p>
        </template>
      </main>
    </div>

    <EvidenceDrawer
      :evidence="evidence"
      :loading="evidenceLoading"
      :project-key="q.project"
      :share-url="typeof window !== 'undefined' ? window.location.href : ''"
      @close="closeEvidence"
      @csv="exportCSV"
    />
  </div>
</template>

<style scoped>
@media print {
  .break-before-page {
    break-before: page;
  }
  .break-before-page:first-of-type {
    break-before: auto;
  }
}
</style>

<style>
/* Print: the report is the artifact — hide the app chrome and reclaim its
   offset. Unscoped because the sidebar lives in the app shell. */
@media print {
  aside {
    display: none !important;
  }
  .pl-12 {
    padding-left: 0 !important;
  }
}
</style>
