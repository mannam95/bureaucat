<script setup lang="ts">
import { Info } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { TaskArea, ProjectArea } from "~/types";

const props = defineProps<{
  taskAreas: TaskArea[];
  projectKey: string;
  taskNum: number;
  projectAreas: ProjectArea[];
  isMember: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { addArea, removeArea } = useTasks();

const loading = ref<string | null>(null);

// TaskArea (chips) and ProjectArea (dropdown) share this shape, keyed by id.
type TokenArea = Pick<ProjectArea, "id" | "name" | "color">;

const selectedTokens = computed<TokenArea[]>(() => props.taskAreas);

// Areas not already on the task — the pool offered in the token dropdown.
const availableTokens = computed<TokenArea[]>(() => {
  const usedIds = new Set(props.taskAreas.map((l) => l.id));
  return props.projectAreas.filter((l) => !usedIds.has(l.id));
});

function areaChipStyle(l: TokenArea) {
  return { backgroundColor: l.color + "20", color: l.color };
}

async function handleAdd(areaId: string) {
  loading.value = areaId;
  const result = await addArea(props.projectKey, props.taskNum, areaId);
  loading.value = null;

  if (result.success) {
    toast.success("Area added");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to add area");
  }
}

async function handleRemove(areaId: string) {
  loading.value = areaId;
  const result = await removeArea(props.projectKey, props.taskNum, areaId);
  loading.value = null;

  if (result.success) {
    toast.success("Area removed");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to remove area");
  }
}
</script>

<template>
  <div class="space-y-2">
    <p class="text-xs text-muted-foreground">Areas</p>

    <!-- No areas defined for this project yet: areas are admin-defined, so a
         member sees a hint rather than an empty picker. -->
    <div
      v-if="isMember && projectAreas.length === 0"
      class="flex items-start gap-1.5 text-xs text-muted-foreground"
    >
      <Info class="mt-0.5 size-3.5 shrink-0" />
      <span>No areas yet. An admin can add area categories in project settings.</span>
    </div>

    <!-- Editable: Gmail-style token input (pick from admin-defined areas only) -->
    <TokenSelect
      v-else-if="isMember"
      :selected="selectedTokens"
      :available="availableTokens"
      :get-key="(l) => l.id"
      :get-search-text="(l) => l.name"
      :chip-style="areaChipStyle"
      :chip-class="() => 'pl-2 pr-1 font-medium'"
      :pending-key="loading"
      placeholder="Add areas..."
      empty-text="No matching areas"
      @add="(l) => handleAdd(l.id)"
      @remove="(l) => handleRemove(l.id)"
    >
      <template #chip="{ item: area }">
        <span class="truncate">{{ area.name }}</span>
      </template>
      <template #option="{ item: area }">
        <div
          class="size-3 shrink-0 rounded-full"
          :style="{ backgroundColor: area.color }"
        />
        {{ area.name }}
      </template>
    </TokenSelect>

    <!-- Read-only view -->
    <div v-else class="flex flex-wrap items-center gap-2">
      <span
        v-for="area in taskAreas"
        :key="area.id"
        class="rounded-md px-2.5 py-1 text-sm font-medium"
        :style="{ backgroundColor: area.color + '20', color: area.color }"
      >
        {{ area.name }}
      </span>
      <span
        v-if="taskAreas.length === 0"
        class="text-sm text-muted-foreground"
      >
        No areas
      </span>
    </div>
  </div>
</template>
