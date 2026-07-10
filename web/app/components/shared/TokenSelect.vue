<script setup lang="ts" generic="T">
import { Loader2, X } from "lucide-vue-next";
import { cn } from "@/lib/utils";

// A Gmail-style freeflow token input: selected items render as inline chips
// inside a single input-shaped box, and typing filters a suggestion dropdown
// anchored below. Enter picks the highlighted suggestion and keeps the cursor
// in place so the user can keep typing to add more; Backspace on an empty query
// removes the last chip.
//
// The caller owns the data: pass `selected` (chips) and `available` (the pool
// offered in the dropdown, already excluding selected). Additions/removals are
// emitted via `@add` / `@remove` — the caller mutates a form array or fires an
// API call. Chip and option contents come from the `#chip` / `#option` slots.

const props = withDefaults(
  defineProps<{
    selected: T[];
    available: T[];
    getKey: (item: T) => string | number;
    getSearchText: (item: T) => string;
    placeholder?: string;
    emptyText?: string;
    disabled?: boolean;
    // Key of an item whose add/remove is in flight — shows a spinner on its
    // chip (used by the async task-detail pickers).
    pendingKey?: string | number | null;
    // Per-item styling for the chip container (so callers control the pill's
    // background/border while TokenSelect owns layout + the remove button).
    chipClass?: (item: T) => string;
    chipStyle?: (item: T) => Record<string, string>;
    contentClass?: string;
    // When true, `available` is treated as the ready-to-show result set and is
    // NOT filtered locally; instead the current query is emitted via `search`
    // so the caller can fetch matches (e.g. a directory API). Useful when the
    // pool is too large to hold client-side.
    serverSearch?: boolean;
    // Shows a spinner in the dropdown; pair with `serverSearch` while a fetch
    // is in flight.
    loading?: boolean;
  }>(),
  {
    placeholder: "Type to search...",
    emptyText: "No matches",
    disabled: false,
    pendingKey: null,
    contentClass: "",
    serverSearch: false,
    loading: false,
  }
);

const emit = defineEmits<{
  add: [item: T];
  remove: [item: T];
  search: [query: string];
}>();

const query = ref("");
const open = ref(false);
const highlighted = ref(0);
const inputRef = ref<HTMLInputElement | null>(null);
const listRef = ref<HTMLElement | null>(null);
let blurTimer: ReturnType<typeof setTimeout> | null = null;

const filtered = computed(() => {
  // In server-search mode the caller has already resolved the matches.
  if (props.serverSearch) return props.available;
  const q = query.value.toLowerCase().trim();
  if (!q) return props.available;
  return props.available.filter((it) =>
    props.getSearchText(it).toLowerCase().includes(q)
  );
});

// Let the caller run the search when it owns the data source.
watch(query, (q) => {
  if (props.serverSearch) emit("search", q);
});

// Keep the highlight in range as the filtered list shrinks while typing.
watch(filtered, (list) => {
  if (highlighted.value >= list.length) {
    highlighted.value = Math.max(0, list.length - 1);
  }
});

// Show the dropdown while focused whenever there are matches to pick, or the
// user has typed a query (so a "no matches" hint can appear). An empty box with
// nothing left to add stays quiet.
const showDropdown = computed(
  () =>
    open.value &&
    (filtered.value.length > 0 || query.value.trim() !== "" || props.loading)
);

function focusInput() {
  if (!props.disabled) inputRef.value?.focus();
}

function onFocus() {
  if (blurTimer) {
    clearTimeout(blurTimer);
    blurTimer = null;
  }
  open.value = true;
  highlighted.value = 0;
}

// Delay closing so a click on an option (which blurs the input first) still
// registers. Option clicks additionally prevent blur via @mousedown.prevent,
// so this mainly handles clicking away from the control.
function onBlur() {
  blurTimer = setTimeout(() => {
    open.value = false;
    query.value = "";
  }, 120);
}

function scrollHighlightedIntoView() {
  nextTick(() => {
    listRef.value
      ?.querySelector<HTMLElement>(`[data-index="${highlighted.value}"]`)
      ?.scrollIntoView({ block: "nearest" });
  });
}

function choose(item: T) {
  emit("add", item);
  query.value = "";
  highlighted.value = 0;
  open.value = true;
  focusInput();
}

function onKeydown(event: KeyboardEvent) {
  const count = filtered.value.length;
  if (event.key === "ArrowDown") {
    event.preventDefault();
    open.value = true;
    if (count === 0) return;
    highlighted.value = (highlighted.value + 1) % count;
    scrollHighlightedIntoView();
  } else if (event.key === "ArrowUp") {
    event.preventDefault();
    if (count === 0) return;
    highlighted.value = (highlighted.value - 1 + count) % count;
    scrollHighlightedIntoView();
  } else if (event.key === "Enter") {
    event.preventDefault();
    const item = filtered.value[highlighted.value];
    if (item !== undefined) choose(item);
  } else if (event.key === "Escape") {
    open.value = false;
    query.value = "";
    inputRef.value?.blur();
  } else if (
    event.key === "Backspace" &&
    query.value === "" &&
    props.selected.length > 0
  ) {
    const last = props.selected[props.selected.length - 1];
    if (last !== undefined) emit("remove", last);
  }
}
</script>

<template>
  <Popover :open="showDropdown">
    <PopoverAnchor as-child>
      <div
        role="group"
        class="flex w-full flex-wrap items-center gap-1.5 text-sm"
        :class="disabled ? 'pointer-events-none opacity-50' : 'cursor-text'"
        @mousedown.self="focusInput"
        @click.self="focusInput"
      >
        <span
          v-for="item in selected"
          :key="getKey(item)"
          class="inline-flex max-w-full items-center gap-1 rounded-md py-0.5 text-sm"
          :class="chipClass ? chipClass(item) : 'border bg-muted/50 pl-1 pr-1'"
          :style="chipStyle ? chipStyle(item) : undefined"
        >
          <slot name="chip" :item="item" />
          <button
            type="button"
            aria-label="Remove"
            class="flex size-4 shrink-0 items-center justify-center rounded-full text-current opacity-60 transition-opacity hover:opacity-100 focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-ring outline-none"
            :disabled="disabled || pendingKey === getKey(item)"
            @click.stop="emit('remove', item)"
            @mousedown.prevent
          >
            <Loader2
              v-if="pendingKey === getKey(item)"
              class="size-2.5 animate-spin"
            />
            <X v-else class="size-2.5" />
          </button>
        </span>
        <input
          ref="inputRef"
          v-model="query"
          type="text"
          :placeholder="selected.length === 0 ? placeholder : ''"
          :disabled="disabled"
          autocomplete="off"
          spellcheck="false"
          class="h-6 min-w-[6rem] flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed"
          @focus="onFocus"
          @blur="onBlur"
          @keydown="onKeydown"
        />
      </div>
    </PopoverAnchor>
    <PopoverContent
      align="start"
      :side-offset="6"
      :class="
        cn('w-[var(--reka-popover-trigger-width)] p-1', contentClass)
      "
      @open-auto-focus.prevent
      @close-auto-focus.prevent
    >
      <div ref="listRef" class="max-h-56 overflow-y-auto">
        <button
          v-for="(item, idx) in filtered"
          :key="getKey(item)"
          type="button"
          :data-index="idx"
          class="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm"
          :class="idx === highlighted ? 'bg-accent' : 'hover:bg-accent'"
          @mousedown.prevent="choose(item)"
          @mouseenter="highlighted = idx"
        >
          <slot name="option" :item="item" :active="idx === highlighted" />
        </button>
        <div
          v-if="loading"
          class="flex items-center justify-center px-2 py-6 text-muted-foreground"
        >
          <Loader2 class="size-4 animate-spin" />
        </div>
        <p
          v-else-if="filtered.length === 0"
          class="px-2 py-6 text-center text-sm text-muted-foreground"
        >
          {{ emptyText }}
        </p>
      </div>
    </PopoverContent>
  </Popover>
</template>
