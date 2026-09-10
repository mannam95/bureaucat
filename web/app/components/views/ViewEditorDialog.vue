<script setup lang="ts">
import { ListTodo, Kanban, Users as UsersIcon, AlertTriangle } from "lucide-vue-next";
import type {
  ViewVisibility,
  ViewDefaultTab,
  FilterTree,
  ViewGroupBy,
  SortKey,
  SortDir,
  ProjectState,
  ProjectLabel,
  ProjectMember,
  CycleSibling,
} from "~/types";

const props = defineProps<{
  open: boolean;
  projectKey: string;
  /** 'create' saves a brand-new view; 'edit' updates editSlug in place. */
  mode: "create" | "edit";
  editSlug?: string | null;
  // Seed values copied into the editor when it opens.
  seedName?: string;
  seedDescription?: string;
  seedVisibility?: ViewVisibility;
  seedDefaultTab?: ViewDefaultTab;
  seedTree: FilterTree;
  seedGroupBy: ViewGroupBy;
  seedSortBy: SortKey;
  seedSortDir: SortDir;
  /** In edit mode, whether the view being edited is shared (drives the warning). */
  editIsShared?: boolean;
  // Vocabulary for the filter editor.
  states: ProjectState[];
  labels: ProjectLabel[];
  members: ProjectMember[];
  cycles: CycleSibling[];
}>();

const emit = defineEmits<{
  "update:open": [open: boolean];
  saved: [slug: string, mode: "create" | "edit"];
}>();

const { createView, updateView } = useViews();

const name = ref("");
const description = ref("");
const visibility = ref<ViewVisibility>("private");
const defaultTab = ref<ViewDefaultTab>("tasks");
const scratchTree = ref<FilterTree>({ children: [] });
const scratchGroupBy = ref<ViewGroupBy>("state");
const scratchSortBy = ref<SortKey>("created_at");
const scratchSortDir = ref<SortDir>("desc");
const saving = ref(false);
const error = ref<string | null>(null);

function clone<T>(v: T): T {
  return JSON.parse(JSON.stringify(v));
}

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    name.value = props.seedName ?? "";
    description.value = props.seedDescription ?? "";
    visibility.value = props.seedVisibility ?? "private";
    defaultTab.value = props.seedDefaultTab ?? "tasks";
    scratchTree.value = clone(props.seedTree ?? { children: [] });
    scratchGroupBy.value = props.seedGroupBy ?? "state";
    scratchSortBy.value = props.seedSortBy ?? "created_at";
    scratchSortDir.value = props.seedSortDir ?? "desc";
    error.value = null;
  }
);

function onReset() {
  scratchTree.value = { children: [] };
  scratchSortBy.value = "created_at";
  scratchSortDir.value = "desc";
}

async function save() {
  if (!name.value.trim()) {
    error.value = "Name is required";
    return;
  }
  saving.value = true;
  try {
    if (props.mode === "edit" && props.editSlug) {
      const result = await updateView(props.projectKey, props.editSlug, {
        name: name.value.trim(),
        description: description.value.trim() || null,
        visibility: visibility.value,
        default_tab: defaultTab.value,
        filter_tree: scratchTree.value,
        group_by: scratchGroupBy.value,
        sort_by: scratchSortBy.value,
        sort_dir: scratchSortDir.value,
      });
      if (!result.success || !result.data) {
        error.value = result.error || "Failed to update view";
        return;
      }
      emit("saved", result.data.slug, "edit");
    } else {
      const result = await createView(props.projectKey, {
        name: name.value.trim(),
        description: description.value.trim() || undefined,
        visibility: visibility.value,
        filter_tree: scratchTree.value,
        group_by: scratchGroupBy.value,
        sort_by: scratchSortBy.value,
        sort_dir: scratchSortDir.value,
        default_tab: defaultTab.value,
      });
      if (!result.success || !result.data) {
        error.value = result.error || "Failed to create view";
        return;
      }
      emit("saved", result.data.slug, "create");
    }
    emit("update:open", false);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent class="sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle>{{ mode === "edit" ? "Edit view" : "Create new view" }}</DialogTitle>
        <DialogDescription>
          {{
            mode === "edit"
              ? "Change this view's filters, sort, name or who can see it."
              : "Name this set of filters so you can return to it — and share it if you like."
          }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4">
        <!-- Shared-edit warning -->
        <div
          v-if="mode === 'edit' && editIsShared"
          class="flex items-start gap-2 rounded-md border border-amber-500/40 bg-amber-500/5 px-3 py-2 text-xs text-amber-700 dark:text-amber-300"
        >
          <AlertTriangle class="mt-0.5 size-3.5 shrink-0" />
          <span>This view is shared — updating it changes it for everyone in the project.</span>
        </div>

        <!-- Filters -->
        <div class="space-y-1.5">
          <Label>Filters</Label>
          <div class="rounded-md border p-3">
            <FilterBar
              :tree="scratchTree"
              :search-query="''"
              :sort-by="scratchSortBy"
              :sort-dir="scratchSortDir"
              :group-by="scratchGroupBy"
              :states="states"
              :labels="labels"
              :members="members"
              :cycles="cycles"
              :show-group-by="defaultTab === 'board'"
              hide-search
              @update:tree="(t) => (scratchTree = t)"
              @update:sort-by="(v) => (scratchSortBy = v)"
              @update:sort-dir="(v) => (scratchSortDir = v)"
              @update:group-by="(v) => (scratchGroupBy = v)"
              @reset="onReset"
            />
            <p
              v-if="scratchTree.children.length === 0"
              class="mt-1 text-xs text-muted-foreground"
            >
              No filters — this view shows all tasks.
            </p>
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div class="space-y-1.5">
            <Label for="view-name">Name</Label>
            <Input id="view-name" v-model="name" placeholder="e.g. Overdue for me" />
          </div>
          <div class="space-y-1.5">
            <Label for="view-desc">Description</Label>
            <Input id="view-desc" v-model="description" placeholder="Optional" />
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div class="space-y-1.5">
            <Label>Visibility</Label>
            <div class="flex gap-2">
              <button
                type="button"
                class="flex-1 rounded-md border px-3 py-2 text-left text-sm transition-colors"
                :class="visibility === 'private' ? 'border-primary bg-primary/5' : 'hover:border-muted-foreground/50'"
                @click="visibility = 'private'"
              >
                <div class="font-medium">Private</div>
                <div class="text-xs text-muted-foreground">Only you</div>
              </button>
              <button
                type="button"
                class="flex-1 rounded-md border px-3 py-2 text-left text-sm transition-colors"
                :class="visibility === 'shared' ? 'border-primary bg-primary/5' : 'hover:border-muted-foreground/50'"
                @click="visibility = 'shared'"
              >
                <div class="flex items-center gap-1.5 font-medium">
                  <UsersIcon class="size-3.5" /> Shared
                </div>
                <div class="text-xs text-muted-foreground">Everyone in the project</div>
              </button>
            </div>
          </div>
          <div class="space-y-1.5">
            <Label>Opens in</Label>
            <div class="flex gap-2">
              <button
                type="button"
                class="flex flex-1 items-center gap-2 rounded-md border px-3 py-2 text-left text-sm transition-colors"
                :class="defaultTab === 'tasks' ? 'border-primary bg-primary/5' : 'hover:border-muted-foreground/50'"
                @click="defaultTab = 'tasks'"
              >
                <ListTodo class="size-4 text-muted-foreground" />
                <span class="font-medium">Tasks</span>
              </button>
              <button
                type="button"
                class="flex flex-1 items-center gap-2 rounded-md border px-3 py-2 text-left text-sm transition-colors"
                :class="defaultTab === 'board' ? 'border-primary bg-primary/5' : 'hover:border-muted-foreground/50'"
                @click="defaultTab = 'board'"
              >
                <Kanban class="size-4 text-muted-foreground" />
                <span class="font-medium">Board</span>
              </button>
            </div>
          </div>
        </div>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

        <DialogFooter>
          <Button type="button" variant="outline" @click="emit('update:open', false)">
            Cancel
          </Button>
          <Button type="button" :disabled="saving" @click="save">
            {{ mode === "edit" ? "Update View" : "Create New View" }}
          </Button>
        </DialogFooter>
      </div>
    </DialogContent>
  </Dialog>
</template>
