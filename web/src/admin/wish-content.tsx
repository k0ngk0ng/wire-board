import { useState, type FormEvent } from "react";
import { Pencil, Save } from "lucide-react";
import type { Wish } from "../wishes";

export function WishContent({
  wish,
  onSaved,
}: {
  wish: Wish;
  onSaved: () => Promise<void>;
}) {
  // A draft belongs to the open editor, not to the five-second overview poll.
  const [draft, setDraft] = useState<{ text: string; revision: number }>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const length = Array.from(draft?.text.trim() ?? "").length;
  async function save(event: FormEvent) {
    event.preventDefault();
    if (!draft || busy || length < 2 || length > 10000) return;
    setBusy(true);
    setError("");
    try {
      const response = await fetch(`/api/admin/wishes/${wish.id}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "edit",
          text: draft.text,
          expectedRevision: draft.revision,
        }),
      });
      const result = await response.json();
      if (!response.ok)
        throw new Error(result.error || "许愿修改未保存，请重试");
      await onSaved();
      setDraft(undefined);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  if (draft)
    return (
      <form
        className="admin-wish-editor"
        onSubmit={save}
        aria-label="编辑许愿正文"
      >
        <label htmlFor={`wish-edit-${wish.id}`}>许愿正文</label>
        <textarea
          id={`wish-edit-${wish.id}`}
          value={draft.text}
          onChange={(e) => setDraft({ ...draft, text: e.target.value })}
          rows={8}
          disabled={busy}
          autoFocus
        />
        <small>{length} / 10000 字</small>
        {error && (
          <p className="admin-message error" role="alert">
            {error}
          </p>
        )}
        <div className="admin-wish-actions">
          <button
            className="primary"
            type="submit"
            data-wish-edit-save
            disabled={busy || length < 2 || length > 10000}
          >
            <Save size={16} />
            {busy ? "保存中…" : "保存修改"}
          </button>
          <button
            className="outline"
            type="button"
            data-wish-edit-cancel
            disabled={busy}
            onClick={() => {
              setDraft(undefined);
              setError("");
            }}
          >
            取消
          </button>
        </div>
      </form>
    );
  return (
    <div className="admin-wish-content">
      <p className="admin-wish-text">{wish.text}</p>
      <div className="admin-wish-actions">
        <button
          className="outline"
          data-admin-wish-edit
          onClick={() => {
            setDraft({ text: wish.text, revision: wish.contentRevision });
            setError("");
          }}
        >
          <Pencil size={16} />
          编辑正文
        </button>
        {wish.editedAt && (
          <small>
            {wish.editedByName} ·{" "}
            {new Date(wish.editedAt * 1000).toLocaleString("zh-CN")} 编辑
          </small>
        )}
      </div>
    </div>
  );
}
