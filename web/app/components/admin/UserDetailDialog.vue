<script setup lang="ts">
import { Loader2 } from "lucide-vue-next";
import type { AdminUserDetail } from "~/composables/useAdmin";

// Read-only drill-down: sign-in method, sessions/tokens and memberships.
// Editing stays with the page's own controls (profile pencil, role, password),
// so this dialog never duplicates a write path.
const props = defineProps<{
  userId: string | null;
}>();

const open = defineModel<boolean>("open", { default: false });

const { getUserDetail } = useAdmin();

const detail = ref<AdminUserDetail | null>(null);
const loading = ref(false);
const error = ref<string | null>(null);

const signInMethod = computed(() => {
  const d = detail.value;
  if (!d) return "";
  if (d.auth_provider && d.has_password) return `SSO (${d.auth_provider}) + password`;
  if (d.auth_provider) return `SSO (${d.auth_provider})`;
  return d.has_password ? "Password" : "None";
});

async function load() {
  if (!props.userId) return;
  loading.value = true;
  error.value = null;
  detail.value = null;
  const result = await getUserDetail(props.userId);
  if (result.success && result.data) {
    detail.value = result.data;
  } else {
    error.value = result.error || "Failed to load user";
  }
  loading.value = false;
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

watch(
  () => [open.value, props.userId] as const,
  ([isOpen]) => {
    if (isOpen) load();
  }
);
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle>User details</DialogTitle>
        <DialogDescription>
          Sign-in method, sessions and memberships. Edits happen from the user list.
        </DialogDescription>
      </DialogHeader>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <Loader2 class="size-6 animate-spin text-muted-foreground" />
      </div>

      <div
        v-else-if="!detail && error"
        role="alert"
        class="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
      >
        {{ error }}
      </div>

      <div v-else-if="detail" class="space-y-6">
        <div class="flex items-center gap-3">
          <Avatar class="size-12">
            <AvatarImage v-if="detail.avatar_url" :src="detail.avatar_url" />
            <AvatarFallback :seed="detail.id">
              {{ detail.first_name[0] }}{{ detail.last_name[0] }}
            </AvatarFallback>
          </Avatar>
          <div class="min-w-0">
            <p class="truncate font-semibold">
              {{ detail.first_name }} {{ detail.last_name }}
              <span class="font-normal text-muted-foreground">@{{ detail.username }}</span>
            </p>
            <p class="truncate text-sm text-muted-foreground">{{ detail.email }}</p>
          </div>
          <Badge :variant="detail.user_type === 'admin' ? 'default' : 'outline'" class="ml-auto shrink-0">
            {{ detail.user_type }}
          </Badge>
        </div>

        <dl class="grid grid-cols-1 gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
          <div>
            <dt class="text-xs text-muted-foreground">Sign-in method</dt>
            <dd>{{ signInMethod }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">Active sessions / API tokens</dt>
            <dd>{{ detail.active_sessions }} / {{ detail.active_tokens }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">Created</dt>
            <dd>{{ formatDate(detail.created_at) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">Last updated</dt>
            <dd>{{ formatDate(detail.updated_at) }}</dd>
          </div>
        </dl>

        <Separator />

        <div class="space-y-2">
          <h4 class="text-sm font-medium">
            Workspaces <span class="font-normal text-muted-foreground">({{ detail.workspaces.length }})</span>
          </h4>
          <p v-if="detail.workspaces.length === 0" class="text-sm text-muted-foreground">None</p>
          <div v-else class="flex flex-wrap gap-1.5">
            <Badge v-for="w in detail.workspaces" :key="w.workspace_key" variant="secondary">
              {{ w.name }}
            </Badge>
          </div>
        </div>

        <div class="space-y-2">
          <h4 class="text-sm font-medium">
            Projects <span class="font-normal text-muted-foreground">({{ detail.projects.length }})</span>
          </h4>
          <p v-if="detail.projects.length === 0" class="text-sm text-muted-foreground">None</p>
          <ul v-else class="max-h-56 divide-y overflow-y-auto rounded-md border">
            <li
              v-for="p in detail.projects"
              :key="p.project_key"
              class="flex items-center gap-2 px-3 py-2 text-sm"
            >
              <span class="shrink-0 font-mono text-xs text-amber-600 dark:text-amber-500">{{ p.project_key }}</span>
              <span class="min-w-0 flex-1 truncate">{{ p.name }}</span>
              <span v-if="p.workspace_name" class="hidden truncate text-xs text-muted-foreground sm:inline">
                {{ p.workspace_name }}
              </span>
              <Badge :variant="p.role === 'admin' ? 'default' : 'outline'" class="shrink-0">{{ p.role }}</Badge>
            </li>
          </ul>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" @click="open = false">Close</Button>
        </DialogFooter>
      </div>
    </DialogContent>
  </Dialog>
</template>
