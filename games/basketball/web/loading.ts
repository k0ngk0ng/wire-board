export type LoadingProgress = { percent: number; label: string };
export type ByteProgress = { loaded: number; total: number };
/** Download bytes, never infer completion from a timer or a Content-Length alone. */
export async function fetchLoadingBytes(
  url: string,
  progress: (p: ByteProgress) => void,
  expected?: { bytes: number; sha256: string },
): Promise<ArrayBuffer> {
  const response = await fetch(url, { signal: AbortSignal.timeout(45000) });
  if (!response.ok) throw Error("资源下载失败，请重试");
  const total =
    expected?.bytes || Number(response.headers.get("content-length")) || 0;
  progress({ loaded: 0, total });
  const chunks: Uint8Array[] = [];
  let loaded = 0;
  if (response.body) {
    const reader = response.body.getReader();
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        loaded += value.byteLength;
        if (expected && loaded > expected.bytes) throw Error("资源大小不匹配");
        chunks.push(value);
        progress({ loaded, total });
      }
    } finally {
      await reader.cancel().catch(() => {});
      reader.releaseLock();
    }
  } else {
    const bytes = new Uint8Array(await response.arrayBuffer());
    chunks.push(bytes);
    loaded = bytes.length;
    progress({ loaded, total });
  }
  const buffer = new ArrayBuffer(loaded),
    bytes = new Uint8Array(buffer);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.length;
  }
  if (expected) {
    const sha = [
      ...new Uint8Array(await crypto.subtle.digest("SHA-256", buffer)),
    ]
      .map((x) => x.toString(16).padStart(2, "0"))
      .join("");
    if (loaded !== expected.bytes || sha !== expected.sha256)
      throw Error("资源校验失败，请重试");
  }
  return buffer;
}
