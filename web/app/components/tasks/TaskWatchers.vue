<script setup lang="ts">
import { toast } from "vue-sonner";
import type { TaskWatcher, ProjectMember } from "~/types";

const props = defineProps<{
  watchers: TaskWatcher[];
  projectKey: string;
  taskNum: number;
  members: ProjectMember[];
  isMember: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { addWatcher, removeWatcher } = useTasks();

const loading = ref<string | null>(null);

// A common shape both TaskWatcher (chips) and ProjectMember (dropdown) satisfy,
// so TokenSelect can treat them as one item type keyed by user_id.
type TokenMember = Pick<
  ProjectMember,
  "user_id" | "username" | "first_name" | "last_name" | "avatar_url"
>;

const selectedTokens = computed<TokenMember[]>(() => props.watchers);

// Members not already watching — the pool offered in the token dropdown. Only
// current members are offered; chips above still show watchers who have left.
const availableTokens = computed<TokenMember[]>(() => {
  const chosen = new Set(props.watchers.map((w) => w.user_id));
  return props.members.filter((m) => !chosen.has(m.user_id));
});

// Watchers who are no longer members of this project (removed or deactivated).
// Their name still shows because it comes from the task, not the members list.
const formerMemberCount = computed(() => {
  const memberIds = new Set(props.members.map((m) => m.user_id));
  return props.watchers.filter((w) => !memberIds.has(w.user_id)).length;
});

function memberSearchText(m: TokenMember) {
  return `${m.first_name} ${m.last_name} ${m.username}`;
}

async function handleAdd(userId: string) {
  loading.value = userId;
  const result = await addWatcher(props.projectKey, props.taskNum, userId);
  loading.value = null;

  if (result.success) {
    toast.success("Watcher added");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to add watcher");
  }
}

async function handleRemove(userId: string) {
  loading.value = userId;
  const result = await removeWatcher(props.projectKey, props.taskNum, userId);
  loading.value = null;

  if (result.success) {
    toast.success("Watcher removed");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to remove watcher");
  }
}
</script>

<template>
  <div class="space-y-2">
    <p class="text-xs text-muted-foreground">Watchers</p>

    <!-- Editable: Gmail-style token input -->
    <TokenSelect
      v-if="isMember"
      :selected="selectedTokens"
      :available="availableTokens"
      :get-key="(m) => m.user_id"
      :get-search-text="memberSearchText"
      :pending-key="loading"
      placeholder="Add watchers..."
      empty-text="No members found"
      @add="(m) => handleAdd(m.user_id)"
      @remove="(m) => handleRemove(m.user_id)"
    >
      <template #chip="{ item: member }">
        <Avatar class="size-5">
          <AvatarImage v-if="member.avatar_url" :src="member.avatar_url" />
          <AvatarFallback class="text-[10px]" :seed="member.user_id">
            {{ member.first_name[0] }}{{ member.last_name[0] }}
          </AvatarFallback>
        </Avatar>
        <span class="truncate">{{ member.first_name }} {{ member.last_name }}</span>
      </template>
      <template #option="{ item: member }">
        <Avatar class="size-6">
          <AvatarImage v-if="member.avatar_url" :src="member.avatar_url" />
          <AvatarFallback class="text-xs" :seed="member.user_id">
            {{ member.first_name[0] }}{{ member.last_name[0] }}
          </AvatarFallback>
        </Avatar>
        {{ member.first_name }} {{ member.last_name }}
      </template>
    </TokenSelect>

    <p v-if="isMember && formerMemberCount > 0" class="text-[11px] text-muted-foreground">
      {{ formerMemberCount }}
      {{ formerMemberCount === 1 ? "watcher is" : "watchers are" }}
      no longer a member of this project.
    </p>

    <!-- Read-only view -->
    <div v-else-if="!isMember" class="flex flex-wrap items-center gap-2">
      <NuxtLink
        v-for="watcher in watchers"
        :key="watcher.id"
        :to="`/profile/${watcher.user_id}`"
        class="flex items-center gap-1.5 rounded-md border bg-muted/50 py-1 pl-1 pr-2.5 transition-opacity hover:opacity-80"
      >
        <Avatar class="size-6">
          <AvatarImage v-if="watcher.avatar_url" :src="watcher.avatar_url" />
          <AvatarFallback class="text-xs" :seed="watcher.user_id">
            {{ watcher.first_name[0] }}{{ watcher.last_name[0] }}
          </AvatarFallback>
        </Avatar>
        <span class="text-sm">
          {{ watcher.first_name }} {{ watcher.last_name }}
        </span>
      </NuxtLink>
      <span v-if="watchers.length === 0" class="text-sm text-muted-foreground">
        No watchers
      </span>
    </div>
  </div>
</template>
