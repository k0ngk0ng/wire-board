/** Encode already SHA-verified FLAC bytes for the platform's data:-only inline
 * media policy. Chunk lengths are divisible by three, so only the last chunk
 * can introduce base64 padding. No unbounded function arguments are created.
 */
export function classicFLACDataURL(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  if (!bytes.length) throw Error("音频循环不能为空");
  const chunks: string[] = [], chunkSize = 24 * 1024;
  for (let start = 0; start < bytes.length; start += chunkSize) {
    const end = Math.min(start + chunkSize, bytes.length);
    let binary = "";
    for (let index = start; index < end; index++) binary += String.fromCharCode(bytes[index]);
    chunks.push(btoa(binary));
  }
  return `data:audio/flac;base64,${chunks.join("")}`;
}
