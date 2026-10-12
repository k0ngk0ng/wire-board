import { useContext, useEffect, useState } from "react";
import { AssetsContext, Modal } from "../ui/board-host";
import type { Act, Room } from "../types";
import { gameModule, gameName } from "./registry";

export function GameBoard({
  room,
  act,
  command,
  busy,
  assets,
}: {
  room: Room;
  act: Act;
  command: (type: string, extra?: Record<string, unknown>) => Promise<void>;
  busy: boolean;
  assets: string;
}) {
  const module = gameModule(room.kind);
  if (!module) return <p role="alert">尚未安装此游戏的界面：{room.kind}</p>;
  const Board = module.Board;
  return (
    <Board
      room={room}
      data={room.game?.data}
      act={act}
      command={command}
      busy={busy}
      assets={assets}
    />
  );
}

export function ModuleCreate({
  kind,
  availableGames,
  onGameChange,
  busy,
  onClose,
  onSubmit,
}: {
  kind: string;
  availableGames: string[];
  onGameChange: (kind: string) => void;
  busy: boolean;
  onClose: () => void;
  onSubmit: (body: unknown) => void;
}) {
  const module = gameModule(kind)!;
  const [capacity, setCapacity] = useState(module.minPlayers);
  const [options, setOptions] = useState<unknown>(module.defaultOptions ?? {});
  const Options = module.Options;
  return (
    <Modal title={`创建${module.name}牌桌`} onClose={onClose}>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          const form = new FormData(event.currentTarget);
          onSubmit({
            kind,
            capacity,
            options,
            name: form.get("name"),
            password: form.get("password"),
          });
        }}
      >
        <label>
          游戏
          <select
            aria-label="游戏"
            value={kind}
            onChange={(event) => onGameChange(event.target.value)}
            disabled={busy}
          >
            {availableGames.map((id) => (
              <option key={id} value={id}>
                {gameName(id)}
              </option>
            ))}
          </select>
        </label>
        <label>
          房间名称
          <input
            name="name"
            required
            maxLength={40}
            defaultValue={`${module.name}好友局`}
          />
        </label>
        <label>
          人数
          <select
            value={capacity}
            onChange={(event) => setCapacity(Number(event.target.value))}
            disabled={busy}
          >
            {Array.from(
              { length: module.maxPlayers - module.minPlayers + 1 },
              (_, i) => i + module.minPlayers,
            ).map((n) => (
              <option key={n} value={n}>
                {n} 人
              </option>
            ))}
          </select>
        </label>
        <label>
          房间密码
          <input
            name="password"
            type="password"
            maxLength={72}
            autoComplete="new-password"
          />
        </label>
        {Options && (
          <Options
            value={options}
            onChange={setOptions}
            capacity={capacity}
            disabled={busy}
          />
        )}
        <button className="primary wide" disabled={busy} type="submit">
          创建房间
        </button>
      </form>
    </Modal>
  );
}

export function ModuleWaiting({
  room,
  busy,
  host,
  command,
}: {
  room: Room;
  busy: boolean;
  host: boolean;
  command: (type: string, extra?: Record<string, unknown>) => void;
}) {
  const module = gameModule(room.kind)!;
  const [options, setOptions] = useState<unknown>(room.options ?? {});
  const savedOptions = JSON.stringify(room.options ?? {});
  useEffect(
    () => setOptions(JSON.parse(savedOptions)),
    [room.id, savedOptions],
  );
  const Options = module.Options;
  return (
    <section className="waiting">
      <h2>{module.name}</h2>
      <p>{module.description}</p>
      <div className="players-strip">
        {room.seats.map((seat) => (
          <div className="player-box" key={seat.id}>
            <strong>{seat.name}</strong> · {seat.ready ? "已准备" : "未准备"}
            {host && seat.bot && (
              <button
                disabled={busy}
                onClick={() => command("remove_bot", { target: seat.id })}
              >
                移除电脑
              </button>
            )}
          </div>
        ))}
      </div>
      {Options && (
        <>
          <Options
            value={options}
            onChange={setOptions}
            capacity={room.capacity}
            disabled={!host || busy}
          />
          {host && (
            <button
              disabled={busy}
              onClick={() => command("options", { options })}
            >
              保存设置
            </button>
          )}
        </>
      )}
      <div className="waiting-actions">
        <button
          className="primary"
          disabled={busy}
          onClick={() => command("ready")}
        >
          {room.seats[room.you]?.ready ? "取消准备" : "准备"}
        </button>
        {host && module.supportsBots && room.seats.length < room.capacity && (
          <button
            className="outline"
            disabled={busy}
            onClick={() => command("add_bot")}
          >
            添加电脑
          </button>
        )}
        {host && (
          <button
            className="primary"
            disabled={
              busy ||
              room.seats.length < module.minPlayers ||
              room.seats.some((seat) => !seat.ready)
            }
            onClick={() => command("start")}
          >
            开始游戏
          </button>
        )}
      </div>
    </section>
  );
}

export function ModulePlayers({ room }: { room: Room }) {
  return (
    <div className="players-strip">
      {room.seats.map((seat, i) => (
        <div className="player-box" data-player-seat={i} key={seat.id}>
          <strong>{seat.name}</strong>
          {room.game?.actors?.includes(i) && <span> · 等待行动</span>}
          {seat.autoPlay && <span> · 托管中</span>}
        </div>
      ))}
    </div>
  );
}

export function ModuleTurn({
  room,
  serverNow,
  receivedAt,
}: {
  room: Room;
  serverNow: number;
  receivedAt: number;
}) {
  const [now, setNow] = useState(performance.now());
  useEffect(() => {
    const id = setInterval(() => setNow(performance.now()), 250);
    return () => clearInterval(id);
  }, []);
  const remaining = Math.max(
    0,
    Math.ceil(
      ((room.turnDeadline || 0) - serverNow - Math.max(0, now - receivedAt)) /
        1000,
    ),
  );
  return (
    <section className="turn-info">
      <h3>
        {room.game?.finished
          ? "本局已结束"
          : room.game?.actors?.includes(room.you)
            ? "请行动"
            : "等待其他玩家"}
      </h3>
      {!!room.turnDeadline && !room.game?.finished && (
        <p>剩余 {remaining} 秒</p>
      )}
    </section>
  );
}

export function ModuleRules({ kind }: { kind: string }) {
  const module = gameModule(kind);
  const Rules = module?.Rules;
  return Rules ? <Rules /> : <p>{module?.description || gameName(kind)}</p>;
}

export function ModuleCover({ kind }: { kind: string }) {
  const module = gameModule(kind)!;
  const Cover = module.Cover;
  const assets = useContext(AssetsContext);
  // AssetsContext remains available to game-owned cover components.
  return (
    <AssetsContext.Provider value={assets}>
      <div className="game-cover">
        {Cover ? <Cover /> : <strong>{module.name}</strong>}
      </div>
    </AssetsContext.Provider>
  );
}
