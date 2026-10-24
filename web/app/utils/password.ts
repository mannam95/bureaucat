// Cryptographically random passwords for the admin dialogs (create user,
// reset password). Satisfies the strictest server policy: length, upper,
// lower, digit and special guaranteed. Visually ambiguous characters
// (0/O, 1/l/I) are excluded so a password read off the screen types back
// correctly.
export function generateStrongPassword(length = 16): string {
  const upper = "ABCDEFGHJKMNPQRSTUVWXYZ";
  const lower = "abcdefghijkmnpqrstuvwxyz";
  const digits = "23456789";
  const specials = "!@#$%^&*-_=+?";
  const all = upper + lower + digits + specials;

  const pick = (set: string, n: number): string[] => {
    const buf = new Uint32Array(n);
    crypto.getRandomValues(buf);
    return Array.from(buf, (v) => set[v % set.length]!);
  };

  // One of each class, the rest from the full set, then a crypto shuffle so
  // the guaranteed characters don't sit in predictable spots.
  const chars = [
    ...pick(upper, 1),
    ...pick(lower, 1),
    ...pick(digits, 1),
    ...pick(specials, 1),
    ...pick(all, Math.max(length, 8) - 4),
  ];
  const rnd = new Uint32Array(chars.length);
  crypto.getRandomValues(rnd);
  for (let i = chars.length - 1; i > 0; i--) {
    const j = rnd[i]! % (i + 1);
    [chars[i], chars[j]] = [chars[j]!, chars[i]!];
  }
  return chars.join("");
}
