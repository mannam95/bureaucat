<script setup lang="ts">
import { toast } from "vue-sonner";
import type { TaskAssignee, ProjectMember } from "~/types";

const props = defineProps<{
  assignees: TaskAssignee[];
  projectKey: string;
  taskNum: number;
  members: ProjectMember[];
  isMember: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { addAssignee, removeAssignee } = useTasks();

const loading = ref<string | null>(null);

// A common shape both TaskAssignee (chips) and ProjectMember (dropdown) satisfy,
// so TokenSelect can treat them as one item type keyed by user_id.
type TokenMember = Pick<
  ProjectMember,
  "user_id" | "username" | "first_name" | "last_name" | "avatar_url"
>;

const selectedTokens = computed<TokenMember[]>(() => props.assignees);

// Members not already assigned — the pool offered in the token dropdown.
const availableTokens = computed<TokenMember[]>(() => {
  const assignedIds = new Set(props.assignees.map((a) => a.user_id));
  return props.members.filter((m) => !assignedIds.has(m.user_id));
});

function memberSearchText(m: TokenMember) {
  return `${m.first_name} ${m.last_name} ${m.username}`;
}

async function handleAdd(userId: string) {
  loading.value = userId;
  const result = await addAssignee(props.projectKey, props.taskNum, userId);
  loading.value = null;

  if (result.success) {
    toast.success("Assignee added");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to add assignee");
  }
}

async function handleRemove(userId: string) {
  loading.value = userId;
  const result = await removeAssignee(props.projectKey, props.taskNum, userId);
  loading.value = null;

  if (result.success) {
    toast.success("Assignee removed");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to remove assignee");
  }
}
</script>

<template>
  <div class="space-y-2">
    <p class="text-xs text-muted-foreground">Assignees</p>

    <!-- Editable: Gmail-style token input -->
    <TokenSelect
      v-if="isMember"
      :selected="selectedTokens"
      :available="availableTokens"
      :get-key="(m) => m.user_id"
      :get-search-text="memberSearchText"
      :pending-key="loading"
      placeholder="Add assignees..."
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

    <!-- Read-only view -->
    <div v-else class="flex flex-wrap items-center gap-2">
      <NuxtLink
        v-for="assignee in assignees"
        :key="assignee.id"
        :to="`/profile/${assignee.user_id}`"
        class="flex items-center gap-1.5 rounded-md border bg-muted/50 py-1 pl-1 pr-2.5 transition-opacity hover:opacity-80"
      >
        <Avatar class="size-6">
          <AvatarImage v-if="assignee.avatar_url" :src="assignee.avatar_url" />
          <AvatarFallback class="text-xs" :seed="assignee.user_id">
            {{ assignee.first_name[0] }}{{ assignee.last_name[0] }}
          </AvatarFallback>
        </Avatar>
        <span class="text-sm">
          {{ assignee.first_name }} {{ assignee.last_name }}
        </span>
      </NuxtLink>
      <span
        v-if="assignees.length === 0"
        class="text-sm text-muted-foreground"
      >
        No assignees
      </span>
    </div>
  </div>
</template>
