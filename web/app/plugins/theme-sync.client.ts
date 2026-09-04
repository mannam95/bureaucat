// Keeps the theme in sync between @nuxtjs/color-mode (the paint layer, which
// applies instantly from its own localStorage with no flash) and the durable
// app.theme preference (so the choice follows the user across devices).
//
// Rules:
//   - color-mode remains authoritative for the initial paint.
//   - Once global preferences load, an explicit stored theme (source "user")
//     is applied. A user who never chose a theme keeps color-mode's local
//     preference — the registry default never overrides a local choice.
//   - A theme change is written back to the preference store while signed in.
export default defineNuxtPlugin(() => {
  const colorMode = useColorMode();
  const prefs = usePreferences();
  const { isAuthenticated } = useAuth();

  // Adopt an explicit stored theme once global preferences are loaded.
  watch(
    prefs.globalLoaded,
    (loaded) => {
      if (!loaded) return;
      const entry = prefs.entry("global", undefined, "app.theme");
      if (
        entry?.source === "user" &&
        typeof entry.value === "string" &&
        entry.value !== colorMode.preference
      ) {
        colorMode.preference = entry.value;
      }
    },
    { immediate: true },
  );

  // Persist the user's theme choice. Non-immediate, so color-mode's initial
  // value (restored from its own localStorage) is not written back as a change.
  watch(
    () => colorMode.preference,
    (pref) => {
      if (!isAuthenticated.value) return;
      if (pref !== "light" && pref !== "dark" && pref !== "system") return;
      prefs.setGlobal("app.theme", pref);
    },
  );
});
