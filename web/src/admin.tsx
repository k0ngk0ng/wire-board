import { useEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  ShieldCheck,
  Users,
  Radio,
  Layers3,
  RefreshCw,
  Search,
  Eye,
  EyeOff,
  LockKeyhole,
  X,
  Activity,
  ScrollText,
  CircleStop,
} from "lucide-react";
import type { Room, User } from "./types";
import "./admin.css";

type ManagedUser = User & {
  role: string;
  disabled: boolean;
  online: boolean;
  connections: number;
  activity: string;
  roomId?: string;
  lastSeen: number;
  lastLogin: number;
  createdAt: number;
};
type Game = { kind: string; name: string; enabled: boolean };
type Overview = {
  users: ManagedUser[];
  rooms: Room[];
  games: Game[];
  stats: { users: number; online: number; playing: number; waiting: number };
  serverNow: number;
};
type Audit = {
  id: number;
  actor: string;
  action: string;
  target: string;
  detail: string;
  createdAt: number;
};
type Decision = {
  title: string;
  text: string;
  path: string;
  body: Record<string, unknown>;
  password?: boolean;
};
const roles: Record<string, string> = {
  superadmin: "永久管理员",
  admin: "管理员",
  player: "玩家",
};
const actions: Record<string, string> = {
  root_created: "创建永久管理员",
  root_promoted: "指定永久管理员",
  root_password_rotated: "轮换管理员密码",
  game_published: "上架桌游",
  game_unpublished: "下架桌游",
  user_enable: "恢复账号",
  user_disable: "停用账号",
  user_logout: "强制退出",
  user_password: "重置密码",
  user_role: "调整权限",
  room_closed: "结束牌桌",
};
const date = (value: number) =>
  value
    ? new Date(value * 1000).toLocaleString("zh-CN", {
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
      })
    : "暂无记录";
async function request(path: string, body?: unknown) {
  const r = await fetch("/api" + path, {
    method: body === undefined ? "GET" : "POST",
    headers:
      body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await r.json();
  if (!r.ok) throw new Error(data.error || "管理操作失败");
  return data;
}

export function AdminDashboard({
  user,
  onBack,
  onChange,
}: {
  user: User;
  onBack: () => void;
  onChange: () => Promise<void>;
}) {
  const [data, setData] = useState<Overview>();
  const [tab, setTab] = useState("games");
  const [query, setQuery] = useState("");
  const [onlineOnly, setOnlineOnly] = useState(false);
  const [decision, setDecision] = useState<Decision>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState("");
  const [logs, setLogs] = useState<Audit[]>([]);
  const [more, setMore] = useState(false);
  const generation = useRef(0);
  const dialog = useRef<HTMLDivElement>(null);
  async function refresh() {
    const n = ++generation.current;
    try {
      const next = await request("/admin");
      if (n === generation.current) {
        setData(next);
        setLoadError("");
      }
    } catch (e) {
      if (n === generation.current) setLoadError((e as Error).message);
    }
  }
  async function audit(append = false) {
    try {
      const next = await request(
        "/admin/audit" +
          (append && logs.length ? `?before=${logs[logs.length - 1].id}` : ""),
      );
      setLogs((old) => (append ? [...old, ...next.entries] : next.entries));
      setMore(next.hasMore);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  useEffect(() => {
    void refresh();
    const timer = setInterval(() => void refresh(), 5000);
    return () => {
      clearInterval(timer);
      generation.current++;
    };
  }, []);
  useEffect(() => {
    if (tab === "audit") void audit();
  }, [tab]);
  useEffect(() => {
    if (!decision) return;
    const previous = document.activeElement as HTMLElement;
    dialog.current?.querySelector<HTMLElement>("input,button")?.focus();
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) setDecision(undefined);
      if (e.key === "Tab") {
        const nodes = Array.from(
          dialog.current?.querySelectorAll<HTMLElement>(
            "input,button:not(:disabled)",
          ) || [],
        );
        const first = nodes[0],
          last = nodes[nodes.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last?.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first?.focus();
        }
      }
    };
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      previous?.focus();
    };
  }, [decision, busy]);
  async function submit(password?: string) {
    if (!decision || busy) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await request(decision.path, {
        ...decision.body,
        ...(password ? { password } : {}),
      });
      setNotice(`${decision.title}，已生效`);
      setDecision(undefined);
      await refresh();
      await onChange();
      if (tab === "audit") await audit();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  function manage(target: ManagedUser, action: string) {
    const title = {
      disable: "停用",
      enable: "恢复",
      logout: "强制退出",
      password: "重置密码",
      role: target.role === "admin" ? "移除管理员权限" : "设为管理员",
    }[action];
    const text =
      action === "disable"
        ? "该账号将立即退出且无法登录。进行中的席位交由电脑托管，等待中的席位会移除，战绩与好友保留。"
        : action === "logout"
          ? "使该账号的所有登录会话失效，正在进行的席位转为电脑托管。玩家可以重新登录。"
          : action === "password"
            ? "设置新密码后，原有登录会话全部失效。请私下把新密码告知玩家。"
            : action === "role"
              ? "管理员可以上下架桌游、管理普通用户、查看在线情况和结束牌桌；不能修改永久管理员或其他管理员。"
              : "恢复登录资格，原有战绩与好友保持不变。";
    setDecision({
      title: `${title} · ${target.name}`,
      text,
      path: `/admin/users/${target.id}`,
      body: {
        action,
        ...(action === "role"
          ? { role: target.role === "admin" ? "player" : "admin" }
          : {}),
      },
      password: action === "password",
    });
  }
  const filtered = (data?.users || [])
    .filter(
      (u) =>
        (!onlineOnly || u.online) &&
        (!query || u.name.toLowerCase().includes(query.toLowerCase())),
    )
    .sort(
      (a, b) =>
        Number(b.online) - Number(a.online) || a.name.localeCompare(b.name),
    );
  return (
    <main className="admin-page">
      <div className="admin-heading">
        <div>
          <span className="eyebrow">KEEP THE TABLE WELCOMING</span>
          <h1>
            <ShieldCheck size={30} />
            围桌管理室
          </h1>
          <p>管理游戏与玩家，照看每一张牌桌。</p>
        </div>
        <button className="outline" onClick={onBack}>
          <ArrowLeft size={16} />
          返回牌桌与大厅
        </button>
      </div>
      <div className="admin-identity">
        <ShieldCheck size={19} />
        <strong>{user.name}</strong>
        <span>{roles[user.role || "player"]}</span>
        {user.role === "superadmin" && (
          <small>永久权限 · 不可被停用或降权</small>
        )}
        <button className="subtle" onClick={() => void refresh()}>
          <RefreshCw size={15} />
          刷新
        </button>
      </div>
      {(error || loadError) && (
        <div className="admin-message error" role="alert">
          {error || loadError}
        </div>
      )}
      {notice && (
        <div className="admin-message" role="status">
          {notice}
          <button aria-label="关闭提示" onClick={() => setNotice("")}>
            <X size={15} />
          </button>
        </div>
      )}
      {!data ? (
        <p className="admin-empty">正在读取管理信息…</p>
      ) : (
        <>
          <section className="admin-metrics" aria-label="站点概览">
            {[
              [Users, "注册玩家", data.stats.users, "含管理员账号"],
              [
                Radio,
                "当前在线",
                data.stats.online,
                "最近 60 秒内有连接或活动",
              ],
              [Activity, "进行中的牌桌", data.stats.playing, "真人与电脑对局"],
              [Layers3, "等待开局", data.stats.waiting, "朋友们正在入座"],
            ].map(([Icon, label, value, description], i) => {
              const I = Icon as typeof Users;
              return (
                <article key={i}>
                  <I size={22} />
                  <span>{label as string}</span>
                  <strong>{value as number}</strong>
                  <small>{description as string}</small>
                </article>
              );
            })}
          </section>
          <nav className="admin-tabs" aria-label="管理分类">
            {[
              ["games", "游戏上架", Layers3],
              ["users", "用户与在线", Users],
              ["rooms", "牌桌管理", Activity],
              ["audit", "操作记录", ScrollText],
            ].map(([key, label, Icon]) => {
              const I = Icon as typeof Users;
              return (
                <button
                  key={key as string}
                  className={tab === key ? "active" : ""}
                  aria-current={tab === key ? "page" : undefined}
                  onClick={() => setTab(key as string)}
                >
                  <I size={17} />
                  {label as string}
                </button>
              );
            })}
          </nav>
          {tab === "games" && (
            <section>
              <div className="admin-section-head">
                <h2>开放给朋友的游戏</h2>
                <p>下架会隐藏入口、关闭等待牌桌；已经开始的对局可以继续。</p>
              </div>
              <div className="admin-games">
                {data.games.map((g, i) => (
                  <article
                    key={g.kind}
                    className={`admin-game-card ${!g.enabled ? "unpublished" : ""}`}
                  >
                    <div className={`admin-game-number tone-${i}`}>
                      {String(i + 1).padStart(2, "0")}
                    </div>
                    <div>
                      <span
                        className={`admin-badge ${g.enabled ? "green" : "gray"}`}
                      >
                        {g.enabled ? "已上架" : "已下架"}
                      </span>
                      <h3>{g.name}</h3>
                      <p>
                        {g.enabled
                          ? "大厅可见，可以创建牌桌"
                          : "已从大厅和创建列表隐藏"}
                      </p>
                    </div>
                    <button
                      className={g.enabled ? "outline" : "primary"}
                      onClick={() =>
                        setDecision({
                          title: `${g.enabled ? "下架" : "上架"} · ${g.name}`,
                          text: g.enabled
                            ? "等待开局的牌桌会关闭，用户不能新建、加入或再开一局。正在进行的对局及历史战绩保留。"
                            : "用户将在大厅看到这款游戏，并可正常创建、加入和开始对局。",
                          path: `/admin/games/${g.kind}`,
                          body: { enabled: !g.enabled },
                        })
                      }
                    >
                      {g.enabled ? <EyeOff size={16} /> : <Eye size={16} />}{" "}
                      {g.enabled ? "下架" : "上架"}
                    </button>
                  </article>
                ))}
              </div>
            </section>
          )}
          {tab === "users" && (
            <section>
              <div className="admin-section-head">
                <h2>玩家与在线情况</h2>
                <p>
                  每 5
                  秒刷新。多个标签页按同一个玩家统计；离线玩家的托管状态也会保留显示。
                </p>
              </div>
              <div className="admin-filter">
                <label className="admin-search">
                  <Search size={17} />
                  <input
                    aria-label="搜索玩家昵称"
                    placeholder="搜索玩家昵称"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                  />
                </label>
                <label className="admin-check">
                  <input
                    type="checkbox"
                    checked={onlineOnly}
                    onChange={(e) => setOnlineOnly(e.target.checked)}
                  />
                  只看在线 <b>{data.stats.online}</b>
                </label>
              </div>
              <div className="admin-user-list">
                {filtered.map((u) => {
                  const protectedUser =
                    u.role === "superadmin" ||
                    u.id === user.id ||
                    (u.role === "admin" && user.role !== "superadmin");
                  return (
                    <article className="admin-user-card" key={u.id}>
                      <div className="admin-user-name">
                        <span className="admin-avatar">{u.name[0]}</span>
                        <div>
                          <h3>
                            {u.name}
                            <span
                              className={`admin-badge ${u.disabled ? "red" : u.role !== "player" ? "gold" : "gray"}`}
                            >
                              {u.disabled ? "已停用" : roles[u.role]}
                            </span>
                          </h3>
                          <p>
                            <span
                              className={`admin-presence ${u.online ? "is-online" : ""}`}
                            />
                            {u.online ? "在线" : "离线"} · {u.activity}
                            {u.connections > 1
                              ? ` · ${u.connections} 个连接`
                              : ""}
                            {u.roomId ? ` · ${u.roomId.toUpperCase()}` : ""}
                          </p>
                        </div>
                      </div>
                      <div className="admin-user-times">
                        <span>
                          最近活跃 <b>{date(u.lastSeen)}</b>
                        </span>
                        <span>
                          最近登录 <b>{date(u.lastLogin)}</b>
                        </span>
                      </div>
                      <div className="admin-user-actions">
                        {protectedUser ? (
                          <span className="admin-protected">
                            <LockKeyhole size={15} />
                            {u.role === "superadmin"
                              ? "永久账号保护"
                              : u.id === user.id
                                ? "当前登录账号"
                                : "需永久管理员操作"}
                          </span>
                        ) : (
                          <>
                            <button
                              className="subtle"
                              onClick={() =>
                                manage(u, u.disabled ? "enable" : "disable")
                              }
                            >
                              {u.disabled ? "恢复账号" : "停用账号"}
                            </button>
                            <button
                              className="subtle"
                              onClick={() => manage(u, "logout")}
                            >
                              强制退出
                            </button>
                            <button
                              className="subtle"
                              onClick={() => manage(u, "password")}
                            >
                              重置密码
                            </button>
                            {user.role === "superadmin" && (
                              <button
                                className="subtle"
                                onClick={() => manage(u, "role")}
                              >
                                {u.role === "admin"
                                  ? "移除管理权限"
                                  : "设为管理员"}
                              </button>
                            )}
                          </>
                        )}
                      </div>
                    </article>
                  );
                })}
                {!filtered.length && (
                  <p className="admin-empty">没有符合条件的玩家。</p>
                )}
              </div>
            </section>
          )}
          {tab === "rooms" && (
            <section>
              <div className="admin-section-head">
                <h2>所有活动牌桌</h2>
                <p>包括已下架游戏中尚未结束的牌桌。管理界面仅展示公开信息。</p>
              </div>
              <div className="admin-room-grid">
                {data.rooms.map((room) => (
                  <article className="admin-room-card" key={room.id}>
                    <span className="admin-badge green">
                      {room.status === "playing" ? "进行中" : "等待开局"}
                    </span>
                    <h3>{room.name}</h3>
                    <p>
                      {data.games.find((g) => g.kind === room.kind)?.name} ·{" "}
                      {room.id.toUpperCase()}
                    </p>
                    <div className="admin-room-seats">
                      {room.seats
                        .filter((p) => !p.left)
                        .map((p) => (
                          <span key={p.id}>
                            {p.bot ? "🤖 " : ""}
                            {p.name}
                            {p.autoPlay ? " · 托管" : ""}
                          </span>
                        ))}
                    </div>
                    <small>
                      {room.seats.filter((p) => !p.left).length} /{" "}
                      {room.capacity} 人 · {room.spectatorCount || 0} 人观战
                    </small>
                    <button
                      className="outline danger"
                      onClick={() =>
                        setDecision({
                          title: `结束牌桌 · ${room.name}`,
                          text: "此操作会立即关闭牌桌，终止当前比赛，本局不计胜负与积分。玩家之后可以离开或重新开局。",
                          path: `/admin/rooms/${room.id}/close`,
                          body: {},
                        })
                      }
                    >
                      <CircleStop size={16} />
                      结束牌桌
                    </button>
                  </article>
                ))}
              </div>
              {!data.rooms.length && (
                <p className="admin-empty">暂时没有活动牌桌。</p>
              )}
            </section>
          )}
          {tab === "audit" && (
            <section>
              <div className="admin-section-head">
                <h2>管理操作记录</h2>
                <p>
                  记录操作人、时间与对象；不会记录密码内容。
                  <button className="subtle" onClick={() => void audit()}>
                    刷新记录
                  </button>
                </p>
              </div>
              <div className="admin-audit">
                {logs.map((log) => (
                  <article key={log.id}>
                    <span className="admin-audit-dot" />
                    <div>
                      <strong>{actions[log.action] || log.action}</strong>
                      <p>{log.detail}</p>
                      <small>
                        {log.actor} · {date(log.createdAt)}
                      </small>
                    </div>
                  </article>
                ))}
              </div>
              {more && (
                <button className="outline" onClick={() => void audit(true)}>
                  加载更早记录
                </button>
              )}
              {!logs.length && (
                <p className="admin-empty">暂无管理操作记录。</p>
              )}
            </section>
          )}
        </>
      )}
      {decision && (
        <div
          className="modal-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget && !busy) setDecision(undefined);
          }}
        >
          <div
            className="modal admin-confirm"
            role="dialog"
            aria-modal="true"
            aria-label={decision.title}
            ref={dialog}
          >
            <div className="modal-head">
              <h2>{decision.title}</h2>
              <button
                className="icon-button"
                disabled={busy}
                aria-label="关闭确认"
                onClick={() => setDecision(undefined)}
              >
                <X size={20} />
              </button>
            </div>
            <p>{decision.text}</p>
            {error && (
              <div className="admin-message error" role="alert">
                {error}
              </div>
            )}
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void submit(
                  new FormData(e.currentTarget).get("password")?.toString(),
                );
              }}
            >
              {decision.password && (
                <label>
                  新密码
                  <input
                    name="password"
                    type="password"
                    required
                    minLength={12}
                    maxLength={72}
                    autoComplete="new-password"
                    placeholder="至少 12 字节，最长 72 字节"
                  />
                </label>
              )}
              <div className="admin-confirm-actions">
                <button
                  className="outline"
                  type="button"
                  disabled={busy}
                  onClick={() => setDecision(undefined)}
                >
                  取消
                </button>
                <button className="primary" disabled={busy}>
                  {busy ? "正在保存…" : "确认操作"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </main>
  );
}
