<script setup lang="ts">
import { Plus, Trash2, GripVertical, Loader2, Star, Pencil, TriangleAlert } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { ProjectState, StateType } from "~/types";

const props = defineProps<{
  states: ProjectState[];
  projectKey: string;
  isAdmin: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { createState, updateState, setDefaultState, deleteState } = useProjects();

const loading = ref(false);
const showCreateForm = ref(false);
const editingId = ref<string | null>(null);
const deletingId = ref<string | null>(null);
const settingDefaultId = ref<string | null>(null);

const newState = ref({
  name: "",
  state_type: "unstarted" as StateType,
  color: "#3B82F6",
  description: "",
});

const stateTypes: { value: StateType; label: string }[] = [
  { value: "backlog", label: "Backlog" },
  { value: "unstarted", label: "Unstarted" },
  { value: "started", label: "Started" },
  { value: "completed", label: "Completed" },
  { value: "cancelled", label: "Cancelled" },
  { value: "archived", label: "Archived" },
];

const presetColors = [
  "#6B7280", "#EF4444", "#F97316", "#EAB308", "#22C55E",
  "#10B981", "#3B82F6", "#8B5CF6", "#EC4899", "#14B8A6",
];

// Group states by type
const groupedStates = computed(() => {
  const groups: Record<string, ProjectState[]> = {};
  for (const type of stateTypes) {
    groups[type.value] = props.states
      .filter((s) => s.state_type === type.value)
      .sort((a, b) => a.position - b.position);
  }
  return groups;
});

async function handleCreate() {
  if (!newState.value.name.trim()) return;

  loading.value = true;
  const result = await createState(props.projectKey, {
    name: newState.value.name,
    state_type: newState.value.state_type,
    color: newState.value.color,
    description: newState.value.description.trim() || undefined,
  });
  loading.value = false;

  if (result.success) {
    toast.success("State created");
    newState.value = { name: "", state_type: "unstarted", color: "#3B82F6", description: "" };
    showCreateForm.value = false;
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to create state");
  }
}

// Inline edit: name, type and color. Tasks reference states by id, so an edit
// applies to every task in the state instantly — a type change re-categorises
// them across progress, filters and the board, which the form warns about.
const editForm = ref({
  name: "",
  state_type: "unstarted" as StateType,
  color: "#3B82F6",
  description: "",
});

const editingState = computed(
  () => props.states.find((s) => s.id === editingId.value) ?? null
);
const editTypeChanged = computed(
  () => !!editingState.value && editForm.value.state_type !== editingState.value.state_type
);

function typeLabel(t: StateType): string {
  return stateTypes.find((x) => x.value === t)?.label ?? t;
}

function openEdit(state: ProjectState) {
  editingId.value = state.id;
  editForm.value = {
    name: state.name,
    state_type: state.state_type,
    color: state.color || "#6B7280",
    description: state.description ?? "",
  };
}

function cancelEdit() {
  editingId.value = null;
}

async function handleSaveEdit() {
  const s = editingState.value;
  if (!s || !editForm.value.name.trim()) return;

  // Send only what changed.
  const updates: { name?: string; color?: string; description?: string; state_type?: StateType } = {};
  if (editForm.value.name.trim() !== s.name) updates.name = editForm.value.name.trim();
  if (editForm.value.color !== (s.color || "#6B7280")) updates.color = editForm.value.color;
  if (editForm.value.description.trim() !== (s.description ?? "")) updates.description = editForm.value.description.trim();
  if (editForm.value.state_type !== s.state_type) updates.state_type = editForm.value.state_type;
  if (Object.keys(updates).length === 0) {
    editingId.value = null;
    return;
  }

  loading.value = true;
  const result = await updateState(props.projectKey, s.id, updates);
  loading.value = false;

  if (result.success) {
    toast.success("State updated");
    editingId.value = null;
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to update state");
  }
}

async function handleSetDefault(state: ProjectState) {
  if (state.is_default) return;

  settingDefaultId.value = state.id;
  const result = await setDefaultState(props.projectKey, state.id);
  settingDefaultId.value = null;

  if (result.success) {
    toast.success(`"${state.name}" is now the default state`);
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to set default state");
  }
}

async function handleDelete(state: ProjectState) {
  if (state.is_default) {
    toast.error("Cannot delete default state");
    return;
  }

  deletingId.value = state.id;
  const result = await deleteState(props.projectKey, state.id);
  deletingId.value = null;

  if (result.success) {
    toast.success("State deleted");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to delete state");
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="font-semibold">Workflow States</h3>
        <p class="text-sm text-muted-foreground">
          Configure the states tasks can be in
        </p>
      </div>
      <Button
        v-if="isAdmin && !showCreateForm"
        size="sm"
        @click="showCreateForm = true"
      >
        <Plus class="mr-1.5 size-4" />
        Add State
      </Button>
    </div>

    <!-- Create form -->
    <Card v-if="showCreateForm" class="border-dashed">
      <CardContent class="pt-6">
        <form class="space-y-4" @submit.prevent="handleCreate">
          <div class="grid gap-4 sm:grid-cols-3">
            <div class="space-y-2">
              <Label>Name</Label>
              <Input
                v-model="newState.name"
                placeholder="State name"
                :disabled="loading"
              />
            </div>
            <div class="space-y-2">
              <Label>Type</Label>
              <NativeSelect v-model="newState.state_type" :disabled="loading">
                <option v-for="type in stateTypes" :key="type.value" :value="type.value">
                  {{ type.label }}
                </option>
              </NativeSelect>
            </div>
            <div class="space-y-2">
              <Label>Color</Label>
              <div class="flex flex-wrap gap-1">
                <button
                  v-for="color in presetColors"
                  :key="color"
                  type="button"
                  :aria-label="`Select color ${color}`"
                  class="size-6 rounded border-2 transition-all focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 outline-none"
                  :class="{
                    'border-foreground scale-110': newState.color === color,
                    'border-transparent': newState.color !== color,
                  }"
                  :style="{ backgroundColor: color }"
                  @click="newState.color = color"
                />
              </div>
            </div>
          </div>
          <div class="space-y-2">
            <Label>Description <span class="text-xs text-muted-foreground">(optional — shown on hover wherever the state appears)</span></Label>
            <Textarea
              v-model="newState.description"
              rows="2"
              placeholder="When does this state apply? E.g. only picked up again on serious purchase interest."
              :disabled="loading"
            />
          </div>
          <div class="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              :disabled="loading"
              @click="showCreateForm = false"
            >
              Cancel
            </Button>
            <Button type="submit" size="sm" :disabled="loading || !newState.name">
              <Loader2 v-if="loading" class="mr-1.5 size-4 animate-spin" />
              Create
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>

    <!-- States by type -->
    <div v-for="type in stateTypes" :key="type.value" class="space-y-2">
      <h4 class="text-sm font-medium text-muted-foreground">{{ type.label }}</h4>
      <div v-if="groupedStates[type.value].length === 0" class="py-2 text-sm text-muted-foreground">
        No states in this group
      </div>
      <div v-else class="space-y-1">
        <div
          v-for="state in groupedStates[type.value]"
          :key="state.id"
          class="rounded-lg border px-3 py-2"
        >
          <!-- Inline edit form -->
          <form
            v-if="editingId === state.id"
            class="space-y-3 py-1"
            @submit.prevent="handleSaveEdit"
          >
            <div class="grid gap-3 sm:grid-cols-3">
              <div class="space-y-1.5">
                <Label>Name</Label>
                <Input v-model="editForm.name" :disabled="loading" />
              </div>
              <div class="space-y-1.5">
                <Label>Type</Label>
                <NativeSelect v-model="editForm.state_type" :disabled="loading">
                  <option v-for="t in stateTypes" :key="t.value" :value="t.value">
                    {{ t.label }}
                  </option>
                </NativeSelect>
              </div>
              <div class="space-y-1.5">
                <Label>Color</Label>
                <div class="flex flex-wrap gap-1 pt-1">
                  <button
                    v-for="color in presetColors"
                    :key="color"
                    type="button"
                    :aria-label="`Select color ${color}`"
                    class="size-6 rounded border-2 transition-all focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 outline-none"
                    :class="{
                      'border-foreground scale-110': editForm.color === color,
                      'border-transparent': editForm.color !== color,
                    }"
                    :style="{ backgroundColor: color }"
                    @click="editForm.color = color"
                  />
                </div>
              </div>
            </div>
            <div class="space-y-1.5">
              <Label>Description <span class="text-xs text-muted-foreground">(shown on hover wherever the state appears)</span></Label>
              <Textarea v-model="editForm.description" rows="2" :disabled="loading" />
            </div>
            <p
              v-if="editTypeChanged && state.task_count > 0"
              class="flex items-start gap-1.5 rounded-md bg-amber-500/10 px-2.5 py-1.5 text-xs text-amber-700 dark:text-amber-300"
            >
              <TriangleAlert class="mt-0.5 size-3.5 shrink-0" />
              <span>
                {{ state.task_count }} task{{ state.task_count === 1 ? "" : "s" }} in this
                state will immediately count as "{{ typeLabel(editForm.state_type) }}"
                in progress, filters and the board.
              </span>
            </p>
            <div class="flex justify-end gap-2">
              <Button type="button" variant="outline" size="sm" :disabled="loading" @click="cancelEdit">
                Cancel
              </Button>
              <Button type="submit" size="sm" :disabled="loading || !editForm.name.trim()">
                <Loader2 v-if="loading" class="mr-1.5 size-4 animate-spin" />
                Save
              </Button>
            </div>
          </form>

          <!-- Normal row -->
          <div v-else class="flex items-center gap-3">
            <div
              class="size-3 rounded-full"
              :style="{ backgroundColor: state.color }"
            />
            <div class="min-w-0">
              <span class="text-sm font-medium" :title="state.description || undefined">{{ state.name }}</span>
              <p v-if="state.description" class="max-w-md truncate text-xs text-muted-foreground" :title="state.description">
                {{ state.description }}
              </p>
            </div>
            <span v-if="state.task_count > 0" class="shrink-0 text-xs text-muted-foreground">
              {{ state.task_count }} task{{ state.task_count === 1 ? "" : "s" }}
            </span>
            <span class="flex-1" />
            <Badge v-if="state.is_default" variant="outline" class="gap-1 text-xs">
              <Star class="size-3 fill-current" />
              Default
            </Badge>
            <Button
              v-else-if="isAdmin"
              variant="ghost"
              size="sm"
              class="h-8 gap-1.5 text-muted-foreground"
              :disabled="settingDefaultId === state.id"
              @click="handleSetDefault(state)"
            >
              <Loader2
                v-if="settingDefaultId === state.id"
                class="size-3.5 animate-spin"
              />
              <Star v-else class="size-3.5" />
              Set default
            </Button>
            <Button
              v-if="isAdmin"
              variant="ghost"
              size="icon"
              aria-label="Edit state"
              title="Edit name, type or color"
              class="size-8 text-muted-foreground"
              @click="openEdit(state)"
            >
              <Pencil class="size-4" />
            </Button>
            <Button
              v-if="isAdmin && !state.is_default"
              variant="ghost"
              size="icon"
              aria-label="Delete state"
              :title="state.task_count > 0 ? 'Cannot delete: move or finish its tasks first' : 'Delete state'"
              class="size-8 text-destructive hover:text-destructive"
              :disabled="deletingId === state.id || state.task_count > 0"
              @click="handleDelete(state)"
            >
              <Loader2
                v-if="deletingId === state.id"
                class="size-4 animate-spin"
              />
              <Trash2 v-else class="size-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
