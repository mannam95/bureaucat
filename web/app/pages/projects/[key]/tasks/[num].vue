<script setup lang="ts">
import {
  ChevronDown,
  Loader2,
  Pencil,
  Plus,
  Trash2,
  FolderInput,
  Lock,
  Check,
  X,
  Calendar as CalendarIcon,
  Clock,
  Link,
  Repeat,
  Layers,
  Circle,
  CircleDot,
  CheckCircle2,
  XCircle,
} from "lucide-vue-next";
import { toast } from "vue-sonner";
import { marked } from "marked";
import { CalendarDate, type DateValue } from "@internationalized/date";
import { PRIORITY_LABELS } from "~/types";

const renderer = new marked.Renderer();
renderer.link = ({ href, title, text }) => {
  const titleAttr = title ? ` title="${title}"` : "";
  return `<a href="${href}"${titleAttr} target="_blank" rel="noopener noreferrer">${text}</a>`;
};
marked.setOptions({ breaks: true, gfm: true, renderer });

definePageMeta({
  middleware: ["auth"],
});

const route = useRoute();
const router = useRouter();

const projectKey = computed(() => route.params.key as string);
const taskNum = computed(() => parseInt(route.params.num as string));

const {
  currentProject,
  members,
  states,
  labels: projectLabels,
  getProject,
  listMembers,
  listStates,
  listLabels,
} = useProjects();

const { currentTask, getTask, updateTask, deleteTask, listSubtasks, attachSubtasks, fetchAllTasks } =
  useTasks();
const { listAllCycles, addTasksToCycle, removeTaskFromCycle } = useCycles();
const { modules, listModules, addTasksToModule, removeTaskFromModule } = useModules();
const { comments, loading: commentsLoading, listComments } = useComments();
const { activities, loading: activitiesLoading, listActivity } = useActivity();
const { listAttachments, attachFile, deleteAttachment } = useAttachments();
const { uploadFiles, uploading: descriptionUploading } = useFileAttach();

useHead({
  title: computed(() => {
    const task = currentTask.value;
    if (task) return `${task.task_id} · ${task.title}`;
    return `${projectKey.value}-${taskNum.value}`;
  }),
});

const loading = ref(true);
const error = ref<string | null>(null);
const editingTitle = ref(false);
const editingDescription = ref(false);
const editTitle = ref("");
const editDescription = ref("");
const updating = ref(false);
const deleting = ref(false);
const showDeleteConfirm = ref(false);

const { user } = useAuth();
const isAdmin = computed(() => currentProject.value?.role === "admin");
// A disabled project is read-only, so write affordances are suppressed even for
// members. The backend enforces this too.
const isDisabled = computed(() => currentProject.value?.disabled ?? false);
const isMember = computed(
  () =>
    !isDisabled.value &&
    (currentProject.value?.role === "admin" || currentProject.value?.role === "member")
);
const isCreator = computed(() => user.value?.id === currentTask.value?.created_by);
const canDelete = computed(() => !isDisabled.value && (isAdmin.value || isCreator.value));

const priorityOptions = Object.entries(PRIORITY_LABELS).map(([value, info]) => ({
  value: parseInt(value),
  label: info.label,
  color: info.color,
}));

const currentPriority = computed(() => {
  const p = currentTask.value?.priority ?? 0;
  return PRIORITY_LABELS[p] || PRIORITY_LABELS[0];
});

const currentState = computed(() =>
  states.value.find((s) => s.id === currentTask.value?.state_id)
);

const stateIconMap: Record<string, typeof Circle> = {
  backlog: Clock,
  unstarted: Circle,
  started: CircleDot,
  completed: CheckCircle2,
  cancelled: XCircle,
};

async function loadData() {
  loading.value = true;
  error.value = null;

  // Load project data if not already loaded
  if (!currentProject.value || currentProject.value.project_key !== projectKey.value) {
    const projectResult = await getProject(projectKey.value);
    if (!projectResult.success) {
      error.value = projectResult.error || "Failed to load project";
      loading.value = false;
      return;
    }
  }

  // Load task
  const taskResult = await getTask(projectKey.value, taskNum.value);
  if (!taskResult.success) {
    error.value = taskResult.error || "Task not found";
    loading.value = false;
    return;
  }

  // Load supporting data in parallel
  await Promise.all([
    listMembers(projectKey.value),
    listStates(projectKey.value),
    listLabels(projectKey.value),
    listComments(projectKey.value, taskNum.value),
    listActivity(projectKey.value, taskNum.value),
    loadTaskAttachments(),
    loadSubtasks(),
    loadLinkOptions(),
  ]);

  loading.value = false;
}

function startEditTitle() {
  editTitle.value = currentTask.value?.title || "";
  editingTitle.value = true;
}

function cancelEditTitle() {
  editingTitle.value = false;
  editTitle.value = "";
}

async function saveTitle() {
  if (!editTitle.value.trim() || editTitle.value === currentTask.value?.title) {
    cancelEditTitle();
    return;
  }

  updating.value = true;
  const result = await updateTask(projectKey.value, taskNum.value, {
    title: editTitle.value,
  });
  updating.value = false;

  if (result.success) {
    toast.success("Title updated");
    cancelEditTitle();
  } else {
    toast.error(result.error || "Failed to update title");
  }
}

function startEditDescription() {
  const desc = currentTask.value?.description || "";
  // Convert markdown to HTML for the editor (handles legacy markdown descriptions)
  editDescription.value = desc.startsWith("<") ? desc : (marked(desc) as string);
  editingDescription.value = true;
}

function cancelEditDescription() {
  editingDescription.value = false;
  editDescription.value = "";
}

async function saveDescription() {
  // Treat empty tiptap content as no description
  const isEmpty = !editDescription.value || editDescription.value === "<p></p>";
  const current = currentTask.value?.description || "";
  if (editDescription.value === current) {
    cancelEditDescription();
    return;
  }

  updating.value = true;
  const result = await updateTask(projectKey.value, taskNum.value, {
    description: isEmpty ? undefined : editDescription.value,
  });
  updating.value = false;

  if (result.success) {
    toast.success("Description updated");
    cancelEditDescription();
  } else {
    toast.error(result.error || "Failed to update description");
  }
}

async function handleStateChange(stateId: string) {
  updating.value = true;
  const result = await updateTask(projectKey.value, taskNum.value, {
    state_id: stateId,
  });
  updating.value = false;

  if (result.success) {
    toast.success("State updated");
    await listActivity(projectKey.value, taskNum.value);
  } else {
    toast.error(result.error || "Failed to update state");
  }
}

async function handlePriorityChange(priority: number) {
  updating.value = true;
  const result = await updateTask(projectKey.value, taskNum.value, {
    priority,
  });
  updating.value = false;

  if (result.success) {
    toast.success("Priority updated");
    await listActivity(projectKey.value, taskNum.value);
  } else {
    toast.error(result.error || "Failed to update priority");
  }
}

const startDateOpen = ref(false);
const dueDateOpen = ref(false);
const startDateDraft = ref<DateValue | undefined>();
const startTimeDraft = ref("09:00");
const dueDateDraft = ref<DateValue | undefined>();
const dueTimeDraft = ref("17:00");

function isoToCalendarDate(iso: string | undefined): DateValue | undefined {
  if (!iso) return undefined;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return undefined;
  return new CalendarDate(d.getFullYear(), d.getMonth() + 1, d.getDate());
}

function isoToTimeInput(iso: string | undefined, fallback: string): string {
  if (!iso) return fallback;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return fallback;
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

function combineDateTime(date: DateValue, time: string): string {
  const [hh, mm] = time.split(":").map((n) => parseInt(n, 10));
  const d = new Date(date.year, date.month - 1, date.day, hh || 0, mm || 0, 0, 0);
  return d.toISOString();
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

watch(startDateOpen, (open) => {
  if (!open) return;
  startDateDraft.value = isoToCalendarDate(currentTask.value?.start_date);
  startTimeDraft.value = isoToTimeInput(currentTask.value?.start_date, "09:00");
});

watch(dueDateOpen, (open) => {
  if (!open) return;
  dueDateDraft.value = isoToCalendarDate(currentTask.value?.due_date);
  dueTimeDraft.value = isoToTimeInput(currentTask.value?.due_date, "17:00");
});

async function updateDate(field: "start_date" | "due_date", value: string | null) {
  updating.value = true;
  const result = await updateTask(projectKey.value, taskNum.value, {
    [field]: value,
  });
  updating.value = false;

  const label = field === "start_date" ? "Start date" : "Due date";
  if (result.success) {
    toast.success(value === null ? `${label} cleared` : `${label} updated`);
    await listActivity(projectKey.value, taskNum.value);
  } else {
    toast.error(result.error || `Failed to update ${label.toLowerCase()}`);
  }
}

const startMaxValue = computed(() => isoToCalendarDate(currentTask.value?.due_date));
const dueMinValue = computed(() => isoToCalendarDate(currentTask.value?.start_date));

async function saveStartDate() {
  if (!startDateDraft.value) return;
  const iso = combineDateTime(startDateDraft.value, startTimeDraft.value);
  const due = currentTask.value?.due_date;
  if (due && new Date(iso) > new Date(due)) {
    toast.error("Start date cannot be after due date");
    return;
  }
  startDateOpen.value = false;
  await updateDate("start_date", iso);
}

async function saveDueDate() {
  if (!dueDateDraft.value) return;
  const iso = combineDateTime(dueDateDraft.value, dueTimeDraft.value);
  const start = currentTask.value?.start_date;
  if (start && new Date(iso) < new Date(start)) {
    toast.error("Due date cannot be before start date");
    return;
  }
  dueDateOpen.value = false;
  await updateDate("due_date", iso);
}

async function clearStartDate() {
  startDateOpen.value = false;
  await updateDate("start_date", null);
}

async function clearDueDate() {
  dueDateOpen.value = false;
  await updateDate("due_date", null);
}

async function handleDelete() {
  deleting.value = true;
  const result = await deleteTask(projectKey.value, taskNum.value);
  deleting.value = false;

  if (result.success) {
    toast.success("Task deleted");
    router.push(`/projects/${projectKey.value}`);
  } else {
    toast.error(result.error || "Failed to delete task");
  }
}

const showMoveDialog = ref(false);

// Subtasks
const subtasks = ref<import("~/types").Subtask[]>([]);
const subtasksLoading = ref(false);
const showCreateSubtask = ref(false);
// A subtask cannot itself have subtasks (one level of nesting).
const isSubtask = computed(() => currentTask.value?.parent_task_id != null);

async function loadSubtasks() {
  subtasksLoading.value = true;
  const result = await listSubtasks(projectKey.value, taskNum.value);
  if (result.success && result.data) {
    subtasks.value = result.data;
  }
  subtasksLoading.value = false;
}

async function onSubtaskCreated() {
  await Promise.all([loadSubtasks(), refreshTask()]);
}

// ---- Custom fields (Figma link / branch / pull request) ----
// Edited one at a time, inline: the edit button swaps the value for a text
// field with explicit save and cancel. Sub-tasks are tasks, so this works on
// them unchanged.
const CUSTOM_FIELDS = [
  {
    key: "figma_link",
    label: "Figma Link",
    hint: "The origin of the design",
    placeholder: "https://figma.com/...",
  },
  {
    key: "branch",
    label: "Branch",
    hint: "The branch where this task is worked on",
    placeholder: "https://github.com/.../tree/feat/my-branch",
  },
  {
    key: "pull_request",
    label: "Pull Request",
    hint: "The PR where this task is worked on",
    placeholder: "https://github.com/.../pull/123",
  },
] as const;

type CustomFieldKey = (typeof CUSTOM_FIELDS)[number]["key"];

const editingField = ref<CustomFieldKey | null>(null);
const customDraft = ref("");
const savingField = ref(false);

function customFieldValue(key: CustomFieldKey): string {
  return currentTask.value?.[key] ?? "";
}

function startEditField(key: CustomFieldKey) {
  editingField.value = key;
  customDraft.value = customFieldValue(key);
}

function cancelEditField() {
  editingField.value = null;
  customDraft.value = "";
}

async function saveField(key: CustomFieldKey) {
  // An empty string clears the field; the API treats omitted as "unchanged".
  const value = customDraft.value.trim();
  const payload =
    key === "figma_link"
      ? { figma_link: value }
      : key === "branch"
        ? { branch: value }
        : { pull_request: value };

  savingField.value = true;
  const result = await updateTask(projectKey.value, taskNum.value, payload);
  savingField.value = false;
  if (result.success) {
    cancelEditField();
    await refreshTask();
  } else {
    toast.error(result.error || "Failed to update");
  }
}

// ---- Cycle / Epic linking + sub-task re-parenting (right sidebar) ----
// A top-level task can be linked to a cycle and an epic. A sub-task instead
// shows its parent's cycle/epic (read-only) and can be moved under another
// parent, so it only needs the list of candidate parents.
const cycleOptions = ref<import("~/types").CycleSibling[]>([]);
const parentOptions = ref<import("~/types").Task[]>([]);

async function loadLinkOptions() {
  if (isSubtask.value) {
    const res = await fetchAllTasks(projectKey.value);
    if (res.success && res.data) {
      parentOptions.value = res.data.filter((t) => t.id !== currentTask.value?.id);
    }
    return;
  }
  const [cyclesRes] = await Promise.all([
    listAllCycles(projectKey.value),
    listModules(projectKey.value, 1, 100),
  ]);
  if (cyclesRes.success && cyclesRes.data) cycleOptions.value = cyclesRes.data;
}

// A task belongs to at most one cycle, so clear the existing link before adding
// the new one. Passing null just clears the current cycle.
async function setCycle(cycleId: string | null) {
  const t = currentTask.value;
  if (!t || t.cycle?.id === cycleId) return;
  updating.value = true;
  if (t.cycle) await removeTaskFromCycle(projectKey.value, t.cycle.id, t.id);
  const res = cycleId
    ? await addTasksToCycle(projectKey.value, cycleId, [t.id])
    : { success: true };
  await refreshTask();
  updating.value = false;
  if (res.success) toast.success(cycleId ? "Cycle updated" : "Cycle cleared");
  else toast.error(res.error || "Failed to update cycle");
}

// Epic (module) is offered as a single-select control here for a clean sidebar,
// so we clear the current epic before linking the new one.
async function setModule(moduleId: string | null) {
  const t = currentTask.value;
  if (!t || t.module?.id === moduleId) return;
  updating.value = true;
  if (t.module) await removeTaskFromModule(projectKey.value, t.module.id, t.id);
  const res = moduleId
    ? await addTasksToModule(projectKey.value, moduleId, [t.id])
    : { success: true };
  await refreshTask();
  updating.value = false;
  if (res.success) toast.success(moduleId ? "Epic updated" : "Epic cleared");
  else toast.error(res.error || "Failed to update epic");
}

// Move this sub-task under a different top-level parent.
async function setParent(parentTaskNum: number) {
  const t = currentTask.value;
  if (!t || parentTaskNum === t.parent_task_number) return;
  updating.value = true;
  const res = await attachSubtasks(projectKey.value, parentTaskNum, [t.id]);
  await refreshTask();
  updating.value = false;
  if (res.success) toast.success("Parent updated");
  else toast.error(res.error || "Failed to change parent");
}

function handleTaskMoved(payload: { targetKey: string; newTaskNumber?: number }) {
  toast.success("Task moved");
  if (payload.newTaskNumber !== undefined) {
    router.push(`/projects/${payload.targetKey}/tasks/${payload.newTaskNumber}`);
  }
}

async function refreshTask() {
  await Promise.all([
    getTask(projectKey.value, taskNum.value),
    listActivity(projectKey.value, taskNum.value),
  ]);
}

async function refreshComments() {
  await listComments(projectKey.value, taskNum.value);
  await listActivity(projectKey.value, taskNum.value);
}

const renderedDescription = computed(() => {
  const desc = currentTask.value?.description;
  if (!desc) return "";
  // If already HTML (from tiptap), render directly; otherwise convert markdown
  return desc.startsWith("<") ? desc : (marked(desc) as string);
});

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

// Task attachments
const taskAttachments = ref<import("~/composables/useAttachments").Attachment[]>([]);
const taskAttachmentsLoading = ref(false);

async function loadTaskAttachments() {
  taskAttachmentsLoading.value = true;
  const result = await listAttachments(projectKey.value, taskNum.value, "task");
  if (result.success && result.data) {
    taskAttachments.value = result.data;
  }
  taskAttachmentsLoading.value = false;
}

async function handleDescriptionFilesDropped(files: File[]) {
  const results = await uploadFiles(files);
  for (const r of results) {
    const result = await attachFile(projectKey.value, taskNum.value, "task", r.uploadId);
    if (result.success && result.data) {
      taskAttachments.value.push(result.data);
    }
  }
  if (results.length > 0) {
    toast.success(`${results.length} file${results.length > 1 ? "s" : ""} attached`);
  }
}

async function handleDeleteTaskAttachment(attachmentId: string) {
  const result = await deleteAttachment(projectKey.value, taskNum.value, "task", attachmentId);
  if (result.success) {
    taskAttachments.value = taskAttachments.value.filter((a) => a.id !== attachmentId);
  }
}

const copiedLink = ref(false);
function copyLink() {
  navigator.clipboard.writeText(window.location.href);
  copiedLink.value = true;
  toast.success("Link copied");
  setTimeout(() => { copiedLink.value = false; }, 2000);
}

onMounted(() => {
  loadData();
});
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <Navbar />

    <main id="main-content" class="flex-1">
      <div class="mx-auto max-w-6xl px-6 py-8">
        <!-- Loading -->
        <div v-if="loading" class="flex items-center justify-center py-20">
          <Loader2 class="size-8 animate-spin text-muted-foreground" />
        </div>

        <!-- Error -->
        <div
          v-else-if="error"
          class="flex flex-col items-center justify-center py-20"
        >
          <p class="text-lg text-destructive">{{ error }}</p>
          <Button class="mt-4" variant="outline" as-child>
            <NuxtLink :to="`/projects/${projectKey}`">
              Back to Project
            </NuxtLink>
          </Button>
        </div>

        <!-- Task content -->
        <template v-else-if="currentTask">
          <!-- Breadcrumb -->
          <nav class="mb-6 flex items-center gap-2 text-sm text-muted-foreground">
            <NuxtLink to="/projects" class="hover:text-foreground">
              Projects
            </NuxtLink>
            <span>/</span>
            <NuxtLink
              :to="`/projects/${projectKey}`"
              class="font-semibold text-amber-600 hover:text-amber-700 dark:text-amber-500 dark:hover:text-amber-400"
            >
              {{ projectKey }}
            </NuxtLink>
            <span>/</span>
            <template v-if="currentTask.parent_task_number != null">
              <NuxtLink
                :to="`/projects/${projectKey}/tasks/${currentTask.parent_task_number}`"
                class="max-w-[16rem] truncate font-semibold text-amber-600 hover:text-amber-700 dark:text-amber-500 dark:hover:text-amber-400"
                :title="currentTask.parent_task_title"
              >
                {{ currentTask.parent_task_number }}
              </NuxtLink>
              <span>/</span>
            </template>
            <NuxtLink
              :to="`/projects/${projectKey}/tasks/${taskNum}`"
              class="font-semibold text-amber-600 hover:text-amber-700 dark:text-amber-500 dark:hover:text-amber-400"
            >
              {{ taskNum }}
            </NuxtLink>
            <button
              aria-label="Copy link"
              class="ml-1 rounded-md p-1 text-muted-foreground/50 hover:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 outline-none"
              @click="copyLink"
            >
              <Check v-if="copiedLink" class="size-3.5 text-emerald-500" />
              <Link v-else class="size-3.5" />
            </button>
          </nav>

          <div
            v-if="isDisabled"
            class="mb-6 flex items-center gap-2 rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-400"
          >
            <Lock class="size-4 shrink-0" />
            <span>
              This project is disabled and read-only. No changes can be made until an admin re-enables it in Settings.
            </span>
          </div>

          <div class="flex flex-col gap-8 md:flex-row">
            <!-- Main content -->
            <div class="min-w-0 flex-1 space-y-6">
              <!-- Title -->
              <div>
                <div v-if="editingTitle" class="space-y-2">
                  <Input
                    v-model="editTitle"
                    class="text-xl font-bold"
                    :disabled="updating"
                    @keydown.enter="saveTitle"
                    @keydown.escape="cancelEditTitle"
                  />
                  <div class="flex gap-2">
                    <Button size="sm" :disabled="updating" @click="saveTitle">
                      <Loader2 v-if="updating" class="mr-1.5 size-3 animate-spin" />
                      <Check v-else class="mr-1.5 size-3" />
                      Save
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      :disabled="updating"
                      @click="cancelEditTitle"
                    >
                      <X class="mr-1.5 size-3" />
                      Cancel
                    </Button>
                  </div>
                </div>
                <div v-else class="group flex items-start gap-2">
                  <h1 class="text-2xl font-bold">{{ currentTask.title }}</h1>
                  <Button
                    v-if="isMember"
                    variant="ghost"
                    size="icon"
                    aria-label="Edit title"
                    class="mt-1 size-7 opacity-0 transition-opacity group-hover:opacity-100 focus:opacity-100"
                    @click="startEditTitle"
                  >
                    <Pencil class="size-3.5" />
                  </Button>
                </div>
                <div class="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                  <div v-if="currentState" class="flex items-center gap-1 rounded-md border bg-muted/50 px-1.5 py-0.5 w-fit">
                    <component
                      :is="stateIconMap[currentState.state_type] || Circle"
                      class="size-3.5 stroke-[2.5]"
                      :style="{ color: currentState.color }"
                    />
                    <span>{{ currentState.name }}</span>
                  </div>
                  <div class="flex items-center gap-1 rounded-md border bg-muted/50 px-1.5 py-0.5 w-fit">
                    <span
                      class="size-2.5 rounded-full ring-1.5 ring-offset-1 ring-offset-background"
                      :style="{ backgroundColor: currentPriority.color, '--tw-ring-color': currentPriority.color }"
                    />
                    <span>{{ currentPriority.label }}</span>
                  </div>
                  <NuxtLink :to="`/profile/${currentTask.created_by}`" class="flex items-center gap-1 rounded-md border bg-muted/50 py-0.5 pl-0.5 pr-1.5 w-fit hover:bg-muted transition-colors">
                    <Avatar class="size-4">
                      <AvatarImage v-if="currentTask.creator_avatar_url" :src="currentTask.creator_avatar_url" />
                      <AvatarFallback class="text-[10px]" :seed="currentTask.created_by">
                        {{ currentTask.creator_first_name?.[0] }}{{ currentTask.creator_last_name?.[0] }}
                      </AvatarFallback>
                    </Avatar>
                    <span>{{ currentTask.creator_first_name }} {{ currentTask.creator_last_name }}</span>
                  </NuxtLink>
                  <span>created on {{ formatDate(currentTask.created_at) }}</span>
                </div>
              </div>

              <!-- Description -->
              <div class="group">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <h2 class="text-sm font-medium text-muted-foreground">
                    Description
                  </h2>
                  <Button
                    v-if="isMember && !editingDescription && currentTask.description"
                    variant="ghost"
                    size="icon"
                    aria-label="Edit description"
                    class="size-6 opacity-0 transition-opacity group-hover:opacity-100 focus:opacity-100"
                    @click="startEditDescription"
                  >
                    <Pencil class="size-3.5" />
                  </Button>
                </div>
                <div v-if="editingDescription" class="space-y-2">
                  <TiptapEditor
                    v-model="editDescription"
                    :disabled="updating"
                    :uploading="descriptionUploading"
                    :members="members"
                    @files-dropped="handleDescriptionFilesDropped"
                  />
                  <div class="flex gap-2">
                    <Button size="sm" :disabled="updating" @click="saveDescription">
                      <Loader2 v-if="updating" class="mr-1.5 size-3 animate-spin" />
                      <Check v-else class="mr-1.5 size-3" />
                      Save
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      :disabled="updating"
                      @click="cancelEditDescription"
                    >
                      <X class="mr-1.5 size-3" />
                      Cancel
                    </Button>
                  </div>
                </div>
                <div v-else>
                  <div v-if="currentTask.description">
                    <div
                      class="prose prose-sm max-w-none dark:prose-invert"
                      v-html="renderedDescription"
                    />
                  </div>
                  <button
                    v-else-if="isMember"
                    type="button"
                    class="w-full rounded-lg border border-dashed p-4 text-left text-sm text-muted-foreground hover:border-solid hover:bg-muted/50"
                    @click="startEditDescription"
                  >
                    Add a description...
                  </button>
                  <p v-else class="text-sm italic text-muted-foreground">
                    No description
                  </p>
                </div>

                <!-- Task attachments -->
                <FileDropZone
                  v-if="isMember"
                  :show-button="false"
                  :uploading="descriptionUploading"
                  accept="*/*"
                  @files-dropped="handleDescriptionFilesDropped"
                >
                  <AttachmentList
                    :attachments="taskAttachments"
                    :can-delete="isMember"
                    :loading="taskAttachmentsLoading"
                    @delete="handleDeleteTaskAttachment"
                  />
                </FileDropZone>
                <AttachmentList
                  v-else
                  :attachments="taskAttachments"
                  :loading="taskAttachmentsLoading"
                />
              </div>

              <!-- Custom fields: its own section, one row per field, edited inline -->
              <div class="overflow-hidden rounded-lg border border-border/60">
                <div class="border-b border-border/60 bg-muted/50 px-4 py-2">
                  <h2 class="text-sm font-semibold">Custom Fields</h2>
                </div>

                <div
                  v-for="field in CUSTOM_FIELDS"
                  :key="field.key"
                  class="group flex items-start gap-4 border-b border-border/60 px-4 py-3 last:border-0"
                >
                  <div class="w-40 shrink-0">
                    <p class="text-sm font-medium leading-tight">{{ field.label }}</p>
                    <p class="mt-0.5 text-xs leading-tight text-muted-foreground">
                      {{ field.hint }}
                    </p>
                  </div>

                  <!-- Editing: long text field with save / cancel -->
                  <template v-if="editingField === field.key">
                    <Input
                      v-model="customDraft"
                      :placeholder="field.placeholder"
                      class="h-8 flex-1"
                      :disabled="savingField"
                      @keyup.enter="saveField(field.key)"
                      @keyup.esc="cancelEditField"
                    />
                    <button
                      type="button"
                      class="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:opacity-50"
                      :disabled="savingField"
                      :aria-label="`Save ${field.label}`"
                      @click="saveField(field.key)"
                    >
                      <Loader2 v-if="savingField" class="size-4 animate-spin" />
                      <Check v-else class="size-4" />
                    </button>
                    <button
                      type="button"
                      class="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:opacity-50"
                      :disabled="savingField"
                      :aria-label="`Cancel editing ${field.label}`"
                      @click="cancelEditField"
                    >
                      <X class="size-4" />
                    </button>
                  </template>

                  <!-- Reading: value (linked when it is a URL) + edit on hover -->
                  <template v-else>
                    <a
                      v-if="customFieldValue(field.key).startsWith('http')"
                      :href="customFieldValue(field.key)"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="min-w-0 flex-1 truncate text-sm text-primary underline underline-offset-2 hover:text-primary/80"
                    >
                      {{ customFieldValue(field.key) }}
                    </a>
                    <span
                      v-else
                      class="min-w-0 flex-1 truncate text-sm"
                      :class="!customFieldValue(field.key) && 'text-muted-foreground'"
                    >
                      {{ customFieldValue(field.key) || "Not set" }}
                    </span>
                    <button
                      v-if="isMember"
                      type="button"
                      class="rounded p-1 text-muted-foreground opacity-0 transition hover:bg-muted hover:text-foreground focus-visible:opacity-100 group-hover:opacity-100"
                      :aria-label="`Edit ${field.label}`"
                      @click="startEditField(field.key)"
                    >
                      <Pencil class="size-3.5" />
                    </button>
                  </template>
                </div>
              </div>

              <!-- Subtasks (one level only, so not shown on a subtask itself) -->
              <div v-if="!isSubtask" class="space-y-3">
                <div class="flex items-center justify-between">
                  <h2 class="text-sm font-semibold text-muted-foreground">
                    Subtasks
                    <span v-if="subtasks.length" class="ml-1 font-normal">({{ subtasks.length }})</span>
                  </h2>
                  <Button
                    v-if="isMember"
                    variant="outline"
                    size="sm"
                    class="h-7 gap-1.5"
                    @click="showCreateSubtask = true"
                  >
                    <Plus class="size-3.5" />
                    Add subtask
                  </Button>
                </div>
                <SubtaskList
                  v-if="subtasks.length"
                  :subtasks="subtasks"
                  :project-key="projectKey"
                  :states="states"
                  :is-member="isMember"
                  @updated="loadSubtasks"
                />
                <p v-else-if="!subtasksLoading" class="text-sm italic text-muted-foreground">
                  No subtasks yet.
                </p>
              </div>

              <Separator />

              <!-- Activity & Comments Combined -->
              <TaskActivityFeed
                :activities="activities"
                :comments="comments"
                :project-key="projectKey"
                :task-num="taskNum"
                :activities-loading="activitiesLoading"
                :comments-loading="commentsLoading"
                :is-member="isMember"
                :members="members"
                @refresh-comments="refreshComments"
                @refresh-activity="listActivity(projectKey, taskNum)"
              />
            </div>

            <!-- Sidebar -->
            <div class="w-full border-border pl-8 md:sticky md:top-24 md:w-64 md:shrink-0 md:self-start md:border-l">
              <div class="divide-y divide-border">
                <!-- State -->
                <div class="flex items-center justify-between py-3">
                  <p class="text-xs text-muted-foreground">State</p>
                  <TaskStateSelector
                    :states="states"
                    :model-value="currentTask.state_id"
                    :disabled="!isMember || updating"
                    compact
                    @update:model-value="handleStateChange"
                  />
                </div>

                <!-- Priority -->
                <div class="flex items-center justify-between py-3">
                  <p class="text-xs text-muted-foreground">Priority</p>
                  <DropdownMenu>
                    <DropdownMenuTrigger as-child>
                      <Button
                        variant="ghost"
                        class="h-auto gap-1.5 px-0 py-0 font-medium hover:bg-transparent"
                        :disabled="!isMember || updating"
                      >
                        <span
                          class="size-3 rounded-full ring-2 ring-offset-1 ring-offset-background"
                          :style="{ backgroundColor: currentPriority.color, '--tw-ring-color': currentPriority.color }"
                        />
                        {{ currentPriority.label }}
                        <ChevronDown class="size-3.5 opacity-50" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" class="w-40">
                      <DropdownMenuItem
                        v-for="p in priorityOptions"
                        :key="p.value"
                        @click="handlePriorityChange(p.value)"
                      >
                        <span
                          class="mr-2 size-2 rounded-full"
                          :style="{ backgroundColor: p.color }"
                        />
                        {{ p.label }}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                <!-- Cycle -->
                <div class="flex items-center justify-between gap-2 py-3">
                  <p class="shrink-0 text-xs text-muted-foreground">Cycle</p>
                  <!-- Sub-task: shows the parent's cycle, read-only -->
                  <span
                    v-if="isSubtask"
                    class="max-w-[9rem] truncate text-sm font-medium"
                    :class="!currentTask.cycle && 'text-muted-foreground'"
                    :title="currentTask.cycle?.title"
                  >
                    {{ currentTask.cycle?.title ?? "None" }}
                  </span>
                  <DropdownMenu v-else>
                    <DropdownMenuTrigger as-child>
                      <Button
                        variant="ghost"
                        class="h-auto gap-1.5 px-0 py-0 font-medium hover:bg-transparent"
                        :class="!currentTask.cycle && 'text-muted-foreground'"
                        :disabled="!isMember || updating"
                      >
                        <Repeat class="size-3.5 shrink-0 opacity-70" />
                        <span class="max-w-[8rem] truncate">{{ currentTask.cycle?.title ?? "Set cycle" }}</span>
                        <ChevronDown class="size-3.5 shrink-0 opacity-50" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" class="max-h-72 w-56 overflow-y-auto">
                      <DropdownMenuItem v-if="currentTask.cycle" @click="setCycle(null)">
                        <X class="mr-2 size-3.5" /> Clear cycle
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        v-for="c in cycleOptions"
                        :key="c.id"
                        @click="setCycle(c.id)"
                      >
                        <Check
                          class="mr-2 size-3.5"
                          :class="currentTask.cycle?.id === c.id ? 'opacity-100' : 'opacity-0'"
                        />
                        <span class="truncate">{{ c.title }}</span>
                      </DropdownMenuItem>
                      <p
                        v-if="cycleOptions.length === 0"
                        class="px-2 py-1.5 text-xs text-muted-foreground"
                      >
                        No cycles in this project
                      </p>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                <!-- Epic -->
                <div class="flex items-center justify-between gap-2 py-3">
                  <p class="shrink-0 text-xs text-muted-foreground">Epic</p>
                  <!-- Sub-task: shows the parent's epic, read-only -->
                  <span
                    v-if="isSubtask"
                    class="max-w-[9rem] truncate text-sm font-medium"
                    :class="!currentTask.module && 'text-muted-foreground'"
                    :title="currentTask.module?.title"
                  >
                    {{ currentTask.module?.title ?? "None" }}
                  </span>
                  <DropdownMenu v-else>
                    <DropdownMenuTrigger as-child>
                      <Button
                        variant="ghost"
                        class="h-auto gap-1.5 px-0 py-0 font-medium hover:bg-transparent"
                        :class="!currentTask.module && 'text-muted-foreground'"
                        :disabled="!isMember || updating"
                      >
                        <Layers class="size-3.5 shrink-0 opacity-70" />
                        <span class="max-w-[8rem] truncate">{{ currentTask.module?.title ?? "Set epic" }}</span>
                        <ChevronDown class="size-3.5 shrink-0 opacity-50" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" class="max-h-72 w-56 overflow-y-auto">
                      <DropdownMenuItem v-if="currentTask.module" @click="setModule(null)">
                        <X class="mr-2 size-3.5" /> Clear epic
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        v-for="m in modules"
                        :key="m.id"
                        @click="setModule(m.id)"
                      >
                        <Check
                          class="mr-2 size-3.5"
                          :class="currentTask.module?.id === m.id ? 'opacity-100' : 'opacity-0'"
                        />
                        <span class="truncate">{{ m.title }}</span>
                      </DropdownMenuItem>
                      <p
                        v-if="modules.length === 0"
                        class="px-2 py-1.5 text-xs text-muted-foreground"
                      >
                        No epics in this project
                      </p>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                <!-- Parent (sub-tasks only) -->
                <div v-if="isSubtask" class="flex items-center justify-between gap-2 py-3">
                  <p class="shrink-0 text-xs text-muted-foreground">Parent</p>
                  <DropdownMenu>
                    <DropdownMenuTrigger as-child>
                      <Button
                        variant="ghost"
                        class="h-auto gap-1.5 px-0 py-0 font-medium hover:bg-transparent"
                        :disabled="!isMember || updating"
                      >
                        <span class="max-w-[9rem] truncate">{{
                          currentTask.parent_task_title ?? `#${currentTask.parent_task_number}`
                        }}</span>
                        <ChevronDown class="size-3.5 shrink-0 opacity-50" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" class="max-h-72 w-64 overflow-y-auto">
                      <DropdownMenuItem
                        v-for="t in parentOptions"
                        :key="t.id"
                        @click="setParent(t.task_number)"
                      >
                        <Check
                          class="mr-2 size-3.5 shrink-0"
                          :class="currentTask.parent_task_id === t.id ? 'opacity-100' : 'opacity-0'"
                        />
                        <span class="mr-2 shrink-0 font-mono text-xs text-muted-foreground">{{ t.task_id }}</span>
                        <span class="truncate">{{ t.title }}</span>
                      </DropdownMenuItem>
                      <p
                        v-if="parentOptions.length === 0"
                        class="px-2 py-1.5 text-xs text-muted-foreground"
                      >
                        No other tasks to parent under
                      </p>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                <!-- Start date -->
                <div
                  class="py-3"
                  :class="currentTask.start_date ? 'space-y-2' : 'flex items-center justify-between gap-2'"
                >
                  <p class="text-xs text-muted-foreground">Start date</p>
                  <Popover v-model:open="startDateOpen">
                    <PopoverTrigger as-child>
                      <Button
                        variant="ghost"
                        class="h-auto gap-1.5 px-0 py-0 font-medium hover:bg-transparent"
                        :class="[
                          currentTask.start_date ? 'w-full justify-start' : '',
                          !currentTask.start_date ? 'text-muted-foreground' : '',
                        ]"
                        :disabled="!isMember || updating"
                      >
                        <CalendarIcon class="size-3.5 opacity-70" />
                        <span>{{ currentTask.start_date ? formatDateTime(currentTask.start_date) : "Set date" }}</span>
                        <ChevronDown class="size-3.5 opacity-50" />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent class="w-auto p-0" align="end">
                      <Calendar
                        v-model="startDateDraft"
                        layout="month-and-year"
                        :max-value="startMaxValue"
                      />
                      <div class="flex items-center gap-2 border-t px-3 py-2">
                        <CalendarIcon class="size-3.5 text-muted-foreground" />
                        <Input
                          v-model="startTimeDraft"
                          type="time"
                          class="h-8 flex-1 text-sm"
                        />
                        <Button
                          size="sm"
                          :disabled="!startDateDraft || updating"
                          @click="saveStartDate"
                        >
                          Save
                        </Button>
                      </div>
                      <div v-if="currentTask.start_date" class="border-t px-3 py-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          class="w-full"
                          :disabled="updating"
                          @click="clearStartDate"
                        >
                          <X class="mr-1.5 size-3.5" />
                          Clear
                        </Button>
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>

                <!-- Due date -->
                <div
                  class="py-3"
                  :class="currentTask.due_date ? 'space-y-2' : 'flex items-center justify-between gap-2'"
                >
                  <p class="text-xs text-muted-foreground">Due date</p>
                  <Popover v-model:open="dueDateOpen">
                    <PopoverTrigger as-child>
                      <Button
                        variant="ghost"
                        class="h-auto gap-1.5 px-0 py-0 font-medium hover:bg-transparent"
                        :class="[
                          currentTask.due_date ? 'w-full justify-start' : '',
                          !currentTask.due_date ? 'text-muted-foreground' : '',
                        ]"
                        :disabled="!isMember || updating"
                      >
                        <CalendarIcon class="size-3.5 opacity-70" />
                        <span>{{ currentTask.due_date ? formatDateTime(currentTask.due_date) : "Set date" }}</span>
                        <ChevronDown class="size-3.5 opacity-50" />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent class="w-auto p-0" align="end">
                      <Calendar
                        v-model="dueDateDraft"
                        layout="month-and-year"
                        :min-value="dueMinValue"
                      />
                      <div class="flex items-center gap-2 border-t px-3 py-2">
                        <CalendarIcon class="size-3.5 text-muted-foreground" />
                        <Input
                          v-model="dueTimeDraft"
                          type="time"
                          class="h-8 flex-1 text-sm"
                        />
                        <Button
                          size="sm"
                          :disabled="!dueDateDraft || updating"
                          @click="saveDueDate"
                        >
                          Save
                        </Button>
                      </div>
                      <div v-if="currentTask.due_date" class="border-t px-3 py-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          class="w-full"
                          :disabled="updating"
                          @click="clearDueDate"
                        >
                          <X class="mr-1.5 size-3.5" />
                          Clear
                        </Button>
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>

                <!-- Assignees -->
                <div class="py-3">
                  <TaskAssignees
                    :assignees="currentTask.assignees || []"
                    :project-key="projectKey"
                    :task-num="taskNum"
                    :members="members"
                    :is-member="isMember"
                    @refresh="refreshTask"
                  />
                </div>

                <!-- Labels -->
                <div class="py-3">
                  <TaskLabels
                    :task-labels="currentTask.labels || []"
                    :project-key="projectKey"
                    :task-num="taskNum"
                    :project-labels="projectLabels"
                    :is-member="isMember"
                    @refresh="refreshTask"
                  />
                </div>

                <!-- Created By -->
                <div class="py-3">
                  <p class="mb-2 text-xs text-muted-foreground">Created by</p>
                  <NuxtLink :to="`/profile/${currentTask.created_by}`" class="flex items-center gap-1.5 rounded-md border bg-muted/50 py-1 pl-1 pr-2.5 w-fit hover:bg-muted transition-colors">
                    <Avatar class="size-6">
                      <AvatarImage v-if="currentTask.creator_avatar_url" :src="currentTask.creator_avatar_url" />
                      <AvatarFallback class="text-xs" :seed="currentTask.created_by">
                        {{ currentTask.creator_first_name?.[0] }}{{ currentTask.creator_last_name?.[0] }}
                      </AvatarFallback>
                    </Avatar>
                    <span class="text-sm">
                      {{ currentTask.creator_first_name }} {{ currentTask.creator_last_name }}
                    </span>
                  </NuxtLink>
                </div>

                <!-- Dates -->
                <div class="space-y-1.5 py-3 text-xs text-muted-foreground">
                  <div class="flex items-center gap-2">
                    <CalendarIcon class="size-3.5" />
                    <span>Created {{ formatDate(currentTask.created_at) }}</span>
                  </div>
                  <div class="flex items-center gap-2">
                    <Clock class="size-3.5" />
                    <span>Updated {{ formatDate(currentTask.updated_at) }}</span>
                  </div>
                </div>
              </div>

              <!-- Move (subtasks move with their parent, so not offered here) -->
              <div v-if="isMember && !isSubtask" class="mt-2 border-t border-border pt-3">
                <Button
                  variant="ghost"
                  size="sm"
                  class="w-full justify-start gap-2"
                  @click="showMoveDialog = true"
                >
                  <FolderInput class="size-3.5" />
                  Move to project
                </Button>
              </div>

              <!-- Delete -->
              <div v-if="canDelete" class="mt-2 border-t border-border pt-3">
                <Button
                  variant="ghost"
                  size="sm"
                  class="w-full justify-start gap-2 text-destructive hover:bg-destructive/10 hover:text-destructive"
                  @click="showDeleteConfirm = true"
                >
                  <Trash2 class="size-3.5" />
                  Delete task
                </Button>
              </div>
            </div>
          </div>
        </template>

        <!-- Move to project -->
        <MoveTaskDialog
          v-model:open="showMoveDialog"
          :project-key="projectKey"
          :task-num="taskNum"
          @moved="handleTaskMoved"
        />

        <!-- Create subtask -->
        <CreateTaskDialog
          v-model:open="showCreateSubtask"
          :project-key="projectKey"
          :states="states"
          :labels="projectLabels"
          :members="members"
          :parent-task-number="taskNum"
          @created="onSubtaskCreated"
        />

        <!-- Delete confirmation -->
        <Dialog v-model:open="showDeleteConfirm">
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Delete Task</DialogTitle>
              <DialogDescription>
                Are you sure you want to delete
                <strong>{{ currentTask?.task_id }}</strong>?
                This action cannot be undone.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button
                variant="outline"
                :disabled="deleting"
                @click="showDeleteConfirm = false"
              >
                Cancel
              </Button>
              <Button
                variant="destructive"
                :disabled="deleting"
                @click="handleDelete"
              >
                <Loader2 v-if="deleting" class="mr-2 size-4 animate-spin" />
                Delete
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </main>

  </div>
</template>
