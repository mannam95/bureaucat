<script setup lang="ts">
// A card that sits first in the Cycles / Modules grid and opens the "Tasks
// Without a Cycle / Epic" backlog view, the same way a cycle or epic card opens
// its detail page.
import { Inbox, ArrowRight } from "lucide-vue-next";

defineProps<{
  title: string;
  subtitle: string;
  count: number;
  to: string;
}>();
</script>

<template>
  <NuxtLink :to="to">
    <Card
      class="group h-full cursor-pointer border-dashed border-border/60 bg-background/50 transition-all hover:border-amber-500/40 hover:shadow-lg hover:shadow-amber-500/5"
    >
      <CardHeader class="pb-3">
        <div class="flex items-start justify-between gap-2">
          <div
            class="flex size-10 items-center justify-center rounded-lg bg-muted transition-colors group-hover:bg-amber-500/10"
          >
            <Inbox
              class="size-5 text-muted-foreground transition-colors group-hover:text-amber-600 dark:group-hover:text-amber-500"
            />
          </div>
          <span
            class="rounded-md border border-muted-foreground/30 bg-muted px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground"
          >
            Backlog
          </span>
        </div>
        <CardTitle class="mt-3 line-clamp-2 text-base font-semibold">
          {{ title }}
        </CardTitle>
        <p class="mt-1 text-xs text-muted-foreground">{{ subtitle }}</p>
      </CardHeader>
      <CardContent class="pt-0">
        <div class="flex items-center justify-between text-xs">
          <span class="text-muted-foreground">
            {{ count }} task{{ count === 1 ? "" : "s" }}
          </span>
          <span class="inline-flex items-center gap-1 font-medium text-amber-600 dark:text-amber-500">
            Open <ArrowRight class="size-3.5" />
          </span>
        </div>
      </CardContent>
    </Card>
  </NuxtLink>
</template>
