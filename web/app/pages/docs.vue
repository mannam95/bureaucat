<script setup lang="ts">
import { ApiReference } from "@scalar/api-reference";
import "@scalar/api-reference/style.css";

useSeoMeta({ title: "API Docs" });

const colorMode = useColorMode();

const configuration = computed(() => ({
  url: "/api/v1/openapi.json",
  darkMode: colorMode.value === "dark",
  hideDarkModeToggle: true,
  hideClientButton: false,
  defaultHttpClient: { targetKey: "shell", clientKey: "curl" },
  metaData: { title: "Bureaucat API" },
}));
</script>

<template>
  <div class="h-screen overflow-y-auto">
    <ApiReference :configuration="configuration" />
  </div>
</template>
