<script setup lang="ts">
import { Plus, Trash2, Pencil, Loader2, Check, X } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { ProjectArea } from "~/types";

const props = defineProps<{
  areas: ProjectArea[];
  projectKey: string;
  isAdmin: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { createArea, updateArea, deleteArea } = useProjects();

const loading = ref(false);
const showCreateForm = ref(false);
const editingId = ref<string | null>(null);
const deletingId = ref<string | null>(null);

const newArea = ref({
  name: "",
  color: "#3B82F6",
});

const editForm = ref({
  name: "",
  color: "",
});

const presetColors = [
  "#EF4444", "#F97316", "#EAB308", "#22C55E", "#10B981",
  "#3B82F6", "#6366F1", "#8B5CF6", "#EC4899", "#6B7280",
];

async function handleCreate() {
  if (!newArea.value.name.trim()) return;

  loading.value = true;
  const result = await createArea(props.projectKey, {
    name: newArea.value.name,
    color: newArea.value.color,
  });
  loading.value = false;

  if (result.success) {
    toast.success("Area created");
    newArea.value = { name: "", color: "#3B82F6" };
    showCreateForm.value = false;
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to create area");
  }
}

function startEdit(area: ProjectArea) {
  editingId.value = area.id;
  editForm.value = { name: area.name, color: area.color };
}

function cancelEdit() {
  editingId.value = null;
  editForm.value = { name: "", color: "" };
}

async function handleUpdate() {
  if (!editingId.value || !editForm.value.name.trim()) return;

  loading.value = true;
  const result = await updateArea(props.projectKey, editingId.value, {
    name: editForm.value.name,
    color: editForm.value.color,
  });
  loading.value = false;

  if (result.success) {
    toast.success("Area updated");
    cancelEdit();
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to update area");
  }
}

async function handleDelete(area: ProjectArea) {
  deletingId.value = area.id;
  const result = await deleteArea(props.projectKey, area.id);
  deletingId.value = null;

  if (result.success) {
    toast.success("Area deleted");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to delete area");
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="font-semibold">Areas</h3>
        <p class="text-sm text-muted-foreground">
          Admin-defined classification values a task can carry several of — e.g. the product areas it touches. Filterable, unlike free-form labels.
        </p>
      </div>
      <Button
        v-if="isAdmin && !showCreateForm"
        size="sm"
        @click="showCreateForm = true"
      >
        <Plus class="mr-1.5 size-4" />
        Add Area
      </Button>
    </div>

    <!-- Create form -->
    <Card v-if="showCreateForm" class="border-dashed">
      <CardContent class="pt-6">
        <form class="space-y-4" @submit.prevent="handleCreate">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-2">
              <Label>Name</Label>
              <Input
                v-model="newArea.name"
                placeholder="Area name"
                :disabled="loading"
              />
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
                    'border-foreground scale-110': newArea.color === color,
                    'border-transparent': newArea.color !== color,
                  }"
                  :style="{ backgroundColor: color }"
                  @click="newArea.color = color"
                />
              </div>
            </div>
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
            <Button type="submit" size="sm" :disabled="loading || !newArea.name">
              <Loader2 v-if="loading" class="mr-1.5 size-4 animate-spin" />
              Create
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>

    <!-- Areas list -->
    <div v-if="areas.length === 0" class="py-8 text-center text-sm text-muted-foreground">
      No areas yet. Create your first area to categorize tasks.
    </div>
    <div v-else class="space-y-2">
      <div
        v-for="area in areas"
        :key="area.id"
        class="flex items-center gap-3 rounded-lg border px-3 py-2"
      >
        <template v-if="editingId === area.id">
          <Input
            v-model="editForm.name"
            class="h-8 flex-1"
            :disabled="loading"
          />
          <div class="flex gap-1">
            <button
              v-for="color in presetColors"
              :key="color"
              type="button"
              :aria-label="`Select color ${color}`"
              class="size-5 rounded transition-all focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 outline-none"
              :class="{
                'ring-2 ring-foreground ring-offset-1': editForm.color === color,
              }"
              :style="{ backgroundColor: color }"
              @click="editForm.color = color"
            />
          </div>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Save"
            class="size-8"
            :disabled="loading"
            @click="handleUpdate"
          >
            <Loader2 v-if="loading" class="size-4 animate-spin" />
            <Check v-else class="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Cancel"
            class="size-8"
            :disabled="loading"
            @click="cancelEdit"
          >
            <X class="size-4" />
          </Button>
        </template>
        <template v-else>
          <span
            class="rounded px-2 py-0.5 text-sm font-medium"
            :style="{
              backgroundColor: area.color + '20',
              color: area.color,
            }"
          >
            {{ area.name }}
          </span>
          <span class="flex-1" />
          <Button
            v-if="isAdmin"
            variant="ghost"
            size="icon"
            aria-label="Edit area"
            class="size-8"
            @click="startEdit(area)"
          >
            <Pencil class="size-4" />
          </Button>
          <Button
            v-if="isAdmin"
            variant="ghost"
            size="icon"
            aria-label="Delete area"
            class="size-8 text-destructive hover:text-destructive"
            :disabled="deletingId === area.id"
            @click="handleDelete(area)"
          >
            <Loader2
              v-if="deletingId === area.id"
              class="size-4 animate-spin"
            />
            <Trash2 v-else class="size-4" />
          </Button>
        </template>
      </div>
    </div>
  </div>
</template>
