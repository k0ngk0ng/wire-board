import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { api } from "../../../web/src/ui/board-host";
import { FCController } from "../../../web/src/controllers/FCController";
import type {
  ControllerProfile,
  ControllerSettingsData,
} from "../../../web/src/controllers/input";
import { CampaignPanel } from "./CampaignPanel";
import { resolveBasketballAssetsBase } from "./asset-base";
import { BasketballNetworkClient } from "./network-client";
import type { BasketballNetworkView } from "./network-scene";
import type { LoadingProgress } from "./loading";
import "./network.css";

export function Board(props: BoardProps<BasketballNetworkView>) {
  const root = useRef<HTMLDivElement>(null),
    canvas = useRef<HTMLCanvasElement>(null);
  const latest = useRef(props);
  latest.current = props;
  const client = useRef<BasketballNetworkClient | undefined>(undefined);
  const [profile, setProfile] = useState<ControllerProfile>(),
    [readyIdentity, setReadyIdentity] = useState(""),
    [error, setError] = useState(""),
    [connected, setConnected] = useState(true),
    [retry, setRetry] = useState(0);
  const [progress, setProgress] = useState<LoadingProgress>({
    percent: 0,
    label: "加载控制器设置",
  });
  const [campaignOpen, setCampaignOpen] = useState(false);
  const campaignOpenRef = useRef(campaignOpen);
  campaignOpenRef.current = campaignOpen;
  const { room, data } = props;
  const watching = !!room.spectating || room.you < 0;
  const userID = watching ? "spectator" : (room.seats[room.you]?.id ?? "");
  const localActor = data.localActor ?? room.you;
  const identity = `${room.id}:${data.epoch}:${room.you}:${localActor}:${userID}`;
  const ready = readyIdentity === identity;
  useEffect(() => {
    if (ready) props.onReady?.();
  }, [ready, props.onReady]);
  useEffect(() => {
    props.onLoadProgress?.(progress.percent, progress.label);
  }, [progress, props.onLoadProgress]);
  useEffect(() => {
    if (error) props.onLoadError?.(error);
  }, [error, props.onLoadError]);
  const enabled = (value: BoardProps<BasketballNetworkView>) =>
    !campaignOpenRef.current &&
    value.room.status === "playing" &&
    !value.room.spectating &&
    value.room.you >= 0 &&
    !value.room.seats[value.room.you]?.autoPlay &&
    !value.room.seats[value.room.you]?.left &&
    !value.busy &&
    value.data.canInput;
  useLayoutEffect(() => {
    let alive = true,
      owned: BasketballNetworkClient | undefined;
    let changedProfile: ControllerProfile | undefined;
    setReadyIdentity("");
    setError("");
    setProgress({ percent: 0, label: "加载控制器设置" });
    setProfile(undefined);
    const model = "classic-2";
    const changed = (event: Event) => {
      const change = (event as CustomEvent).detail;
      if (change?.userId !== userID || change?.model !== model) return;
      changedProfile = change.profile;
      setProfile(change.profile);
      owned?.configure(change.profile);
    };
    window.addEventListener("controller-settings-changed", changed);
    void (async () => {
      const settings: ControllerSettingsData = await api("/controllers");
      if (!alive) return;
      const fetched = settings.profiles[model];
      const saved =
        changedProfile &&
        (!fetched || changedProfile.revision >= fetched.revision)
          ? changedProfile
          : fetched;
      if (!saved) throw Error("未找到当前用户的控制器设置");
      setProfile(saved);
      const base = resolveBasketballAssetsBase(props.assets);
      const created = await BasketballNetworkClient.create({
        root: root.current!,
        canvas: canvas.current!,
        profile: saved,
        roomID: room.id,
        userID,
        seat: watching ? undefined : room.you,
        controller: watching ? undefined : localActor,
        runtimeURL: "/basketball/v1/runtime",
        assetsURL: base + "/basketball/v1/graphics",
        postResult: !!data.campaign,
        court: (data.runtime.config as { match: { Court: number } }).match
          .Court,
        onProgress: (p) => {
          if (alive) setProgress(p);
        },
        audio: props.audio,
        audioOptions: { assetsBase: base },
        inputWindow: () => latest.current.inputWindow ?? 1,
        send: (action) => {
          const current = latest.current;
          const currentUser =
            current.room.you < 0
              ? "spectator"
              : (current.room.seats[current.room.you]?.id ?? "");
          if (
            !alive ||
            `${current.room.id}:${current.data.epoch}:${current.room.you}:${current.data.localActor ?? current.room.you}:${currentUser}` !==
              identity
          )
            return Promise.reject(Error("控制席位已改变"));
          return current.act({ ...action });
        },
        onError: (e) => {
          if (alive) {
            console.error("Basketball network scene", e);
            setError("对局画面暂时无法同步，请重新连接。");
          }
        },
        onConnection: (value) => {
          if (alive) setConnected(value);
        },
      });
      if (!alive) {
        created.dispose();
        return;
      }
      owned = created;
      client.current = created;
      if (changedProfile && changedProfile.revision > saved.revision)
        created.configure(changedProfile);
      const current = latest.current;
      created.update(
        current.data,
        current.room.version,
        enabled(current),
        current.clock,
      );
      setReadyIdentity(identity);
      setProgress({ percent: 100, label: "本机加载完成" });
    })().catch((e) => {
      if (alive) setError(e instanceof Error ? e.message : "无法加载比赛画面");
    });
    return () => {
      owned?.dispose();
      alive = false;
      window.removeEventListener("controller-settings-changed", changed);
      if (client.current === owned) client.current = undefined;
    };
  }, [identity, props.assets, props.audio, retry]);
  useEffect(() => {
    try {
      client.current?.update(data, room.version, enabled(props), props.clock);
    } catch (e) {
      console.error("Basketball snapshot", e);
      setError("对局画面暂时无法同步，请重新连接。");
    }
  }, [data, room.version, props.busy, campaignOpen]);
  const needsAck =
    !!data.loading &&
    !watching &&
    !room.seats[room.you]?.autoPlay &&
    !data.loading.ready[localActor];
  useEffect(() => {
    if (!ready || !needsAck) return;
    let alive = true;
    let timer: ReturnType<typeof setTimeout>;
    const confirm = async () => {
      const current = latest.current;
      if (
        !alive ||
        current.data.phase !== "loading" ||
        current.data.loading?.ready[localActor]
      )
        return;
      if (current.busy) {
        timer = setTimeout(confirm, 300);
        return;
      }
      try {
        await current.act({ type: "loaded", epoch: current.data.epoch });
      } catch {
        if (alive) setError("加载已完成，但开赛确认失败，请重新连接。");
      }
      if (alive) timer = setTimeout(confirm, 1500);
    };
    void confirm();
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [ready, needsAck, identity]);
  const waitingNames = (data.seatActors ?? []).flatMap((actor, seat) =>
    data.loading && !data.loading.ready[actor] && !data.inputs[actor].automated
      ? [room.seats[seat]?.name ?? "玩家"]
      : [],
  );
  const phase =
    data.phase === "loading"
      ? "等待玩家加载完成"
      : data.phase === "ending-confirm"
        ? "结局播放完毕 · 第一席按 START 确认"
        : data.phase === "ending"
          ? "正在播放通关结局"
          : data.phase === "reward-video"
            ? "正在展示比赛奖励"
            : data.phase.startsWith("campaign-")
              ? data.campaign?.stage === "prepare"
                ? "下一场准备"
                : data.campaign?.stage === "complete"
                  ? "战役已通关"
                  : data.campaign?.stage === "quit"
                    ? "战役已结束"
                    : "本场结束 · 战役确认"
              : data.phase === "paused"
                ? "比赛暂停 · 第一席按 START 继续"
                : data.phase === "intermission"
                  ? "中场休息 · 第一席按 START 开始下半场"
                  : data.phase === "result"
                    ? "比赛结束"
                    : watching
                      ? "正在观战"
                      : room.seats[room.you]?.autoPlay
                        ? "电脑正在托管"
                        : "比赛进行中";
  return (
    <div
      ref={root}
      className="basketball-board"
      tabIndex={0}
      data-basketball-ready={ready}
      aria-label="热血篮球比赛"
    >
      <div className="basketball-screen">
        <p className="basketball-status" role="status">
          {!ready && !error
            ? "正在加载球场…"
            : !connected
              ? "连接中断，正在等待同步…"
              : phase}
        </p>
        <CampaignPanel
          data={data}
          assets={props.assets}
          busy={props.busy}
          act={props.act}
          clock={props.clock}
          onOpenChange={setCampaignOpen}
        />
        {error && (
          <div className="basketball-error" role="alert">
            {error}
            <button onClick={() => setRetry((n) => n + 1)}>重新连接</button>
          </div>
        )}
        <div
          className="basketball-stage"
          onPointerDown={() => root.current?.focus({ preventScroll: true })}
        >
          {(!ready || progress.percent < 100 || data.phase === "loading") && (
            <div className="basketball-loading" aria-live="polite">
              <strong>
                {error
                  ? "加载遇到问题"
                  : progress.percent < 100
                    ? progress.label
                    : "本机加载完成"}
              </strong>
              <progress
                aria-label="篮球加载进度"
                max={100}
                value={Math.floor(progress.percent)}
              />
              <span>{Math.floor(progress.percent)}%</span>
              {ready && data.phase === "loading" && (
                <span>
                  {waitingNames.length
                    ? `等待：${waitingNames.join("、")}`
                    : "正在确认开赛…"}
                </span>
              )}
            </div>
          )}
          <canvas
            ref={canvas}
            width={256}
            height={240}
            aria-label="热血篮球球场"
          />
        </div>
      </div>
      {!watching && profile && (
        <aside className="basketball-controls" aria-label="游戏控制器">
          <FCController
            profile={profile}
            mode="play"
            disabled={!ready || !enabled(props) || !!error}
            sink={client.current?.controllerSink}
          />
        </aside>
      )}
    </div>
  );
}
