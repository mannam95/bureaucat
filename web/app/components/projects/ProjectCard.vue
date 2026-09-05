<script setup lang="ts">
import { FolderKanban, Building2, Shield, ChevronRight } from "lucide-vue-next";
import type { Project } from "~/types";

const props = defineProps<{
  project: Project;
  // When true, show which workspace the project belongs to (used when the
  // list spans all workspaces).
  showWorkspace?: boolean;
  // Compact horizontal card for dense rows (e.g. the dashboard projects strip).
  compact?: boolean;
}>();

const { workspaces } = useWorkspaces();

const workspaceName = computed(
  () => workspaces.value.find((w) => w.id === props.project.workspace_id)?.name ?? ""
);

const isAdmin = computed(() => props.project.role === "admin");

const roleBadgeVariant = computed(() => {
  switch (props.project.role) {
    case "admin":
      return "default";
    case "member":
      return "secondary";
    default:
      return "outline";
  }
});
</script>

<template>
  <!-- Compact horizontal card: icon | name / key / workspace | chevron. The
       role is shown only for admins, as a small shield (hover reveals "Admin"). -->
  <NuxtLink v-if="compact" :to="`/projects/${project.project_key}`">
    <div
      class="group flex h-full cursor-pointer items-center gap-3 rounded-xl border border-border/50 bg-background/50 p-3 shadow-sm transition-all hover:border-amber-500/30 hover:shadow-md hover:shadow-amber-500/5"
    >
      <div
        class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted transition-colors group-hover:bg-amber-500/10"
      >
        <FolderKanban
          class="size-4.5 text-muted-foreground transition-colors group-hover:text-amber-600 dark:group-hover:text-amber-500"
        />
      </div>

      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-1.5">
          <span class="truncate text-sm font-semibold">{{ project.name }}</span>
          <span
            v-if="isAdmin"
            title="Admin"
            aria-label="You are an admin of this project"
            class="inline-flex shrink-0 text-muted-foreground"
          >
            <Shield class="size-3.5" />
          </span>
        </div>
        <span
          class="font-mono text-xs text-muted-foreground transition-colors group-hover:text-amber-600 dark:group-hover:text-amber-500"
        >
          {{ project.project_key }}
        </span>
        <div
          v-if="showWorkspace && workspaceName"
          class="mt-1 inline-flex w-fit items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground"
        >
          <Building2 class="size-3 shrink-0" />
          {{ workspaceName }}
        </div>
      </div>

      <ChevronRight
        class="size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5"
      />
    </div>
  </NuxtLink>

  <!-- Full card (default). -->
  <NuxtLink v-else :to="`/projects/${project.project_key}`">
    <Card
      class="group h-full cursor-pointer border-border/50 bg-background/50 transition-all hover:border-amber-500/30 hover:shadow-lg hover:shadow-amber-500/5"
    >
      <CardHeader class="pb-3">
        <div class="flex items-start justify-between">
          <div
            class="flex size-10 items-center justify-center rounded-lg bg-muted transition-colors group-hover:bg-amber-500/10"
          >
            <FolderKanban
              class="size-5 text-muted-foreground transition-colors group-hover:text-amber-600 dark:group-hover:text-amber-500"
            />
          </div>
          <Badge :variant="roleBadgeVariant" class="text-xs capitalize">
            {{ project.role }}
          </Badge>
        </div>
        <CardTitle class="mt-3 text-base font-semibold">{{ project.name }}</CardTitle>
        <span
          class="font-mono text-xs text-muted-foreground transition-colors group-hover:text-amber-600 dark:group-hover:text-amber-500"
        >
          {{ project.project_key }}
        </span>
        <div
          v-if="showWorkspace && workspaceName"
          class="mt-1 inline-flex w-fit items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-xs text-muted-foreground"
        >
          <Building2 class="size-3 shrink-0" />
          {{ workspaceName }}
        </div>
      </CardHeader>
      <CardContent class="pt-0">
        <p
          v-if="project.description"
          class="line-clamp-2 text-sm leading-relaxed text-muted-foreground"
        >
          {{ project.description }}
        </p>
        <p v-else class="text-sm italic text-muted-foreground/50">No description</p>
      </CardContent>
    </Card>
  </NuxtLink>
</template>
