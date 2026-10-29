import { useEffect, useRef, useState } from "react";
import { FCController } from "../../../web/src/controllers/FCController";
import type { ControllerProfile } from "../../../web/src/controllers/input";
import type { BoardProps } from "../../../web/src/game-api/module";
import type { ContraAudioResources } from "./audio";
import { RuntimeClient, type RuntimeTransport } from "./runtime-client";
import type {
  RuntimeBridge,
  RuntimeManifest,
  RuntimeSnapshot,
} from "./runtime-bridge";
import "./runtime.css";

type Props = Pick<
  BoardProps,
  "audio" | "onReady" | "onLoadProgress" | "onLoadError"
> & {
  profile: ControllerProfile;
  manifest: RuntimeManifest;
  bridge: RuntimeBridge;
  transport: RuntimeTransport;
  initial: RuntimeSnapshot;
  audioResources: ContraAudioResources;
  onClient?: (client: RuntimeClient) => void;
  onSnapshot?: (snapshot: RuntimeSnapshot) => void;
  showController?: boolean;
  controllerDisabled?: boolean;
};

/** Development-ready game surface, not a lobby/room framework. The final Board
 * supplies its platform profile, audio port, loader callbacks and transport. */
export function RuntimeBoard(props: Props) {
  const root = useRef<HTMLDivElement>(null),
    canvas = useRef<HTMLCanvasElement>(null),
    client = useRef<RuntimeClient>(undefined);
  const [ready, setReady] = useState(false),
    [progress, setProgress] = useState(0),
    [label, setLabel] = useState("加载画面"),
    [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    const current = new RuntimeClient({
      root: root.current!,
      canvas: canvas.current!,
      profile: props.profile,
      audio: props.audio,
      audioResources: props.audioResources,
      manifest: props.manifest,
      bridge: props.bridge,
      transport: props.transport,
      initial: props.initial,
      onProgress: (p, label) => {
        if (active) {
          setProgress(p);
          setLabel(label);
          props.onLoadProgress?.(p, label);
        }
      },
      onSnapshot: (value) => {
        if (active) {
          root.current!.dataset.frame = value.frame;
          root.current!.dataset.phase = value.phase;
          root.current!.dataset.revision = value.revision;
          root.current!.dataset.reviewFrame = value.review?.frame ?? "";
          root.current!.dataset.reviewRevision = value.review?.revision ?? "";
          root.current!.dataset.digest = value.digest;
          root.current!.dataset.audioTick = value.audioTick;
          root.current!.dataset.paused = String(value.paused);
          root.current!.dataset.lives = JSON.stringify(
            value.players.map((player) => player.lives),
          );
          root.current!.dataset.checked = String(current.evidence.checked);
          root.current!.dataset.resyncs = String(current.evidence.resyncs);
          props.onSnapshot?.(value);
        }
      },
      onVisual: (value, predicted) => {
        if (!active) return;
        root.current!.dataset.visualFrame = value.frame;
        root.current!.dataset.visualRevision = value.revision;
        root.current!.dataset.predicted = String(predicted);
        // Drawing diagnostics are separate from verified public state/digest.
        root.current!.dataset.visualPlayers = JSON.stringify(value.players);
      },
      onError: (e) => {
        if (active) {
          setError(e.message);
          props.onLoadError?.(e.message);
        }
      },
    });
    client.current = current;
    props.onClient?.(current);
    void current
      .initialize()
      .then(() => {
        if (active) {
          setReady(true);
          props.onReady?.();
        }
      })
      .catch((e) => {
        if (active) {
          setError(String(e));
          props.onLoadError?.(String(e));
        }
      });
    return () => {
      active = false;
      current.dispose();
      client.current = undefined;
    };
  }, [
    props.initial.epoch,
    props.bridge,
    props.transport,
    props.audio,
    props.audioResources,
    props.manifest,
  ]);
  useEffect(() => {
    client.current?.input.configure(props.profile);
  }, [props.profile]);
  return (
    <div
      className="contra-runtime"
      ref={root}
      tabIndex={-1}
      data-contra-runtime
    >
      <div className="contra-runtime-screen">
        <canvas
          ref={canvas}
          width={256}
          height={240}
          aria-label="魂斗罗游戏画面"
        />
        {!ready && !error && (
          <div className="contra-runtime-loading" role="status">
            {label}
            <progress max={100} value={progress} />
          </div>
        )}
      </div>
      {error && <p role="alert">{error}</p>}
      {props.showController !== false && (
        <div className="contra-runtime-controller">
          <FCController
            profile={props.profile}
            mode="play"
            disabled={!ready || !!error || props.controllerDisabled}
            sink={client.current?.input}
          />
        </div>
      )}
    </div>
  );
}
