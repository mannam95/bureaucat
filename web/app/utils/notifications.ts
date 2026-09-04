import type { Component } from "vue";
import {
  Plus,
  Edit2,
  Trash2,
  UserPlus,
  UserMinus,
  Tag,
  Tags,
  ArrowRight,
  ArrowUpDown,
  Repeat,
  Layers,
  Paperclip,
  MessageSquarePlus,
  MessageSquareDiff,
  MessageSquareX,
  AtSign,
} from "lucide-vue-next";
import type { ActivityType, NotificationEntry } from "~/types";

// Icon per activity type, shared by the notification popover and the full page.
// Partial + a Circle fallback at the call site keeps new activity types safe.
export const NOTIFICATION_ICONS: Partial<Record<ActivityType, Component>> = {
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
  comment_created: MessageSquarePlus,
  comment_updated: MessageSquareDiff,
  comment_deleted: MessageSquareX,
  mentioned: AtSign,
};

// Compact "just now / 5m ago / Mar 3" relative time used across notifications.
export function formatNotificationDate(dateStr: string): string {
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

// Deep-link to the task, and to the specific comment when the notification is
// tied to one, so the task page scrolls to and highlights it.
export function notificationLink(a: NotificationEntry): string {
  const base = `/projects/${a.project_key}/tasks/${a.task_number}`;
  return a.comment_id ? `${base}#comment-${a.comment_id}` : base;
}
