<script setup lang="ts">
import { Loader2, UserPlus } from "lucide-vue-next";
import { toast } from "vue-sonner";

const props = defineProps<{
  projectKey: string;
  existingMemberIds: string[];
}>();

const open = defineModel<boolean>("open", { default: false });

const emit = defineEmits<{
  added: [];
}>();

const { addMember, searchUsers: searchUsersApi } = useProjects();

type DirectoryUser = {
  id: string;
  username: string;
  email: string;
  first_name: string;
  last_name: string;
};

const loading = ref(false);
const searchLoading = ref(false);
const error = ref<string | null>(null);
const searchResults = ref<DirectoryUser[]>([]);
// Users queued to be added; each renders as a chip in the token input.
const selectedUsers = ref<DirectoryUser[]>([]);
const selectedRole = ref("member");

const roles = [
  { value: "guest", label: "Guest", description: "Read-only access" },
  { value: "member", label: "Member", description: "Can create and edit tasks" },
  { value: "admin", label: "Admin", description: "Full project control" },
];

// The dropdown pool: search hits minus existing members and already-queued users.
const availableUsers = computed(() => {
  const excluded = new Set([
    ...props.existingMemberIds,
    ...selectedUsers.value.map((u) => u.id),
  ]);
  return searchResults.value.filter((u) => !excluded.has(u.id));
});

function userSearchText(u: DirectoryUser) {
  return `${u.first_name} ${u.last_name} ${u.username} ${u.email}`;
}

let searchTimeout: ReturnType<typeof setTimeout>;
function onSearch(query: string) {
  clearTimeout(searchTimeout);
  const q = query.trim();
  if (q.length < 2) {
    searchResults.value = [];
    searchLoading.value = false;
    return;
  }
  searchLoading.value = true;
  searchTimeout = setTimeout(async () => {
    const result = await searchUsersApi(props.projectKey, q);
    searchLoading.value = false;
    searchResults.value = result.success && result.data ? result.data : [];
  }, 300);
}

function addToken(u: DirectoryUser) {
  if (!selectedUsers.value.some((s) => s.id === u.id)) {
    selectedUsers.value = [...selectedUsers.value, u];
  }
}

function removeToken(u: DirectoryUser) {
  selectedUsers.value = selectedUsers.value.filter((s) => s.id !== u.id);
}

function resetForm() {
  searchResults.value = [];
  selectedUsers.value = [];
  selectedRole.value = "member";
  error.value = null;
  searchLoading.value = false;
}

watch(open, (isOpen) => {
  if (isOpen) resetForm();
});

async function handleSubmit() {
  if (selectedUsers.value.length === 0) return;

  loading.value = true;
  error.value = null;

  const results = await Promise.all(
    selectedUsers.value.map((u) =>
      addMember(props.projectKey, { user_id: u.id, role: selectedRole.value })
    )
  );

  loading.value = false;

  const added = results.filter((r) => r.success).length;
  const failed = results.length - added;

  if (added > 0) {
    toast.success(
      added === 1 ? "Member added" : `${added} members added`
    );
    emit("added");
  }

  if (failed > 0) {
    error.value =
      results.find((r) => !r.success)?.error ||
      `Failed to add ${failed} member${failed === 1 ? "" : "s"}`;
    // Keep only the users that failed so the admin can retry them.
    const failedIds = new Set(
      selectedUsers.value.filter((_, i) => !results[i]?.success).map((u) => u.id)
    );
    selectedUsers.value = selectedUsers.value.filter((u) => failedIds.has(u.id));
  } else {
    open.value = false;
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Add Members</DialogTitle>
        <DialogDescription>
          Search for users to add to this project. Select as many as you like,
          then add them all at once.
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
          <Label>Users</Label>
          <div class="rounded-md border px-3 py-2">
            <TokenSelect
              :selected="selectedUsers"
              :available="availableUsers"
              :get-key="(u) => u.id"
              :get-search-text="userSearchText"
              server-search
              :loading="searchLoading"
              placeholder="Search by name, username or email..."
              empty-text="No users found"
              :disabled="loading"
              @search="onSearch"
              @add="addToken"
              @remove="removeToken"
            >
              <template #chip="{ item: u }">
                <Avatar class="size-5">
                  <AvatarFallback class="text-[10px]" :seed="u.id">
                    {{ u.first_name[0] }}{{ u.last_name[0] }}
                  </AvatarFallback>
                </Avatar>
                <span class="truncate">{{ u.first_name }} {{ u.last_name }}</span>
              </template>
              <template #option="{ item: u }">
                <Avatar class="size-8">
                  <AvatarFallback class="text-xs" :seed="u.id">
                    {{ u.first_name[0] }}{{ u.last_name[0] }}
                  </AvatarFallback>
                </Avatar>
                <div class="min-w-0 flex-1">
                  <p class="truncate text-sm font-medium">
                    {{ u.first_name }} {{ u.last_name }}
                  </p>
                  <p class="truncate text-xs text-muted-foreground">
                    @{{ u.username }} · {{ u.email }}
                  </p>
                </div>
              </template>
            </TokenSelect>
          </div>
          <p class="text-xs text-muted-foreground">
            Press Enter to add the highlighted user.
          </p>
        </div>

        <div class="space-y-2">
          <Label>Role</Label>
          <p class="text-xs text-muted-foreground">
            Applied to everyone added in this batch.
          </p>
          <div class="space-y-2">
            <label
              v-for="role in roles"
              :key="role.value"
              class="flex cursor-pointer items-center gap-3 rounded-lg border p-3 transition-colors hover:bg-muted/50"
              :class="{ 'border-primary bg-primary/5': selectedRole === role.value }"
            >
              <input
                v-model="selectedRole"
                type="radio"
                :value="role.value"
                class="sr-only"
                :disabled="loading"
              />
              <div
                class="size-4 rounded-full border-2"
                :class="{
                  'border-primary bg-primary': selectedRole === role.value,
                  'border-muted-foreground': selectedRole !== role.value,
                }"
              />
              <div>
                <p class="font-medium">{{ role.label }}</p>
                <p class="text-sm text-muted-foreground">{{ role.description }}</p>
              </div>
            </label>
          </div>
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            :disabled="loading"
            @click="open = false"
          >
            Cancel
          </Button>
          <Button type="submit" :disabled="loading || selectedUsers.length === 0">
            <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
            <UserPlus v-else class="mr-2 size-4" />
            {{
              selectedUsers.length > 1
                ? `Add ${selectedUsers.length} Members`
                : "Add Member"
            }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
