export interface ActivityLogEntry {
  id: string;
  task_id: string;
  activity_type: ActivityType;
  actor_id: string;
  username: string;
  first_name: string;
  last_name: string;
  avatar_url?: string;
  field_name?: string;
  old_value?: unknown;
  new_value?: unknown;
  created_at: string;
}

export type ActivityType =
  | "task_created"
  | "task_updated"
  | "task_deleted"
  | "task_moved"
  | "assignee_added"
  | "assignee_removed"
  | "label_added"
  | "label_removed"
  | "state_changed"
  | "cycle_added"
  | "cycle_removed"
  | "module_added"
  | "module_removed"
  | "attachment_added"
  | "attachment_removed"
  | "comment_created"
  | "comment_updated"
  | "comment_deleted"
  | "mentioned";

export const ACTIVITY_TYPE_LABELS: Record<ActivityType, string> = {
  task_created: "created the task",
  task_updated: "updated",
  task_deleted: "deleted the task",
  task_moved: "moved the task",
  assignee_added: "added assignee",
  assignee_removed: "removed assignee",
  label_added: "added label",
  label_removed: "removed label",
  state_changed: "changed state",
  cycle_added: "added to a cycle",
  cycle_removed: "removed from a cycle",
  module_added: "added to an epic",
  module_removed: "removed from an epic",
  attachment_added: "added an attachment",
  attachment_removed: "removed an attachment",
  comment_created: "added a comment",
  comment_updated: "edited a comment",
  comment_deleted: "deleted a comment",
  mentioned: "mentioned you",
};

export interface VerifyActivityResponse {
  valid: boolean;
  message: string;
}

export interface UserActivityEntry extends ActivityLogEntry {
  task_number: number;
  project_key: string;
  task_title: string;
}

// A persisted, per-user notification. Extends the activity shape with batching
// (event_count) and read/unread state (is_read).
export interface NotificationEntry extends UserActivityEntry {
  event_count: number;
  is_read: boolean;
  updated_at: string;
  comment_id?: string;
}

export interface NotificationListResponse {
  activities: NotificationEntry[];
  unread_count: number;
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

export interface UserActivityDateCount {
  date: string;
  count: number;
}
