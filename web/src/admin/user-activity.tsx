import { useEffect, useState } from "react";
import { Modal } from "../ui/board-host";
import type { User } from "../types";
import { RefreshButton } from "../ui/refresh-button";
import type { HistoryTarget } from "./room-history";
const labels: Record<string, string> = {
  register: "注册账号",
  login: "登录",
  logout: "退出登录",
  password_change: "修改密码",
  create_room: "创建牌桌",
  join: "加入牌桌",
  leave: "离开牌桌",
  ready: "切换准备状态",
  start: "开始对局",
  close: "关闭牌桌",
  restart: "重新开局",
  watch: "切换观战",
  add_bot: "添加电脑",
  remove_bot: "移除电脑",
  autoplay: "切换托管",
  friend_request: "发送好友申请",
  friend_accept: "接受好友申请",
  friend_remove: "移除好友或申请",
  wish: "提交许愿",
};
type Entry = {
  room?: {
    id: string;
    kind?: string;
    name?: string;
    gameName?: string;
    matchId?: string;
  };
  id: number;
  action: string;
  target: string;
  ip: string;
  createdAt: number;
};
export function UserActivity({
  user,
  onClose,
  onHistory,
}: {
  user: User;
  onClose: () => void;
  onHistory: (target: HistoryTarget) => void;
}) {
  const [entries, setEntries] = useState<Entry[]>([]),
    [more, setMore] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  async function load(append = false) {
    setBusy(true);
    setError("");
    try {
      const r = await fetch(
        `/api/admin/users/${user.id}/activity${append && entries.length ? `?before=${entries.at(-1)!.id}` : ""}`,
      );
      const data = await r.json();
      if (!r.ok) throw Error(data.error || "无法读取玩家动态");
      setEntries((old) => (append ? [...old, ...data.entries] : data.entries));
      setMore(data.hasMore);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  useEffect(() => {
    void load();
  }, [user.id]);
  return (
    <Modal
      title={`${user.name} 的动态`}
      onClose={onClose}
      className="user-activity-modal"
    >
      <RefreshButton onRefresh={() => load()} disabled={busy} />
      {error && <p role="alert">{error}</p>}
      <div className="admin-audit admin-user-activity">
        {entries.map((e) => (
          <article key={e.id}>
            <div>
              <strong>{labels[e.action] || e.action}</strong>
              {e.room ? (
                <>
                  <p>
                    {e.room.gameName || "游戏未记录"}
                    {e.room.name ? ` · ${e.room.name}` : ""}
                  </p>
                  <small>牌桌 {e.room.id.toUpperCase()}</small>
                  <button
                    className="subtle"
                    onClick={() =>
                      onHistory(
                        e.room?.kind
                          ? {
                              roomId: e.room.id,
                              kind: e.room.kind,
                              title: `牌桌 ${e.room.id.toUpperCase()} 的记录`,
                            }
                          : {
                              userId: user.id,
                              title: `${user.name} 的历史牌桌`,
                            },
                      )
                    }
                  >
                    {e.room.kind ? "查看牌桌记录" : "查看玩家历史"}
                  </button>
                </>
              ) : (
                e.target && <p>{e.target}</p>
              )}
              <small>
                {new Date(e.createdAt * 1000).toLocaleString("zh-CN")} · IP：
                {e.ip || "暂无记录"}
              </small>
            </div>
          </article>
        ))}
      </div>
      {!entries.length && !busy && !error && <p>暂无动态记录。</p>}
      {more && (
        <button
          className="outline"
          disabled={busy}
          onClick={() => void load(true)}
        >
          加载更早动态
        </button>
      )}
    </Modal>
  );
}
