export default defineNuxtPlugin(async () => {
  const { initAuth, isAuthenticated } = useAuth();
  await initAuth();

  // Populate the workspace switcher once we know the user is signed in.
  if (isAuthenticated.value) {
    const { listWorkspaces } = useWorkspaces();
    await listWorkspaces();
  }
});
