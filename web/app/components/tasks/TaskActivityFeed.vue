<script setup lang="ts">
import {
  History,
  Loader2,
  ShieldCheck,
  ShieldAlert,
  Plus,
  Edit2,
  Trash2,
  UserPlus,
  UserMinus,
  Tag,
  Tags,
  ArrowRight,
  MessageSquare,
  Circle,
  ArrowUpDown,
  Repeat,
  Layers,
  Paperclip,
  AtSign,
  Link2,
  Link2Off,
  Workflow,
} from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { ActivityLogEntry, ActivityType, Comment, ProjectMember } from "~/types";
import { ACTIVITY_TYPE_LABELS, activityActorName, activityLabel } from "~/types";

const props = defineProps<{
  activities: ActivityLogEntry[];
  comments: Comment[];
  projectKey: string;
  taskNum: number;
  activitiesLoading?: boolean;
  commentsLoading?: boolean;
  isMember: boolean;
  members: ProjectMember[];
}>();

const emit = defineEmits<{
  refreshComments: [];
  refreshActivity: [];
}>();

const { user } = useAuth();
const { verifyActivity } = useActivity();

const verifying = ref(false);
const verificationResult = ref<{ valid: boolean; message: string } | null>(null);

// Newest/oldest activity ordering, persisted globally through the preference
// store so it follows the user across tasks and devices.
const activitySort = usePreferences().globalRef<"newest" | "oldest">(
  "tasks.detail.activity_sort",
  "newest",
);
const newestFirst = computed(() => activitySort.value === "newest");

function toggleSort() {
  activitySort.value = newestFirst.value ? "oldest" : "newest";
}

// Icon map for activity types (Partial + a Circle fallback at the call site so
// new types are safe even before an icon is chosen).
const iconMap: Partial<Record<ActivityType, typeof Plus>> = {
  task_created: Plus,
  task_updated: Edit2,
  task_deleted: Trash2,
  task_moved: ArrowUpDown,
  assignee_added: UserPlus,
  assignee_removed: UserMinus,
  label_added: Tag,
  label_removed: Tags,
  state_changed: ArrowRight,
  cycle_added: Repeat,
  cycle_removed: Repeat,
  module_added: Layers,
  module_removed: Layers,
  attachment_added: Paperclip,
  attachment_removed: Paperclip,
  comment_created: MessageSquare,
  comment_updated: Edit2,
  comment_deleted: Trash2,
  mentioned: AtSign,
  blocker_added: Link2,
  blocker_removed: Link2Off,
};

interface ActivityFeedItem {
  type: "activity";
  data: ActivityLogEntry;
  merged: ActivityLogEntry[];
  workflow: ActivityLogEntry[];
  timestamp: Date;
}

type FeedItem = ActivityFeedItem | { type: "comment"; data: Comment; timestamp: Date };

const MERGE_WINDOW_MS = 60_000;
const MERGEABLE = new Set<ActivityType>(["assignee_added", "assignee_removed", "label_added", "label_removed"]);
const SEGMENT_LABELS: Partial<Record<ActivityType, [string, string]>> = {
  assignee_added: ["added assignee", "added assignees"],
  assignee_removed: ["removed assignee", "removed assignees"],
  label_added: ["added label", "added labels"],
  label_removed: ["removed label", "removed labels"],
};

function lastAt(item: ActivityFeedItem): number {
  const all = [...item.merged, ...item.workflow];
  return new Date(all[all.length - 1]!.created_at).getTime();
}

// Workflow assignee changes fold into the state change/creation that triggered them;
// back-to-back assignee/label changes by the same actor merge into one row.
function groupActivities(activities: ActivityLogEntry[]): ActivityFeedItem[] {
  const sorted = [...activities].sort(
    (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  );
  const out: ActivityFeedItem[] = [];
  const triggerByActor = new Map<string, ActivityFeedItem>();
  let prev: ActivityFeedItem | undefined;

  for (const activity of sorted) {
    const type = activity.activity_type;
    if (type === "comment_created" || type === "comment_updated" || type === "comment_deleted") continue;
    const at = new Date(activity.created_at).getTime();

    if (activity.via === "workflow" && (type === "assignee_added" || type === "assignee_removed")) {
      const trigger = triggerByActor.get(activity.actor_id);
      if (trigger && at - lastAt(trigger) <= MERGE_WINDOW_MS) {
        trigger.workflow.push(activity);
        continue;
      }
    }

    if (
      prev &&
      MERGEABLE.has(type) &&
      MERGEABLE.has(prev.data.activity_type) &&
      prev.data.actor_id === activity.actor_id &&
      prev.data.via === activity.via &&
      at - lastAt(prev) <= MERGE_WINDOW_MS
    ) {
      prev.merged.push(activity);
      continue;
    }

    const item: ActivityFeedItem = {
      type: "activity",
      data: activity,
      merged: [activity],
      workflow: [],
      timestamp: new Date(at),
    };
    out.push(item);
    prev = item;
    if (activity.via !== "workflow" && (type === "state_changed" || type === "task_created")) {
      triggerByActor.set(activity.actor_id, item);
    }
  }
  return out;
}

function mergedSegments(entries: ActivityLogEntry[]): { type: ActivityType; label: string; names: string }[] {
  const segments: { type: ActivityType; label: string; names: string }[] = [];
  for (const type of new Set(entries.map((e) => e.activity_type))) {
    const names = entries
      .filter((e) => e.activity_type === type)
      .map((e) => getActivityDetail(e))
      .filter(Boolean) as string[];
    const [singular, plural] = SEGMENT_LABELS[type] ?? [ACTIVITY_TYPE_LABELS[type], ACTIVITY_TYPE_LABELS[type]];
    segments.push({ type, label: names.length > 1 ? plural : singular, names: names.join(", ") });
  }
  return segments;
}

function workflowChanges(entries: ActivityLogEntry[]): string {
  return [...new Set(entries.map((e) => e.activity_type))]
    .map((type) => {
      const names = entries
        .filter((e) => e.activity_type === type)
        .map((e) => getActivityDetail(e) ?? "someone")
        .join(", ");
      return `${type === "assignee_added" ? "assigned" : "unassigned"} ${names}`;
    })
    .join(" · ");
}

// Extract comment edit history from activity logs
interface CommentVersion {
  content: string;
  version: number;
  editedAt: string;
  editedBy: string;
}

const commentEditHistory = computed<Map<string, CommentVersion[]>>(() => {
  const history = new Map<string, CommentVersion[]>();

  for (const activity of props.activities) {
    if (activity.activity_type === "comment_updated") {
      const oldData = parseActivityValue(activity.old_value);
      if (oldData?.comment_id && oldData?.content) {
        const commentId = oldData.comment_id as string;
        if (!history.has(commentId)) {
          history.set(commentId, []);
        }
        history.get(commentId)!.push({
          content: oldData.content as string,
          version: oldData.version as number,
          editedAt: activity.created_at,
          editedBy: `${activity.first_name} ${activity.last_name}`,
        });
      }
    }
  }

  // Sort each comment's history by version descending (newest first)
  for (const [, versions] of history) {
    versions.sort((a, b) => b.version - a.version);
  }

  return history;
});

// Lens: which slice of the feed is shown. Always opens on Comments (the
// content people fear missing); deliberately not persisted.
type FeedLens = "comments" | "activity" | "all";
const lens = ref<FeedLens>("comments");

// System entries, grouped: workflow-triggered assignee changes fold into their
// trigger, back-to-back changes by the same actor merge, and comment lifecycle
// events are skipped (the actual comments are shown instead).
const activityItems = computed<FeedItem[]>(() => groupActivities(props.activities));

const commentItems = computed<FeedItem[]>(() =>
  props.comments.map((c) => ({
    type: "comment" as const,
    data: c,
    timestamp: new Date(c.created_at),
  }))
);

// The visible slice, sorted by the one shared newest/oldest preference.
const feedItems = computed<FeedItem[]>(() => {
  const items: FeedItem[] = [];
  if (lens.value !== "comments") items.push(...activityItems.value);
  if (lens.value !== "activity") items.push(...commentItems.value);

  items.sort((a, b) =>
    newestFirst.value
      ? b.timestamp.getTime() - a.timestamp.getTime()
      : a.timestamp.getTime() - b.timestamp.getTime()
  );

  return items;
});

const LENSES: { key: FeedLens; label: string }[] = [
  { key: "comments", label: "Comments" },
  { key: "activity", label: "Activity" },
  { key: "all", label: "All" },
];

function lensCount(key: FeedLens): number | null {
  if (key === "comments") return commentItems.value.length;
  if (key === "activity") return activityItems.value.length;
  return null;
}

// Comments come from the comments table, so their channel is read from the matching comment_created activity.
const commentVia = computed(() => {
  const map = new Map<string, ActivityLogEntry["via"]>();
  for (const activity of props.activities) {
    if (activity.activity_type !== "comment_created") continue;
    const id = parseActivityValue(activity.new_value)?.comment_id;
    if (typeof id === "string") map.set(id, activity.via);
  }
  return map;
});

function getCommentHistory(commentId: string): CommentVersion[] {
  return commentEditHistory.value.get(commentId) || [];
}

const loading = computed(
  () => props.activitiesLoading || props.commentsLoading
);

function canEditComment(comment: Comment): boolean {
  return props.isMember && comment.created_by === user.value?.id;
}

async function handleVerify() {
  verifying.value = true;
  verificationResult.value = null;

  const result = await verifyActivity(props.projectKey, props.taskNum);

  verifying.value = false;

  if (result.success && result.data) {
    verificationResult.value = result.data;
    if (result.data.valid) {
      toast.success("Activity log verified successfully");
    } else {
      toast.error("Activity log integrity compromised");
    }
  } else {
    toast.error(result.error || "Failed to verify activity");
  }
}

function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: date.getFullYear() !== now.getFullYear() ? "numeric" : undefined,
  });
}

function getActivityLabel(activity: ActivityLogEntry): string {
  return activityLabel(activity);
}

function getFieldLabel(fieldName?: string): string | null {
  if (!fieldName) return null;

  const labels: Record<string, string> = {
    title: "title",
    description: "description",
    state: "state",
    priority: "priority",
    assignees: "assignees",
    labels: "labels",
    content: "content",
  };
  return labels[fieldName] || fieldName;
}

// Parse activity value to extract meaningful details
function parseActivityValue(value: unknown): Record<string, unknown> | null {
  if (!value) return null;

  // If it's a string (JSON), try to parse it
  if (typeof value === "string") {
    try {
      return JSON.parse(value);
    } catch {
      return null;
    }
  }

  // If it's already an object
  if (typeof value === "object") {
    return value as Record<string, unknown>;
  }

  return null;
}

// Get display text for activity details (assignee name, label name, etc.)
// parseScalarValue returns a logged scalar (string, number, boolean) as-is;
// JSON-encoded strings are decoded first. Objects return null - they belong
// to parseActivityValue.
function parseScalarValue(value: unknown): unknown {
  if (value === null || value === undefined) return null;
  let v = value;
  if (typeof v === "string") {
    try {
      v = JSON.parse(v);
    } catch {
      // a plain string that never was JSON
    }
  }
  return typeof v === "object" ? null : v;
}

// formatFieldValue renders a logged field value for the feed: dates read as
// dates, long strings truncate, empty reads as a dash.
function formatFieldValue(value: unknown): string | null {
  const v = parseScalarValue(value);
  if (v === null || v === undefined || v === "") return null;
  if (typeof v === "string") {
    if (/^\d{4}-\d{2}-\d{2}T/.test(v)) {
      const d = new Date(v);
      if (!isNaN(d.getTime())) {
        return d.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
      }
    }
    return v.length > 60 ? v.slice(0, 57) + "…" : v;
  }
  return String(v);
}

// Activity types whose payload names a person.
const PERSON_EVENTS = new Set<ActivityType>([
  "assignee_added", "assignee_removed",
  "originator_added", "originator_removed",
  "watcher_added", "watcher_removed",
  "contributor_added", "contributor_removed",
]);

// getActivityDetail answers "who or what, exactly" for a feed row: the person
// added or removed, the label/area/cycle/epic/file by name, the blocker by
// key, and for field updates the old → new values. Every answer comes from the
// logged payload, so the feed reads the history, not today's state.
function getActivityDetail(activity: ActivityLogEntry): string | null {
  const type = activity.activity_type;
  const added = !type.endsWith("_removed");
  const data = parseActivityValue(added ? activity.new_value : activity.old_value);

  if (PERSON_EVENTS.has(type)) {
    if (data?.first_name || data?.last_name) {
      return `${data.first_name ?? ""} ${data.last_name ?? ""}`.trim();
    }
    if (data?.username) return data.username as string;
  }

  if (type === "label_added" || type === "label_removed" || type === "area_added" || type === "area_removed") {
    if (data?.name) return data.name as string;
  }

  if (type === "cycle_added" || type === "cycle_removed" || type === "module_added" || type === "module_removed") {
    if (data?.title) return data.title as string;
  }

  if (type === "attachment_added" || type === "attachment_removed") {
    if (data?.filename) return data.filename as string;
  }

  if (type === "blocker_added" || type === "blocker_removed") {
    if (data?.task_id) {
      return data.title ? `${data.task_id} ${data.title}` : (data.task_id as string);
    }
  }

  if (type === "task_created" && data?.copied_from) {
    return `copied from ${data.copied_from}`;
  }

  if (type === "task_moved") {
    const from = parseActivityValue(activity.old_value)?.project_key;
    const to = parseActivityValue(activity.new_value)?.project_key;
    if (from && to) return `${from} → ${to}`;
  }

  if (type === "task_updated" && activity.field_name) {
    // The description is content, not a value - "updated description" is enough.
    if (activity.field_name === "description") return null;
    const from = formatFieldValue(activity.old_value);
    const to = formatFieldValue(activity.new_value);
    if (from && to) return `${from} → ${to}`;
    if (to) return to;
    if (from) return `${from} → —`;
  }

  return null;
}

// Get state change details separately for special formatting
function getStateChangeDetail(activity: ActivityLogEntry): { from: string; to: string } | null {
  if (activity.activity_type !== "state_changed") return null;

  const oldData = parseActivityValue(activity.old_value);
  const newData = parseActivityValue(activity.new_value);

  if (oldData?.name && newData?.name) {
    return {
      from: oldData.name as string,
      to: newData.name as string,
    };
  }

  return null;
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex items-center gap-3">
        <h3 class="flex items-center gap-2 font-semibold">
          <History class="size-4" />
          Activity
        </h3>
        <!-- Lens: comments, system activity, or the interleaved story. -->
        <div class="flex items-center rounded-md border p-0.5">
          <button
            v-for="l in LENSES"
            :key="l.key"
            type="button"
            class="flex items-center gap-1 rounded px-2 py-1 text-xs font-medium transition-colors"
            :class="lens === l.key
              ? 'bg-muted text-foreground'
              : 'text-muted-foreground hover:text-foreground'"
            @click="lens = l.key"
          >
            {{ l.label }}
            <span
              v-if="lensCount(l.key) !== null"
              class="rounded-full bg-muted-foreground/10 px-1 text-[10px] tabular-nums"
            >
              {{ lensCount(l.key) }}
            </span>
          </button>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          @click="toggleSort"
        >
          <ArrowUpDown class="mr-1.5 size-3.5" />
          {{ newestFirst ? "Newest first" : "Oldest first" }}
        </Button>
        <Button
          variant="outline"
          size="sm"
          :disabled="verifying"
          @click="handleVerify"
        >
          <Loader2 v-if="verifying" class="mr-1.5 size-3.5 animate-spin" />
          <ShieldCheck v-else class="mr-1.5 size-3.5" />
          Verify Log
        </Button>
      </div>
    </div>

    <!-- Verification result -->
    <Alert
      v-if="verificationResult"
      :variant="verificationResult.valid ? 'success' : 'destructive'"
    >
      <component
        :is="verificationResult.valid ? ShieldCheck : ShieldAlert"
        class="size-4"
      />
      <AlertTitle>
        {{ verificationResult.valid ? "Verified" : "Integrity Issue" }}
      </AlertTitle>
      <AlertDescription>
        {{ verificationResult.message }}
      </AlertDescription>
    </Alert>

    <!-- Comment Form (newest first = top) -->
    <CommentForm
      v-if="isMember && newestFirst"
      :project-key="projectKey"
      :task-num="taskNum"
      :members="members"
      @created="emit('refreshComments')"
    />

    <!-- Loading -->
    <div v-if="loading && feedItems.length === 0" class="flex items-center justify-center py-8">
      <Loader2 class="size-5 animate-spin text-muted-foreground" />
    </div>

    <!-- Feed -->
    <div v-else-if="feedItems.length > 0" class="space-y-0">
      <div
        v-for="(item, index) in feedItems"
        :key="item.type === 'comment' ? `c-${item.data.id}` : `a-${item.data.id}`"
        class="relative flex gap-3 pb-6"
      >
        <!-- Timeline line -->
        <div
          v-if="index < feedItems.length - 1"
          class="absolute left-[11px] top-6 h-full w-px bg-border"
        />

        <!-- Activity item -->
        <template v-if="item.type === 'activity'">
          <div
            class="relative z-10 flex size-6 shrink-0 items-center justify-center rounded-full border bg-background"
          >
            <component
              :is="iconMap[item.data.activity_type] || Circle"
              class="size-3 text-muted-foreground"
            />
          </div>
          <div class="min-w-0 flex-1 pt-0.5">
            <p class="text-sm">
              <span class="font-medium">
                {{ activityActorName(item.data) }}
              </span>
              <ViaBadge
                :via="item.data.via"
                :triggered-by="`${item.data.first_name} ${item.data.last_name}`"
                class="ml-1"
              />
              <span v-if="item.merged.length > 1" class="text-muted-foreground">
                <template v-for="(seg, i) in mergedSegments(item.merged)" :key="seg.type">
                  {{ i > 0 ? " · " : " " }}{{ seg.label }}
                  <span class="font-medium">{{ seg.names }}</span>
                </template>
              </span>
              <span v-else class="text-muted-foreground">
                {{ " " }}{{ getActivityLabel(item.data) }}
                <template v-if="getStateChangeDetail(item.data)">
                  from <span class="font-medium">{{ getStateChangeDetail(item.data)!.from }}</span>
                  to <span class="font-medium">{{ getStateChangeDetail(item.data)!.to }}</span>
                </template>
                <template v-else-if="item.data.activity_type === 'task_updated' && getFieldLabel(item.data.field_name)">
                  <span class="font-medium">{{ getFieldLabel(item.data.field_name) }}</span>
                  <template v-if="getActivityDetail(item.data)">
                    : <span class="font-medium">{{ getActivityDetail(item.data) }}</span>
                  </template>
                </template>
                <template v-else-if="getActivityDetail(item.data)">
                  <span class="font-medium">{{ getActivityDetail(item.data) }}</span>
                </template>
                <template v-else-if="getFieldLabel(item.data.field_name)">
                  <span class="font-medium">{{
                    getFieldLabel(item.data.field_name)
                  }}</span>
                </template>
              </span>
            </p>
            <p
              v-if="item.workflow.length"
              class="mt-1 flex items-start gap-1.5 text-xs text-muted-foreground"
              title="Assignee changes made by workflow rules"
            >
              <Workflow class="mt-0.5 size-3 shrink-0" />
              <span>Workflow: {{ workflowChanges(item.workflow) }}</span>
            </p>
            <p class="mt-0.5 text-xs text-muted-foreground">
              {{ formatDate(item.data.created_at) }}
            </p>
          </div>
        </template>

        <!-- Comment item -->
        <template v-else>
          <Avatar class="relative z-10 size-6 shrink-0">
            <AvatarImage v-if="item.data.avatar_url" :src="item.data.avatar_url" />
            <AvatarFallback class="text-[10px]" :seed="item.data.created_by">
              {{ item.data.first_name?.[0] }}{{ item.data.last_name?.[0] }}
            </AvatarFallback>
          </Avatar>
          <div class="min-w-0 flex-1">
            <CommentItem
              :comment="item.data"
              :project-key="projectKey"
              :task-num="taskNum"
              :can-edit="canEditComment(item.data)"
              :edit-history="getCommentHistory(item.data.id)"
              :via="commentVia.get(item.data.id)"
              :members="members"
              compact
              @deleted="emit('refreshComments')"
              @updated="emit('refreshComments')"
            />
          </div>
        </template>
      </div>
    </div>

    <!-- Empty state, phrased for the visible lens -->
    <div
      v-else
      class="rounded-lg border border-dashed py-8 text-center text-sm text-muted-foreground"
    >
      {{ lens === "comments" ? "No comments yet" : lens === "activity" ? "No activity yet" : "Nothing here yet" }}
    </div>

    <!-- Comment Form (oldest first = bottom) -->
    <CommentForm
      v-if="isMember && !newestFirst"
      :project-key="projectKey"
      :task-num="taskNum"
      :members="members"
      @created="emit('refreshComments')"
    />
  </div>
</template>
