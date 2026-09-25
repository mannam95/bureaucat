<script setup lang="ts">
import { Plus, FolderKanban, Search, Loader2, ChevronLeft, ChevronRight, Link, Check, Menu } from "lucide-vue-next";
import { toast } from "vue-sonner";

definePageMeta({
  middleware: ["auth"],
});

useSeoMeta({ title: "Projects" });

const copied = ref(false);
function copyLink() {
  navigator.clipboard.writeText(window.location.href);
  copied.value = true;
  toast.success("Link copied");
  setTimeout(() => { copied.value = false; }, 2000);
}

const { projects, loading, listProjects, reorderProjects, total, page, totalPages } = useProjects();
const { user } = useAuth();
const { currentWorkspace } = useWorkspaces();
// Shares the dashboard's "all workspaces" preference (same localStorage key).
const { showAllWorkspaces } = useDashboardScope();

const showCreateDialog = ref(false);
const searchQuery = ref("");
const perPage = 12;

let debounceTimer: ReturnType<typeof setTimeout> | null = null;

function fetchProjects(p = 1) {
  // Scope to the active workspace unless the user opted into all workspaces.
  listProjects(p, perPage, searchQuery.value, !showAllWorkspaces.value);
}

watch(searchQuery, () => {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    fetchProjects(1);
  }, 300);
});

// Reload the list when the active workspace or scope toggle changes.
watch([currentWorkspace, showAllWorkspaces], () => {
  fetchProjects(1);
});

async function handleCreated() {
  fetchProjects(1);
}

function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return;
  fetchProjects(p);
}

onMounted(() => {
  fetchProjects(1);
});

// ---- Drag-and-drop ordering (site admins only) ----
// One global order shared by everyone; the dashboard renders the same order
// but only this page can change it. Disabled while searching, since the
// positions are global and a filtered subset would scatter them.
const canReorder = computed(
  () => user.value?.user_type === "admin" && !searchQuery.value.trim()
);
const dragIndex = ref<number | null>(null);
const dragOverIndex = ref<number | null>(null);
// The HTML5 draggable flag is armed from the grip handle only, so the card's
// link stays clickable and never starts a drag on its own.
const armedIndex = ref<number | null>(null);

function onDragStart(idx: number, e: DragEvent) {
  if (!canReorder.value || armedIndex.value !== idx) {
    e.preventDefault();
    return;
  }
  dragIndex.value = idx;
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = "move";
    // Firefox refuses to start a drag with no data attached.
    e.dataTransfer.setData("text/plain", String(idx));
  }
}

function onDragEnd() {
  dragIndex.value = null;
  dragOverIndex.value = null;
  armedIndex.value = null;
}

async function onDrop(targetIdx: number) {
  const from = dragIndex.value;
  onDragEnd();
  if (from === null || from === targetIdx) return;
  const next = [...projects.value];
  const moved = next.splice(from, 1)[0]!;
  next.splice(targetIdx, 0, moved);
  // Positions are global: page 1 gets 1..12, page 2 continues after it.
  const items = next.map((p, i) => ({
    id: p.id,
    position: (page.value - 1) * perPage + i + 1,
  }));
  const res = await reorderProjects(items, next);
  if (!res.success) toast.error(res.error || "Failed to reorder projects");
}
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <Navbar />

    <main id="main-content" class="flex-1">
      <div class="mx-auto max-w-6xl px-6 py-8">
        <nav class="mb-4 flex items-center gap-2 text-sm text-muted-foreground">
          <ChevronLeft class="size-4" />
          <span class="font-semibold text-amber-600 dark:text-amber-500">Projects</span>
          <button
            aria-label="Copy link"
            class="ml-1 rounded-md p-1 text-muted-foreground/50 hover:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 outline-none"
            @click="copyLink"
          >
            <Check v-if="copied" class="size-3.5 text-emerald-500" />
            <Link v-else class="size-3.5" />
          </button>
        </nav>

        <div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 class="flex items-center gap-3 text-3xl font-bold tracking-tight">
              <FolderKanban class="size-8" />
              Projects
            </h1>
            <p class="mt-2 text-muted-foreground">
              Manage your projects and track approvals
            </p>
          </div>
          <Button @click="showCreateDialog = true">
            <Plus class="mr-2 size-4" />
            Create Project
          </Button>
        </div>

        <!-- Search + workspace scope -->
        <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div class="relative w-full sm:max-w-sm">
            <Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              v-model="searchQuery"
              placeholder="Search projects..."
              class="pl-9"
            />
          </div>
          <div class="flex items-center gap-2">
            <Switch
              id="all-workspaces"
              :checked="showAllWorkspaces"
              aria-label="Show projects from all workspaces"
              @update:checked="showAllWorkspaces = $event"
            />
            <Label for="all-workspaces" class="cursor-pointer text-xs text-muted-foreground">
              {{ showAllWorkspaces ? "Showing all workspaces" : "Current workspace only" }}
            </Label>
          </div>
        </div>

        <!-- Loading -->
        <div v-if="loading" class="flex items-center justify-center py-12">
          <Loader2 class="size-8 animate-spin text-muted-foreground" />
        </div>

        <!-- Empty state (no projects at all, no search) -->
        <div
          v-else-if="projects.length === 0 && !searchQuery"
          class="flex flex-col items-center justify-center rounded-lg border border-dashed py-16"
        >
          <div class="flex size-16 items-center justify-center rounded-full bg-muted">
            <FolderKanban class="size-8 text-muted-foreground" />
          </div>
          <h3 class="mt-4 text-lg font-semibold">No projects yet</h3>
          <p class="mt-2 text-sm text-muted-foreground">
            Create your first project to get started
          </p>
          <Button class="mt-4" @click="showCreateDialog = true">
            <Plus class="mr-2 size-4" />
            Create Project
          </Button>
        </div>

        <!-- No search results -->
        <div
          v-else-if="projects.length === 0 && searchQuery"
          class="flex flex-col items-center justify-center rounded-lg border border-dashed py-16"
        >
          <Search class="size-8 text-muted-foreground" />
          <h3 class="mt-4 text-lg font-semibold">No projects found</h3>
          <p class="mt-2 text-sm text-muted-foreground">
            Try a different search term
          </p>
        </div>

        <!-- Projects grid -->
        <template v-else>
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div
              v-for="(project, idx) in projects"
              :key="project.id"
              class="group/card relative h-full"
              :class="[
                dragOverIndex === idx && dragIndex !== null && dragIndex !== idx
                  ? 'rounded-xl ring-2 ring-amber-500/50'
                  : '',
                dragIndex === idx ? 'opacity-50' : '',
              ]"
              :draggable="canReorder && armedIndex === idx ? true : undefined"
              @dragstart="onDragStart(idx, $event)"
              @dragover.prevent="canReorder && dragIndex !== null && (dragOverIndex = idx)"
              @drop.prevent="canReorder && onDrop(idx)"
              @dragend="onDragEnd"
            >
              <ProjectCard
                class="block h-full"
                :project="project"
                :show-workspace="showAllWorkspaces"
              />
              <button
                v-if="canReorder"
                type="button"
                aria-label="Drag to reorder"
                title="Drag to reorder"
                class="absolute bottom-2 right-2 cursor-grab rounded-md p-1.5 text-muted-foreground/50 opacity-0 transition-opacity hover:bg-muted hover:text-foreground focus-visible:opacity-100 active:cursor-grabbing group-hover/card:opacity-100"
                @mousedown="armedIndex = idx"
                @mouseup="armedIndex = null"
              >
                <Menu class="size-4" />
              </button>
            </div>
          </div>

          <!-- Pagination -->
          <div
            v-if="totalPages > 1"
            class="mt-8 flex items-center justify-between"
          >
            <p class="text-sm text-muted-foreground">
              {{ total }} project{{ total === 1 ? '' : 's' }}
            </p>
            <div class="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                :disabled="page <= 1"
                @click="goToPage(page - 1)"
              >
                <ChevronLeft class="mr-1 size-4" />
                Prev
              </Button>
              <span class="text-sm text-muted-foreground">
                Page {{ page }} of {{ totalPages }}
              </span>
              <Button
                variant="outline"
                size="sm"
                :disabled="page >= totalPages"
                @click="goToPage(page + 1)"
              >
                Next
                <ChevronRight class="ml-1 size-4" />
              </Button>
            </div>
          </div>
        </template>

        <CreateProjectDialog
          v-model:open="showCreateDialog"
          @created="handleCreated"
        />
      </div>
    </main>
  </div>
</template>
