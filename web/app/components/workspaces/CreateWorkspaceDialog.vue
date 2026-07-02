<script setup lang="ts">
import { Loader2 } from "lucide-vue-next";
import { toast } from "vue-sonner";

const open = defineModel<boolean>("open", { default: false });

const emit = defineEmits<{
  created: [workspaceId: string];
}>();

const { createWorkspace, setCurrentWorkspace } = useWorkspaces();

const loading = ref(false);
const error = ref<string | null>(null);
const form = ref({
  workspace_key: "",
  name: "",
  description: "",
});

function resetForm() {
  form.value = { workspace_key: "", name: "", description: "" };
  error.value = null;
}

watch(open, (isOpen) => {
  if (isOpen) resetForm();
});

function validateKey(e: Event) {
  const input = e.target as HTMLInputElement;
  input.value = input.value.replace(/[^a-zA-Z0-9]/g, "").toUpperCase();
  form.value.workspace_key = input.value;
}

async function handleSubmit() {
  loading.value = true;
  error.value = null;

  const result = await createWorkspace({
    workspace_key: form.value.workspace_key.toUpperCase(),
    name: form.value.name,
    description: form.value.description || undefined,
  });

  loading.value = false;

  if (result.success && result.data) {
    toast.success("Workspace created");
    // Switch into the workspace we just created.
    setCurrentWorkspace(result.data);
    open.value = false;
    emit("created", result.data.id);
  } else {
    error.value = result.error || "Failed to create workspace";
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Create Workspace</DialogTitle>
        <DialogDescription>
          A workspace groups related projects together.
        </DialogDescription>
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="handleSubmit">
        <div
          v-if="error"
          class="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {{ error }}
        </div>
        <div class="space-y-2">
          <Label for="workspace_key">Workspace Key</Label>
          <Input
            id="workspace_key"
            :model-value="form.workspace_key"
            placeholder="ENG"
            maxlength="10"
            required
            :disabled="loading"
            class="font-mono uppercase"
            @input="validateKey"
          />
          <p class="text-xs text-muted-foreground">
            2-10 alphanumeric characters. Unique identifier for the workspace.
          </p>
        </div>
        <div class="space-y-2">
          <Label for="ws_name">Name</Label>
          <Input
            id="ws_name"
            v-model="form.name"
            placeholder="Engineering"
            required
            :disabled="loading"
          />
        </div>
        <div class="space-y-2">
          <Label for="ws_description">Description</Label>
          <Textarea
            id="ws_description"
            v-model="form.description"
            placeholder="Brief description of the workspace..."
            rows="3"
            :disabled="loading"
          />
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" :disabled="loading" @click="open = false">
            Cancel
          </Button>
          <Button type="submit" :disabled="loading || !form.workspace_key || !form.name">
            <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
            Create Workspace
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
