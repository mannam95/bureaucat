<script setup lang="ts">
import { Loader2, Search, Check, Copy } from "lucide-vue-next";
import { watchDebounced } from "@vueuse/core";
import type {
  Project,
  ProjectState,
  ProjectLabel,
  ProjectMember,
  CycleSibling,
  Task,
} from "~/types";

// Copy one task into another project. Everything is carried over as it is —
// description (with a back-link added), attachments, estimation, sub-tasks —
// except the fields below, which are project-specific and picked here.
const props = defineProps<{
  projectKey: string;
  taskNum: number;
  task: Task;
}>();

const open = defineModel<boolean>("open", { default: false });
const emit = defineEmits<{
  copied: [payload: { projectKey: string; taskNumber: number; taskId: string }];
}>();

const { listProjects } = useProjects();
const { copyTask } = useTasks();
const { getAuthHeader } = useAuth();

// ---- Step 1: pick the target project (searchable, source excluded) ----
const projects = ref<Project[]>([]);
const projectsLoading = ref(false);
const search = ref("");
const selectedKey = ref<string | null>(null);
const submitting = ref(false);
const error = ref<string | null>(null);

const filtered = computed(() =>
  projects.value.filter((p) => p.project_key !== props.projectKey)
);

async function loadProjects(query = "") {
  projectsLoading.value = true;
  // Destinations span every workspace the user belongs to.
  const res = await listProjects(1, 50, query.trim(), false);
  projectsLoading.value = false;
  if (res.success) projects.value = res.data?.projects || [];
}

watchDebounced(
  search,
  (q) => {
    if (open.value) loadProjects(q);
  },
  { debounce: 300 }
);

// ---- Step 2: the target's project-specific fields ----
// Fetched directly (not via the shared composable state) so the page we came
// from keeps its own states/labels/members untouched.
const metaLoading = ref(false);
const targetStates = ref<ProjectState[]>([]);
const targetLabels = ref<ProjectLabel[]>([]);
const targetMembers = ref<ProjectMember[]>([]);
const targetCycles = ref<CycleSibling[]>([]);

const form = ref({
  state_id: "",
  priority: 0,
  assignees: [] as string[],
  labels: [] as string[],
  cycle_id: "",
});

async function fetchJSON<T>(path: string): Promise<T | null> {
  try {
    const res = await fetch(`/api/v1/projects/${selectedKey.value}${path}`, {
      headers: getAuthHeader(),
    });
    if (!res.ok) return null;
    return (await res.json()) as T;
  } catch {
    return null;
  }
}

async function selectProject(key: string) {
  selectedKey.value = key;
  metaLoading.value = true;
  const [states, labels, members, cycles] = await Promise.all([
    fetchJSON<ProjectState[]>("/states"),
    fetchJSON<ProjectLabel[]>("/labels"),
    fetchJSON<ProjectMember[]>("/members"),
    fetchJSON<CycleSibling[]>("/cycles/all"),
  ]);
  targetStates.value = states ?? [];
  targetLabels.value = labels ?? [];
  targetMembers.value = members ?? [];
  targetCycles.value = cycles ?? [];

  // Sensible prefills: the target's default state, the original's priority,
  // assignees who are members of both projects, and labels matching by name.
  const memberIds = new Set(targetMembers.value.map((m) => m.user_id));
  const sourceLabelNames = new Set((props.task.labels ?? []).map((l) => l.name));
  form.value = {
    state_id: targetStates.value.find((s) => s.is_default)?.id ?? targetStates.value[0]?.id ?? "",
    priority: props.task.priority,
    assignees: (props.task.assignees ?? [])
      .map((a) => a.user_id)
      .filter((id) => memberIds.has(id)),
    labels: targetLabels.value.filter((l) => sourceLabelNames.has(l.name)).map((l) => l.id),
    cycle_id: "",
  };
  metaLoading.value = false;
}

watch(open, (isOpen) => {
  if (!isOpen) return;
  error.value = null;
  search.value = "";
  selectedKey.value = null;
  loadProjects();
});

const priorities = [
  { value: 0, label: "No priority" },
  { value: 1, label: "Low" },
  { value: 2, label: "Medium" },
  { value: 3, label: "High" },
  { value: 4, label: "Urgent" },
];

const priorityValue = computed({
  get: () => String(form.value.priority),
  set: (v: string) => {
    form.value.priority = Number(v);
  },
});
const NO_CYCLE = "__none__";
const cycleValue = computed({
  get: () => form.value.cycle_id || NO_CYCLE,
  set: (v: string) => {
    form.value.cycle_id = v === NO_CYCLE ? "" : v;
  },
});

function toggleAssignee(userId: string) {
  if (form.value.assignees.includes(userId)) {
    form.value.assignees = form.value.assignees.filter((id) => id !== userId);
  } else {
    form.value.assignees = [...form.value.assignees, userId];
  }
}

function toggleLabel(labelId: string) {
  if (form.value.labels.includes(labelId)) {
    form.value.labels = form.value.labels.filter((id) => id !== labelId);
  } else {
    form.value.labels = [...form.value.labels, labelId];
  }
}

async function handleCopy() {
  if (!selectedKey.value || !form.value.state_id || submitting.value) return;
  submitting.value = true;
  error.value = null;
  const res = await copyTask(props.projectKey, props.taskNum, {
    target_project_key: selectedKey.value,
    state_id: form.value.state_id,
    priority: form.value.priority,
    assignees: form.value.assignees,
    labels: form.value.labels,
    cycle_id: form.value.cycle_id || undefined,
  });
  submitting.value = false;
  if (!res.success || !res.data) {
    error.value = res.error || "Failed to copy the task";
    return;
  }
  open.value = false;
  emit("copied", {
    projectKey: res.data.project_key,
    taskNumber: res.data.task_number,
    taskId: res.data.task_id,
  });
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="max-h-[85vh] overflow-y-auto sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>Copy to another project</DialogTitle>
        <DialogDescription>
          Everything is copied as it is — description, attachments, estimation
          and sub-tasks. The copy links back to {{ task.task_id }}, and
          {{ task.task_id }} gets a comment linking to the copy. Comments and
          watchers stay on the original.
        </DialogDescription>
      </DialogHeader>

      <div v-if="error" role="alert" class="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
        {{ error }}
      </div>

      <!-- Target project -->
      <div class="space-y-2">
        <Label>Project</Label>
        <div class="relative">
          <Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input v-model="search" placeholder="Search projects…" class="pl-9" />
        </div>
        <div class="max-h-40 overflow-y-auto rounded-md border">
          <div v-if="projectsLoading" class="flex items-center justify-center py-4">
            <Loader2 class="size-4 animate-spin text-muted-foreground" />
          </div>
          <p v-else-if="filtered.length === 0" class="px-3 py-4 text-center text-sm text-muted-foreground">
            No other projects
          </p>
          <button
            v-for="p in filtered"
            v-else
            :key="p.id"
            type="button"
            class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent"
            :class="selectedKey === p.project_key ? 'bg-accent' : ''"
            @click="selectProject(p.project_key)"
          >
            <Check class="size-3.5 shrink-0" :class="selectedKey === p.project_key ? 'opacity-100' : 'opacity-0'" />
            <span class="shrink-0 font-mono text-xs text-muted-foreground">{{ p.project_key }}</span>
            <span class="truncate">{{ p.name }}</span>
          </button>
        </div>
      </div>

      <!-- Project-specific fields, once a target is chosen -->
      <div v-if="selectedKey" class="space-y-4">
        <p class="text-xs text-muted-foreground">
          These fields are project-specific — pick them for the target:
        </p>
        <div v-if="metaLoading" class="flex items-center justify-center py-4">
          <Loader2 class="size-4 animate-spin text-muted-foreground" />
        </div>
        <template v-else>
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label>State</Label>
              <Select v-model="form.state_id" :disabled="submitting">
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="Select a state" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="st in targetStates" :key="st.id" :value="st.id" :title="st.description || undefined">
                    {{ st.name }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <p class="text-[11px] text-muted-foreground">Sub-task copies also start in this state.</p>
            </div>
            <div class="space-y-2">
              <Label>Priority</Label>
              <Select v-model="priorityValue" :disabled="submitting">
                <SelectTrigger class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="p in priorities" :key="p.value" :value="String(p.value)">
                    {{ p.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div v-if="targetCycles.length > 0" class="space-y-2">
            <Label>Cycle <span class="text-xs text-muted-foreground">(optional)</span></Label>
            <Select v-model="cycleValue" :disabled="submitting">
              <SelectTrigger class="w-full">
                <SelectValue placeholder="No cycle" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="NO_CYCLE">No cycle</SelectItem>
                <SelectItem v-for="c in targetCycles" :key="c.id" :value="c.id">
                  {{ c.title }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div v-if="targetMembers.length > 0" class="space-y-2">
            <Label>Assignees <span class="text-xs text-muted-foreground">(members of both projects come preselected)</span></Label>
            <div class="flex max-h-28 flex-wrap gap-1.5 overflow-y-auto">
              <button
                v-for="m in targetMembers"
                :key="m.user_id"
                type="button"
                class="rounded-full border px-2.5 py-1 text-xs transition-colors"
                :class="form.assignees.includes(m.user_id)
                  ? 'border-primary bg-primary/10 font-medium'
                  : 'text-muted-foreground hover:border-foreground/30'"
                @click="toggleAssignee(m.user_id)"
              >
                {{ `${m.first_name} ${m.last_name}`.trim() || m.username }}
              </button>
            </div>
          </div>

          <div v-if="targetLabels.length > 0" class="space-y-2">
            <Label>Labels <span class="text-xs text-muted-foreground">(name matches come preselected)</span></Label>
            <div class="flex max-h-28 flex-wrap gap-1.5 overflow-y-auto">
              <button
                v-for="l in targetLabels"
                :key="l.id"
                type="button"
                class="rounded-full border px-2.5 py-1 text-xs font-medium transition-colors"
                :style="form.labels.includes(l.id)
                  ? { backgroundColor: l.color + '20', color: l.color, borderColor: l.color }
                  : {}"
                :class="form.labels.includes(l.id) ? '' : 'text-muted-foreground hover:border-foreground/30'"
                @click="toggleLabel(l.id)"
              >
                {{ l.name }}
              </button>
            </div>
          </div>
        </template>
      </div>

      <DialogFooter>
        <Button variant="outline" :disabled="submitting" @click="open = false">
          Cancel
        </Button>
        <Button :disabled="submitting || !selectedKey || !form.state_id || metaLoading" @click="handleCopy">
          <Loader2 v-if="submitting" class="mr-2 size-4 animate-spin" />
          <Copy v-else class="mr-2 size-4" />
          Copy
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
