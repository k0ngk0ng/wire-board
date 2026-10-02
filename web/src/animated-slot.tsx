import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";

// Keep the outgoing artwork briefly, without retaining any interactive controls.
export function AnimatedSlot({
  identity,
  children,
}: {
  identity: string;
  children: ReactNode;
}) {
  const previous = useRef({ identity, children });
  const [leaving, setLeaving] = useState<{
    node: ReactNode;
    key: string;
  } | null>(null);
  useLayoutEffect(() => {
    if (previous.current.identity !== identity) {
      setLeaving({ node: previous.current.children, key: identity });
    }
    previous.current = { identity, children };
  }, [identity, children]);
  useEffect(() => {
    if (!leaving) return;
    const timer = setTimeout(() => setLeaving(null), 380);
    return () => clearTimeout(timer);
  }, [leaving]);
  return (
    <div
      className={`card-slot ${leaving ? "is-changing" : ""}`}
      aria-busy={!!leaving}
    >
      <div className="slot-current" key={identity}>
        {children}
      </div>
      {leaving && (
        <div
          className="slot-outgoing"
          key={`out-${leaving.key}`}
          aria-hidden="true"
          inert
        >
          {leaving.node}
        </div>
      )}
    </div>
  );
}
