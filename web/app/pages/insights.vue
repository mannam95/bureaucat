<script setup lang="ts">
import { toast } from "vue-sonner";
import { Copy, Printer, ArrowLeft } from "lucide-vue-next";
import type {
  EvidenceQuery,
  InsightsEvidence,
  InsightsPortfolio,
  PersonInsights,
  ProjectInsights,
  Project,
} from "~/types";

// The insights report: a narrative answer with a question rail, where every
// number opens its exact evidence. The whole view state lives in the URL, so
// a copied link reproduces it for anyone with access.
definePageMeta({ middleware: ["auth"] });
useSeoMeta({ title: "Insights" });

const route = useRoute();
const router = useRouter();
const { fetchPortfolio, fetchProject, fetchPerson, fetchEvidence, downloadEvidenceCSV } = useInsights();
const { listProjects } = useProjects();

const projects = ref<Project[]>([]);
const portfolio = ref<InsightsPortfolio | null>(null);
const report = ref<ProjectInsights | null>(null);
const person = ref<PersonInsights | null>(null);
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
      const res = await fetchPortfolio(win.value);
      if (!res.success) error.value = res.error || "Failed to load";
      else portfolio.value = res.data!;
      return;
    }
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

onMounted(async () => {
  // Projects across all workspaces: insights is scoped by membership, not by
  // the active workspace.
  const res = await listProjects(1, 100, "", false);
  if (res.success && res.data) projects.value = res.data.projects;
});

async function copyViewLink() {
  try {
    await navigator.clipboard.writeText(window.location.href);
    toast.success("View link copied");
  } catch {
    toast.error("Could not copy the link");
  }
}

function printReport() {
  window.print();
}

async function exportCSV() {
  const eq = evidenceQuery.value;
  if (!eq || !q.value.project) return;
  const ok = await downloadEvidenceCSV(q.value.project, win.value, eq);
  if (!ok) toast.error("CSV export failed");
  else toast.success("CSV exported — exactly the drawer's rows");
}

const scope = computed(() => person.value?.scope ?? report.value?.scope ?? portfolio.value?.scope ?? null);

function fmtDate(iso?: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", { day: "numeric", month: "short", year: "numeric" });
}

const questions = computed(() => {
  if (!q.value.project) return [{ id: "q0", label: "0 · Which projects need attention?" }];
  const list = [
    { id: "q1", label: "1 · How did it land?" },
    { id: "q2", label: "2 · What did each person deliver?" },
    { id: "q3", label: "3 · Is the project healthy now?" },
  ];
  if (q.value.person && person.value) {
    list.push({ id: "q5", label: `5 · What did ${person.value.user.first_name} deliver?` });
  }
  return list;
});

function jumpTo(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
}
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
        </div>

        <div class="ml-auto flex items-center gap-1.5">
          <Button variant="outline" size="sm" class="h-8 gap-1.5 text-xs" @click="copyViewLink">
            <Copy class="size-3.5" /> Copy link
          </Button>
          <Button variant="outline" size="sm" class="h-8 gap-1.5 text-xs" @click="printReport">
            <Printer class="size-3.5" /> Print report
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
            class="block w-full rounded-md px-2 py-1.5 text-left text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            @click="jumpTo(item.id)"
          >
            {{ item.label }}
          </button>
        </nav>

        <div v-if="scope" class="mt-auto rounded-lg border bg-muted/30 p-3">
          <p class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Scope of every number</p>
          <p class="mt-1.5 text-xs">
            {{ scope.project_name || "All your projects" }}
            <template v-if="scope.cycle_title"> · {{ scope.cycle_title }}</template>
          </p>
          <p class="text-xs text-muted-foreground">{{ fmtDate(scope.from) }} – {{ fmtDate(scope.to) }} · {{ scope.timezone }}</p>
          <p class="mt-1.5 text-[11px] leading-snug text-muted-foreground">{{ scope.counting_rule }}</p>
        </div>
      </aside>

      <!-- The report body -->
      <main class="min-w-0 flex-1 px-6 py-6 lg:px-10">
        <div v-if="error" class="rounded-md border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm">{{ error }}</div>
        <div v-else-if="loading && !report && !portfolio" class="space-y-4">
          <Skeleton class="h-8 w-72" />
          <Skeleton class="h-40 w-full" />
          <Skeleton class="h-64 w-full" />
        </div>

        <!-- Portfolio (question 0) -->
        <template v-else-if="!q.project && portfolio">
          <div class="mb-6">
            <h1 class="text-2xl font-semibold">Insights · All projects</h1>
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
            <div id="q5" class="scroll-mt-16">
              <button class="mb-2 flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground" @click="setQuery({ person: undefined })">
                <ArrowLeft class="size-3" /> Back to the project report
              </button>
              <PersonSection :person="person" @open="(m, uid) => openEvidence(m, { user_id: uid })" />
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
              <HealthSection :health="report.health" :project-name="report.scope.project_name || ''" @open="(m, p) => openEvidence(m, p)" />
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
