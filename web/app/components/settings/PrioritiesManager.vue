<script setup lang="ts">
import { Plus, Trash2, Loader2, Pencil, ArrowUp, ArrowDown, EyeOff } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { ProjectPriority } from "~/types";

const props = defineProps<{
  priorities: ProjectPriority[];
  projectKey: string;
  isAdmin: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { createPriority, updatePriority, deletePriority } = useProjects();

const loading = ref(false);
const showCreateForm = ref(false);
const editingId = ref<string | null>(null);
const deletingId = ref<string | null>(null);
const togglingId = ref<string | null>(null);

const presetColors = [
  "#6B7280", "#EF4444", "#F97316", "#EAB308", "#22C55E",
  "#10B981", "#3B82F6", "#8B5CF6", "#EC4899", "#14B8A6",
];

// Highest rank first — the order pickers and the board use.
const sorted = computed(() =>
  [...props.priorities].sort((a, b) => b.rank - a.rank || a.name.localeCompare(b.name))
);

const activeCount = computed(() => props.priorities.filter((p) => p.active).length);

const newPriority = ref({ name: "", description: "", color: "#6B7280" });

async function handleCreate() {
  if (!newPriority.value.name.trim()) return;
  loading.value = true;
  const result = await createPriority(props.projectKey, {
    name: newPriority.value.name.trim(),
    description: newPriority.value.description.trim() || undefined,
    color: newPriority.value.color,
    // New levels land on top: one rank above the current highest.
    rank: (sorted.value[0]?.rank ?? -1) + 1,
  });
  loading.value = false;
  if (result.success) {
    toast.success("Priority created");
    newPriority.value = { name: "", description: "", color: "#6B7280" };
    showCreateForm.value = false;
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to create priority");
  }
}

const editForm = ref({ name: "", description: "", color: "#6B7280" });

function openEdit(p: ProjectPriority) {
  editingId.value = p.id;
  editForm.value = {
    name: p.name,
    description: p.description ?? "",
    color: p.color || "#6B7280",
  };
}

async function handleSaveEdit() {
  const p = props.priorities.find((x) => x.id === editingId.value);
  if (!p || !editForm.value.name.trim()) return;
  const updates: { name?: string; description?: string; color?: string } = {};
  if (editForm.value.name.trim() !== p.name) updates.name = editForm.value.name.trim();
  if (editForm.value.description.trim() !== (p.description ?? "")) updates.description = editForm.value.description.trim();
  if (editForm.value.color !== (p.color || "#6B7280")) updates.color = editForm.value.color;
  if (Object.keys(updates).length === 0) {
    editingId.value = null;
    return;
  }
  loading.value = true;
  const result = await updatePriority(props.projectKey, p.id, updates);
  loading.value = false;
  if (result.success) {
    toast.success("Priority updated");
    editingId.value = null;
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to update priority");
  }
}

// Reorder by swapping ranks with the neighbour above/below.
async function move(p: ProjectPriority, dir: -1 | 1) {
  const list = sorted.value;
  const idx = list.findIndex((x) => x.id === p.id);
  const other = list[idx + dir];
  if (!other) return;
  loading.value = true;
  const a = await updatePriority(props.projectKey, p.id, { rank: other.rank });
  const b = await updatePriority(props.projectKey, other.id, { rank: p.rank });
  loading.value = false;
  if (a.success && b.success) {
    emit("refresh");
  } else {
    toast.error(a.error || b.error || "Failed to reorder");
    emit("refresh");
  }
}

async function toggleActive(p: ProjectPriority) {
  togglingId.value = p.id;
  const result = await updatePriority(props.projectKey, p.id, { active: !p.active });
  togglingId.value = null;
  if (result.success) {
    toast.success(p.active ? `"${p.name}" hidden from pickers` : `"${p.name}" is available again`);
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to update priority");
  }
}

async function handleDelete(p: ProjectPriority) {
  deletingId.value = p.id;
  const result = await deletePriority(props.projectKey, p.id);
  deletingId.value = null;
  if (result.success) {
    toast.success("Priority deleted");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to delete priority");
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="font-semibold">Priorities</h3>
        <p class="text-sm text-muted-foreground">
          This project's priority levels, most urgent first. Rename, describe
          (shown on hover wherever a priority appears), recolor, reorder, add
          new levels, or deactivate unused ones.
        </p>
      </div>
      <Button v-if="isAdmin && !showCreateForm" size="sm" @click="showCreateForm = true">
        <Plus class="mr-1.5 size-4" />
        Add Priority
      </Button>
    </div>

    <!-- Create form -->
    <Card v-if="showCreateForm" class="border-dashed">
      <CardContent class="pt-6">
        <form class="space-y-4" @submit.prevent="handleCreate">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-2">
              <Label>Name</Label>
              <Input v-model="newPriority.name" placeholder="Priority name" :disabled="loading" />
            </div>
            <div class="space-y-2">
              <Label>Color</Label>
              <div class="flex flex-wrap gap-1 pt-1">
                <button
                  v-for="color in presetColors"
                  :key="color"
                  type="button"
                  :aria-label="`Select color ${color}`"
                  class="size-6 rounded border-2 transition-all focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 outline-none"
                  :class="{
                    'border-foreground scale-110': newPriority.color === color,
                    'border-transparent': newPriority.color !== color,
                  }"
                  :style="{ backgroundColor: color }"
                  @click="newPriority.color = color"
                />
              </div>
            </div>
          </div>
          <div class="space-y-2">
            <Label>Description <span class="text-xs text-muted-foreground">(optional — defines when this level applies)</span></Label>
            <Textarea v-model="newPriority.description" rows="2" :disabled="loading" />
          </div>
          <div class="flex justify-end gap-2">
            <Button type="button" variant="outline" size="sm" :disabled="loading" @click="showCreateForm = false">
              Cancel
            </Button>
            <Button type="submit" size="sm" :disabled="loading || !newPriority.name.trim()">
              <Loader2 v-if="loading" class="mr-1.5 size-4 animate-spin" />
              Create
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>

    <!-- Levels, most urgent first -->
    <div class="space-y-1">
      <div
        v-for="(p, idx) in sorted"
        :key="p.id"
        class="rounded-lg border px-3 py-2"
        :class="p.active ? '' : 'opacity-60'"
      >
        <!-- Inline edit form -->
        <form v-if="editingId === p.id" class="space-y-3 py-1" @submit.prevent="handleSaveEdit">
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="space-y-1.5">
              <Label>Name</Label>
              <Input v-model="editForm.name" :disabled="loading" />
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
            <Label>Description <span class="text-xs text-muted-foreground">(shown on hover wherever this priority appears)</span></Label>
            <Textarea v-model="editForm.description" rows="2" :disabled="loading" />
          </div>
          <div class="flex justify-end gap-2">
            <Button type="button" variant="outline" size="sm" :disabled="loading" @click="editingId = null">
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
          <div class="size-3 shrink-0 rounded-full" :style="{ backgroundColor: p.color || '#6B7280' }" />
          <div class="min-w-0">
            <span class="text-sm font-medium" :title="p.description || undefined">{{ p.name }}</span>
            <span v-if="!p.active" class="ml-2 rounded bg-muted px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-muted-foreground">hidden</span>
            <p v-if="p.description" class="max-w-md truncate text-xs text-muted-foreground" :title="p.description">
              {{ p.description }}
            </p>
          </div>
          <span v-if="p.task_count > 0" class="shrink-0 text-xs text-muted-foreground">
            {{ p.task_count }} task{{ p.task_count === 1 ? "" : "s" }}
          </span>
          <span class="flex-1" />
          <template v-if="isAdmin">
            <Button
              variant="ghost"
              size="icon"
              aria-label="Move up"
              title="More urgent"
              class="size-7 text-muted-foreground"
              :disabled="idx === 0 || loading"
              @click="move(p, -1)"
            >
              <ArrowUp class="size-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Move down"
              title="Less urgent"
              class="size-7 text-muted-foreground"
              :disabled="idx === sorted.length - 1 || loading"
              @click="move(p, 1)"
            >
              <ArrowDown class="size-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              :aria-label="p.active ? 'Hide from pickers' : 'Show in pickers'"
              :title="p.active ? 'Hide from pickers (existing tasks keep it)' : 'Show in pickers again'"
              class="size-7 text-muted-foreground"
              :disabled="togglingId === p.id || (p.active && activeCount <= 1)"
              @click="toggleActive(p)"
            >
              <Loader2 v-if="togglingId === p.id" class="size-3.5 animate-spin" />
              <EyeOff v-else class="size-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Edit priority"
              title="Edit name, description or color"
              class="size-7 text-muted-foreground"
              @click="openEdit(p)"
            >
              <Pencil class="size-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Delete priority"
              :title="p.task_count > 0 ? 'Cannot delete: tasks still use this level' : 'Delete priority'"
              class="size-7 text-destructive hover:text-destructive"
              :disabled="deletingId === p.id || p.task_count > 0"
              @click="handleDelete(p)"
            >
              <Loader2 v-if="deletingId === p.id" class="size-3.5 animate-spin" />
              <Trash2 v-else class="size-3.5" />
            </Button>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
