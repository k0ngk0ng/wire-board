import { createContext, useContext, useEffect, useState } from "react";
import type { User } from "./types";

export const ProfileContext = createContext<(id: string) => void>(() => {});
export function PlayerName({
  user,
  children,
}: {
  user: User & { bot?: boolean };
  children?: React.ReactNode;
}) {
  const open = useContext(ProfileContext);
  return user.bot ? (
    <span>{children || user.name}</span>
  ) : (
    <button
      className="player-link"
      title={`查看 ${user.name} 的玩家页面`}
      onClick={() => open(user.id)}
    >
      {children || user.name}
    </button>
  );
}
type History = {
  id: string;
  room: string;
  kind: string;
  status: string;
  ended: number;
  players: (User & { bot?: boolean; won: boolean; score?: number })[];
};
type Profile = {
  user: User;
  stats: Record<string, { played: number; wins: number }>;
  history: History[];
  total: number;
  hasMore: boolean;
  relationship: string;
};
type Friend = User & { relationship: string };
async function request(path: string, body?: unknown) {
  const response = await fetch("/api" + path, {
    method: body ? "POST" : "GET",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await response.json();
  if (!response.ok) throw Error(data.error || "操作失败");
  return data;
}
const name = (kind: string) => (kind === "rail" ? "铁路环游" : "璀璨宝石");
export function ProfilePage({ id, self }: { id: string; self: string }) {
  const open = useContext(ProfileContext);
  const [profile, setProfile] = useState<Profile>();
  const [friends, setFriends] = useState<Friend[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState("");
  const [players, setPlayers] = useState<User[]>([]);
  const refresh = async () => {
    const data = await request(`/players/${id}`);
    setProfile(data);
    if (id === self) setFriends((await request("/friends")).friends);
  };
  useEffect(() => {
    let alive = true;
    request(`/players/${id}`)
      .then((data) => {
        if (alive) setProfile(data);
      })
      .catch((e) => {
        if (alive) setError(e.message);
      });
    if (id === self)
      request("/friends")
        .then((data) => {
          if (alive) setFriends(data.friends);
        })
        .catch((e) => {
          if (alive) setError(e.message);
        });
    return () => {
      alive = false;
    };
  }, [id, self]);
  const run = async (fn: () => Promise<unknown>) => {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const friendAction = (target: string, action: string) =>
    void run(async () => {
      await request(`/players/${target}/friend`, { action });
      await refresh();
    });
  if (!profile) return <p role="status">{error || "正在读取玩家资料…"}</p>;
  return (
    <div className="profile-page">
      {id !== self && (
        <button className="subtle" onClick={() => open(self)}>
          ← 我的战绩与好友
        </button>
      )}
      {error && (
        <p role="alert" className="profile-error">
          {error}
        </p>
      )}
      <header className="profile-heading">
        <span className="avatar">{profile.user.name[0]}</span>
        <div>
          <h3>{profile.user.name}</h3>
          <small>{id === self ? "我的战绩与好友" : "玩家战绩"}</small>
        </div>
        <button
          className="subtle"
          disabled={busy}
          onClick={() => void run(refresh)}
        >
          刷新
        </button>
        {profile.relationship === "none" && (
          <button
            disabled={busy}
            className="primary"
            onClick={() => friendAction(id, "request")}
          >
            加好友
          </button>
        )}
        {profile.relationship === "incoming" && (
          <button
            disabled={busy}
            className="primary"
            onClick={() => friendAction(id, "accept")}
          >
            接受好友申请
          </button>
        )}
        {["friends", "outgoing", "incoming"].includes(profile.relationship) && (
          <button
            disabled={busy}
            className="subtle"
            onClick={() => friendAction(id, "remove")}
          >
            {profile.relationship === "friends"
              ? "移除好友"
              : profile.relationship === "outgoing"
                ? "撤回申请"
                : "拒绝"}
          </button>
        )}
      </header>
      <div className="profile-stats">
        {Object.entries(profile.stats).map(([kind, stats]) => (
          <article key={kind}>
            <strong>{name(kind)}</strong>
            <b>
              {stats.wins} 胜 / {stats.played} 局
            </b>
            <small>
              胜率{" "}
              {stats.played ? Math.round((stats.wins / stats.played) * 100) : 0}
              % · 中止不计入
            </small>
          </article>
        ))}
      </div>
      {id === self && (
        <section className="profile-friends">
          <h3>好友与申请</h3>
          {friends.length ? (
            friends.map((friend) => (
              <div className="friend-row" key={friend.id}>
                <PlayerName user={friend} />
                <small>
                  {
                    (
                      {
                        friends: "好友",
                        incoming: "请求加你为好友",
                        outgoing: "等待对方接受",
                      } as Record<string, string>
                    )[friend.relationship]
                  }
                </small>
                {friend.relationship === "incoming" && (
                  <button
                    disabled={busy}
                    className="outline"
                    onClick={() => friendAction(friend.id, "accept")}
                  >
                    接受
                  </button>
                )}
                <button
                  disabled={busy}
                  className="subtle"
                  onClick={() => friendAction(friend.id, "remove")}
                >
                  {friend.relationship === "friends"
                    ? "移除"
                    : friend.relationship === "incoming"
                      ? "拒绝"
                      : "撤回"}
                </button>
              </div>
            ))
          ) : (
            <p>还没有好友或申请，搜索昵称，或者点击牌桌上的玩家姓名。</p>
          )}
          <form
            className="player-search"
            onSubmit={(e) => {
              e.preventDefault();
              void run(async () =>
                setPlayers(
                  (await request(`/players?q=${encodeURIComponent(query)}`))
                    .players,
                ),
              );
            }}
          >
            <input
              aria-label="搜索玩家昵称"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="搜索玩家昵称"
              maxLength={30}
            />
            <button className="outline" disabled={busy}>
              搜索玩家
            </button>
          </form>
          <div className="player-search-results">
            {players.map((user) => (
              <PlayerName key={user.id} user={user} />
            ))}
          </div>
        </section>
      )}
      <section className="profile-history">
        <h3>
          对局历史 <small>{profile.total} 局</small>
        </h3>
        {!profile.total && <p>暂时没有保存的对局记录。</p>}
        {profile.history.map((match) => (
          <article key={match.id}>
            <header>
              <strong>
                {name(match.kind)} · {match.room}
              </strong>
              <span>{match.status === "finished" ? "已结算" : "已中止"}</span>
            </header>
            <time>{new Date(match.ended * 1000).toLocaleString("zh-CN")}</time>
            <div>
              {match.players.map((player) => (
                <span
                  className={`history-player ${player.won ? "winner" : ""}`}
                  key={player.id}
                >
                  <PlayerName user={player} />
                  {player.bot && " · 电脑"}
                  {player.score !== undefined && ` · ${player.score} 分`}
                  {player.won && " · 胜"}
                </span>
              ))}
            </div>
          </article>
        ))}
        {profile.hasMore && (
          <button
            disabled={busy}
            className="outline wide"
            onClick={() =>
              void run(async () => {
                const next: Profile = await request(
                  `/players/${id}?offset=${profile.history.length}`,
                );
                setProfile({
                  ...next,
                  history: [...profile.history, ...next.history],
                });
              })
            }
          >
            加载更多记录
          </button>
        )}
      </section>
    </div>
  );
}
