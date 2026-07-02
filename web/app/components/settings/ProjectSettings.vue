<script setup lang="ts">
import { Loader2 } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { Project } from "~/types";

const props = defineProps<{
  project: Project;
  isAdmin: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { updateProject, setProjectDisabled } = useProjects();

const loading = ref(false);
const togglingDisabled = ref(false);

async function handleToggleDisabled(disabled: boolean) {
  togglingDisabled.value = true;
  const result = await setProjectDisabled(props.project.project_key, disabled);
  togglingDisabled.value = false;

  if (result.success) {
    toast.success(disabled ? "Project disabled" : "Project enabled");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to update project");
  }
}

const form = ref({
  name: props.project.name,
  description: props.project.description || "",
});

watch(
  () => props.project,
  (p) => {
    form.value = {
      name: p.name,
      description: p.description || "",
    };
  },
  { immediate: true }
);

async function handleSave() {
  loading.value = true;
  const result = await updateProject(props.project.project_key, {
    name: form.value.name,
    description: form.value.description || undefined,
  });
  loading.value = false;

  if (result.success) {
    toast.success("Project updated");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to update project");
  }
}

const hasChanges = computed(() => {
  return (
    form.value.name !== props.project.name ||
    form.value.description !== (props.project.description || "")
  );
});
</script>

<template>
  <div class="space-y-8">
    <!-- General settings -->
    <div class="space-y-4">
      <div>
        <h3 class="font-semibold">General</h3>
        <p class="text-sm text-muted-foreground">
          Basic project information
        </p>
      </div>

      <Card>
        <CardContent class="pt-6">
          <form class="space-y-4" @submit.prevent="handleSave">
            <div class="space-y-2">
              <Label for="project-key">Project Key</Label>
              <input
                id="project-key"
                :value="project.project_key"
                disabled
                class="border-input dark:bg-input/30 h-9 w-full rounded-md border bg-transparent px-3 py-1 font-mono text-base shadow-xs disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
              />
              <p class="text-xs text-muted-foreground">
                Cannot be changed after creation
              </p>
            </div>

            <div class="space-y-2">
              <Label for="name">Name</Label>
              <Input
                id="name"
                v-model="form.name"
                :disabled="loading || !isAdmin || project.disabled"
              />
            </div>

            <div class="space-y-2">
              <Label for="description">Description</Label>
              <Textarea
                id="description"
                v-model="form.description"
                rows="3"
                :disabled="loading || !isAdmin || project.disabled"
              />
            </div>

            <div v-if="isAdmin" class="flex justify-end">
              <Button type="submit" :disabled="loading || !hasChanges || project.disabled">
                <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
                Save Changes
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>

    <!-- Availability -->
    <div v-if="isAdmin" class="space-y-4">
      <div>
        <h3 class="font-semibold">Availability</h3>
        <p class="text-sm text-muted-foreground">
          Disable the project to make it read-only
        </p>
      </div>

      <Card>
        <CardContent class="flex items-center justify-between gap-4 pt-6">
          <div class="space-y-1">
            <Label>Disable project</Label>
            <p class="text-sm text-muted-foreground">
              When disabled, the project becomes read-only: no tasks can be
              created, edited, moved, or commented on until it is re-enabled.
            </p>
          </div>
          <Switch
            :checked="project.disabled"
            :disabled="togglingDisabled"
            aria-label="Disable project"
            @update:checked="handleToggleDisabled"
          />
        </CardContent>
      </Card>
    </div>

  </div>
</template>
