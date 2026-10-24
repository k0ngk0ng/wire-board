const alphabet =
  "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-";

/** Twenty uniformly sampled URL-safe characters, from browser cryptographic entropy. */
export function generateResetPassword(
  random: Pick<Crypto, "getRandomValues"> = crypto,
): string {
  return Array.from(
    random.getRandomValues(new Uint8Array(20)),
    (byte) => alphabet[byte & 63],
  ).join("");
}
