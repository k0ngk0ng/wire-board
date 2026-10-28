import type { GameSetupFormProps } from "../ui/create-room";
import { useContext, useEffect, useState } from "react";
import { ArrowRight, Check, Clock, Crown, Plus } from "lucide-react";
import { AssetsContext } from "../ui/board-host";
import type { Act, Room } from "../types";
import "./host.css";
import type { GameAudioPort, GameClock } from "./module";
import { PlayerName } from "../profiles";
import { gameModule, gameName } from "./registry";
import { cooperativeRatingNotice } from "../ui/cooperative-rating";

export function GameBoard({
  room,
  act,
  command,
  busy,
  assets,
  sound,
  audio,
  clock,
  inputWindow,
  loadEvents,
}: {
  room: Room;
  act: Act;
  command: (type: string, extra?: Record<string, unknown>) => Promise<void>;
  busy: boolean;
  assets: string;
  sound: boolean;
  audio: GameAudioPort;
  clock: GameClock;
  inputWindow?: number;
  loadEvents?: Pick<
    import("./module").BoardProps,
    "onReady" | "onLoadProgress" | "onLoadError"
  >;
}) {
  const module = gameModule(room.kind);
  if (!module) return <p role="alert">尚未安装此游戏的界面：{room.kind}</p>;
  const Board = module.Board;
  return (
    <Board
      {...loadEvents}
      room={room}
      data={room.game?.data}
      act={act}
      inputWindow={inputWindow}
      command={command}
      busy={busy}
      assets={assets}
      sound={sound}
      audio={audio}
      clock={clock}
    />
  );
}

export function ModuleCreate({
  kind,
  busy,
  onSubmit,
  draft,
  onDraftChange,
}: GameSetupFormProps) {
  const module = gameModule(kind)!;
  const [capacity, setCapacity] = useState(module.minPlayers);
  const [options, setOptions] = useState<unknown>(module.defaultOptions ?? {});
  const Options = module.Options;
  return (
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
        房间名称
        <input
          name="name"
          required
          maxLength={40}
          placeholder="例如：周五晚上的快乐"
          value={draft.name}
          disabled={busy}
          onChange={(e) => onDraftChange({ name: e.target.value })}
        />
      </label>
      <div className="form-grid">
        <label>
          座位上限
          <select
            aria-label="座位上限"
            value={capacity}
            disabled={busy}
            onChange={(e) => setCapacity(Number(e.target.value))}
          >
            {(
              module.playerCounts ??
              Array.from(
                { length: module.maxPlayers - module.minPlayers + 1 },
                (_, i) => module.minPlayers + i,
              )
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
            value={draft.password}
            disabled={busy}
            onChange={(e) => onDraftChange({ password: e.target.value })}
            type="password"
            maxLength={72}
            placeholder="留空即可直接加入"
            autoComplete="off"
          />
        </label>
      </div>
      {Options && (
        <Options
          value={options}
          onChange={setOptions}
          capacity={capacity}
          disabled={busy}
        />
      )}
      <p className="muted small">
        {module.requiresFullTable
          ? `需要坐满 ${capacity} 人后开始。`
          : `至少 ${module.minPlayers} 个座位即可开始，不必坐满。`}
        {module.supportsBots && "一个人也可以在房间里添加电脑玩家体验。"}
      </p>
      <button className="primary wide" disabled={busy} type="submit">
        创建房间
        <ArrowRight size={18} />
      </button>
      {module.ratingPolicy === "coop_campaign_v1" && (
        <p className="muted small">
          {cooperativeRatingNotice(!!module.supportsBots)}
        </p>
      )}
    </form>
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
  const assets = useContext(AssetsContext);
  const Preparation = module.Preparation;
  const [options, setOptions] = useState<unknown>(room.options ?? {});
  const savedOptions = JSON.stringify(room.options ?? {});
  useEffect(
    () => setOptions(JSON.parse(savedOptions)),
    [room.id, savedOptions],
  );
  const Options = module.Options;
  const optionsDirty = !!Options && JSON.stringify(options) !== savedOptions;
  // Ready confirmations belong to the saved rules. A host previewing a new
  // draft must not carry human confirmations over to that draft; bots remain
  // ready, just as they do when the server saves the changed options.
  const draftNeedsReady = host && optionsDirty;
  return (
    <div className="waiting-layout">
      <div className="waiting-cover">
        <ModuleCover kind={room.kind} />
        <h2>{module.name}</h2>
        <p>{module.description}</p>
        <span className="tag">
          {module.playerCounts?.join(" / ") ??
            `${module.minPlayers}～${module.maxPlayers}`}{" "}
          人 ·{" "}
          {module.supportsBots
            ? "支持电脑与托管"
            : module.minPlayers === 1
              ? "单人或好友同玩"
              : "好友对局"}
        </span>
      </div>
      <div className="waiting-seats">
        <span className="eyebrow">TAKE YOUR SEAT</span>
        <h2>
          {room.capacity === 1
            ? "准备好，就能开局。"
            : module.supportsBots
              ? "朋友或电脑，到齐就开局。"
              : "好友到齐，就能开局。"}
        </h2>
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
                className="outline"
                disabled={busy || !optionsDirty}
                onClick={() => command("options", { options })}
              >
                保存设置
              </button>
            )}
            {draftNeedsReady && (
              <p className="muted small" role="status">
                地图或规则尚未保存；保存后真人需重新准备。
                {module.supportsBots && "电脑保持准备。"}
              </p>
            )}
          </>
        )}
        {Preparation && (
          <Preparation
            room={room}
            options={room.options ?? module.defaultOptions ?? {}}
            host={host}
            busy={busy}
            assets={assets}
            prepare={(action) => command("prepare", { action })}
          />
        )}
        <div className="seats-grid">
          {Array.from({ length: room.capacity }, (_, i) => {
            const seat = room.seats[i];
            return (
              <div key={i} className={`waiting-seat ${seat ? "filled" : ""}`}>
                <span
                  className="avatar"
                  style={{
                    background: seat
                      ? seatColors[i % seatColors.length]
                      : undefined,
                  }}
                >
                  {seat ? seat.name[0] : <Plus size={22} />}
                </span>
                <h3>
                  {seat ? <PlayerName user={seat} /> : "虚位以待"}
                  {seat?.id === room.host && <Crown size={14} />}{" "}
                  {seat?.bot && <small className="bot-badge">AI</small>}
                </h3>
                <span className="waiting-seat-status">
                  {seat
                    ? seat.ready && (!draftNeedsReady || seat.bot)
                      ? "✓ 已准备"
                      : "等待准备"
                    : "邀请一位朋友"}
                </span>
                {host && seat?.bot && (
                  <button
                    className="bot-remove"
                    disabled={busy}
                    aria-label={`移除${seat.name}`}
                    onClick={() => command("remove_bot", { target: seat.id })}
                  >
                    移除
                  </button>
                )}
              </div>
            );
          })}
        </div>
        {module.ratingPolicy === "coop_campaign_v1" && (
          <p className="muted small">
            {cooperativeRatingNotice(!!module.supportsBots)}
          </p>
        )}
        {!room.spectating && (
          <div className="waiting-actions">
            {host &&
              module.supportsBots &&
              room.seats.length < room.capacity && (
                <button
                  className="outline"
                  disabled={busy}
                  onClick={() => command("add_bot")}
                >
                  添加电脑玩家
                </button>
              )}
            <button
              className={
                draftNeedsReady
                  ? "primary"
                  : room.seats[room.you]?.ready
                    ? "outline"
                    : "primary"
              }
              disabled={busy}
              onClick={() =>
                command("ready", draftNeedsReady ? { options } : undefined)
              }
            >
              {draftNeedsReady
                ? "保存并准备"
                : room.seats[room.you]?.ready
                  ? "取消准备"
                  : "我准备好了"}
              <Check size={18} />
            </button>
            {host && (
              <button
                className="primary gold"
                disabled={
                  busy ||
                  optionsDirty ||
                  room.seats.length < module.minPlayers ||
                  (!!module.requiresFullTable &&
                    room.seats.length !== room.capacity) ||
                  room.seats.some((s) => !s.ready)
                }
                onClick={() => command("start")}
              >
                开始游戏
                <ArrowRight size={18} />
              </button>
            )}
          </div>
        )}
        <p className="muted small">
          {room.spectating
            ? "等待玩家准备下一局。"
            : host
              ? optionsDirty
                ? "先保存地图与规则设置，才能开始游戏。"
                : module.supportsBots
                  ? "电脑自动准备。你准备好后即可开始，也可以继续邀请朋友。"
                  : room.capacity === 1
                    ? "准备好后即可开始。"
                    : "好友到齐并准备好后即可开始。"
              : "准备好后，等待房主开始游戏。"}
        </p>
      </div>
    </div>
  );
}
const seatColors = [
  "#b74134",
  "#2c75a7",
  "#c8981c",
  "#368161",
  "#8656a5",
  "#cb752c",
  "#4f8f7d",
  "#8a6f4d",
  "#6f7f9d",
  "#9a6a7d",
  "#4f8b83",
  "#8a783c",
];

export function ModulePlayers({ room }: { room: Room }) {
  const assets = useContext(AssetsContext);
  const Summary = gameModule(room.kind)?.PlayerSummary;
  return (
    <div className="players-strip module-players">
      {room.seats.map((seat, i) => (
        <div
          className={`player-panel ${(room.game?.attention ?? room.game?.actors)?.includes(i) ? "current" : ""}`}
          data-player-seat={i}
          key={seat.id}
        >
          <span
            className="avatar"
            style={{ background: seatColors[i % seatColors.length] }}
          >
            {seat.name[0]}
          </span>
          <div className="player-info">
            <strong>
              <PlayerName user={seat} />
              {room.you === i && <small>你</small>}
            </strong>
            <small>
              {room.game?.eliminated?.includes(i)
                ? "已出局"
                : seat.left
                  ? "已离桌"
                  : seat.autoPlay
                    ? "托管中"
                    : (room.game?.attention ?? room.game?.actors)?.includes(i)
                      ? "等待行动"
                      : "已入座"}
            </small>
          </div>
          {Summary && (
            <div className="player-extras">
              <Summary
                room={room}
                data={room.game?.data}
                seat={i}
                assets={assets}
              />
            </div>
          )}
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
  const hasClock =
    !!room.turnDeadline && !room.game?.finished && !room.game?.paused;
  return (
    <section
      className={`turn-card ${(room.game?.attention ?? room.game?.actors)?.includes(room.you) ? "mine" : ""}`}
    >
      <span className="eyebrow">TURN · 行动提示</span>
      <h3>
        {room.game?.finished
          ? "本局已结束"
          : room.game?.paused
            ? "对局已暂停"
            : (room.game?.attention ?? room.game?.actors)?.includes(room.you)
              ? "请行动"
              : room.game?.eliminated?.includes(room.you)
                ? "你已出局，可以观战或离桌"
                : (room.game?.attention ?? room.game?.actors)?.length
                  ? "等待其他玩家"
                  : "对局进行中"}
      </h3>
      <div
        className={`turn-clock ${hasClock ? (remaining <= 20 ? "urgent" : "") : "turn-clock-placeholder"}`}
        aria-hidden={hasClock ? undefined : true}
      >
        <div className="clock-reading">
          <Clock size={20} />
          <time aria-label={`本回合剩余 ${remaining} 秒`}>
            {String(Math.floor(remaining / 60)).padStart(2, "0")}:
            {String(remaining % 60).padStart(2, "0")}
          </time>
          <span>超时自动托管</span>
        </div>
      </div>
    </section>
  );
}

export function ModuleRules({ kind }: { kind: string }) {
  const module = gameModule(kind);
  const Rules = module?.Rules;
  return Rules ? <Rules /> : <p>{module?.description || gameName(kind)}</p>;
}

export function ModuleCover({
  kind,
  mini = false,
}: {
  kind: string;
  mini?: boolean;
}) {
  const module = gameModule(kind)!;
  const Cover = module.Cover;
  const assets = useContext(AssetsContext);
  // AssetsContext remains available to game-owned cover components.
  return (
    <AssetsContext.Provider value={assets}>
      <div className={`cover ${kind} ${mini ? "mini" : ""}`} aria-hidden="true">
        {Cover ? <Cover /> : <strong>{module.name}</strong>}
      </div>
    </AssetsContext.Provider>
  );
}
