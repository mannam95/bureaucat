<script setup lang="ts">
import { Toaster as Sonner } from "@/components/ui/sonner";

const { isAuthenticated } = useAuth();
// Assistant panel - HIDDEN (self-host): the in-app LLM assistant is not used;
// re-enable by restoring useAssistant() wiring and the <AssistantPanel> mount.
const assistantOpen = computed(() => false);
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <a href="#main-content" class="skip-to-main">Skip to main content</a>
    <NuxtRouteAnnouncer />
    <Sidebar v-if="isAuthenticated" />
    <AssistantPanel v-if="false" />
    <!-- --app-inset is the width taken by the rail (+ docked assistant); full-bleed sections use it. -->
    <div
      class="pl-[var(--app-inset)]"
      :class="!isAuthenticated ? '[--app-inset:0px]' : assistantOpen ? '[--app-inset:3rem] md:[--app-inset:31rem] lg:[--app-inset:39rem]' : '[--app-inset:3rem]'"
    >
      <NuxtPage />
    </div>
    <Sonner
      position="top-right"
      :expand="true"
      rich-colors
      :toast-options="{ class: '[&_[data-title]]:break-words [&_[data-description]]:break-words' }"
    />
  </div>
</template>
