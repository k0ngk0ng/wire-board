import { createRoot } from "react-dom/client";
import { useEffect, useRef, useState } from "react";
import { Chat } from "../../web/src/chat";
import { ProfileContext } from "../../web/src/profiles";
import { Modal } from "../../web/src/ui/board-host";
import { GameFeedback } from "../../web/src/ui/game-overlay-root";
import {
  GameFullscreen,
  type GameFullscreenHandle,
} from "../../web/src/ui/room-fullscreen";
import type { Room } from "../../web/src/types";
import "../../web/src/style.css";

const self = { id: "self", name: "本机玩家" };
const peer = { id: "peer", name: "同桌玩家" };
const fixture = {
  sent: [] as { text: string; nonce: string }[],
  gameKeys: [] as string[],
  releaseKeys: [] as string[],
  incoming: (_text: string) => {},
  finishSend: () => {},
  leave: () => {},
};
Object.assign(window, { chatFixture: fixture });

function Harness() {
  const [present, setPresent] = useState(true);
  const [playing, setPlaying] = useState(false);
  const [chat, setChat] = useState<NonNullable<Room["chat"]>>([]);
  const [chatTarget, setChatTarget] = useState<HTMLElement | null>(null);
  const [profileID, setProfileID] = useState("");
  const [ready, setReady] = useState(false);
  const fullscreen = useRef<GameFullscreenHandle>(null);
  const room = { id: "chat-harness", chat } as Room;
  useEffect(() => {
    const keydown = (e: KeyboardEvent) => fixture.gameKeys.push(e.key);
    const keyup = (e: KeyboardEvent) => fixture.releaseKeys.push(e.key);
    window.addEventListener("keydown", keydown);
    window.addEventListener("keyup", keyup);
    return () => {
      window.removeEventListener("keydown", keydown);
      window.removeEventListener("keyup", keyup);
    };
  }, []);
  fixture.incoming = (text) =>
    setChat((messages) => [
      ...messages,
      {
        id: crypto.randomUUID(),
        sender: peer,
        text,
        sentAt: Date.now(),
      },
    ]);
  fixture.leave = () => setPresent(false);
  return (
    <ProfileContext.Provider value={setProfileID}>
      <header className="topbar">平台菜单</header>
      <main className="room-page">
        <h1 className="room-heading">聊天生命周期验收</h1>
        <div className="players-strip">公共玩家栏</div>
        <div className="room-tools">
          <button data-start onClick={() => setPlaying(true)}>
            开始
          </button>
          {playing && present && (
            <button
              data-enter
              disabled={!ready}
              onClick={() => fullscreen.current?.enter()}
            >
              全屏
            </button>
          )}
        </div>
        {playing && present && (
          <GameFullscreen
            ref={fullscreen}
            scope="chat-harness"
            enabled
            onReadyChange={setReady}
            onChatTargetChange={setChatTarget}
          >
            <div style={{ minHeight: 280, background: "#e1e7d9" }}>
              <button data-game-control>游戏操作</button>
            </div>
          </GameFullscreen>
        )}
      </main>
      {present && (
        <Chat
          room={room}
          userId={self.id}
          fullscreenTarget={chatTarget}
          send={(text, nonce) => {
            fixture.sent.push({ text, nonce });
            return new Promise<void>((resolve) => {
              fixture.finishSend = () => {
                setChat((messages) => [
                  ...messages,
                  {
                    id: nonce,
                    sender: self,
                    text,
                    sentAt: Date.now(),
                  },
                ]);
                resolve();
              };
            });
          }}
        />
      )}
      {profileID && (
        <GameFeedback>
          <Modal title="玩家页面" onClose={() => setProfileID("")}>
            <p>{profileID === peer.id ? peer.name : self.name}</p>
            <button data-profile-action>玩家资料操作</button>
          </Modal>
        </GameFeedback>
      )}
    </ProfileContext.Provider>
  );
}

createRoot(document.getElementById("root")!).render(<Harness />);
