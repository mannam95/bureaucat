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

const { listModules, addTasksToModule, removeTaskFromModule } = useModules();

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

async function loadModules() {
  loadingModules.value = true;
  const result = await listModules(props.projectKey, 1, 100);
  if (result.success && result.data) {
    available.value = result.data.modules || [];
  } else {
    toast.error(result.error || "Failed to load modules");
  }
  loadingModules.value = false;
}

watch(open, (isOpen) => {
  if (isOpen && !available.value.length) loadModules();
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

      <DropdownMenu v-if="canEdit" v-model:open="open">
        <DropdownMenuTrigger as-child>
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
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-56">
          <div v-if="loadingModules" class="flex items-center justify-center py-3">
            <Loader2 class="size-4 animate-spin text-muted-foreground" />
          </div>
          <template v-else>
            <p
              v-if="!available.length"
              class="px-2 py-3 text-center text-xs text-muted-foreground"
            >
              No modules in this project
            </p>
            <DropdownMenuItem
              v-for="module in available"
              :key="module.id"
              class="gap-2"
              @select.prevent="toggleModule(module)"
            >
              <Check
                class="size-3.5 shrink-0"
                :class="selectedIds.has(module.id) ? 'opacity-100' : 'opacity-0'"
              />
              <span class="min-w-0 flex-1 truncate">{{ module.title }}</span>
            </DropdownMenuItem>
          </template>
        </DropdownMenuContent>
      </DropdownMenu>

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
