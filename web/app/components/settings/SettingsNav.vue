<script setup lang="ts">
// Side navigation for project settings: grouped sections with item counts,
// one section shown at a time (mock "Settings 1 · Side navigation"). Collapses
// to a native select below md.
const props = defineProps<{
  modelValue: string;
  /** Item counts shown next to the Work entries, keyed by section. */
  counts: Record<string, number>;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const groups = [
  {
    label: "Project",
    items: [
      { key: "general", label: "General" },
      { key: "workspace", label: "Workspace" },
      { key: "availability", label: "Availability" },
    ],
  },
  {
    label: "Work",
    items: [
      { key: "states", label: "Workflow states" },
      { key: "labels", label: "Labels" },
      { key: "areas", label: "Areas" },
      { key: "priorities", label: "Priorities" },
      { key: "templates", label: "Templates" },
    ],
  },
];

function select(key: string) {
  if (key !== props.modelValue) emit("update:modelValue", key);
}
</script>

<template>
  <div>
    <!-- Mobile: one native select covering every section -->
    <div class="md:hidden">
      <NativeSelect
        :model-value="modelValue"
        aria-label="Settings section"
        @update:model-value="(v) => select(v as string)"
      >
        <optgroup v-for="group in groups" :key="group.label" :label="group.label">
          <option v-for="item in group.items" :key="item.key" :value="item.key">
            {{ item.label }}{{ counts[item.key] != null ? ` (${counts[item.key]})` : "" }}
          </option>
        </optgroup>
        <optgroup label="Danger zone">
          <option value="danger">Delete project</option>
        </optgroup>
      </NativeSelect>
    </div>

    <!-- Desktop: grouped side navigation -->
    <nav class="hidden md:block" aria-label="Settings sections">
      <div v-for="group in groups" :key="group.label" class="mb-6">
        <p class="mb-2 px-3 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
          {{ group.label }}
        </p>
        <ul class="space-y-0.5">
          <li v-for="item in group.items" :key="item.key">
            <button
              type="button"
              class="flex w-full items-center justify-between gap-2 rounded-md px-3 py-1.5 text-left text-sm transition-colors"
              :class="
                modelValue === item.key
                  ? 'border border-border bg-background font-medium shadow-xs'
                  : 'border border-transparent text-muted-foreground hover:bg-accent hover:text-foreground'
              "
              @click="select(item.key)"
            >
              <span class="truncate">{{ item.label }}</span>
              <span
                v-if="counts[item.key] != null"
                class="shrink-0 text-xs tabular-nums text-muted-foreground"
              >
                {{ counts[item.key] }}
              </span>
            </button>
          </li>
        </ul>
      </div>

      <div class="border-t pt-4">
        <button
          type="button"
          class="w-full rounded-md px-3 py-1.5 text-left text-sm text-destructive transition-colors hover:bg-destructive/10"
          :class="modelValue === 'danger' ? 'bg-destructive/10 font-medium' : ''"
          @click="select('danger')"
        >
          Delete project
        </button>
      </div>
    </nav>
  </div>
</template>
