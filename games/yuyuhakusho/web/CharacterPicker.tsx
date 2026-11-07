import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import portraits from "../assets/character-portraits.png";
import { characters } from "./labels";

function Portrait({ character }: { character: number }) {
  return <span className="yuyu-portrait" aria-hidden="true">
    <img src={portraits} alt="" draggable={false} style={{ left: `${-character * 100}%` }} />
  </span>;
}

type PickerProps = {
  value: number;
  seat?: number;
  label: string;
  disabled: boolean;
  onChange: (value: number) => void;
};
export function ImagePicker({ value, seat, label, disabled, onChange, choices, renderImage, kind = "character" }: PickerProps & {
  choices: readonly { value: number; name: string }[];
  renderImage: (value: number) => ReactNode;
  kind?: "character" | "arena";
}) {
  const selected = choices.find(choice => choice.value === value)?.name ?? "";
  const root = useRef<HTMLDetailsElement>(null);
  const menu = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [placement, setPlacement] = useState({ left: 16, top: 16, maxHeight: 360 });
  const close = () => { if (root.current) root.current.open = false; setOpen(false); };
  useEffect(() => {
    if (disabled) close();
  }, [disabled]);
  useEffect(() => {
    const outside = (event: PointerEvent) => {
      if (event.target instanceof Node && !root.current?.contains(event.target) && !menu.current?.contains(event.target)) close();
    };
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, []);
  useLayoutEffect(() => {
    if (!open) return;
    const place = () => {
      const rect = root.current?.getBoundingClientRect();
      const grid = menu.current;
      if (!rect || !grid) return;
      const below = innerHeight - rect.bottom - 16, above = rect.top - 16;
      const upward = grid.scrollHeight > below && above > below;
      const maxHeight = Math.max(120, Math.min(360, upward ? above : below));
      const width = Math.min(360, innerWidth - 48);
      setPlacement({ left: Math.max(16, Math.min(rect.left, innerWidth - width - 16)),
        top: upward ? rect.top - Math.min(grid.scrollHeight, maxHeight) - 6 : rect.bottom + 6, maxHeight });
    };
    place();
    menu.current?.querySelector<HTMLButtonElement>('[aria-pressed="true"]')?.focus({ preventScroll: true });
    window.addEventListener("resize", place);
    document.addEventListener("scroll", place, true);
    return () => { window.removeEventListener("resize", place); document.removeEventListener("scroll", place, true); };
  }, [open]);
  return <details ref={root} className="yuyu-character-picker" data-character-seat={kind === "character" ? seat : undefined}
    data-character={kind === "character" ? value : undefined} data-arena-picker={kind === "arena" ? true : undefined}
    data-arena={kind === "arena" ? value : undefined}
    onToggle={event => setOpen(event.currentTarget.open)}
    onBlurCapture={event => {
      const next = event.relatedTarget;
      if (!(next instanceof Node) || (!root.current?.contains(next) && !menu.current?.contains(next))) close();
    }}
    onKeyDown={event => {
      if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); close(); root.current?.querySelector("summary")?.focus(); }
    }}>
    <summary className="yuyu-character-summary" tabIndex={disabled ? -1 : 0} aria-label={`${label}：${selected}`} aria-disabled={disabled}
      onClick={event => { if (disabled) event.preventDefault(); else setOpen(!root.current?.open); }}>
      {renderImage(value)}
      <span>{selected}</span>
      {!disabled && <span className="yuyu-character-chevron" aria-hidden="true">⌄</span>}
    </summary>
    {open && createPortal(<div ref={menu} className={`yuyu-choice-grid yuyu-${kind}-grid`} role="group" aria-label={label}
      style={{ left: placement.left, top: placement.top, maxHeight: placement.maxHeight }}>
      {choices.map(choice => <button key={choice.value} type="button" className="yuyu-character-option"
        data-character-option={kind === "character" ? choice.value : undefined}
        data-arena-option={kind === "arena" ? choice.value : undefined} aria-pressed={value === choice.value} disabled={disabled}
        onClick={() => { if (!disabled) { onChange(choice.value); close(); root.current?.querySelector("summary")?.focus(); } }}>
        {renderImage(choice.value)}<span>{choice.name}</span>
      </button>)}
    </div>, root.current?.closest('[role="dialog"]') ?? document.body)}
  </details>;
}

export function CharacterPicker(props: PickerProps & { seat: number }) {
  return <ImagePicker {...props} choices={characters.map((name, value) => ({ value, name }))}
    renderImage={value => <Portrait character={value} />} />;
}
