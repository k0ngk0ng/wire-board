import { useEffect, useRef, useState } from "react";
import { RefreshButton } from "../ui/refresh-button";

export type HistoryTarget = {
  roomId?: string;
  userId?: string;
  kind?: string;
  title?: string;
};
type Entry = {
  id: string;
  roomId?: string;
  room: string;
  kind: string;
  gameName: string;
  status: string;
  ended: number;
  rated: boolean;
  draw?: boolean;
  players: {
    id: string;
    name: string;
    bot?: boolean;
    won: boolean;
    score?: number;
  }[];
};
export function RoomHistory({
  games,
  initial = {},
  heading = true,
}: {
  games: { kind: string; name: string }[];
  initial?: HistoryTarget;
  heading?: boolean;
}) {
  const [query, setQuery] = useState(""),
    [search, setSearch] = useState(""),
    [kind, setKind] = useState(initial.kind ?? ""),
    [status, setStatus] = useState("");
  const [entries, setEntries] = useState<Entry[]>([]),
    [cursor, setCursor] = useState(""),
    [more, setMore] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const pending = useRef<AbortController | undefined>(undefined);
  useEffect(() => {
    const timer = setTimeout(() => setSearch(query), 250);
    return () => clearTimeout(timer);
  }, [query]);
  async function load(append = false) {
    pending.current?.abort();
    const controller = new AbortController();
    pending.current = controller;
    setBusy(true);
    setError("");
    if (!append) setEntries([]);
    const values = {
      q: search,
      kind,
      status,
      roomId: initial.roomId ?? "",
      userId: initial.userId ?? "",
      before: append ? cursor : "",
    };
    const params = new URLSearchParams(
      Object.entries(values).filter(([, v]) => v),
    );
    try {
      const response = await fetch(`/api/admin/rooms/history?${params}`, {
        signal: controller.signal,
      });
      const data = await response.json();
      if (!response.ok) throw Error(data.error || "无法读取历史牌桌");
      if (controller.signal.aborted) return;
      setEntries((old) =>
        append
          ? [
              ...old,
              ...(data.entries as Entry[]).filter(
                (e) => !old.some((p) => p.id === e.id),
              ),
            ]
          : data.entries,
      );
      setCursor(data.nextBefore ?? "");
      setMore(data.hasMore);
    } catch (e) {
      if (!controller.signal.aborted) {
        setError((e as Error).message);
        setEntries([]);
        setMore(false);
      }
    } finally {
      if (pending.current === controller) setBusy(false);
    }
  }
  useEffect(() => {
    void load();
    return () => pending.current?.abort();
  }, [search, kind, status, initial.roomId, initial.userId]);
  return (
    <div className="admin-history" data-admin-room-history>
      <div className="admin-section-head">
        {heading && <h2>历史牌桌</h2>}
        <RefreshButton disabled={busy} onRefresh={() => load()} />
      </div>
      <div className="admin-history-filters">
        <label>
          搜索
          <input
            type="search"
            aria-label="搜索历史牌桌"
            placeholder="牌桌号、对局号、名称或玩家"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
        <label>
          游戏
          <select
            aria-label="历史牌桌游戏"
            value={kind}
            onChange={(e) => setKind(e.target.value)}
          >
            <option value="">全部游戏</option>
            {games.map((g) => (
              <option key={g.kind} value={g.kind}>
                {g.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          结果
          <select
            aria-label="历史牌桌状态"
            value={status}
            onChange={(e) => setStatus(e.target.value)}
          >
            <option value="">全部结果</option>
            <option value="finished">已结束</option>
            <option value="closed">已关闭</option>
          </select>
        </label>
      </div>
      {error && <p role="alert">{error}</p>}
      <div className="admin-room-grid">
        {entries.map((e) => (
          <article
            className="admin-room-card admin-history-card"
            key={e.id}
            data-admin-history-id={e.id}
            data-admin-history-kind={e.kind}
          >
            <span className="admin-badge">
              {e.status === "closed" ? "已关闭" : e.draw ? "和局" : "已结束"}
            </span>
            <h3>{e.gameName}</h3>
            <p>{e.room}</p>
            <div className="admin-history-identity">
              {e.roomId && <small>牌桌 {e.roomId.toUpperCase()}</small>}
              <small>对局 {e.id.toUpperCase()}</small>
              <small>{new Date(e.ended * 1000).toLocaleString("zh-CN")}</small>
            </div>
            <div className="admin-room-seats">
              {e.players.map((p) => (
                <span key={p.id}>
                  {p.bot ? "🤖 " : ""}
                  {p.name}
                  {p.won ? " · 获胜" : ""}
                  {p.score !== undefined ? ` · ${p.score} 分` : ""}
                </span>
              ))}
            </div>
          </article>
        ))}
      </div>
      {!entries.length && !busy && !error && (
        <p className="admin-empty">暂无符合条件的已结束或已关闭对局。</p>
      )}
      {more && (
        <button className="outline" disabled={busy} onClick={() => load(true)}>
          加载更早牌桌
        </button>
      )}
    </div>
  );
}
