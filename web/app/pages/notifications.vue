<script setup lang="ts">
import { Bell, Loader2, Trash2, Circle, ChevronLeft, ChevronRight } from "lucide-vue-next";
import type { NotificationEntry } from "~/types";
import { ACTIVITY_TYPE_LABELS } from "~/types";
import { NOTIFICATION_ICONS, formatNotificationDate, notificationLink } from "~/utils/notifications";

definePageMeta({ middleware: ["auth"] });
useHead({ title: "Notifications" });

const { user } = useAuth();
const { listNotifications, markRead, clearAll } = useNotifications();

const items = ref<NotificationEntry[]>([]);
const loading = ref(true);
const page = ref(1);
const totalPages = ref(1);
const total = ref(0);
const perPage = 30;

async function load(p = 1) {
  if (!user.value) return;
  loading.value = true;
  const { success, data } = await listNotifications(p, perPage);
  if (success && data) {
    items.value = data.activities ?? [];
    page.value = data.page ?? p;
    totalPages.value = data.total_pages ?? 1;
    total.value = data.total ?? items.value.length;
  }
  loading.value = false;
}

function goTo(p: number) {
  if (p < 1 || p > totalPages.value || p === page.value) return;
  load(p);
}

function onItemClick(a: NotificationEntry) {
  if (!a.is_read) {
    a.is_read = true;
    markRead(a.id);
  }
}

const showDeleteConfirm = ref(false);
const deleting = ref(false);

async function onDeleteAll() {
  deleting.value = true;
  const ok = await clearAll();
  deleting.value = false;
  if (ok) {
    items.value = [];
    total.value = 0;
    totalPages.value = 1;
    page.value = 1;
  }
  showDeleteConfirm.value = false;
}

onMounted(() => load(1));
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <Navbar />
    <main id="main-content" class="flex-1">
      <div class="mx-auto max-w-3xl px-6 py-8">
        <header class="mb-6 flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-3">
            <div class="flex size-10 items-center justify-center rounded-lg bg-muted">
              <Bell class="size-5 text-amber-600 dark:text-amber-500" />
            </div>
            <div>
              <h1 class="text-2xl font-bold tracking-tight">Notifications</h1>
              <p class="text-sm text-muted-foreground">
                {{ total }} notification{{ total === 1 ? "" : "s" }}
              </p>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <Button
              v-if="items.length > 0"
              variant="outline"
              size="sm"
              class="h-9 text-muted-foreground hover:text-destructive"
              @click="showDeleteConfirm = true"
            >
              <Trash2 class="mr-1.5 size-4" />
              Delete all
            </Button>
          </div>
        </header>

        <div v-if="loading" class="flex items-center justify-center py-16">
          <Loader2 class="size-6 animate-spin text-muted-foreground" />
        </div>

        <div
          v-else-if="items.length === 0"
          class="flex flex-col items-center justify-center rounded-lg border border-dashed py-20 text-center"
        >
          <div class="flex size-14 items-center justify-center rounded-full bg-muted">
            <Bell class="size-6 text-muted-foreground" />
          </div>
          <h3 class="mt-4 text-base font-semibold">No notifications</h3>
          <p class="mt-1 max-w-sm text-sm text-muted-foreground">
            Mentions, assignments and updates on tasks you follow show up here.
          </p>
        </div>

        <template v-else>
          <ul class="overflow-hidden rounded-lg border bg-background">
            <li v-for="a in items" :key="a.id">
              <NuxtLink
                :to="notificationLink(a)"
                class="flex gap-3 border-b border-border/60 px-4 py-3 transition-colors last:border-0 hover:bg-muted/50"
                :class="a.is_read ? '' : 'bg-amber-500/5'"
                @click="onItemClick(a)"
              >
                <!-- Unread marker rail -->
                <span
                  class="mt-1 w-1 shrink-0 self-stretch rounded-full"
                  :class="a.is_read ? 'bg-transparent' : 'bg-amber-500'"
                  aria-hidden="true"
                />
                <div class="flex size-7 shrink-0 items-center justify-center rounded-full border bg-background">
                  <component
                    :is="NOTIFICATION_ICONS[a.activity_type] || Circle"
                    class="size-3.5 text-muted-foreground"
                  />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-sm" :class="a.is_read ? '' : 'font-medium'">
                    <span class="font-medium text-foreground">{{ a.first_name }} {{ a.last_name }}</span>
                    <span :class="a.is_read ? 'text-muted-foreground' : 'text-foreground'">
                      {{ ' ' }}{{ ACTIVITY_TYPE_LABELS[a.activity_type] || a.activity_type }}
                      <template v-if="a.field_name">
                        <span class="font-medium text-foreground">{{ a.field_name }}</span>
                      </template>
                    </span>
                    <span v-if="a.event_count > 1" class="text-muted-foreground">
                      {{ ' ' }}· {{ a.event_count }} updates
                    </span>
                  </p>
                  <div class="mt-0.5 flex items-center gap-1.5">
                    <span class="text-xs font-medium text-amber-600 dark:text-amber-500">
                      {{ a.project_key }}-{{ a.task_number }}
                    </span>
                    <span class="truncate text-xs text-muted-foreground">{{ a.task_title }}</span>
                  </div>
                  <p class="mt-0.5 text-[11px] text-muted-foreground/70">
                    {{ formatNotificationDate(a.updated_at || a.created_at) }}
                  </p>
                </div>
                <span
                  v-if="!a.is_read"
                  class="mt-1 inline-flex items-center rounded-full bg-amber-500/15 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-amber-700 dark:text-amber-400"
                >
                  New
                </span>
              </NuxtLink>
            </li>
          </ul>

          <div
            v-if="totalPages > 1"
            class="mt-4 flex items-center justify-between border-t pt-4"
          >
            <p class="text-sm text-muted-foreground">Page {{ page }} of {{ totalPages }}</p>
            <div class="flex items-center gap-2">
              <Button variant="outline" size="sm" :disabled="page <= 1" @click="goTo(page - 1)">
                <ChevronLeft class="size-4" />
              </Button>
              <Button variant="outline" size="sm" :disabled="page >= totalPages" @click="goTo(page + 1)">
                <ChevronRight class="size-4" />
              </Button>
            </div>
          </div>
        </template>
      </div>
    </main>

    <!-- Delete-all confirmation -->
    <Dialog v-model:open="showDeleteConfirm">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Delete all notifications?</DialogTitle>
          <DialogDescription>
            This permanently removes your entire notification history. This can't
            be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" :disabled="deleting" @click="showDeleteConfirm = false">
            Cancel
          </Button>
          <Button variant="destructive" :disabled="deleting" @click="onDeleteAll">
            <Loader2 v-if="deleting" class="mr-1.5 size-4 animate-spin" />
            Delete all
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
