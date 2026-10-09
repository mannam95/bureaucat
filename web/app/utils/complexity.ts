/**
 * Complexity score = Difficulty (1-5) x Effort (1-5), computed server-side.
 * Bands make the 1-25 scale readable at a glance.
 */

// Shared wording for the two 1-5 scales, so pickers read the same everywhere
// (task sidebar, create dialog, /tasks/new).
export const DIFFICULTY_LABELS: Record<number, string> = {
  1: "Trivial",
  2: "Easy",
  3: "Moderate",
  4: "Hard",
  5: "Extreme",
};

export const EFFORT_LABELS: Record<number, string> = {
  1: "Hours",
  2: "A day",
  3: "Days",
  4: "A week",
  5: "Weeks",
};

export function complexityBand(score: number): { label: string; color: string } | null {
  if (score <= 0) return null;
  if (score <= 4) return { label: "Low", color: "#22C55E" };
  if (score <= 9) return { label: "Medium", color: "#EAB308" };
  if (score <= 16) return { label: "High", color: "#F97316" };
  return { label: "Very High", color: "#EF4444" };
}
