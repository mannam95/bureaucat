<script setup lang="ts">
import { toast } from "vue-sonner";
import type { TaskContributor, ProjectMember } from "~/types";

const props = defineProps<{
  contributors: TaskContributor[];
  projectKey: string;
  taskNum: number;
  members: ProjectMember[];
  isMember: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
}>();

const { addContributor, removeContributor } = useTasks();

const loading = ref<string | null>(null);

// A common shape both TaskContributor (chips) and ProjectMember (dropdown)
// satisfy, so TokenSelect can treat them as one item type keyed by user_id.
type TokenMember = Pick<
  ProjectMember,
  "user_id" | "username" | "first_name" | "last_name" | "avatar_url"
>;

const selectedTokens = computed<TokenMember[]>(() => props.contributors);

// Members not already recorded — the pool offered in the token dropdown. Only
// current members are offered; chips above still show contributors who left.
const availableTokens = computed<TokenMember[]>(() => {
  const chosen = new Set(props.contributors.map((c) => c.user_id));
  return props.members.filter((m) => !chosen.has(m.user_id));
});

// "auto" rows came from the hand-over rule (unassigned while the task was in a
// started-type state); surfaced as a tooltip so a wrong auto-credit is easy to
// spot and remove.
const sourceByUser = computed(() => {
  const map = new Map<string, string>();
  for (const c of props.contributors) map.set(c.user_id, c.source);
  return map;
});

function sourceTitle(userId: string): string | undefined {
  const source = sourceByUser.value.get(userId);
  if (source === "auto") return "Recorded automatically when unassigned during work";
  if (source === "backfill") return "Recorded by the history backfill";
  return undefined;
}

function memberSearchText(m: TokenMember) {
  return `${m.first_name} ${m.last_name} ${m.username}`;
}

async function handleAdd(userId: string) {
  loading.value = userId;
  const result = await addContributor(props.projectKey, props.taskNum, userId);
  loading.value = null;

  if (result.success) {
    toast.success("Contributor added");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to add contributor");
  }
}

async function handleRemove(userId: string) {
  loading.value = userId;
  const result = await removeContributor(props.projectKey, props.taskNum, userId);
  loading.value = null;

  if (result.success) {
    toast.success("Contributor removed");
    emit("refresh");
  } else {
    toast.error(result.error || "Failed to remove contributor");
  }
}
</script>

<template>
  <div class="space-y-2">
    <p class="text-xs text-muted-foreground">Contributors</p>

    <!-- Editable: Gmail-style token input -->
    <TokenSelect
      v-if="isMember"
      :selected="selectedTokens"
      :available="availableTokens"
      :get-key="(m) => m.user_id"
      :get-search-text="memberSearchText"
      :pending-key="loading"
      placeholder="Add contributors..."
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
        <span class="truncate" :title="sourceTitle(member.user_id)">
          {{ member.first_name }} {{ member.last_name }}
        </span>
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
        v-for="contributor in contributors"
        :key="contributor.id"
        :to="`/profile/${contributor.user_id}`"
        class="flex items-center gap-1.5 rounded-md border bg-muted/50 py-1 pl-1 pr-2.5 transition-opacity hover:opacity-80"
      >
        <Avatar class="size-6">
          <AvatarImage v-if="contributor.avatar_url" :src="contributor.avatar_url" />
          <AvatarFallback class="text-xs" :seed="contributor.user_id">
            {{ contributor.first_name[0] }}{{ contributor.last_name[0] }}
          </AvatarFallback>
        </Avatar>
        <span class="text-sm">
          {{ contributor.first_name }} {{ contributor.last_name }}
        </span>
      </NuxtLink>
      <span v-if="contributors.length === 0" class="text-sm text-muted-foreground">
        No contributors
      </span>
    </div>
  </div>
</template>
