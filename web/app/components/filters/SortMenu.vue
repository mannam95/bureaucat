<script setup lang="ts">
import { ArrowUpDown, Check, ArrowDown, ArrowUp } from "lucide-vue-next";
import type { SortKey, SortDir } from "~/types";

defineProps<{
  sortBy: SortKey;
  sortDir: SortDir;
}>();

const emit = defineEmits<{
  "update:sortBy": [value: SortKey];
  "update:sortDir": [value: SortDir];
}>();

// Canonical option order, harmonized across all sort menus: work attributes
// first, then dates in the task's lifecycle order. Surfaces lacking a field
// omit it without reordering the rest.
const WORK_OPTIONS: { key: SortKey; label: string }[] = [
  { key: "state", label: "State" },
  { key: "priority_rating", label: "Priority rating" },
  { key: "title", label: "Title" },
];
const DATE_OPTIONS: { key: SortKey; label: string }[] = [
  { key: "created_at", label: "Created date" },
  { key: "start_date", label: "Start date" },
  { key: "due_date", label: "Due date" },
  { key: "updated_at", label: "Last updated" },
];
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button variant="outline" size="sm" class="gap-1.5">
        <ArrowUpDown class="size-3.5" />
        Sort
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="w-52">
      <DropdownMenuLabel class="text-xs uppercase tracking-wider text-muted-foreground">
        Sort by
      </DropdownMenuLabel>
      <DropdownMenuItem
        v-for="opt in WORK_OPTIONS"
        :key="opt.key"
        class="flex items-center justify-between"
        @click="emit('update:sortBy', opt.key)"
      >
        <span>{{ opt.label }}</span>
        <Check v-if="sortBy === opt.key" class="size-3.5 text-primary" />
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem
        v-for="opt in DATE_OPTIONS"
        :key="opt.key"
        class="flex items-center justify-between"
        @click="emit('update:sortBy', opt.key)"
      >
        <span>{{ opt.label }}</span>
        <Check v-if="sortBy === opt.key" class="size-3.5 text-primary" />
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem
        class="flex items-center justify-between"
        @click="emit('update:sortDir', 'asc')"
      >
        <span class="flex items-center gap-2">
          <ArrowUp class="size-3.5" /> Ascending
        </span>
        <Check v-if="sortDir === 'asc'" class="size-3.5 text-primary" />
      </DropdownMenuItem>
      <DropdownMenuItem
        class="flex items-center justify-between"
        @click="emit('update:sortDir', 'desc')"
      >
        <span class="flex items-center gap-2">
          <ArrowDown class="size-3.5" /> Descending
        </span>
        <Check v-if="sortDir === 'desc'" class="size-3.5 text-primary" />
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
