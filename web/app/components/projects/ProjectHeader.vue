<script setup lang="ts">
import { FolderKanban, Users, ChevronLeft, Link, Check, ChevronDown } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { Project } from "~/types";

const props = defineProps<{
  project: Project;
  memberCount?: number;
}>();

const copied = ref(false);
function copyLink() {
  navigator.clipboard.writeText(window.location.href);
  copied.value = true;
  toast.success("Link copied");
  setTimeout(() => { copied.value = false; }, 2000);
}

// Description collapses to one line; the expander only appears when there is
// plausibly more to read than the collapsed line shows.
const descExpanded = ref(false);
const showReadMore = computed(() => (props.project.description?.length ?? 0) > 140);
</script>

<template>
  <!-- Header card: warm amber wash with soft decorative blobs (design1). -->
  <div
    class="relative overflow-hidden rounded-xl border border-amber-500/15 bg-gradient-to-br from-amber-500/10 via-amber-500/5 to-transparent p-5 dark:border-amber-500/10 dark:from-amber-500/10 dark:via-amber-500/5"
  >
    <!-- Decorative blobs -->
    <div class="pointer-events-none absolute -right-10 -top-14 size-44 rounded-full bg-amber-500/10 blur-sm" aria-hidden="true" />
    <div class="pointer-events-none absolute -bottom-16 -left-8 size-36 rounded-full bg-amber-500/5 blur-sm" aria-hidden="true" />

    <div class="relative space-y-4">
      <nav class="flex items-center gap-2 text-sm text-muted-foreground">
        <ChevronLeft class="size-4" />
        <NuxtLink to="/projects" class="hover:text-foreground">
          Projects
        </NuxtLink>
        <span>/</span>
        <NuxtLink
          :to="`/projects/${project.project_key}`"
          class="font-semibold text-amber-600 hover:text-amber-700 dark:text-amber-500 dark:hover:text-amber-400"
        >
          {{ project.project_key }}
        </NuxtLink>
        <button
          aria-label="Copy link"
          class="ml-1 rounded-md p-1 text-muted-foreground/50 hover:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 outline-none"
          @click="copyLink"
        >
          <Check v-if="copied" class="size-3.5 text-emerald-500" />
          <Link v-else class="size-3.5" />
        </button>
      </nav>

      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-center gap-4">
          <div class="flex size-14 shrink-0 items-center justify-center rounded-xl border bg-background/60">
            <FolderKanban class="size-7 text-muted-foreground" />
          </div>
          <h1 class="text-2xl font-bold tracking-tight">{{ project.name }}</h1>
        </div>

        <div class="flex items-center gap-3">
          <!-- Members pill: clicks through to the Members tab. -->
          <NuxtLink
            v-if="memberCount"
            :to="`/projects/${project.project_key}?tab=members`"
            class="flex items-center gap-2 rounded-lg border bg-background/60 px-3 py-1.5 text-sm transition-colors hover:bg-background"
          >
            <Users class="size-4 text-muted-foreground" />
            <span>{{ memberCount }} member{{ memberCount === 1 ? "" : "s" }}</span>
          </NuxtLink>
          <!-- The current viewer's role in this project. -->
          <Badge variant="secondary" class="capitalize">{{ project.role }}</Badge>
        </div>
      </div>

      <!-- Description bar: collapsed to one line, expandable. -->
      <div
        v-if="project.description"
        class="flex items-start gap-3 rounded-lg border bg-background/60 px-4 py-3"
      >
        <p
          class="min-w-0 flex-1 text-sm text-muted-foreground"
          :class="descExpanded ? 'whitespace-pre-line' : 'truncate'"
        >
          {{ project.description }}
        </p>
        <button
          v-if="showReadMore"
          type="button"
          class="flex shrink-0 items-center gap-1 border-l pl-3 text-sm font-medium text-foreground/80 hover:text-foreground"
          @click="descExpanded = !descExpanded"
        >
          {{ descExpanded ? "Read less" : "Read more" }}
          <ChevronDown class="size-4 transition-transform" :class="descExpanded ? 'rotate-180' : ''" />
        </button>
      </div>
    </div>
  </div>
</template>
