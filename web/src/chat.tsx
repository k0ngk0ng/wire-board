import { useEffect, useRef, useState } from "react";
import { MessageCircle, Send, X } from "lucide-react";
import type { Room } from "./types";

export function Chat({
  room,
  userId,
  send,
}: {
  room: Room;
  userId: string;
  send: (text: string, nonce: string) => Promise<void>;
}) {
  const messages = room.chat || [];
  const latest = messages.at(-1)?.id;
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);
  const [seen, setSeen] = useState(() => new Set(messages.map((m) => m.id)));
  const list = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLTextAreaElement>(null);
  const toggle = useRef<HTMLButtonElement>(null);
  const nearBottom = useRef(true);
  const pending = useRef(false);
  const retry = useRef<{ text: string; nonce: string } | undefined>(undefined);
  const unread = messages.filter(
    (m) => m.sender.id !== userId && !seen.has(m.id),
  ).length;
  const length = Array.from(draft.trim()).length;
  useEffect(() => {
    if (!open) return;
    setSeen(new Set(messages.map((m) => m.id)));
    if (nearBottom.current && list.current)
      list.current.scrollTop = list.current.scrollHeight;
  }, [open, latest]);
  useEffect(() => {
    if (open) input.current?.focus();
  }, [open]);
  const close = () => {
    setOpen(false);
    toggle.current?.focus();
  };
  const submit = async () => {
    const text = draft.trim();
    if (pending.current || !text || Array.from(text).length > 500) return;
    pending.current = true;
    setSending(true);
    setError("");
    if (retry.current?.text !== text)
      retry.current = { text, nonce: crypto.randomUUID() };
    try {
      await send(text, retry.current.nonce);
      retry.current = undefined;
      setDraft((current) => (current.trim() === text ? "" : current));
      nearBottom.current = true;
      if (list.current) list.current.scrollTop = list.current.scrollHeight;
    } catch (e) {
      setError((e as Error).message || "发送失败，请重试");
    } finally {
      pending.current = false;
      setSending(false);
    }
  };
  return (
    <div className="table-chat">
      {open && (
        <section
          className="chat-panel"
          id="table-chat-panel"
          role="dialog"
          aria-label="牌桌聊天"
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              e.stopPropagation();
              close();
            }
          }}
        >
          <header className="chat-heading">
            <div>
              <h2>牌桌聊天</h2>
              <small>本桌玩家与观战者可见 · 最近 100 条</small>
            </div>
            <button
              className="icon-button"
              onClick={close}
              aria-label="收起聊天"
            >
              <X size={20} />
            </button>
          </header>
          <div
            className="chat-messages"
            ref={list}
            role="log"
            aria-label="聊天记录"
            aria-live="polite"
            aria-relevant="additions"
            tabIndex={0}
            onScroll={() => {
              const el = list.current;
              if (el)
                nearBottom.current =
                  el.scrollHeight - el.scrollTop - el.clientHeight < 48;
            }}
          >
            {!messages.length && (
              <p className="chat-empty">和同桌的朋友打个招呼吧。</p>
            )}
            {messages.map((m) => (
              <article
                className={`chat-message ${m.sender.id === userId ? "mine" : ""}`}
                key={m.id}
              >
                <div>
                  <strong>
                    {m.sender.name}
                    {m.spectator && (
                      <small className="spectator-badge">观战</small>
                    )}
                  </strong>
                  <time dateTime={new Date(m.sentAt).toISOString()}>
                    {new Date(m.sentAt).toLocaleTimeString([], {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </time>
                </div>
                <p>{m.text}</p>
              </article>
            ))}
          </div>
          <form
            className="chat-compose"
            onSubmit={(e) => {
              e.preventDefault();
              void submit();
            }}
          >
            {error && (
              <p role="alert" className="chat-error">
                {error}
              </p>
            )}
            <textarea
              ref={input}
              aria-label="聊天消息"
              placeholder="说点什么…"
              value={draft}
              rows={2}
              maxLength={1000}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (
                  e.key === "Enter" &&
                  !e.shiftKey &&
                  !e.nativeEvent.isComposing &&
                  e.keyCode !== 229
                ) {
                  e.preventDefault();
                  void submit();
                }
              }}
            />
            <div>
              <small>{length}/500 · Shift+Enter 换行</small>
              <button
                className="primary"
                disabled={sending || length === 0 || length > 500}
              >
                <Send size={15} />
                {sending ? "发送中…" : "发送"}
              </button>
            </div>
          </form>
        </section>
      )}
      <button
        ref={toggle}
        className="chat-toggle"
        aria-label={
          open
            ? "收起牌桌聊天"
            : `打开牌桌聊天${unread ? `，${unread} 条未读` : ""}`
        }
        aria-expanded={open}
        aria-controls="table-chat-panel"
        onClick={() => {
          if (open) close();
          else {
            nearBottom.current = true;
            setOpen(true);
          }
        }}
      >
        <MessageCircle size={22} />
        <span>聊天</span>
        {!open && unread > 0 && <b>{unread > 99 ? "99+" : unread}</b>}
      </button>
    </div>
  );
}
