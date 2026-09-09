<script setup lang="ts">
import { Users, Plus, Trash2, Loader2, ChevronLeft, ChevronRight, Shield, ShieldOff, KeyRound, Search, X, Lock, UserCheck, UserX, RotateCcw } from "lucide-vue-next";

definePageMeta({
  middleware: ["admin"],
});

useSeoMeta({ title: "Manage Users" });

const { listUsers, createUser, deleteUser, updateUserRole, resetUserPassword, setUserActive, restoreUser } = useAdmin();

type UserStatus = "active" | "deactivated" | "deleted";

interface User {
  id: string;
  username: string;
  email: string;
  first_name: string;
  last_name: string;
  user_type: string;
  created_at: string;
  // True for the protected break-glass superadmin (from SUPERADMIN_EMAIL).
  is_super_admin?: boolean;
  // True when an admin has deactivated the account (kept, but cannot sign in).
  is_deactivated?: boolean;
  is_deleted?: boolean;
  deactivated_at?: string;
  deactivated_by_name?: string;
  deleted_at?: string;
  deleted_by_name?: string;
}

// State
const users = ref<User[]>([]);
const loading = ref(true);
const page = ref(1);
const perPage = ref(20);
const total = ref(0);
const totalPages = ref(0);
const error = ref<string | null>(null);
const searchQuery = ref("");
let searchDebounce: ReturnType<typeof setTimeout> | null = null;

// Which state's tab is open, and the per-state counts shown on the tabs.
const status = ref<UserStatus>("active");
const counts = ref({ active: 0, deactivated: 0, deleted: 0 });

const tabs = computed(() => [
  { key: "active" as UserStatus, label: "Active", count: counts.value.active },
  { key: "deactivated" as UserStatus, label: "Deactivated", count: counts.value.deactivated },
  { key: "deleted" as UserStatus, label: "Deleted", count: counts.value.deleted },
]);

const statusHint = computed(() => {
  switch (status.value) {
    case "deactivated":
      return "These users can't sign in. Reactivate an account to restore access.";
    case "deleted":
      return "Deleted users are hidden and can't sign in. Restore brings an account back as active; nothing is erased.";
    default:
      return "Active users can sign in and use Bureaucat normally.";
  }
});

const whenColumnLabel = computed(() =>
  status.value === "deactivated" ? "Deactivated" : status.value === "deleted" ? "Deleted" : "Created"
);

const searchPlaceholder = computed(() => `Search ${status.value} users by username, email, or name...`);

function whenDate(user: User): string {
  if (status.value === "deactivated") return user.deactivated_at || user.created_at;
  if (status.value === "deleted") return user.deleted_at || user.created_at;
  return user.created_at;
}

function whenActor(user: User): string {
  if (status.value === "deactivated") return user.deactivated_by_name || "";
  if (status.value === "deleted") return user.deleted_by_name || "";
  return "";
}

function selectStatus(s: UserStatus) {
  if (status.value === s) return;
  status.value = s;
  page.value = 1;
  fetchUsers();
}

// Create dialog state
const showCreateDialog = ref(false);
const createLoading = ref(false);
const createError = ref<string | null>(null);
const createForm = ref({
  username: "",
  email: "",
  password: "",
  first_name: "",
  last_name: "",
  user_type: "user",
});

// Delete dialog state
const showDeleteDialog = ref(false);
const deleteLoading = ref(false);
const userToDelete = ref<User | null>(null);

// Role toggle dialog state
const showRoleDialog = ref(false);
const roleLoading = ref(false);
const userToToggleRole = ref<User | null>(null);

// Activate/deactivate dialog state
const showActiveDialog = ref(false);
const activeLoading = ref(false);
const userToToggleActive = ref<User | null>(null);

// Password reset dialog state
const showPasswordDialog = ref(false);
const passwordLoading = ref(false);
const passwordError = ref<string | null>(null);
const userToResetPassword = ref<User | null>(null);
const newPassword = ref("");

// Restore dialog state
const showRestoreDialog = ref(false);
const restoreLoading = ref(false);
const userToRestore = ref<User | null>(null);

async function fetchUsers() {
  loading.value = true;
  error.value = null;
  const result = await listUsers(page.value, perPage.value, searchQuery.value, status.value);
  if (result.success && result.data) {
    users.value = result.data.users || [];
    total.value = result.data.total;
    totalPages.value = result.data.total_pages;
    if (result.data.counts) counts.value = result.data.counts;
  } else {
    error.value = result.error || "Failed to fetch users";
  }
  loading.value = false;
}

function resetCreateForm() {
  createForm.value = {
    username: "",
    email: "",
    password: "",
    first_name: "",
    last_name: "",
    user_type: "user",
  };
  createError.value = null;
}

async function handleCreateUser() {
  createLoading.value = true;
  createError.value = null;
  const result = await createUser(createForm.value);
  createLoading.value = false;

  if (result.success) {
    showCreateDialog.value = false;
    resetCreateForm();
    await fetchUsers();
  } else {
    createError.value = result.error || "Failed to create user";
  }
}

function confirmDelete(user: User) {
  userToDelete.value = user;
  showDeleteDialog.value = true;
}

async function handleDeleteUser() {
  if (!userToDelete.value) return;

  deleteLoading.value = true;
  const result = await deleteUser(userToDelete.value.id);
  deleteLoading.value = false;

  if (result.success) {
    showDeleteDialog.value = false;
    userToDelete.value = null;
    await fetchUsers();
  } else {
    error.value = result.error || "Failed to delete user";
  }
}

function confirmToggleRole(user: User) {
  userToToggleRole.value = user;
  showRoleDialog.value = true;
}

async function handleToggleRole() {
  if (!userToToggleRole.value) return;

  roleLoading.value = true;
  const newType = userToToggleRole.value.user_type === "admin" ? "user" : "admin";
  const result = await updateUserRole(userToToggleRole.value.id, newType);
  roleLoading.value = false;

  if (result.success) {
    showRoleDialog.value = false;
    userToToggleRole.value = null;
    await fetchUsers();
  } else {
    error.value = result.error || "Failed to update role";
    showRoleDialog.value = false;
  }
}

function confirmToggleActive(user: User) {
  userToToggleActive.value = user;
  showActiveDialog.value = true;
}

async function handleToggleActive() {
  if (!userToToggleActive.value) return;

  activeLoading.value = true;
  const nextActive = userToToggleActive.value.is_deactivated === true;
  const result = await setUserActive(userToToggleActive.value.id, nextActive);
  activeLoading.value = false;

  if (result.success) {
    showActiveDialog.value = false;
    userToToggleActive.value = null;
    await fetchUsers();
  } else {
    error.value = result.error || "Failed to update user status";
    showActiveDialog.value = false;
  }
}

function confirmRestore(user: User) {
  userToRestore.value = user;
  showRestoreDialog.value = true;
}

async function handleRestore() {
  if (!userToRestore.value) return;

  restoreLoading.value = true;
  const result = await restoreUser(userToRestore.value.id);
  restoreLoading.value = false;

  if (result.success) {
    showRestoreDialog.value = false;
    userToRestore.value = null;
    await fetchUsers();
  } else {
    error.value = result.error || "Failed to restore user";
    showRestoreDialog.value = false;
  }
}

function openPasswordReset(user: User) {
  userToResetPassword.value = user;
  newPassword.value = "";
  passwordError.value = null;
  showPasswordDialog.value = true;
}

async function handleResetPassword() {
  if (!userToResetPassword.value) return;

  passwordLoading.value = true;
  passwordError.value = null;
  const result = await resetUserPassword(userToResetPassword.value.id, newPassword.value);
  passwordLoading.value = false;

  if (result.success) {
    showPasswordDialog.value = false;
    userToResetPassword.value = null;
    newPassword.value = "";
  } else {
    passwordError.value = result.error || "Failed to reset password";
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

// Search
function onSearchInput() {
  if (searchDebounce) clearTimeout(searchDebounce);
  searchDebounce = setTimeout(() => {
    page.value = 1;
    fetchUsers();
  }, 300);
}

function clearSearch() {
  searchQuery.value = "";
  page.value = 1;
  fetchUsers();
}

// Pagination
function prevPage() {
  if (page.value > 1) {
    page.value--;
    fetchUsers();
  }
}

function nextPage() {
  if (page.value < totalPages.value) {
    page.value++;
    fetchUsers();
  }
}

onMounted(() => {
  fetchUsers();
});
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <Navbar />

    <main id="main-content" class="flex-1">
      <div class="mx-auto max-w-6xl px-6 py-12">
        <div class="mb-8 flex items-center justify-between">
          <div>
            <h1 class="flex items-center gap-2 text-3xl font-bold tracking-tight">
              <Users class="size-8" />
              User Management
            </h1>
            <p class="mt-2 text-muted-foreground">
              Manage all users in the system
            </p>
          </div>
          <Button @click="showCreateDialog = true; resetCreateForm()">
            <Plus class="mr-2 size-4" />
            Create User
          </Button>
        </div>

        <div v-if="error" role="alert" class="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {{ error }}
        </div>

        <!-- Status tabs -->
        <div class="mb-3 flex items-center gap-1 border-b">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            class="relative -mb-px flex items-center gap-1.5 border-b-2 px-4 py-2 text-sm font-medium transition-colors"
            :class="status === tab.key
              ? 'border-primary text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'"
            @click="selectStatus(tab.key)"
          >
            {{ tab.label }}
            <span class="rounded-full bg-muted px-1.5 py-0.5 text-xs tabular-nums">{{ tab.count }}</span>
          </button>
        </div>
        <p class="mb-4 text-sm text-muted-foreground">{{ statusHint }}</p>

        <div class="mb-4 relative">
          <Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            v-model="searchQuery"
            :placeholder="searchPlaceholder"
            class="pl-9 pr-9"
            @input="onSearchInput"
          />
          <button
            v-if="searchQuery"
            aria-label="Clear search"
            class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 rounded-sm outline-none"
            @click="clearSearch"
          >
            <X class="size-4" />
          </button>
        </div>

        <Card>
          <CardContent class="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Username</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>{{ whenColumnLabel }}</TableHead>
                  <TableHead class="w-[100px]">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-if="loading">
                  <TableCell colspan="6" class="py-8 text-center">
                    <Loader2 class="mx-auto size-6 animate-spin" />
                  </TableCell>
                </TableRow>
                <TableRow v-else-if="users.length === 0">
                  <TableCell colspan="6" class="py-8 text-center text-muted-foreground">
                    No users found
                  </TableCell>
                </TableRow>
                <TableRow v-for="user in users" :key="user.id">
                  <TableCell class="font-medium">{{ user.username }}</TableCell>
                  <TableCell>{{ user.email }}</TableCell>
                  <TableCell>{{ user.first_name }} {{ user.last_name }}</TableCell>
                  <TableCell>
                    <Badge :variant="user.user_type === 'admin' ? 'default' : 'secondary'">
                      {{ user.user_type }}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <div class="text-sm">{{ formatDate(whenDate(user)) }}</div>
                    <div v-if="whenActor(user)" class="text-xs text-muted-foreground">
                      by {{ whenActor(user) }}
                    </div>
                  </TableCell>
                  <TableCell>
                    <!-- Protected break-glass account: no destructive controls. -->
                    <div
                      v-if="user.is_super_admin"
                      class="flex items-center gap-1.5 text-muted-foreground"
                      title="Protected break-glass account — can only be changed at the database level or via the SUPERADMIN_EMAIL setting"
                    >
                      <Lock class="size-3.5" />
                      <span class="text-xs">Protected</span>
                    </div>

                    <!-- Deleted tab: restore only. -->
                    <div v-else-if="status === 'deleted'" class="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label="Restore user"
                        title="Restore user"
                        @click="confirmRestore(user)"
                      >
                        <RotateCcw class="mr-1.5 size-4" />
                        Restore
                      </Button>
                    </div>

                    <!-- Deactivated tab: reactivate or delete. -->
                    <div v-else-if="status === 'deactivated'" class="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Reactivate user"
                        title="Reactivate user"
                        @click="confirmToggleActive(user)"
                      >
                        <UserCheck class="size-4 text-emerald-600 dark:text-emerald-500" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Delete user"
                        title="Delete user"
                        class="text-destructive hover:text-destructive"
                        @click="confirmDelete(user)"
                      >
                        <Trash2 class="size-4" />
                      </Button>
                    </div>

                    <!-- Active tab: role, deactivate, reset password, delete. -->
                    <div v-else class="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        :aria-label="user.user_type === 'admin' ? 'Demote to user' : 'Promote to admin'"
                        :title="user.user_type === 'admin' ? 'Demote to user' : 'Promote to admin'"
                        @click="confirmToggleRole(user)"
                      >
                        <Shield v-if="user.user_type !== 'admin'" class="size-4" />
                        <ShieldOff v-else class="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Deactivate user"
                        title="Deactivate user"
                        @click="confirmToggleActive(user)"
                      >
                        <UserX class="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Reset password"
                        title="Reset password"
                        @click="openPasswordReset(user)"
                      >
                        <KeyRound class="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Delete user"
                        title="Delete user"
                        class="text-destructive hover:text-destructive"
                        @click="confirmDelete(user)"
                      >
                        <Trash2 class="size-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
          <CardFooter class="flex items-center justify-between border-t px-6 py-4">
            <p class="text-sm text-muted-foreground">
              Showing {{ users.length }} of {{ total }} users
            </p>
            <div class="flex items-center gap-2">
              <Button variant="outline" size="sm" aria-label="Previous page" :disabled="page === 1" @click="prevPage">
                <ChevronLeft class="size-4" />
              </Button>
              <span class="text-sm">Page {{ page }} of {{ totalPages || 1 }}</span>
              <Button variant="outline" size="sm" aria-label="Next page" :disabled="page >= totalPages" @click="nextPage">
                <ChevronRight class="size-4" />
              </Button>
            </div>
          </CardFooter>
        </Card>

        <!-- Create User Dialog -->
        <Dialog v-model:open="showCreateDialog">
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create New User</DialogTitle>
              <DialogDescription>
                Fill in the details to create a new user account.
              </DialogDescription>
            </DialogHeader>
            <form @submit.prevent="handleCreateUser" class="space-y-4">
              <div v-if="createError" role="alert" class="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                {{ createError }}
              </div>
              <div class="grid grid-cols-2 gap-4">
                <div class="space-y-2">
                  <Label for="first_name">First Name</Label>
                  <Input id="first_name" v-model="createForm.first_name" required :disabled="createLoading" />
                </div>
                <div class="space-y-2">
                  <Label for="last_name">Last Name</Label>
                  <Input id="last_name" v-model="createForm.last_name" required :disabled="createLoading" />
                </div>
              </div>
              <div class="space-y-2">
                <Label for="username">Username</Label>
                <Input id="username" v-model="createForm.username" required :disabled="createLoading" />
              </div>
              <div class="space-y-2">
                <Label for="email">Email</Label>
                <Input id="email" type="email" v-model="createForm.email" required :disabled="createLoading" />
              </div>
              <div class="space-y-2">
                <Label for="password">Password</Label>
                <Input id="password" type="password" v-model="createForm.password" required :disabled="createLoading" />
              </div>
              <div class="space-y-2">
                <Label for="user_type">User Type</Label>
                <NativeSelect id="user_type" v-model="createForm.user_type" :disabled="createLoading">
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                </NativeSelect>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" @click="showCreateDialog = false" :disabled="createLoading">
                  Cancel
                </Button>
                <Button type="submit" :disabled="createLoading">
                  <Loader2 v-if="createLoading" class="mr-2 size-4 animate-spin" />
                  Create
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>

        <!-- Toggle Role Dialog -->
        <Dialog v-model:open="showRoleDialog">
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Change User Role</DialogTitle>
              <DialogDescription>
                <template v-if="userToToggleRole?.user_type === 'admin'">
                  Demote "{{ userToToggleRole?.username }}" from admin to regular user?
                  They will lose access to admin features.
                </template>
                <template v-else>
                  Promote "{{ userToToggleRole?.username }}" to admin?
                  They will gain access to all admin features.
                </template>
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" @click="showRoleDialog = false" :disabled="roleLoading">
                Cancel
              </Button>
              <Button :disabled="roleLoading" @click="handleToggleRole">
                <Loader2 v-if="roleLoading" class="mr-2 size-4 animate-spin" />
                {{ userToToggleRole?.user_type === 'admin' ? 'Demote to User' : 'Promote to Admin' }}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <!-- Activate / Deactivate Dialog -->
        <Dialog v-model:open="showActiveDialog">
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{{ userToToggleActive?.is_deactivated ? 'Activate User' : 'Deactivate User' }}</DialogTitle>
              <DialogDescription>
                <template v-if="userToToggleActive?.is_deactivated">
                  Reactivate "{{ userToToggleActive?.username }}"? They will be able to sign in again
                  with their existing password.
                </template>
                <template v-else>
                  Deactivate "{{ userToToggleActive?.username }}"? They will be signed out and cannot
                  sign in until reactivated. The account and password are kept, not deleted.
                </template>
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" @click="showActiveDialog = false" :disabled="activeLoading">
                Cancel
              </Button>
              <Button
                :variant="userToToggleActive?.is_deactivated ? 'default' : 'destructive'"
                :disabled="activeLoading"
                @click="handleToggleActive"
              >
                <Loader2 v-if="activeLoading" class="mr-2 size-4 animate-spin" />
                {{ userToToggleActive?.is_deactivated ? 'Activate' : 'Deactivate' }}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <!-- Reset Password Dialog -->
        <Dialog v-model:open="showPasswordDialog">
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Reset Password</DialogTitle>
              <DialogDescription>
                Set a new password for "{{ userToResetPassword?.username }}".
                This will revoke all their active sessions.
              </DialogDescription>
            </DialogHeader>
            <form @submit.prevent="handleResetPassword" class="space-y-4">
              <div v-if="passwordError" role="alert" class="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                {{ passwordError }}
              </div>
              <div class="space-y-2">
                <Label for="new_password">New Password</Label>
                <Input
                  id="new_password"
                  type="password"
                  v-model="newPassword"
                  required
                  minlength="8"
                  placeholder="Minimum 8 characters"
                  :disabled="passwordLoading"
                />
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" @click="showPasswordDialog = false" :disabled="passwordLoading">
                  Cancel
                </Button>
                <Button type="submit" :disabled="passwordLoading">
                  <Loader2 v-if="passwordLoading" class="mr-2 size-4 animate-spin" />
                  Reset Password
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>

        <!-- Delete Confirmation Dialog -->
        <Dialog v-model:open="showDeleteDialog">
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Delete User</DialogTitle>
              <DialogDescription>
                Delete the user "{{ userToDelete?.username }}"? They'll be signed out and
                can no longer sign in. The account is kept, not erased — you can bring it
                back from the Deleted tab.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" @click="showDeleteDialog = false" :disabled="deleteLoading">
                Cancel
              </Button>
              <Button variant="destructive" :disabled="deleteLoading" @click="handleDeleteUser">
                <Loader2 v-if="deleteLoading" class="mr-2 size-4 animate-spin" />
                Delete
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <!-- Restore Confirmation Dialog -->
        <Dialog v-model:open="showRestoreDialog">
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Restore User</DialogTitle>
              <DialogDescription>
                Restore "{{ userToRestore?.username }}"? The account becomes active again
                and can sign in with its existing password.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" @click="showRestoreDialog = false" :disabled="restoreLoading">
                Cancel
              </Button>
              <Button :disabled="restoreLoading" @click="handleRestore">
                <Loader2 v-if="restoreLoading" class="mr-2 size-4 animate-spin" />
                Restore
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </main>
  </div>
</template>
