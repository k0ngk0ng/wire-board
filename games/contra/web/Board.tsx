import { useEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import type {
  ControllerProfile,
  ControllerSettingsData,
} from "../../../web/src/controllers/input";
import { api } from "../../../web/src/ui/board-host";
import { createContraAudioResources, type ContraAudioResources } from "./audio";
import { RuntimeBoard } from "./runtime-board";
import { RuntimeClient } from "./runtime-client";
import { runtimeManifestURL } from "./runtime-url";
import {
  publishRuntimeVisible,
  runtimeVisibleKey,
  useRuntimeVisible,
} from "./runtime-visible";
import { createRoomTransport, type RoomRuntimeView } from "./runtime-room";
import {
  installRuntimeStage,
  fetchRuntimeManifest,
  fetchRuntimeAsset,
  loadRuntimeBridge,
  resolveRuntimeURL,
  type RuntimeBridge,
  type RuntimeManifest,
  type RuntimeSnapshot,
} from "./runtime-bridge";

type Loaded = {
  identity: string;
  manifest: RuntimeManifest;
  bridge: RuntimeBridge;
  transport: ReturnType<typeof createRoomTransport>;
  resources: ContraAudioResources;
  initial: RuntimeSnapshot;
};

export function Board(props: BoardProps<RoomRuntimeView>) {
  const latest = useRef(props);
  latest.current = props;
  const client = useRef<RuntimeClient | undefined>(undefined);
  const [loaded, setLoaded] = useState<Loaded>(),
    [profile, setProfile] = useState<ControllerProfile>();
  const [error, setError] = useState(""),
    [retry, setRetry] = useState(0),
    [progress, setProgress] = useState({ percent: 0, label: "加载控制器设置" });
  const { room, data } = props;
  const watching = !!room.spectating || room.you < 0;
  const userID = watching ? "spectator" : (room.seats[room.you]?.id ?? "");
  const identity = `${room.id}:${data.epoch}:${room.you}:${data.actor}:${userID}:${data.rulesBuildID}:${data.runtimeBuildID}`;
  const visibleKey = runtimeVisibleKey(room.id, data.epoch);
  const snapshot = useRuntimeVisible(visibleKey, data.snapshot);

  useEffect(() => {
    let active = true,
      transport: Loaded["transport"] | undefined,
      changedProfile: ControllerProfile | undefined;
    client.current = undefined;
    setLoaded(undefined);
    setProfile(undefined);
    setError("");
    const report = (percent: number, label: string) => {
      if (active) {
        setProgress({ percent, label });
        latest.current.onLoadProgress?.(percent, label);
      }
    };
    const changed = (event: Event) => {
      const change = (event as CustomEvent).detail;
      if (!active || change?.userId !== userID || change?.model !== "classic-2")
        return;
      if (changedProfile && changedProfile.revision > change.profile.revision)
        return;
      changedProfile = change.profile;
      setProfile(change.profile);
    };
    window.addEventListener("controller-settings-changed", changed);
    void (async () => {
      report(0, "加载控制器设置");
      const settings: ControllerSettingsData = await api("/controllers");
      if (!active) return;
      const fetched = settings.profiles["classic-2"];
      const saved =
        changedProfile &&
        (!fetched || changedProfile.revision >= fetched.revision)
          ? changedProfile
          : fetched;
      if (!saved) throw Error("未找到当前用户的控制器设置");
      setProfile(saved);
      const url = runtimeManifestURL(data.runtimeBuildID, location.href);
      const manifest = await fetchRuntimeManifest(url, data);
      if (
        manifest.rulesBuildID !== data.rulesBuildID ||
        manifest.bundle?.id !== data.bundle.id ||
        !manifest.stagePages
      )
        throw Error("游戏程序或关卡资源版本不匹配");
      const level = data.stage.level;
      const bridge = await loadRuntimeBridge(manifest, report, level);
      if (!active) return;
      if (!manifest.audioCatalog) throw Error("缺少已绑定的音效清单");
      const sound = JSON.parse(
        new TextDecoder().decode(
          await fetchRuntimeAsset(manifest.audioCatalog),
        ),
      );
      const resources = createContraAudioResources(
        sound.source,
        sound.loops,
        (path, sha) => {
          const asset = sound.paths[path];
          return asset?.sha256 === sha
            ? [resolveRuntimeURL(asset.url, manifest.audio)]
            : undefined;
        },
      );
      const stages = new Map<number, Promise<void>>([
        [level, Promise.resolve()],
      ]);
      const prepare = async (view: RoomRuntimeView) => {
        if (!active) throw Error("对局已切换");
        if (
          view.rulesBuildID !== manifest.rulesBuildID ||
          view.bundle.id !== manifest.bundle!.id
        )
          throw Error("对局资源版本已改变");
        const next = view.stage.level;
        let pending = stages.get(next);
        if (!pending) {
          const asset = manifest.stages?.[String(next)];
          if (!asset) throw Error("缺少当前关卡资源");
          pending = installRuntimeStage(
            bridge,
            manifest.bundle!,
            next,
            asset,
          ).catch((e) => {
            stages.delete(next);
            throw e;
          });
          stages.set(next, pending);
        }
        await pending;
        await client.current?.prepareStage(next);
        await client.current?.preparePresentation(view.snapshot.presentation);
      };
      transport = createRoomTransport({
        runtimeBuildID: data.runtimeBuildID,
        roomID: room.id,
        epoch: data.epoch,
        actor: data.actor,
        current: () => ({
          room: latest.current.room,
          view: latest.current.data,
        }),
        act: (action) => latest.current.act(action),
        sync: async () => {
          const result = await api(
            `/rooms/${encodeURIComponent(room.id)}/sync`,
          );
          if (
            result.roomId !== room.id ||
            !result.data?.view ||
            !result.data.checkpoint
          )
            throw Error("恢复快照格式无效");
          return { version: result.version, ...result.data };
        },
        prepare,
      });
      if (!active) {
        transport.dispose();
        return;
      }
      setLoaded({
        identity,
        manifest,
        bridge,
        transport,
        resources,
        initial: data.snapshot,
      });
    })().catch((e) => {
      if (active) {
        const text = e instanceof Error ? e.message : String(e);
        setError(text);
        latest.current.onLoadError?.(text);
      }
    });
    return () => {
      active = false;
      window.removeEventListener("controller-settings-changed", changed);
      publishRuntimeVisible(visibleKey);
      // RuntimeBoard owns the mounted client's lifetime and drains its final
      // release before closing transport. A late loader closes its own result.
    };
  }, [identity, retry]);

  const disabled =
    watching ||
    room.status !== "playing" ||
    !!room.loading ||
    !!data.stageLoading ||
    !!room.seats[room.you]?.autoPlay ||
    snapshot.finalized;
  return (
    <div className="contra-board">
      {loaded?.identity === identity && profile ? (
        <>
          <div className="contra-game-resources" aria-label="队伍状态">
            {room.seats.map((seat, index) => (
              <span
                key={seat.id}
                aria-current={room.you === index ? "true" : undefined}
              >
                <strong>{seat.name}</strong>
                <span>
                  备用生命{" "}
                  {snapshot.players[data.seatActors[index]]?.lives ?? 0}
                </span>
              </span>
            ))}
          </div>
          <RuntimeBoard
            initial={loaded.initial}
            profile={profile}
            manifest={loaded.manifest}
            bridge={loaded.bridge}
            transport={loaded.transport}
            audioResources={loaded.resources}
            audio={props.audio}
            showController={!watching}
            controllerDisabled={disabled}
            onClient={(value) => {
              client.current = value;
            }}
            onSnapshot={(value) => {
              publishRuntimeVisible(visibleKey, value);
            }}
            onReady={() => latest.current.onReady?.()}
            onLoadProgress={(percent, label) =>
              latest.current.onLoadProgress?.(percent, label)
            }
            onLoadError={(text) => {
              setError(text);
              latest.current.onLoadError?.(text);
            }}
          />
          {(data.stageLoading || snapshot.paused) && (
            <p className="contra-state" role="status">
              {data.stageLoading
                ? "正在加载下一关，等待队友就绪"
                : "已暂停 · Start 继续"}
            </p>
          )}
        </>
      ) : (
        !error && (
          <div className="contra-board-loading" role="status">
            <p>{progress.label}</p>
            <progress max={100} value={progress.percent} />
          </div>
        )
      )}
      {error && (
        <div className="contra-board-error" role="alert">
          <p>{error}</p>
          <button
            onClick={() => {
              client.current = undefined;
              setRetry((value) => value + 1);
            }}
          >
            重新连接
          </button>
        </div>
      )}
    </div>
  );
}
