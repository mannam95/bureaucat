<script setup lang="ts">
import { Download } from "lucide-vue-next";

defineProps<{
  open: boolean;
  src: string;
  alt?: string;
}>();

const emit = defineEmits<{
  "update:open": [value: boolean];
}>();
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <!-- Do not add `relative` here: DialogContent is already `fixed`, and a
         second position utility overrides it and throws off the centering. The
         download link below uses `absolute`, which anchors to this fixed box. -->
    <DialogContent class="max-w-[90vw] max-h-[90vh] border-none bg-transparent p-0 shadow-none sm:max-w-[90vw]">
      <DialogTitle class="sr-only">{{ alt || 'Image preview' }}</DialogTitle>

      <!-- Download the image while viewing it in-app. -->
      <a
        :href="src"
        :download="alt || 'image'"
        class="absolute right-2 top-2 z-10 flex items-center gap-1.5 rounded-md bg-background/80 px-2 py-1 text-xs text-foreground shadow-sm backdrop-blur transition-colors hover:bg-background"
        aria-label="Download image"
        @click.stop
      >
        <Download class="size-3.5" />
        Download
      </a>

      <!-- White backdrop so transparent images (SVG/PNG diagrams, screenshots)
           stay visible instead of blending into the dark overlay. -->
      <img
        :src="src"
        :alt="alt || 'Image preview'"
        class="max-h-[85vh] w-full rounded-lg bg-white object-contain"
      />
    </DialogContent>
  </Dialog>
</template>
