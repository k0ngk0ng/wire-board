import { useEffect, useState } from "react";
import { Modal } from "../ui/board-host";
import type { User } from "../types";
import { RefreshButton } from "../ui/refresh-button";
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
  id: number;
  action: string;
  target: string;
  ip: string;
  createdAt: number;
};
export function UserActivity({
  user,
  onClose,
}: {
  user: User;
  onClose: () => void;
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
              {e.target && <p>{e.target}</p>}
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
