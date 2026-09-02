<script setup lang="ts">
import { ChevronLeft, Inbox } from "lucide-vue-next";

definePageMeta({ middleware: ["auth"] });

const route = useRoute();
const projectKey = computed(() => route.params.key as string);

const { currentProject, getProject } = useProjects();
const { listAllModules, listTasksInNoModule, addTasksToModule } = useModules();

const isAdmin = computed(() => currentProject.value?.role === "admin");

const targets = ref<{ id: string; title: string }[]>([]);
async function loadTargets() {
  const r = await listAllModules(projectKey.value);
  if (r.success && r.data) {
    targets.value = r.data.map((m) => ({ id: m.id, title: m.title }));
  }
}

function loadBacklogTasks(search: string, limit: number) {
  return listTasksInNoModule(projectKey.value, search, limit);
}
function addToModule(moduleId: string, ids: string[]) {
  return addTasksToModule(projectKey.value, moduleId, ids);
}

useHead({ title: "Tasks Without an Epic" });

onMounted(async () => {
  if (!currentProject.value || currentProject.value.project_key !== projectKey.value) {
    await getProject(projectKey.value);
  }
  await loadTargets();
});
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <Navbar />
    <main id="main-content" class="flex-1">
      <div class="mx-auto max-w-6xl px-6 py-8">
        <nav class="mb-6 flex items-center gap-2 text-sm text-muted-foreground">
          <ChevronLeft class="size-4" />
          <NuxtLink to="/projects" class="hover:text-foreground">Projects</NuxtLink>
          <span>/</span>
          <NuxtLink :to="`/projects/${projectKey}`" class="hover:text-foreground">
            {{ currentProject?.name ?? projectKey }}
          </NuxtLink>
          <span>/</span>
          <NuxtLink :to="`/projects/${projectKey}?tab=modules`" class="hover:text-foreground">
            Modules
          </NuxtLink>
          <span>/</span>
          <span class="font-semibold text-amber-600 dark:text-amber-500">
            Tasks Without an Epic
          </span>
        </nav>

        <header class="mb-6 flex items-center gap-3">
          <div class="flex size-10 items-center justify-center rounded-lg bg-muted">
            <Inbox class="size-5 text-amber-600 dark:text-amber-500" />
          </div>
          <div>
            <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">
              Tasks Without an Epic
            </h1>
            <p class="text-sm text-muted-foreground">
              Top-level tasks not in any epic yet. Select and add them to an epic.
            </p>
          </div>
        </header>

        <BacklogView
          :project-key="projectKey"
          target-noun="epic"
          :targets="targets"
          :can-add="isAdmin"
          show-cycle
          :load-tasks="loadBacklogTasks"
          :add-tasks="addToModule"
          @added="loadTargets"
        />
      </div>
    </main>
  </div>
</template>
