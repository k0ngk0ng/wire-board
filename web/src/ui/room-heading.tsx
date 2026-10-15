import { useLayoutEffect, useRef, useState, type ReactNode } from "react";

// Ending a match replaces room controls as well as showing its result. Preserve
// the current header's footprint so those controls do not move the final board.
export function RoomHeading({
  roomId,
  finished,
  children,
}: {
  roomId: string;
  finished: boolean;
  children: ReactNode;
}) {
  const element = useRef<HTMLDivElement>(null);
  const playingSize = useRef<
    | {
        roomId: string;
        width: number;
        height: number;
      }
    | undefined
  >(undefined);
  const [reserved, setReserved] = useState(0);
  useLayoutEffect(() => {
    const el = element.current;
    if (!el) return;
    const measure = () => {
      const { width, height } = el.getBoundingClientRect();
      if (!finished) {
        playingSize.current = { roomId, width, height };
        setReserved(0);
      } else {
        const previous = playingSize.current;
        if (previous?.roomId === roomId && Math.abs(previous.width - width) < 1)
          setReserved(previous.height);
        else {
          playingSize.current = undefined;
          setReserved(0);
        }
      }
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [roomId, finished]);
  return (
    <div
      ref={element}
      className="room-heading"
      style={finished && reserved ? { minHeight: reserved } : undefined}
    >
      {children}
    </div>
  );
}
