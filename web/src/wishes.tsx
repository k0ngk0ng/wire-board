import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  CheckCircle2,
  Clock3,
  ImagePlus,
  Send,
  Sparkles,
  X,
  XCircle,
} from "lucide-react";

export type Wish = {
  id: string;
  userName: string;
  text: string;
  imageUrl: string;
  status: "pending" | "approved" | "rejected";
  note: string;
  createdAt: number;
  reviewedAt?: number;
  approvedAt?: number;
  reviewedByName?: string;
};

const statuses: Record<
  Wish["status"],
  { label: string; icon: typeof Clock3; className: string }
> = {
  pending: { label: "待审核", icon: Clock3, className: "pending" },
  approved: { label: "已通过", icon: CheckCircle2, className: "approved" },
  rejected: { label: "已驳回", icon: XCircle, className: "rejected" },
};

const imageTypes = ["image/png", "image/jpeg", "image/webp", "image/gif"];
const imageLimit = 5 * 1024 * 1024;

function date(value: number) {
  return value
    ? new Date(value * 1000).toLocaleString("zh-CN", {
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
      })
    : "";
}

export function WishStatus({ wish }: { wish: Wish }) {
  const status = statuses[wish.status];
  const Icon = status.icon;
  return (
    <span className={`wish-status ${status.className}`}>
      <Icon size={14} />
      {status.label}
    </span>
  );
}

export function WishDialog({
  onClose,
  onChanged,
}: {
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const [wishes, setWishes] = useState<Wish[]>();
  const [text, setText] = useState("");
  const [file, setFile] = useState<File>();
  const [preview, setPreview] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [loadError, setLoadError] = useState("");
  const dialog = useRef<HTMLDivElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  async function load() {
    try {
      const response = await fetch("/api/wishes");
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || "无法读取许愿");
      setWishes(data.wishes || []);
      setLoadError("");
    } catch (e) {
      setLoadError((e as Error).message);
    }
  }
  useEffect(() => {
    void load();
    const previous = document.activeElement as HTMLElement;
    dialog.current?.querySelector<HTMLElement>("textarea")?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busy) onClose();
      if (event.key !== "Tab") return;
      const nodes = Array.from(
        dialog.current?.querySelectorAll<HTMLElement>(
          "button:not(:disabled),input:not(:disabled),textarea:not(:disabled),a[href]",
        ) || [],
      );
      const first = nodes[0];
      const last = nodes[nodes.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    };
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      previous?.focus();
    };
  }, [busy, onClose]);
  useEffect(() => {
    if (!file) {
      setPreview("");
      return;
    }
    // Data URLs stay inside the existing img-src policy; blob: previews would
    // be blocked by the production CSP.
    const reader = new FileReader();
    reader.onload = () => setPreview(String(reader.result || ""));
    reader.readAsDataURL(file);
    return () => {
      reader.onload = null;
    };
  }, [file]);

  function pick(next?: File) {
    setError("");
    if (!next) {
      setFile(undefined);
      return;
    }
    if (!imageTypes.includes(next.type)) {
      setError("图片仅支持 PNG、JPEG、WebP 或 GIF");
      return;
    }
    if (next.size > imageLimit) {
      setError("图片不能超过 5 MB");
      return;
    }
    setFile(next);
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    const content = text.trim();
    if (content.length < 2) {
      setError("至少写下两个字的许愿内容");
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const body = new FormData();
      body.append("text", content);
      if (file) body.append("image", file);
      const response = await fetch("/api/wishes", {
        method: "POST",
        body,
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || "许愿提交失败");
      setText("");
      setFile(undefined);
      if (fileInput.current) fileInput.current.value = "";
      setNotice("许愿已提交，管理员审核通过后会进入制作队列。");
      await load();
      await onChanged();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      className="modal-backdrop wish-backdrop"
      onClick={(event) => {
        if (event.target === event.currentTarget && !busy) onClose();
      }}
    >
      <div
        className="modal wish-modal"
        role="dialog"
        aria-modal="true"
        aria-label="游戏许愿池"
        ref={dialog}
      >
        <div className="modal-head">
          <div>
            <h2>
              <Sparkles size={22} />
              游戏许愿池
            </h2>
            <p>写下想玩的游戏，也可以附上一张参考图。</p>
          </div>
          <button
            className="icon-button"
            type="button"
            aria-label="关闭许愿池"
            disabled={busy}
            onClick={onClose}
          >
            <X size={20} />
          </button>
        </div>
        <form className="wish-form" onSubmit={(event) => void submit(event)}>
          <label className="wish-text-label">
            想实现什么游戏？
            <textarea
              value={text}
              maxLength={2000}
              rows={5}
              required
              placeholder="例如：一款 2v2 潜艇对战，能选择不同潜艇，地图里有深海热泉和随机补给……"
              onChange={(event) => {
                setText(event.target.value);
                setError("");
              }}
            />
          </label>
          <div className="wish-form-foot">
            <div className="wish-upload">
              <button
                type="button"
                className="outline"
                onClick={() => fileInput.current?.click()}
              >
                <ImagePlus size={16} />
                {file ? "更换参考图" : "添加参考图"}
              </button>
              <input
                ref={fileInput}
                type="file"
                accept={imageTypes.join(",")}
                aria-label="上传许愿参考图"
                onChange={(event) => pick(event.target.files?.[0])}
              />
              <small>可选，PNG／JPEG／WebP／GIF，最大 5 MB</small>
            </div>
            <span className="wish-counter">{text.length} / 2000</span>
          </div>
          {preview && (
            <figure className="wish-preview">
              <img src={preview} alt="待上传的参考图预览" />
              <button
                type="button"
                className="subtle"
                onClick={() => {
                  setFile(undefined);
                  if (fileInput.current) fileInput.current.value = "";
                }}
              >
                移除图片
              </button>
            </figure>
          )}
          {error && (
            <div className="admin-message error" role="alert">
              {error}
            </div>
          )}
          {notice && (
            <div className="admin-message" role="status">
              {notice}
            </div>
          )}
          <div className="wish-submit">
            <button className="primary" disabled={busy || text.trim().length < 2}>
              <Send size={16} />
              {busy ? "正在提交…" : "提交许愿"}
            </button>
          </div>
        </form>
        <section className="wish-history" aria-label="我的许愿">
          <div className="wish-history-head">
            <h3>我的许愿</h3>
            <small>只有管理员能看到全部许愿；这里仅显示你自己的记录。</small>
          </div>
          {loadError && (
            <div className="admin-message error" role="alert">
              {loadError}
            </div>
          )}
          {!wishes ? (
            <p className="admin-empty">正在读取…</p>
          ) : !wishes.length ? (
            <p className="admin-empty">还没有许愿，写下第一个想法吧。</p>
          ) : (
            <div className="wish-list">
              {wishes.map((wish) => (
                <article key={wish.id} className="wish-item">
                  <header>
                    <WishStatus wish={wish} />
                    <small>{date(wish.createdAt)}</small>
                  </header>
                  <p>{wish.text}</p>
                  {wish.imageUrl && (
                    <a
                      href={wish.imageUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="wish-image-link"
                    >
                      <img src={wish.imageUrl} alt="许愿参考图" loading="lazy" />
                    </a>
                  )}
                  {wish.note && (
                    <div className="wish-note">管理员备注：{wish.note}</div>
                  )}
                </article>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
