/** Rejections remain rejections so controllers release/resync their input. */
export function isTransientSyncError(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const e = error as {
    transientSync?: boolean;
    status?: number;
    message?: string;
  };
  return (
    !!e.transientSync ||
    [409, 429, 503].includes(e.status ?? 0) ||
    (error instanceof TypeError &&
      /fetch|network|load failed|网络/i.test(e.message ?? "")) ||
    /实时连接|实时操作确认超时|局面已更新|挥拍时间已过期|来球已更新|服务器已重启/.test(
      e.message ?? "",
    )
  );
}
export function quietSyncError(error: unknown): unknown {
  return isTransientSyncError(error)
    ? Object.assign(new Error(""), {
        name: "SyncPendingError",
        transientSync: true,
        cause: error,
      })
    : error;
}
