<script setup lang="ts">
import { Info } from "lucide-vue-next";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "~/components/ui/tooltip";

// The info dot: a small (i) beside a label whose tooltip spells out the rule
// behind a number, one bullet per line (the API joins bullets with newlines).
const props = defineProps<{ label: string; rule: string }>();
const lines = computed(() => props.rule.split("\n").filter(Boolean));
</script>

<template>
  <TooltipProvider :delay-duration="150">
    <Tooltip>
      <TooltipTrigger
        as="span"
        tabindex="0"
        class="inline-flex cursor-help rounded-sm align-middle outline-none focus-visible:ring-2 focus-visible:ring-ring"
        :aria-label="`${label} rule`"
        @click.stop.prevent
      >
        <Info class="size-3 text-muted-foreground" />
      </TooltipTrigger>
      <TooltipContent side="bottom" class="max-w-xs px-3 py-2 text-left">
        <p class="font-semibold">{{ label }}</p>
        <ul class="mt-1 list-disc space-y-0.5 pl-4">
          <li v-for="(line, i) in lines" :key="i">{{ line }}</li>
        </ul>
      </TooltipContent>
    </Tooltip>
  </TooltipProvider>
</template>
