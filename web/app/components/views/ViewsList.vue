<script setup lang="ts">
import { Lock, Users as UsersIcon, Play, Pencil, Trash2, Filter, Layers, Calendar, Loader2, Pin, PinOff } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { ProjectView } from "~/types";

const props = defineProps<{
  projectKey: string;
  views: ProjectView[];
  activeSlug: string | null;
  currentUserId?: string;
  isAdmin: boolean;
}>();

const emit = defineEmits<{
  "apply:view": [slug: string];
  "edit:view": [view: ProjectView];
  "refresh": [];
}>();

const { deleteView, setDefaultView, clearDefaultView } = useViews();

const showDeleteDialog = ref(false);
const deleting = ref(false);
const viewToDelete = ref<ProjectView | null>(null);

// Default-view pin (project admins, shared views only). One default per
// project; setting or removing it always goes through a confirm dialog since
// it changes what everyone else lands on.
const showDefaultDialog = ref(false);
const savingDefault = ref(false);
const defaultTarget = ref<ProjectView | null>(null);
const defaultAction = computed<"set" | "clear">(() =>
  defaultTarget.value?.is_default ? "clear" : "set"
);

function requestDefaultChange(view: ProjectView) {
  defaultTarget.value = view;
  showDefaultDialog.value = true;
}

async function confirmDefaultChange() {
  const v = defaultTarget.value;
  if (!v) return;
  savingDefault.value = true;
  const res = v.is_default
    ? await clearDefaultView(props.projectKey, v.slug)
    : await setDefaultView(props.projectKey, v.slug);
  savingDefault.value = false;
  showDefaultDialog.value = false;
  if (res.success) {
    toast.success(
      v.is_default
        ? "Default view removed"
        : `"${v.name}" is now the default view for this project`
    );
  } else {
    toast.error(res.error || "Failed to update default view");
  }
  defaultTarget.value = null;
}

function requestDelete(view: ProjectView) {
  viewToDelete.value = view;
  showDeleteDialog.value = true;
}

async function confirmDelete() {
  if (!viewToDelete.value) return;
  deleting.value = true;
  const res = await deleteView(props.projectKey, viewToDelete.value.slug);
  deleting.value = false;
  showDeleteDialog.value = false;
  if (res.success) {
    emit("refresh");
  } else {
    toast.error(res.error || "Failed to delete view");
  }
  viewToDelete.value = null;
}

function isOwner(v: ProjectView): boolean {
  return props.currentUserId !== undefined && v.owner_id === props.currentUserId;
}

function canEdit(v: ProjectView): boolean {
  return isOwner(v) || (v.visibility === "shared" && props.isAdmin);
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function predicateCount(v: ProjectView): number {
  return (v.filter_tree?.children ?? []).filter((c) => c.predicate).length;
}

function groupByLabel(groupBy: string): string {
  if (!groupBy || groupBy === "none") return "None";
  return groupBy.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}
</script>

<template>
  <div>
    <div
      v-if="views.length === 0"
      class="flex flex-col items-center justify-center rounded-lg border border-dashed py-16"
    >
      <UsersIcon class="size-8 text-muted-foreground" />
      <h3 class="mt-4 font-semibold">No views yet</h3>
      <p class="mt-1 max-w-sm text-center text-sm text-muted-foreground">
        Save a combination of filters as a view to jump back to it quickly. Share
        a view with the project so teammates can see the same slice of work.
      </p>
    </div>

    <div v-else class="space-y-2">
      <div
        v-for="v in views"
        :key="v.id"
        class="group relative rounded-lg border bg-card transition-colors hover:border-border/80 hover:bg-accent/30"
        :class="activeSlug === v.slug ? 'border-amber-500/40 bg-amber-500/5' : ''"
      >
        <div class="flex items-center gap-4 px-4 py-3">
          <!-- Visibility icon -->
          <div
            class="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted/50"
            :title="v.visibility === 'shared' ? 'Shared — visible to everyone in the project' : 'Private — only you'"
          >
            <component
              :is="v.visibility === 'shared' ? UsersIcon : Lock"
              class="size-3.5 text-muted-foreground"
            />
          </div>

          <!-- Name & description -->
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="text-left font-medium hover:underline"
                @click="emit('apply:view', v.slug)"
              >
                {{ v.name }}
              </button>
              <span
                v-if="activeSlug === v.slug"
                class="rounded-full bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider text-amber-700 dark:text-amber-300"
              >
                active
              </span>
              <span
                v-if="v.is_default"
                class="flex items-center gap-1 rounded-full bg-sky-500/15 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider text-sky-700 dark:text-sky-300"
                title="Applied automatically for members who haven't customised their filters"
              >
                <Pin class="size-2.5" />
                default
              </span>
            </div>
            <p
              v-if="v.description"
              class="mt-0.5 line-clamp-1 text-xs text-muted-foreground"
            >
              {{ v.description }}
            </p>
          </div>

          <!-- Meta pills -->
          <div class="hidden items-center gap-3 sm:flex">
            <div class="flex items-center gap-1.5 text-xs text-muted-foreground" :title="`${predicateCount(v)} filter${predicateCount(v) === 1 ? '' : 's'}`">
              <Filter class="size-3" />
              <span>{{ predicateCount(v) }}</span>
            </div>
            <div v-if="v.group_by && v.group_by !== 'none'" class="flex items-center gap-1.5 text-xs text-muted-foreground" :title="`Grouped by ${groupByLabel(v.group_by)}`">
              <Layers class="size-3" />
              <span>{{ groupByLabel(v.group_by) }}</span>
            </div>
            <div class="hidden items-center gap-1.5 text-xs text-muted-foreground md:flex" :title="formatDate(v.created_at)">
              <Calendar class="size-3" />
              <span>{{ formatDate(v.created_at) }}</span>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-1">
            <Button
              size="sm"
              variant="outline"
              class="h-7 gap-1.5 px-3 text-xs"
              @click="emit('apply:view', v.slug)"
            >
              <Play class="size-3" />
              Apply
            </Button>
            <Button
              v-if="isAdmin && v.visibility === 'shared'"
              size="sm"
              variant="ghost"
              class="h-7 w-7 p-0"
              :class="v.is_default ? 'text-sky-600 dark:text-sky-400' : 'text-muted-foreground'"
              :aria-label="v.is_default ? 'Remove as default view' : 'Make default view'"
              :title="v.is_default ? 'Remove as default view' : 'Make this the default view for everyone'"
              @click="requestDefaultChange(v)"
            >
              <component :is="v.is_default ? PinOff : Pin" class="size-3.5" />
            </Button>
            <Button
              v-if="canEdit(v)"
              size="sm"
              variant="ghost"
              class="h-7 gap-1.5 px-2 text-xs"
              @click="emit('edit:view', v)"
            >
              <Pencil class="size-3.5" />
              Edit
            </Button>
            <Button
              v-if="canEdit(v)"
              size="sm"
              variant="ghost"
              class="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
              aria-label="Delete view"
              title="Delete view"
              @click="requestDelete(v)"
            >
              <Trash2 class="size-3.5" />
            </Button>
          </div>
        </div>
      </div>
    </div>

    <Dialog v-model:open="showDefaultDialog">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {{ defaultAction === "set" ? "Set default view" : "Remove default view" }}
          </DialogTitle>
          <DialogDescription>
            <template v-if="defaultAction === 'set'">
              <strong>"{{ defaultTarget?.name }}"</strong> will be the default
              view for everyone in this project. It is applied automatically for
              members who haven't customised their own filters; anyone can still
              change or clear the filters afterwards.
            </template>
            <template v-else>
              <strong>"{{ defaultTarget?.name }}"</strong> will no longer be the
              default view. Members without their own filters will land on the
              standard task list instead.
            </template>
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" :disabled="savingDefault" @click="showDefaultDialog = false">
            Cancel
          </Button>
          <Button :disabled="savingDefault" @click="confirmDefaultChange">
            <Loader2 v-if="savingDefault" class="mr-2 size-4 animate-spin" />
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="showDeleteDialog">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete view</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete the view
            <strong>"{{ viewToDelete?.name }}"</strong>?
            <template v-if="viewToDelete?.visibility === 'shared'">
              It's shared, so it will disappear for everyone in the project.
            </template>
            This can't be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" :disabled="deleting" @click="showDeleteDialog = false">
            Cancel
          </Button>
          <Button variant="destructive" :disabled="deleting" @click="confirmDelete">
            <Loader2 v-if="deleting" class="mr-2 size-4 animate-spin" />
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
