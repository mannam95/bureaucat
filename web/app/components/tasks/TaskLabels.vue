<script setup lang="ts">
import { Info } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { TaskLabel, ProjectLabel } from "~/types";

const props = defineProps<{
  taskLabels: TaskLabel[];
  projectKey: string;
  taskNum: number;
  projectLabels: ProjectLabel[];
  isMember: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { addLabel, removeLabel } = useTasks();

const loading = ref<string | null>(null);

// TaskLabel (chips) and ProjectLabel (dropdown) share this shape, keyed by id.
type TokenLabel = Pick<ProjectLabel, "id" | "name" | "color">;

const selectedTokens = computed<TokenLabel[]>(() => props.taskLabels);

// Labels not already on the task — the pool offered in the token dropdown.
const availableTokens = computed<TokenLabel[]>(() => {
  const usedIds = new Set(props.taskLabels.map((l) => l.id));
  return props.projectLabels.filter((l) => !usedIds.has(l.id));
});

function labelChipStyle(l: TokenLabel) {
  return { backgroundColor: l.color + "20", color: l.color };
}

async function handleAdd(labelId: string) {
  loading.value = labelId;
  const result = await addLabel(props.projectKey, props.taskNum, labelId);
  loading.value = null;

  if (result.success) {
    toast.success("Label added");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to add label");
  }
}

async function handleRemove(labelId: string) {
  loading.value = labelId;
  const result = await removeLabel(props.projectKey, props.taskNum, labelId);
  loading.value = null;

  if (result.success) {
    toast.success("Label removed");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to remove label");
  }
}
</script>

<template>
  <div class="space-y-2">
    <p class="text-xs text-muted-foreground">Labels</p>

    <!-- No labels defined for this project yet: labels are admin-defined, so a
         member sees a hint rather than an empty picker. -->
    <div
      v-if="isMember && projectLabels.length === 0"
      class="flex items-start gap-1.5 text-xs text-muted-foreground"
    >
      <Info class="mt-0.5 size-3.5 shrink-0" />
      <span>No labels yet. An admin can add label categories in project settings.</span>
    </div>

    <!-- Editable: Gmail-style token input (pick from admin-defined labels only) -->
    <TokenSelect
      v-else-if="isMember"
      :selected="selectedTokens"
      :available="availableTokens"
      :get-key="(l) => l.id"
      :get-search-text="(l) => l.name"
      :chip-style="labelChipStyle"
      :chip-class="() => 'pl-2 pr-1 font-medium'"
      :pending-key="loading"
      placeholder="Add labels..."
      empty-text="No matching labels"
      @add="(l) => handleAdd(l.id)"
      @remove="(l) => handleRemove(l.id)"
    >
      <template #chip="{ item: label }">
        <span class="truncate">{{ label.name }}</span>
      </template>
      <template #option="{ item: label }">
        <div
          class="size-3 shrink-0 rounded-full"
          :style="{ backgroundColor: label.color }"
        />
        {{ label.name }}
      </template>
    </TokenSelect>

    <!-- Read-only view -->
    <div v-else class="flex flex-wrap items-center gap-2">
      <span
        v-for="label in taskLabels"
        :key="label.id"
        class="rounded-md px-2.5 py-1 text-sm font-medium"
        :style="{ backgroundColor: label.color + '20', color: label.color }"
      >
        {{ label.name }}
      </span>
      <span
        v-if="taskLabels.length === 0"
        class="text-sm text-muted-foreground"
      >
        No labels
      </span>
    </div>
  </div>
</template>
