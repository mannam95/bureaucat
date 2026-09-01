import { computed, onBeforeUnmount, onMounted, toValue, watch } from "vue";
import type { MaybeRefOrGetter, Ref } from "vue";

const PREFIX = "bureaucat:draft:";
const SAVE_DELAY = 400;

// Tiptap emits `<p></p>` and friends for a visually-empty editor; plain-text
// fields fall through the tag strip unchanged.
function isBlank(value: string): boolean {
  return value.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

/**
 * Persists an unsent composer field to localStorage so a refresh doesn't lose it.
 * Drafts are local-only and never sent to the server; call `clear()` once the
 * content has been submitted.
 */
export function useDraft(
  key: MaybeRefOrGetter<string>,
  model: Ref<string>,
  options: { autoRestore?: boolean } = {}
) {
  // An empty key disables persistence (e.g. before a project is chosen).
  const storageKey = computed(() => {
    const k = toValue(key);
    return k ? PREFIX + k : "";
  });
  let timer: ReturnType<typeof setTimeout> | undefined;
  // Set by clear() so the submitted-but-still-populated form isn't written back
  // by the unmount flush; any further edit re-arms saving.
  let cleared = false;
  // Only a composer that actually held content may erase a stored draft. Several
  // composers share a key (the create dialog is mounted on every page), and an
  // untouched one would otherwise wipe the draft when it unmounts.
  let hadContent = !isBlank(model.value);

  function save() {
    clearTimeout(timer);
    if (cleared || !storageKey.value) return;
    const blank = isBlank(model.value);
    if (blank && !hadContent) return;
    try {
      if (blank) localStorage.removeItem(storageKey.value);
      else localStorage.setItem(storageKey.value, model.value);
    } catch {
      // Private mode / quota — drafts are best-effort.
    }
  }

  function restore() {
    if (!storageKey.value || !isBlank(model.value)) return;
    try {
      const saved = localStorage.getItem(storageKey.value);
      if (saved) {
        model.value = saved;
        hadContent = true;
      }
    } catch {
      // ignore
    }
  }

  function clear() {
    clearTimeout(timer);
    cleared = true;
    hadContent = false;
    if (!storageKey.value) return;
    try {
      localStorage.removeItem(storageKey.value);
    } catch {
      // ignore
    }
  }

  watch(model, (val) => {
    cleared = false;
    if (!isBlank(val)) hadContent = true;
    clearTimeout(timer);
    timer = setTimeout(save, SAVE_DELAY);
  });

  // The key can change mid-compose (the dialog's project picker); pull in that
  // key's draft if the composer is still empty.
  watch(storageKey, () => restore());

  onMounted(() => {
    if (options.autoRestore !== false) restore();
    // Flush a pending debounce when the tab is closed or reloaded mid-typing.
    window.addEventListener("beforeunload", save);
  });

  onBeforeUnmount(() => {
    save();
    window.removeEventListener("beforeunload", save);
  });

  return { restore, clear };
}
