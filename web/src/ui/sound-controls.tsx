import { Play, Volume1, Volume2, VolumeX } from "lucide-react";
import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import "./sound-controls.css";

export function SoundControls({
  enabled,
  volume,
  onVolume,
  onMute,
  onPreview,
}: {
  enabled: boolean;
  volume: number;
  onVolume: (value: number) => void;
  onMute: () => void;
  onPreview: () => Promise<string>;
}) {
  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState("");
  const [position, setPosition] = useState({ left: 0, top: 0 });
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const request = useRef(0);
  const id = useId();
  const percent = Math.round(volume * 100);
  const Icon = !enabled ? VolumeX : volume <= 0.5 ? Volume1 : Volume2;

  useEffect(() => {
    request.current++;
    setStatus("");
  }, [enabled, open]);
  useEffect(
    () => () => {
      request.current++;
    },
    [],
  );
  useLayoutEffect(() => {
    if (!open) return;
    const place = () => {
      const anchor = trigger.current?.getBoundingClientRect();
      const popup = panel.current?.getBoundingClientRect();
      if (!anchor || !popup) return;
      const viewport = window.visualViewport;
      const left = viewport?.offsetLeft ?? 0;
      const top = viewport?.offsetTop ?? 0;
      const width = viewport?.width ?? innerWidth;
      const height = viewport?.height ?? innerHeight;
      setPosition({
        left: Math.max(
          left + 8,
          Math.min(anchor.right - popup.width, left + width - popup.width - 8),
        ),
        top: Math.max(
          top + 8,
          Math.min(anchor.bottom + 8, top + height - popup.height - 8),
        ),
      });
    };
    place();
    panel.current
      ?.querySelector<HTMLInputElement>("input")
      ?.focus({ preventScroll: true });
    const outside = (event: PointerEvent) => {
      const target = event.target as Node;
      if (
        !panel.current?.contains(target) &&
        !trigger.current?.contains(target)
      )
        setOpen(false);
    };
    document.addEventListener("pointerdown", outside, true);
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true);
    window.visualViewport?.addEventListener("resize", place);
    window.visualViewport?.addEventListener("scroll", place);
    return () => {
      document.removeEventListener("pointerdown", outside, true);
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
      window.visualViewport?.removeEventListener("resize", place);
      window.visualViewport?.removeEventListener("scroll", place);
    };
  }, [open]);

  return (
    <div className="sound-control" onClick={(event) => event.stopPropagation()}>
      <button
        ref={trigger}
        className="sound-control-trigger"
        data-sound-trigger
        aria-label="声音控制"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        title={enabled ? `音量 ${percent}%` : "已静音"}
        onClick={() => setOpen(!open)}
      >
        <Icon size={20} aria-hidden="true" />
      </button>
      {open &&
        createPortal(
          <div
            ref={panel}
            id={id}
            className="sound-control-panel"
            data-sound-panel
            role="dialog"
            aria-label="声音控制"
            style={position}
            onKeyDown={(event) => {
              // Range keys, Space and Escape belong to this popup, not the game.
              event.stopPropagation();
              if (event.key === "Escape") {
                event.preventDefault();
                setOpen(false);
                trigger.current?.focus({ preventScroll: true });
              }
            }}
            onBlur={(event) => {
              const next = event.relatedTarget as Node | null;
              if (
                next &&
                !event.currentTarget.contains(next) &&
                !trigger.current?.contains(next)
              )
                setOpen(false);
            }}
          >
            <div className="sound-control-heading">
              <label htmlFor={`${id}-volume`}>音量</label>
              <output htmlFor={`${id}-volume`}>{percent}%</output>
            </div>
            <div className="sound-control-level">
              <button
                className="sound-control-mute"
                data-sound-mute
                aria-label={enabled ? "静音" : "取消静音"}
                aria-pressed={!enabled}
                title={enabled ? "静音" : "取消静音"}
                onClick={onMute}
              >
                <Icon size={20} aria-hidden="true" />
              </button>
              <input
                id={`${id}-volume`}
                data-volume
                type="range"
                min={0}
                max={100}
                step={1}
                value={percent}
                aria-valuetext={`${percent}%`}
                onChange={(event) =>
                  onVolume(Number(event.currentTarget.value) / 100)
                }
              />
            </div>
            <button
              className="sound-control-preview"
              data-sound-preview
              disabled={!enabled}
              onClick={async () => {
                const current = ++request.current;
                const message = await onPreview();
                if (current === request.current) setStatus(message);
              }}
            >
              <Play size={15} aria-hidden="true" />
              试听音效
            </button>
            <div
              className="sound-control-status"
              data-sound-status
              role="status"
            >
              {status}
            </div>
          </div>,
          document.body,
        )}
    </div>
  );
}
