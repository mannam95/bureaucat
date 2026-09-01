<script setup lang="ts">
/**
 * The 1-10 "star" priority rating. Two modes:
 *  - display (default): a compact read-only badge ("★ 7" / "—").
 *  - editable: a badge trigger that opens a 1-10 star picker in a popover,
 *    plus a Clear action. Clicking the current value also clears it.
 * 0 (or undefined) means unset.
 */
import { Star, ChevronDown } from "lucide-vue-next";

const props = withDefaults(
  defineProps<{
    modelValue?: number;
    editable?: boolean;
    disabled?: boolean;
  }>(),
  { modelValue: 0, editable: false, disabled: false }
);

const emit = defineEmits<{ "update:modelValue": [value: number] }>();

const value = computed(() => props.modelValue ?? 0);
const open = ref(false);
const hover = ref(0);

// The number of filled stars to show in the picker: the hovered value while
// hovering, otherwise the current value.
const shown = computed(() => hover.value || value.value);

function set(n: number) {
  // Re-picking the current rating clears it, so a single control both sets and
  // unsets without a separate toggle.
  emit("update:modelValue", n === value.value ? 0 : n);
  close();
}

function clear() {
  emit("update:modelValue", 0);
  close();
}

// Reset the hover preview on close so reopening always reflects the committed
// value (the picker stays mounted, so a stale hover would otherwise linger).
function close() {
  hover.value = 0;
  open.value = false;
}
</script>

<template>
  <!-- Display-only compact badge -->
  <span
    v-if="!editable"
    class="inline-flex items-center gap-0.5 text-xs"
    :class="value > 0 ? 'text-amber-500' : 'text-muted-foreground/60'"
    :title="value > 0 ? `Priority rating ${value}/10` : 'No priority rating'"
  >
    <Star class="size-3.5" :class="value > 0 ? 'fill-amber-400' : ''" />
    <span class="tabular-nums font-medium">{{ value > 0 ? value : "—" }}</span>
  </span>

  <!-- Editable: badge trigger + 1-10 picker popover -->
  <Popover v-else v-model:open="open">
    <PopoverTrigger as-child>
      <Button
        variant="ghost"
        class="h-auto gap-1 px-0 py-0 font-medium hover:bg-transparent"
        :class="value > 0 ? 'text-amber-500' : 'text-muted-foreground'"
        :disabled="disabled"
      >
        <Star class="size-3.5" :class="value > 0 ? 'fill-amber-400' : ''" />
        <span class="tabular-nums">{{ value > 0 ? `${value}/10` : "Set rating" }}</span>
        <ChevronDown class="size-3.5 opacity-50" />
      </Button>
    </PopoverTrigger>
    <PopoverContent align="end" class="w-auto p-2" @mouseleave="hover = 0">
      <div class="flex items-center gap-0.5">
        <button
          v-for="n in 10"
          :key="n"
          type="button"
          class="rounded p-0.5 transition-colors"
          :class="n <= shown ? 'text-amber-500' : 'text-muted-foreground/50 hover:text-amber-400'"
          :aria-label="`Set priority rating to ${n}`"
          @mouseenter="hover = n"
          @click="set(n)"
        >
          <Star class="size-4" :class="n <= shown ? 'fill-amber-400' : ''" />
        </button>
      </div>
      <button
        v-if="value > 0"
        type="button"
        class="mt-1.5 w-full rounded px-2 py-1 text-center text-xs text-muted-foreground hover:bg-muted"
        @click="clear"
      >
        Clear rating
      </button>
    </PopoverContent>
  </Popover>
</template>
