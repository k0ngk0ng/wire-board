import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "../../web/node_modules/react-dom/client";
import { GameAudio } from "../../web/src/audio";
import type { ControllerProfile } from "../../web/src/controllers/input";
import { RuntimeBoard } from "../../games/contra/web/runtime-board";
import {
  loadRuntimeBridge,
  type RuntimeManifest,
  type RuntimeBridge,
  type RuntimeSnapshot,
} from "../../games/contra/web/runtime-bridge";
import {
  developmentTransport,
  type RuntimeClient,
} from "../../games/contra/web/runtime-client";
import {
  createContraAudioResources,
  type ContraAudioResources,
} from "../../games/contra/web/audio";
import "./browser_style.css";

type ProofWindow = Window &
  typeof globalThis & {
    contraBrowser?: {
      ready: boolean;
      client?: RuntimeClient;
      snapshot?: RuntimeSnapshot;
      error?: string;
      requests?: string[];
    };
  };
const proof = ((window as ProofWindow).contraBrowser = {
  ready: false,
} as NonNullable<ProofWindow["contraBrowser"]>);
function App() {
  const [audio] = useState(() => new GameAudio()),
    port = useMemo(() => audio.channel("contra-browser"), [audio]);
  const [profile, setProfile] = useState<ControllerProfile>(),
    [manifest, setManifest] = useState<RuntimeManifest>(),
    [bridge, setBridge] = useState<RuntimeBridge>(),
    [resources, setResources] = useState<ContraAudioResources>(),
    [initial, setInitial] = useState<RuntimeSnapshot>(),
    [error, setError] = useState(""),
    [progress, setProgress] = useState("准备运行层");
  const transport = useMemo(
    () => (initial ? developmentTransport(initial) : undefined),
    [initial],
  );
  useEffect(() => {
    audio.mount();
    const hidden = () => {
      if (document.hidden) audio.stop();
    };
    document.addEventListener("visibilitychange", hidden);
    void (async () => {
      const controller = await fetch("/api/controllers").then(async (r) => {
        if (!r.ok) throw Error("请先通过验收脚本登录平台账户");
        return r.json();
      });
      setProfile(controller.profiles["classic-2"]);
      const manifest = await fetch("/manifest.json").then((r) => r.json());
      setManifest(manifest);
      const code = await loadRuntimeBridge(manifest, (_, label) =>
        setProgress(label),
      );
      setBridge(code);
      const sound = await fetch(manifest.audio).then((r) => r.json());
      const resolved = createContraAudioResources(
        sound.source,
        sound.loops,
        (path, sha) => {
          const asset = sound.paths[path];
          return asset?.sha256 === sha ? [asset.url] : undefined;
        },
      );
      setResources(resolved);
      proof.ready = true;
      setProgress("就绪");
    })().catch((e) => {
      setError(String(e));
      proof.error = String(e);
    });
    return () => {
      audio.dispose();
      document.removeEventListener("visibilitychange", hidden);
    };
  }, [audio]);
  const start = async () => {
    try {
      await audio.unlock();
      const response = await fetch("/runtime/new", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ players: 1 }),
      });
      const value = await response.json();
      if (!response.ok) throw Error(value.error);
      setInitial(value);
    } catch (e) {
      setError(String(e));
      proof.error = String(e);
    }
  };
  return (
    <main className="browser-proof">
      <header>
        <h1>魂斗罗</h1>
        <span>开发验收</span>
      </header>
      {!initial && (
        <button id="begin" disabled={!proof.ready || !!error} onClick={start}>
          开始
        </button>
      )}
      {!initial && <p role="status">{progress}</p>}
      {error && <p role="alert">{error}</p>}
      {initial && profile && bridge && manifest && resources && transport && (
        <RuntimeBoard
          initial={initial}
          profile={profile}
          bridge={bridge}
          manifest={manifest}
          audioResources={resources}
          audio={port}
          transport={transport}
          onClient={(client) => {
            proof.client = client;
          }}
          onSnapshot={(snapshot) => {
            proof.snapshot = snapshot;
          }}
          onLoadError={(message) => {
            proof.error = message;
          }}
        />
      )}
      <input
        aria-label="输入隔离验收"
        className="browser-proof-input"
        placeholder="输入隔离验收"
      />
    </main>
  );
}
createRoot(document.getElementById("root")!).render(<App />);
