export default defineNuxtPlugin(async () => {
  const { initAuth, isAuthenticated } = useAuth();
  await initAuth();

  // Once we know the user is signed in, populate the workspace switcher and
  // hydrate durable preferences from the server before preference-dependent
  // screens render. Both are independent, so run them together.
  if (isAuthenticated.value) {
    const { listWorkspaces } = useWorkspaces();
    const { hydrateGlobal } = usePreferences();
    await Promise.all([listWorkspaces(), hydrateGlobal()]);
  }
});
