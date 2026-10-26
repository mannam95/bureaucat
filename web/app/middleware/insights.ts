// The insights page is feature-gated (super-admin managed). Bouncing an
// ungated user here only spares them a page of failing requests — the API
// enforces the gate on every insights route regardless.
export default defineNuxtRouteMiddleware(async () => {
  const { isLoading, user } = useAuth();

  // Wait for auth initialization to complete
  if (isLoading.value) {
    await new Promise<void>((resolve) => {
      const checkAuth = () => {
        if (!isLoading.value) {
          resolve();
        } else {
          setTimeout(checkAuth, 50);
        }
      };
      checkAuth();
    });
  }

  if (!user.value?.features?.includes("insights")) {
    return navigateTo("/dashboard");
  }
});
