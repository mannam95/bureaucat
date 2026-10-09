/**
 * Complexity score = Difficulty (1-5) x Effort (1-5), computed server-side.
 * Bands make the 1-25 scale readable at a glance.
 */
export function complexityBand(score: number): { label: string; color: string } | null {
  if (score <= 0) return null;
  if (score <= 4) return { label: "Low", color: "#22C55E" };
  if (score <= 9) return { label: "Medium", color: "#EAB308" };
  if (score <= 16) return { label: "High", color: "#F97316" };
  return { label: "Very High", color: "#EF4444" };
}
