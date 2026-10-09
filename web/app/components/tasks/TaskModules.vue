<script setup lang="ts">
import { ChevronDown, Loader2, ArrowUpRight, Check } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { Module, TaskModule } from "~/types";

const props = withDefaults(
  defineProps<{
    projectKey: string;
    taskId: string;
    modules: TaskModule[];
    canEdit: boolean;
    dense?: boolean;
  }>(),
  { dense: false }
);

const emit = defineEmits<{
  refresh: [];
}>();

const { searchModules, addTasksToModule, removeTaskFromModule } = useModules();

const open = ref(false);
const available = ref<Module[]>([]);
const loadingModules = ref(false);
const updating = ref(false);

const selectedIds = computed(() => new Set(props.modules.map((m) => m.id)));

// One module reads as its title; several collapse to a count.
const label = computed(() => {
  if (!props.modules.length) return "None";
  if (props.modules.length === 1) return props.modules[0]!.title;
  return `${props.modules.length} modules`;
});

// Server-searched and capped: the picker never downloads a whole project's
// module list, and typing narrows the results on the server.
let searchDebounce: ReturnType<typeof setTimeout> | null = null;

async function loadModules(search = "") {
  loadingModules.value = true;
  const result = await searchModules(props.projectKey, search, 50);
  if (result.success && result.data) {
    available.value = result.data;
  } else {
    toast.error(result.error || "Failed to load modules");
  }
  loadingModules.value = false;
}

function onSearchChange(q: string) {
  if (searchDebounce) clearTimeout(searchDebounce);
  searchDebounce = setTimeout(() => loadModules(q), 300);
}

watch(open, (isOpen) => {
  if (isOpen) loadModules();
});

async function toggleModule(module: Module) {
  updating.value = true;
  const isMember = selectedIds.value.has(module.id);
  const result = isMember
    ? await removeTaskFromModule(props.projectKey, module.id, props.taskId)
    : await addTasksToModule(props.projectKey, module.id, [props.taskId]);
  updating.value = false;

  if (result.success) {
    toast.success(isMember ? `Removed from ${module.title}` : `Added to ${module.title}`);
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to update modules");
  }
}
</script>

<template>
  <div class="flex items-start justify-between gap-2">
    <p class="shrink-0 pt-0.5 text-xs text-muted-foreground">Modules</p>

    <div class="flex min-w-0 items-center gap-1">
      <NuxtLink
        v-if="canEdit && modules.length === 1"
        :to="`/projects/${projectKey}/modules/${modules[0]!.id}`"
        aria-label="Open module"
        class="shrink-0 text-muted-foreground/60 hover:text-foreground"
      >
        <ArrowUpRight class="size-3.5" />
      </NuxtLink>

      <SearchableSelect
        v-if="canEdit"
        v-model:open="open"
        :items="available"
        :get-search-text="(m) => m.title"
        :get-key="(m) => m.id"
        server-filtered
        :loading="loadingModules"
        :close-on-select="false"
        placeholder="Search modules…"
        empty-text="No modules found"
        align="end"
        @search-change="onSearchChange"
        @select="toggleModule"
      >
        <template #trigger>
          <Button
            variant="ghost"
            class="h-auto min-w-0 shrink gap-1.5 px-0 py-0 font-medium hover:bg-transparent has-[>svg]:pl-0"
            :class="[modules.length ? '' : 'text-muted-foreground', dense ? 'text-xs' : '']"
            :disabled="updating"
          >
            <Loader2 v-if="updating" class="size-3.5 animate-spin" />
            <span class="truncate" :title="modules.map((m) => m.title).join(', ')">
              {{ label }}
            </span>
            <ChevronDown class="shrink-0 opacity-50" :class="dense ? 'size-3' : 'size-3.5'" />
          </Button>
        </template>
        <template #option="{ item: module }">
          <Check
            class="size-3.5 shrink-0"
            :class="selectedIds.has(module.id) ? 'opacity-100' : 'opacity-0'"
          />
          <span class="min-w-0 flex-1 truncate">{{ module.title }}</span>
        </template>
      </SearchableSelect>

      <!-- Read-only: each module as its own pill link (a task can be in several,
           so collapsing to first + "+N" hid the rest). -->
      <span v-else-if="modules.length" class="flex min-w-0 flex-wrap justify-end gap-1">
        <NuxtLink
          v-for="m in modules"
          :key="m.id"
          :to="`/projects/${projectKey}/modules/${m.id}`"
          class="max-w-full truncate rounded-md border bg-muted/50 px-1.5 py-0.5 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground/30 hover:text-foreground"
          :title="m.title"
        >
          {{ m.title }}
        </NuxtLink>
      </span>
      <span
        v-else-if="!canEdit"
        class="text-muted-foreground"
        :class="dense ? 'text-xs' : 'text-sm'"
      >
        None
      </span>
    </div>
  </div>
</template>
