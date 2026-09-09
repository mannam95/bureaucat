<script setup lang="ts">
import { toast } from "vue-sonner";
import type { TaskOriginator, ProjectMember } from "~/types";

const props = defineProps<{
  originators: TaskOriginator[];
  projectKey: string;
  taskNum: number;
  members: ProjectMember[];
  isMember: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { addOriginator, removeOriginator } = useTasks();

const loading = ref<string | null>(null);

// A common shape both TaskOriginator (chips) and ProjectMember (dropdown)
// satisfy, so TokenSelect can treat them as one item type keyed by user_id.
type TokenMember = Pick<
  ProjectMember,
  "user_id" | "username" | "first_name" | "last_name" | "avatar_url"
>;

const selectedTokens = computed<TokenMember[]>(() => props.originators);

// Members not already a requester — the pool offered in the token dropdown.
// Only current members are offered, so a non-member can't be added; but chips
// above still show requesters who have since left the project.
const availableTokens = computed<TokenMember[]>(() => {
  const chosen = new Set(props.originators.map((o) => o.user_id));
  return props.members.filter((m) => !chosen.has(m.user_id));
});

// Requesters who are no longer members of this project (removed or deactivated).
// Their name still shows because it comes from the task, not the members list.
const formerMemberCount = computed(() => {
  const memberIds = new Set(props.members.map((m) => m.user_id));
  return props.originators.filter((o) => !memberIds.has(o.user_id)).length;
});

function memberSearchText(m: TokenMember) {
  return `${m.first_name} ${m.last_name} ${m.username}`;
}

async function handleAdd(userId: string) {
  loading.value = userId;
  const result = await addOriginator(props.projectKey, props.taskNum, userId);
  loading.value = null;

  if (result.success) {
    toast.success("Requester added");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to add requester");
  }
}

async function handleRemove(userId: string) {
  // A task must always keep at least one requester.
  if (props.originators.length <= 1) {
    toast.error("A task must have at least one requester");
    return;
  }
  loading.value = userId;
  const result = await removeOriginator(props.projectKey, props.taskNum, userId);
  loading.value = null;

  if (result.success) {
    toast.success("Requester removed");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to remove requester");
  }
}
</script>

<template>
  <div class="space-y-2">
    <p class="text-xs text-muted-foreground">Originator / Requester</p>

    <!-- Editable: Gmail-style token input -->
    <TokenSelect
      v-if="isMember"
      :selected="selectedTokens"
      :available="availableTokens"
      :get-key="(m) => m.user_id"
      :get-search-text="memberSearchText"
      :pending-key="loading"
      placeholder="Add requesters..."
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
      {{ formerMemberCount === 1 ? "requester is" : "requesters are" }}
      no longer a member of this project.
    </p>

    <!-- Read-only view -->
    <div v-else-if="!isMember" class="flex flex-wrap items-center gap-2">
      <NuxtLink
        v-for="originator in originators"
        :key="originator.id"
        :to="`/profile/${originator.user_id}`"
        class="flex items-center gap-1.5 rounded-md border bg-muted/50 py-1 pl-1 pr-2.5 transition-opacity hover:opacity-80"
      >
        <Avatar class="size-6">
          <AvatarImage v-if="originator.avatar_url" :src="originator.avatar_url" />
          <AvatarFallback class="text-xs" :seed="originator.user_id">
            {{ originator.first_name[0] }}{{ originator.last_name[0] }}
          </AvatarFallback>
        </Avatar>
        <span class="text-sm">
          {{ originator.first_name }} {{ originator.last_name }}
        </span>
      </NuxtLink>
      <span v-if="originators.length === 0" class="text-sm text-muted-foreground">
        No requester
      </span>
    </div>
  </div>
</template>
