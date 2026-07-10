<script setup lang="ts">
import { Trash2, Loader2, ChevronLeft, ChevronRight, RotateCcw, Building2 } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { DeletedProject } from "~/composables/useAdmin";

definePageMeta({
  middleware: ["admin"],
});

useSeoMeta({ title: "Deleted Projects" });

const { listDeletedProjects, restoreProject } = useAdmin();

const projects = ref<DeletedProject[]>([]);
const loading = ref(true);
const page = ref(1);
const perPage = ref(20);
const total = ref(0);
const totalPages = ref(0);
const error = ref<string | null>(null);

// Restore confirmation dialog state
const showRestoreDialog = ref(false);
const restoreLoading = ref(false);
const projectToRestore = ref<DeletedProject | null>(null);

async function fetchProjects() {
  loading.value = true;
  error.value = null;
  const result = await listDeletedProjects(page.value, perPage.value);
  if (result.success && result.data) {
    projects.value = result.data.projects || [];
    total.value = result.data.total;
    totalPages.value = result.data.total_pages;
  } else {
    error.value = result.error || "Failed to fetch deleted projects";
  }
  loading.value = false;
}

function confirmRestore(project: DeletedProject) {
  projectToRestore.value = project;
  showRestoreDialog.value = true;
}

async function handleRestore() {
  if (!projectToRestore.value) return;

  restoreLoading.value = true;
  const result = await restoreProject(projectToRestore.value.id);
  restoreLoading.value = false;

  if (result.success) {
    toast.success(`"${projectToRestore.value.name}" restored`);
    showRestoreDialog.value = false;
    projectToRestore.value = null;
    // If the last item on a page was restored, step back a page.
    if (projects.value.length === 1 && page.value > 1) page.value--;
    await fetchProjects();
  } else {
    toast.error(result.error || "Failed to restore project");
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function prevPage() {
  if (page.value > 1) {
    page.value--;
    fetchProjects();
  }
}

function nextPage() {
  if (page.value < totalPages.value) {
    page.value++;
    fetchProjects();
  }
}

onMounted(() => {
  fetchProjects();
});
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <Navbar />

    <main id="main-content" class="flex-1">
      <div class="mx-auto max-w-6xl px-6 py-12">
        <div class="mb-8">
          <h1 class="flex items-center gap-2 text-3xl font-bold tracking-tight">
            <Trash2 class="size-8" />
            Deleted Projects
          </h1>
          <p class="mt-2 text-muted-foreground">
            Soft-deleted projects across all workspaces. Restoring a project
            makes it and all of its data accessible again.
          </p>
        </div>

        <div v-if="error" role="alert" class="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {{ error }}
        </div>

        <Card class="overflow-hidden py-0">
          <CardContent class="p-0">
            <Table>
              <TableHeader>
                <TableRow class="border-b bg-muted/50 hover:bg-muted/50">
                  <TableHead class="h-11 px-4 text-xs font-medium uppercase tracking-wide text-muted-foreground">Project</TableHead>
                  <TableHead class="h-11 px-4 text-xs font-medium uppercase tracking-wide text-muted-foreground">Workspace</TableHead>
                  <TableHead class="h-11 px-4 text-xs font-medium uppercase tracking-wide text-muted-foreground">Created By</TableHead>
                  <TableHead class="h-11 px-4 text-xs font-medium uppercase tracking-wide text-muted-foreground">Deleted</TableHead>
                  <TableHead class="h-11 w-[120px] px-4 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-if="loading">
                  <TableCell colspan="5" class="py-8 text-center">
                    <Loader2 class="mx-auto size-6 animate-spin" />
                  </TableCell>
                </TableRow>
                <TableRow v-else-if="projects.length === 0">
                  <TableCell colspan="5" class="py-12 text-center text-muted-foreground">
                    No deleted projects
                  </TableCell>
                </TableRow>
                <TableRow v-for="project in projects" v-else :key="project.id">
                  <TableCell class="px-4 py-3">
                    <div>
                      <p class="font-medium">{{ project.name }}</p>
                      <p class="font-mono text-xs text-muted-foreground">
                        {{ project.project_key }}
                      </p>
                    </div>
                  </TableCell>
                  <TableCell class="px-4 py-3">
                    <span class="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
                      <Building2 class="size-3.5 shrink-0" />
                      {{ project.workspace_name }}
                    </span>
                  </TableCell>
                  <TableCell class="px-4 py-3">
                    <p class="text-sm">{{ project.creator_name }}</p>
                    <p class="text-xs text-muted-foreground">@{{ project.creator_username }}</p>
                  </TableCell>
                  <TableCell class="px-4 py-3 text-muted-foreground">
                    {{ formatDate(project.deleted_at) }}
                  </TableCell>
                  <TableCell class="px-4 py-3 text-right">
                    <Button variant="outline" size="sm" @click="confirmRestore(project)">
                      <RotateCcw class="mr-1.5 size-3.5" />
                      Restore
                    </Button>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
          <CardFooter class="flex items-center justify-between border-t px-6 py-4">
            <p class="text-sm text-muted-foreground">
              Showing {{ projects.length }} of {{ total }} deleted project{{ total === 1 ? "" : "s" }}
            </p>
            <div class="flex items-center gap-2">
              <Button variant="outline" size="sm" aria-label="Previous page" :disabled="page === 1" @click="prevPage">
                <ChevronLeft class="size-4" />
              </Button>
              <span class="text-sm">Page {{ page }} of {{ totalPages || 1 }}</span>
              <Button variant="outline" size="sm" aria-label="Next page" :disabled="page >= totalPages" @click="nextPage">
                <ChevronRight class="size-4" />
              </Button>
            </div>
          </CardFooter>
        </Card>

        <!-- Restore Confirmation Dialog -->
        <Dialog v-model:open="showRestoreDialog">
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Restore Project</DialogTitle>
              <DialogDescription>
                Restore <strong>{{ projectToRestore?.name }}</strong> ({{ projectToRestore?.project_key }})?
                The project and all of its tasks, comments, and activity will
                become accessible again.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" :disabled="restoreLoading" @click="showRestoreDialog = false">
                Cancel
              </Button>
              <Button :disabled="restoreLoading" @click="handleRestore">
                <Loader2 v-if="restoreLoading" class="mr-2 size-4 animate-spin" />
                <RotateCcw v-else class="mr-2 size-4" />
                Restore
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </main>
  </div>
</template>
