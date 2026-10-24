import {
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Binoculars,
  Castle,
  Check,
  Coins,
  Flag,
  Footprints,
  Minus,
  Pause,
  Play,
  Plus,
  RotateCcw,
  ScrollText,
  ShieldMinus,
  Swords,
  Trash2,
  UserPlus,
  X,
} from "lucide-react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import { viewPoint, visibleCameraRect } from "./camera";
import { useCamera } from "./useCamera";
import { createCursor, consumeEvents } from "./events";
import {
  marchAmount,
  recruitCapacity,
  recruitShortages,
  troopAmount,
} from "./map-actions";
import { TroopAmount } from "./TroopAmount";
import { Crown } from "./Crown";
import { useHoldAction } from "./hold-action";
import type { ConquestMap, Event, MapProvince, Order, View } from "./types";

const loaders: Record<string, () => Promise<{ default: ConquestMap }>> = {
  world: () => import("../maps/world.json"),
  europe: () => import("../maps/europe.json"),
  usa: () => import("../maps/usa.json"),
  china: () => import("../maps/china.json"),
};
const number = (n: number) => n.toLocaleString("zh-CN");
const force = (n: number) =>
  n < 0
    ? "?"
    : n >= 10000
      ? `${(n / 10000).toFixed(n >= 100000 ? 0 : 1)}万`
      : String(n);

export function Board(props: BoardProps<View>) {
  const { data, onLoadError, onLoadProgress } = props;
  const [map, setMap] = useState<ConquestMap | null>(null),
    [error, setError] = useState("");
  useEffect(() => {
    let alive = true;
    setMap(null);
    setError("");
    onLoadProgress?.(5, "加载地图");
    const load = loaders[data.mapId];
    if (!load) {
      const message = "未找到这张地图";
      setError(message);
      onLoadError?.(message);
      return;
    }
    void load()
      .then(({ default: next }) => {
        if (!alive) return;
        if (next.version !== data.mapVersion || next.sha256 !== data.mapSHA256)
          throw new Error("地图版本与对局不一致，请刷新后重试");
        onLoadProgress?.(75, "绘制地形与军队");
        setMap(next);
      })
      .catch((e: unknown) => {
        if (!alive) return;
        const message = e instanceof Error ? e.message : "地图加载失败";
        setError(message);
        onLoadError?.(message);
      });
    return () => {
      alive = false;
    };
  }, [
    data.mapId,
    data.mapVersion,
    data.mapSHA256,
    onLoadError,
    onLoadProgress,
  ]);
  return map ? (
    <MapBoard
      key={`${props.room.id}:${data.session}:${map.sha256}`}
      {...props}
      map={map}
    />
  ) : (
    <div className="cq-loading" role="status">
      {error || "正在绘制地图…"}
    </div>
  );
}

function MapBoard({
  room,
  data,
  map,
  act,
  busy,
  clock,
  audio,
  assets,
  sound,
  onReady,
  onLoadProgress,
}: BoardProps<View> & { map: ConquestMap }) {
  const camera = useCamera(map),
    uid = useId().replaceAll(":", "");
  const renderCount = useRef(0);
  renderCount.current++;
  const [selected, setSelected] = useState(""),
    [target, setTarget] = useState(""),
    [marching, setMarching] = useState(false),
    [recruiting, setRecruiting] = useState(false),
    [requested, setRequested] = useState<number | null>(null),
    [king, setKing] = useState(false);
  const [notice, setNotice] = useState(""),
    [pending, setPending] = useState(false),
    [plansOpen, setPlansOpen] = useState(false),
    [financeOpen, setFinanceOpen] = useState(false),
    [resignOpen, setResignOpen] = useState(false);
  const [tax, setTax] = useState(10),
    [activeEvents, setActiveEvents] = useState<Event[]>([]);
  const [marchQueue, setMarchQueue] = useState<
    {
      from: string;
      to: string;
      king: boolean;
      amount: number | null;
      epoch: number;
      session: string;
    }[]
  >([]);
  const awaitedOrder = useRef<{
    after: number;
    from: string;
    to: string;
    amount: number;
    king: boolean;
  } | null>(null);
  const cursor = useRef(
    createCursor({ events: data.events ?? [], round: data.round }, clock.epoch),
  );
  const latestClock = useRef(clock);
  latestClock.current = clock;
  const ready = useRef(false),
    operation = useRef(false);
  const landOutline = useMemo(
    () => map.provinces.map((p) => p.path).join(""),
    [map],
  );
  const geography = useMemo(
    () => new Map(map.provinces.map((p) => [p.id, p])),
    [map],
  );
  const provinces = useMemo(
    () => new Map(data.provinces.map((p) => [p.id, p])),
    [data.provinces],
  );
  const projection = useMemo(
    () => new Map(data.projected.map((p) => [p.id, p])),
    [data.projected],
  );
  const me = data.players[room.you],
    selectedGeo = geography.get(selected),
    selectedState = provinces.get(selected),
    selectedPlan = projection.get(selected) ?? selectedState;
  const own = selectedState?.owner === room.you && !room.spectating;
  const controlled =
    !room.spectating &&
    !room.seats[room.you]?.autoPlay &&
    room.status === "playing";
  const canPlan =
    controlled &&
    data.controls.canPlan &&
    !busy &&
    !pending &&
    marchQueue.length === 0;
  const adjacent = useMemo(
    () =>
      new Set(
        selectedGeo
          ? [...selectedGeo.neighbors, ...selectedGeo.seaNeighbors]
          : [],
      ),
    [selectedGeo],
  );
  const plan = data.orders ?? [];
  const provinceName = (id?: string) =>
    geography.get(id ?? "")?.name ?? id ?? "";
  const ownerName = (owner: number) =>
    owner < 0 ? "中立" : (room.seats[owner]?.name ?? "未知势力");
  const color = (owner: number) =>
    owner < 0 ? "#d3d1bb" : (data.players[owner]?.color ?? "#758974");
  const now = () =>
    latestClock.current.serverNow +
    Math.max(0, performance.now() - latestClock.current.receivedAt);
  const fullRecruit =
    own && selectedPlan
      ? recruitCapacity(
          data.availableGold,
          selectedPlan.population,
          selectedPlan.troops,
          data.availableMovement,
          data.controls.recruitCost,
          data.controls.recruitMovement,
        )
      : 0;
  const building = selectedPlan?.fort && !selectedPlan.town ? "town" : "fort";
  const buildCost =
    building === "town"
      ? (data.controls.townCost ?? 75000)
      : data.controls.fortCost;
  const buildMovement =
    building === "town"
      ? (data.controls.townMovement ?? 5)
      : data.controls.fortMovement;
  const canBuild =
    own &&
    canPlan &&
    data.availableGold >= buildCost &&
    data.availableMovement >= buildMovement &&
    (building === "town" ? !selectedPlan?.town : !selectedPlan?.fort);
  const canTower =
    own &&
    canPlan &&
    !!data.controls.canBuildTower &&
    !selectedPlan?.tower &&
    data.availableGold >= data.controls.towerCost &&
    data.availableMovement >= data.controls.towerMovement;
  const recruit = troopAmount(fullRecruit, requested);
  const recruitReason =
    own && selectedPlan
      ? recruitShortages(
          data.availableGold,
          selectedPlan.population,
          selectedPlan.troops,
          data.availableMovement,
          data.controls.recruitCost,
          data.controls.recruitMovement,
        )
          .map(
            (reason) =>
              ({
                movement: "行动点不足",
                gold: "金币不足",
                population: "人口不足（至少需要 2 人）",
                capacity: "驻军已达上限",
              })[reason],
          )
          .join("、")
      : "";
  const holdIdentity = `${data.session}:${data.epoch}:${selected}`;
  const fortPress = useHoldAction({
    enabled: canBuild,
    identity: holdIdentity,
    onTap: () => send("build", { province: selected, building }),
    onHold: () => {
      if (selectedPlan?.town) {
        setNotice("这块领地已经有城镇");
        return;
      }
      const cost = data.controls.townCost ?? 75000,
        points = data.controls.townMovement ?? 5;
      if (
        !canPlan ||
        data.availableGold < cost ||
        data.availableMovement < points
      ) {
        setNotice(`建城镇需要 ${number(cost)} 金币和 ${points} 行动点`);
        return;
      }
      return send("build", { province: selected, building: "town" });
    },
  });
  const visibleLabels = useMemo(() => {
    const candidates = map.provinces.filter((p) => {
      const s = provinces.get(p.id);
      return (
        !!s &&
        (p.id === selected ||
          p.id === target ||
          (s.owner >= 0 && (s.troops > 0 || s.fort)))
      );
    });
    candidates.sort(
      (a, b) =>
        (a.id === selected || a.id === target
          ? 0
          : provinces.get(a.id)?.owner === room.you
            ? 1
            : 2) -
        (b.id === selected || b.id === target
          ? 0
          : provinces.get(b.id)?.owner === room.you
            ? 1
            : 2),
    );
    const labels: { p: MapProvince; point: { x: number; y: number } }[] = [];
    for (const p of candidates) {
      const point = viewPoint(camera.camera, p.label);
      // Keep known armies mounted outside the viewport too. The SVG clips
      // them; camera transforms can bring them into view before React settles.
      if (
        p.id !== selected &&
        p.id !== target &&
        labels.some(
          (x) =>
            Math.abs(x.point.x - point.x) < 46 &&
            Math.abs(x.point.y - point.y) < 28,
        )
      )
        continue;
      labels.push({ p, point });
    }
    return labels;
  }, [map, provinces, selected, target, camera.camera, camera.size, room.you]);

  useEffect(() => {
    if (ready.current || camera.size.width < 2) return;
    let first = 0,
      second = 0,
      alive = true;
    void document.fonts.ready.then(() => {
      if (!alive) return;
      if (camera.size.width <= 620 && !room.spectating) {
        const home =
          geography.get(me?.king ?? "") ??
          map.provinces.find((p) => provinces.get(p.id)?.owner === room.you);
        if (home) camera.focus(home.label);
      }
      first = requestAnimationFrame(() => {
        second = requestAnimationFrame(() => {
          if (!alive) return;
          ready.current = true;
          onLoadProgress?.(100, "地图已就绪");
          onReady?.();
        });
      });
    });
    return () => {
      alive = false;
      cancelAnimationFrame(first);
      cancelAnimationFrame(second);
    };
  }, [camera.size.width, onReady, onLoadProgress]);
  useEffect(() => {
    const next = consumeEvents(
      cursor.current,
      { events: data.events ?? [], round: data.round },
      clock.epoch,
      now(),
      sound && !document.hidden,
      !document.hidden &&
        !matchMedia("(prefers-reduced-motion: reduce)").matches,
    );
    if (next.reset) {
      setActiveEvents([]);
      audio.stop();
      return;
    }
    if (next.events.length)
      setActiveEvents((previous) =>
        [...previous, ...next.events].filter((e) => e.until > now()),
      );
    for (const clip of next.clips) {
      const local = `/conquest/v1/audio/${clip}.wav`;
      void audio.playWithFallback(
        assets ? [assets.replace(/\/$/, "") + local, local] : [local],
      );
    }
  }, [data.events, data.round, clock.epoch, sound, audio, assets]);
  useEffect(() => {
    if (!activeEvents.length) return;
    const timer = setTimeout(
      () => setActiveEvents((v) => v.filter((e) => e.until > now())),
      Math.max(12, Math.min(...activeEvents.map((e) => e.until)) - now() + 8),
    );
    return () => clearTimeout(timer);
  }, [activeEvents]);
  useEffect(() => {
    const release = () => {
      if (document.hidden) {
        Object.assign(
          cursor.current,
          createCursor(
            { events: data.events ?? [], round: data.round },
            clock.epoch,
          ),
        );
        setActiveEvents([]);
        audio.stop();
      }
    };
    document.addEventListener("visibilitychange", release);
    return () => document.removeEventListener("visibilitychange", release);
  }, [data.events, data.round, clock.epoch, audio]);
  useEffect(() => () => audio.stop(), [audio]);
  useEffect(() => {
    if (!own || me?.king !== selected) setKing(false);
  }, [own, me?.king, selected]);
  useLayoutEffect(() => {
    if (!selected || !own || camera.size.width > 620) return;
    const board = camera.viewport.current?.closest<HTMLElement>(
      "[data-conquest-board]",
    );
    const toolbar = board?.querySelector<HTMLElement>(
      '[aria-label="领地行动"]',
    );
    if (!board || !toolbar) return;
    const viewportTop = window.visualViewport?.offsetTop ?? 0;
    const viewportBottom =
      viewportTop + (window.visualViewport?.height ?? window.innerHeight);
    const bounds = toolbar.getBoundingClientRect();
    if (bounds.bottom > viewportBottom - 8 || bounds.top < viewportTop + 8) {
      window.scrollTo({
        top: Math.max(
          0,
          window.scrollY + board.getBoundingClientRect().top - viewportTop - 8,
        ),
        behavior: "instant",
      });
    }
  }, [selected, own, camera.size.width]);
  useEffect(() => {
    if (me?.tax !== undefined) setTax(me.tax);
  }, [me?.tax]);
  useEffect(() => {
    if (!data.controls.canPlan) {
      setMarching(false);
      setRecruiting(false);
    }
  }, [data.controls.canPlan]);
  useEffect(() => {
    setMarchQueue([]);
    setRecruiting(false);
    setMarching(false);
    setRequested(null);
    awaitedOrder.current = null;
  }, [data.session, data.epoch, controlled]);
  useEffect(() => {
    if (!notice) return;
    const timer = setTimeout(() => setNotice(""), 3000);
    return () => clearTimeout(timer);
  }, [notice]);

  async function send(type: string, extra: Record<string, unknown> = {}) {
    if (operation.current) return false;
    operation.current = true;
    setPending(true);
    setNotice("");
    try {
      await act({ type, session: data.session, epoch: data.epoch, ...extra });
      return true;
    } catch (e) {
      setNotice(e instanceof Error ? e.message : "操作未能完成，请重试");
      return false;
    } finally {
      operation.current = false;
      setPending(false);
    }
  }
  useEffect(() => {
    const receipt = awaitedOrder.current;
    if (receipt) {
      const confirmed = plan.some(
        (o) =>
          o.id > receipt.after &&
          o.type === "move" &&
          o.from === receipt.from &&
          o.to === receipt.to &&
          o.amount === receipt.amount &&
          !!o.king === receipt.king,
      );
      if (confirmed) awaitedOrder.current = null;
      else if (!pending && !busy && !operation.current) {
        // Platform act() finishes after refreshing even when the request was
        // rejected. No matching authoritative order means there is no receipt
        // to wait for; do not leave the next click permanently blocked.
        awaitedOrder.current = null;
        setMarchQueue([]);
        setNotice("计划已变化，请重新点选目标");
        return;
      }
    }
    if (!marchQueue.length) return;
    const intent = marchQueue[0];
    if (
      !controlled ||
      !data.controls.canPlan ||
      intent.epoch !== data.epoch ||
      intent.session !== data.session
    ) {
      setMarchQueue([]);
      awaitedOrder.current = null;
      return;
    }
    if (pending || busy || operation.current || awaitedOrder.current) return;
    if (plan.length >= data.controls.maxOrders) {
      setMarchQueue([]);
      setNotice("本轮计划已满，请先撤销一些操作");
      return;
    }
    const from = projection.get(intent.from),
      geo = geography.get(intent.from);
    const amount =
      from?.owner === room.you && geo
        ? marchAmount(
            from.mobile,
            data.availableGold,
            data.availableMovement,
            geo.seaNeighbors.includes(intent.to),
            data.controls.seaCost ?? 2,
            data.controls.moveCost,
            intent.amount,
          )
        : 0;
    if (!amount) {
      setMarchQueue([]);
      setNotice(
        !from || from.owner !== room.you
          ? "领地归属已变化，请重新选择"
          : from.mobile <= 0
            ? "这块领地没有可移动的兵力"
            : data.availableMovement < data.controls.moveCost
              ? "本轮行动点不足"
              : geo?.seaNeighbors.includes(intent.to)
                ? `海运这支部队需要 ${number(troopAmount(from.mobile, intent.amount) * (data.controls.seaCost ?? 2))} 金币`
                : "当前无法派兵，请重新选择",
      );
      return;
    }
    const carryKing =
      intent.king && !plan.some((o) => o.type === "move" && o.king);
    awaitedOrder.current = {
      after: Math.max(0, ...plan.map((o) => o.id)),
      from: intent.from,
      to: intent.to,
      amount,
      king: carryKing,
    };
    setMarchQueue((q) => q.slice(1));
    void send("move", {
      from: intent.from,
      to: intent.to,
      amount,
      king: carryKing,
    }).then((ok) => {
      if (!ok) {
        awaitedOrder.current = null;
        setMarchQueue([]);
      }
      if (ok && carryKing) setKing(false);
    });
  }, [
    marchQueue,
    pending,
    busy,
    controlled,
    data,
    projection,
    geography,
    room.you,
  ]);
  async function choose(id: string) {
    if (!geography.has(id)) return;
    if (marching && own && id !== selected) {
      if (!controlled || !data.controls.canPlan) return;
      if (!adjacent.has(id)) {
        setNotice("只能向相邻领地派兵");
        return;
      }
      setTarget(id);
      setMarchQueue((q) =>
        q.length < 16
          ? [
              ...q,
              {
                from: selected,
                to: id,
                king,
                amount:
                  requested === null
                    ? null
                    : troopAmount(selectedPlan?.mobile ?? 0, requested),
                epoch: data.epoch,
                session: data.session,
              },
            ]
          : q,
      );
      return;
    }
    setSelected(id);
    setRecruiting(false);
    setRequested(null);
    setTarget("");
    setMarching(false);
    setMarchQueue([]);
    setKing(false);
    setNotice("");
  }
  function cancelAmount() {
    setMarching(false);
    setRecruiting(false);
    setRequested(null);
    setMarchQueue([]);
    setTarget("");
  }
  function clear() {
    cancelAmount();
    setMarchQueue([]);
    setSelected("");
    setTarget("");
    setNotice("");
  }
  function keyChoose(
    event: React.KeyboardEvent<SVGPathElement>,
    p: MapProvince,
  ) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      void choose(p.id);
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      clear();
      return;
    }
    const direction: Record<string, { x: number; y: number }> = {
      ArrowLeft: { x: -1, y: 0 },
      ArrowRight: { x: 1, y: 0 },
      ArrowUp: { x: 0, y: -1 },
      ArrowDown: { x: 0, y: 1 },
    };
    const d = direction[event.key];
    if (!d) return;
    event.preventDefault();
    const next = map.provinces
      .filter(
        (q) =>
          q.id !== p.id &&
          (q.label.x - p.label.x) * d.x + (q.label.y - p.label.y) * d.y > 0,
      )
      .sort((a, b) => {
        const cost = (q: MapProvince) => {
          const dx = q.label.x - p.label.x,
            dy = q.label.y - p.label.y;
          return Math.hypot(dx, dy) + Math.abs(dx * d.y - dy * d.x) * 2;
        };
        return cost(a) - cost(b);
      })[0];
    if (next) {
      camera.focus(next.label);
      camera.viewport.current
        ?.querySelector<SVGPathElement>(`[data-cq-province="${next.id}"]`)
        ?.focus({ preventScroll: true });
    }
  }
  function miniLocate(event: React.PointerEvent<SVGSVGElement>) {
    const r = event.currentTarget.getBoundingClientRect();
    camera.locate({
      x: ((event.clientX - r.left) / r.width) * map.width,
      y: ((event.clientY - r.top) / r.height) * map.height,
    });
  }
  const miniRelease = (e: React.PointerEvent<SVGSVGElement>) => {
    if (e.currentTarget.hasPointerCapture(e.pointerId))
      e.currentTarget.releasePointerCapture(e.pointerId);
    camera.settle();
  };
  const visibleMap = visibleCameraRect(camera.camera, map, camera.size);
  const status = room.spectating
    ? "正在观战"
    : me?.eliminated
      ? "你的势力已退出"
      : data.phase === "resolving"
        ? "各方正在行动"
        : data.phase === "paused"
          ? "已暂停"
          : data.phase === "finished"
            ? "对局结束"
            : me?.submitted
              ? "等待其他势力"
              : marching
                ? "点相邻领地派兵，再点继续追加"
                : selectedGeo
                  ? ""
                  : "选择你的领地";
  const movementTotal = Math.max(0, me?.movement ?? 0);
  const movementUsed =
    Math.round(Math.max(0, movementTotal - data.availableMovement) * 10) / 10;
  const amountOpen = own && (recruiting || marching);
  const orderLabel = (o: Order) =>
    o.type === "move"
      ? `${provinceName(o.from)} → ${provinceName(o.to)} · ${number(o.amount ?? 0)}`
      : o.type === "tax"
        ? `税率 ${o.value}%`
        : `${provinceName(o.province)} · ${o.type === "recruit" ? "征兵 " + number(o.amount ?? 0) : o.type === "disband" ? "解散 " + number(o.amount ?? 0) : o.building === "fort" ? "筑堡" : o.building === "tower" ? "瞭望塔" : "城镇"}`;

  return (
    <section
      className="cq-board cq-map-first"
      data-conquest-board
      data-phase={data.phase}
      data-round={data.round}
      data-map-render-count={renderCount.current}
      data-cq-selected={selected}
    >
      <header className="cq-campaign-strip">
        <div>
          <strong>{map.name}</strong>
          <span>第 {data.round} 轮</span>
        </div>
        {!room.spectating && me && (
          <div className="cq-campaign-budget">
            <button
              className="cq-text-button"
              aria-label="财政与税率"
              onClick={() => setFinanceOpen(true)}
            >
              <Coins size={15} />
              <b>{number(data.availableGold)}</b>
            </button>
            <span title="可用行动点">
              <Footprints size={15} />
              {data.availableMovement.toFixed(1)}
            </span>
          </div>
        )}
      </header>
      <div className={`cq-map-stage ${amountOpen ? "cq-amount-open" : ""}`}>
        <div className="cq-viewport" ref={camera.viewport} {...camera.handlers}>
          <svg
            className="cq-map"
            width="100%"
            height="100%"
            aria-label={`${map.name}征服地图`}
            onClick={(e) => {
              if (
                camera.canClick(e.detail) &&
                (e.target === e.currentTarget ||
                  (e.target as Element).hasAttribute("data-cq-water"))
              )
                clear();
            }}
          >
            <defs>
              <pattern
                id={`${uid}-water`}
                width="60"
                height="40"
                patternUnits="userSpaceOnUse"
              >
                <path
                  d="M4 18q9-5 18 0t18 0M34 35q7-4 14 0"
                  fill="none"
                  stroke="#d7e6de"
                  strokeWidth="1"
                  opacity=".55"
                />
              </pattern>
              <pattern
                id={`${uid}-land`}
                width="18"
                height="18"
                patternUnits="userSpaceOnUse"
              >
                <path
                  d="m4 7 2-3 2 3m4 7 2-3 2 3"
                  fill="none"
                  stroke="#5d6752"
                  strokeWidth=".5"
                  opacity=".16"
                />
              </pattern>
              <marker
                id={`${uid}-arrow`}
                viewBox="0 0 10 10"
                refX="8"
                refY="5"
                markerWidth="5"
                markerHeight="5"
                orient="auto-start-reverse"
              >
                <path d="M0 0 10 5 0 10 3 5Z" fill="#243e32" />
              </marker>
              <symbol id={`${uid}-army`} viewBox="0 0 24 24">
                <path
                  d="M5 22V8m14 14V7M3 7l2-5 2 5m10-1 2-5 2 5"
                  fill="none"
                  stroke="#34483a"
                  strokeWidth="1.3"
                />
                <path
                  d="M7 12q5-6 10 0l-1 9H8Z"
                  fill="currentColor"
                  stroke="#2f4435"
                  strokeWidth="1.1"
                />
                <path
                  d="M7 12V9a5 5 0 0 1 10 0v3Z"
                  fill="#e0d4ac"
                  stroke="#475842"
                  strokeWidth="1.2"
                />
                <path d="M9 10h6v3H9Z" fill="#34483a" />
              </symbol>
            </defs>
            <rect data-cq-water width="100%" height="100%" fill="#a8c6c8" />
            <rect
              data-cq-water
              width="100%"
              height="100%"
              fill={`url(#${uid}-water)`}
            />
            <g
              ref={camera.sceneRef}
              className="cq-map-layer"
              transform={`translate(${camera.camera.x} ${camera.camera.y}) scale(${camera.camera.scale})`}
            >
              {map.insets?.map((inset) => (
                <rect
                  key={inset.title}
                  x={inset.x}
                  y={inset.y}
                  width={inset.width}
                  height={inset.height}
                  fill="none"
                  stroke="#637d75"
                  strokeWidth="1"
                  strokeDasharray="4 3"
                  vectorEffect="non-scaling-stroke"
                  pointerEvents="none"
                />
              ))}
              {map.provinces.map((p) => {
                const s = provinces.get(p.id);
                const reachable = marching && adjacent.has(p.id);
                return (
                  <path
                    key={p.id}
                    data-cq-province={p.id}
                    data-owner={s?.owner ?? -1}
                    d={p.path}
                    fill={color(s?.owner ?? -1)}
                    fillRule="evenodd"
                    vectorEffect="non-scaling-stroke"
                    className={`cq-province ${p.id === selected ? "selected" : ""} ${p.id === target ? "target" : ""} ${reachable ? "reachable" : ""}`}
                    role="button"
                    tabIndex={
                      p.id === selected || (!selected && s?.owner === room.you)
                        ? 0
                        : -1
                    }
                    aria-label={`${p.name}，${ownerName(s?.owner ?? -1)}，兵力 ${force(s?.troops ?? -1)}${s?.town ? "，城镇" : ""}${s?.fort ? "，要塞" : ""}${s?.tower ? "，瞭望塔" : ""}`}
                    aria-pressed={p.id === selected}
                    onClick={(e) => {
                      e.stopPropagation();
                      if (camera.canClick(e.detail)) void choose(p.id);
                    }}
                    onKeyDown={(e) => keyChoose(e, p)}
                  />
                );
              })}
              <path
                data-cq-flat-land
                d={landOutline}
                fill={`url(#${uid}-land)`}
                fillRule="evenodd"
                pointerEvents="none"
              />
              {map.seaRoutes
                .filter((r) => r.from === selected || r.to === selected)
                .map((r, i) => (
                  <path
                    key={i}
                    d={r.path}
                    fill="none"
                    className="cq-sea-route"
                    vectorEffect="non-scaling-stroke"
                    pointerEvents="none"
                  />
                ))}
              {plan
                .filter((o) => o.type === "move")
                .map((o) => {
                  const a = geography.get(o.from ?? "")?.label,
                    b = geography.get(o.to ?? "")?.label;
                  return a && b ? (
                    <path
                      key={o.id}
                      data-cq-planned={o.id}
                      d={`M${a.x} ${a.y}L${b.x} ${b.y}`}
                      className="cq-plan-arrow"
                      markerEnd={`url(#${uid}-arrow)`}
                      vectorEffect="non-scaling-stroke"
                      pointerEvents="none"
                    />
                  ) : null;
                })}
              {activeEvents.map((e) => {
                const a = geography.get(e.from ?? "")?.label,
                  b = geography.get(e.to ?? e.province ?? "")?.label;
                if (!b) return null;
                const start = a ?? b,
                  progress = Math.max(
                    0,
                    Math.min(1, (now() - e.at) / Math.max(1, e.until - e.at)),
                  );
                const x = start.x + (b.x - start.x) * progress,
                  y = start.y + (b.y - start.y) * progress;
                return (
                  <g key={e.id} data-cq-event={e.id} pointerEvents="none">
                    <path
                      d={`M${start.x} ${start.y}L${b.x} ${b.y}`}
                      className="cq-event-route"
                      vectorEffect="non-scaling-stroke"
                    />
                    <circle
                      r={4 / camera.camera.scale}
                      fill={color(e.seat)}
                      stroke="#fff3c2"
                      strokeWidth={1.5 / camera.camera.scale}
                    >
                      <animateMotion
                        path={`M${x} ${y}L${b.x} ${b.y}`}
                        dur={`${Math.max(1, e.until - now())}ms`}
                        fill="freeze"
                      />
                    </circle>
                  </g>
                );
              })}
            </g>
            <g ref={camera.overlayRef} className="cq-army-layer">
              {map.insets?.map((inset) => {
                const p = viewPoint(camera.camera, {
                  x: inset.x + 6,
                  y: inset.y + 17,
                });
                return (
                  <g
                    key={inset.title}
                    transform={`translate(${p.x} ${p.y})`}
                    pointerEvents="none"
                  >
                    <g className="cq-map-annotation">
                      <text>{inset.title}</text>
                    </g>
                  </g>
                );
              })}
              {visibleLabels.map(({ p, point }) => {
                const actual = provinces.get(p.id)!;
                const projected = projection.get(p.id);
                const troops =
                  actual.owner === room.you && data.phase === "planning"
                    ? (projected?.mobile ?? actual.troops)
                    : actual.troops;
                const ruler = data.players.some(
                  (q) => !q.eliminated && q.king === p.id,
                );
                return (
                  <g key={p.id} transform={`translate(${point.x} ${point.y})`}>
                    <g
                      className={`cq-army-marker ${p.id === selected ? "selected" : ""}`}
                      data-cq-label={p.id}
                      onClick={(e) => {
                        e.stopPropagation();
                        if (camera.canClick(e.detail)) void choose(p.id);
                      }}
                      role="button"
                      aria-label={`${p.name}，兵力 ${force(troops)}${ruler ? "，君主" : ""}`}
                    >
                      <use
                        href={`#${uid}-army`}
                        x="-28"
                        y="-13"
                        width="20"
                        height="20"
                        style={{ color: color(actual.owner) }}
                      />
                      <text
                        data-cq-troops={p.id}
                        x="-4"
                        y="4"
                        textAnchor="start"
                      >
                        {force(troops)}
                      </text>
                      {ruler && (
                        <Crown
                          className="cq-ruler"
                          data-cq-crown={p.id}
                          x={-11}
                          y={-31}
                          width={24}
                          height={21}
                        />
                      )}
                      {actual.fort && (
                        <Castle
                          data-cq-fort={p.id}
                          x={12}
                          y={-27}
                          width={16}
                          height={16}
                          strokeWidth={1.8}
                          className="cq-flat-fort"
                          aria-hidden="true"
                        />
                      )}
                    </g>
                  </g>
                );
              })}
            </g>
          </svg>
        </div>
        {selectedGeo && selectedState && (
          <div className="cq-land-info" aria-live="polite">
            <strong>{selectedGeo.name}</strong>
            <span>
              {ownerName(selectedState.owner)} ·{" "}
              {own && data.phase === "planning" ? "可用 " : "兵力 "}
              {force(
                own && data.phase === "planning"
                  ? (selectedPlan?.mobile ?? selectedState.troops)
                  : selectedState.troops,
              )}
              {selectedState.fort ? " · 筑有要塞" : ""}
            </span>
            {own && selectedPlan && (
              <span data-cq-population={selectedPlan.population}>
                人口 {number(selectedPlan.population)} · 可征{" "}
                {number(fullRecruit)}
              </span>
            )}
            {recruitReason && (
              <span className="cq-recruit-reason" data-cq-recruit-reason>
                {recruitReason}
              </span>
            )}
          </div>
        )}
        <div className="cq-map-zoom" aria-label="地图缩放">
          <button
            aria-label="放大地图"
            title="放大"
            disabled={camera.ratio >= camera.maxRatio - 0.01}
            onClick={() => camera.zoom(1.45)}
          >
            <Plus size={19} />
          </button>
          <button
            aria-label="缩小地图"
            title="缩小"
            disabled={camera.ratio <= 1.001}
            onClick={() => camera.zoom(1 / 1.45)}
          >
            <Minus size={19} />
          </button>
          <button aria-label="复位地图" title="全图" onClick={camera.reset}>
            <RotateCcw size={17} />
          </button>
        </div>
        <svg
          data-camera-control
          className="cq-minimap"
          viewBox={`0 0 ${map.width} ${map.height}`}
          preserveAspectRatio="none"
          aria-label="拖动小地图定位"
          onPointerDown={(e) => {
            e.currentTarget.setPointerCapture(e.pointerId);
            miniLocate(e);
          }}
          onPointerMove={(e) => {
            if (e.currentTarget.hasPointerCapture(e.pointerId)) miniLocate(e);
          }}
          onPointerUp={miniRelease}
          onPointerCancel={miniRelease}
        >
          <rect width={map.width} height={map.height} fill="#92b6b9" />
          {map.provinces.map((p) => (
            <path
              key={p.id}
              d={p.path}
              fill={color(provinces.get(p.id)?.owner ?? -1)}
              fillRule="evenodd"
            />
          ))}
          <rect
            ref={camera.miniViewportRef}
            x={visibleMap.x}
            y={visibleMap.y}
            width={visibleMap.width}
            height={visibleMap.height}
            fill="none"
            stroke="#fff4cf"
            strokeWidth={map.width / 70}
          />
        </svg>
        {selectedGeo && own && data.phase !== "finished" && (
          <div
            className={`cq-territory-controls ${amountOpen ? "amount-open" : ""}`}
            data-camera-control
          >
            {amountOpen && (
              <TroopAmount
                mode={recruiting ? "recruit" : "march"}
                maximum={
                  recruiting
                    ? fullRecruit
                    : Math.max(0, selectedPlan?.mobile ?? 0)
                }
                requested={requested}
                enabled={
                  canPlan &&
                  (recruiting ||
                    data.availableMovement >= data.controls.moveCost)
                }
                onChange={setRequested}
                onCancel={cancelAmount}
                onConfirm={() => {
                  if (canPlan && recruit > 0)
                    void send("recruit", {
                      province: selected,
                      amount: recruit,
                    });
                }}
              />
            )}
            <div
              className="cq-territory-actions"
              role="toolbar"
              aria-label="领地行动"
            >
              <button
                data-cq-action="recruit"
                aria-label="征兵"
                title="选择征兵人数"
                aria-pressed={recruiting}
                className={recruiting ? "active" : ""}
                disabled={!canPlan}
                onClick={() => {
                  setRecruiting(!recruiting);
                  setMarching(false);
                  setRequested(null);
                  setTarget("");
                  setMarchQueue([]);
                }}
              >
                <UserPlus />
                <span>征兵</span>
              </button>
              <button
                data-cq-action={marching ? "cancel" : "attack"}
                aria-label={marching ? "取消进攻" : "进攻"}
                title={marching ? "取消派兵选择" : "选择派兵人数，再点相邻领地"}
                className={marching ? "active" : ""}
                disabled={
                  !marching &&
                  (!canPlan ||
                    (selectedPlan?.mobile ?? 0) <= 0 ||
                    data.availableMovement < data.controls.moveCost)
                }
                onClick={() => {
                  setMarching(!marching);
                  setRecruiting(false);
                  setRequested(null);
                  setMarchQueue([]);
                  setTarget("");
                }}
              >
                {marching ? <X /> : <Swords />}
                <span>{marching ? "取消" : "进攻"}</span>
              </button>
              <button
                data-cq-action="fort"
                aria-label={building === "town" ? "建城镇" : "筑堡"}
                title={`${building === "town" ? "建城镇" : "筑堡"} · ${number(buildCost)} 金币；长按直接建城镇（Shift+Enter）`}
                aria-keyshortcuts="Shift+Enter"
                data-holding={fortPress.holding}
                disabled={!canBuild}
                {...fortPress.handlers}
              >
                <Castle />
                <span>{building === "town" ? "城镇" : "筑堡"}</span>
              </button>
              {me?.king === selected && (
                <button
                  data-cq-action="king"
                  aria-label="君主随军"
                  aria-pressed={king}
                  className={king ? "active" : ""}
                  disabled={
                    !canPlan || plan.some((o) => o.type === "move" && o.king)
                  }
                  title="君主随下一支军队移动"
                  onClick={() => setKing(!king)}
                >
                  <Crown />
                  <span>君主</span>
                </button>
              )}
              {data.options.fog && (
                <button
                  data-cq-action="tower"
                  aria-label="建瞭望塔"
                  title={`瞭望塔 · ${number(data.controls.towerCost)} 金币`}
                  disabled={!canTower}
                  onClick={() =>
                    void send("build", {
                      province: selected,
                      building: "tower",
                    })
                  }
                >
                  <Binoculars />
                  <span>瞭望</span>
                </button>
              )}
              <button
                data-cq-action="disband"
                aria-label="解散部队"
                title="将一半可用部队遣返回民间"
                disabled={
                  !canPlan ||
                  (selectedPlan?.mobile ?? 0) < 1 ||
                  data.availableMovement < 0.1
                }
                onClick={() =>
                  void send("disband", {
                    province: selected,
                    amount: Math.ceil((selectedPlan?.mobile ?? 0) / 2),
                  })
                }
              >
                <ShieldMinus />
                <span>解散</span>
              </button>
            </div>
          </div>
        )}
        <div
          className={`cq-map-message ${notice || status ? "visible" : ""}`}
          role="status"
          aria-live="polite"
        >
          {notice || status}
        </div>
      </div>
      <footer className="cq-round-controls">
        <button
          aria-label="查看行动计划"
          data-cq-action="plans"
          onClick={() => setPlansOpen(true)}
        >
          <ScrollText size={18} />
          <span className="cq-plan-caption">
            <span>
              计划 <b>{plan.length}</b>
            </span>
            {!room.spectating && (
              <small
                data-cq-action-points
                aria-label={`本轮行动点：已用 ${movementUsed}，共 ${movementTotal}`}
                title={`本轮行动点：已用 ${movementUsed}，共 ${movementTotal}`}
              >
                {movementUsed}/{movementTotal}
              </small>
            )}
          </span>
        </button>
        <button
          aria-label={data.phase === "paused" ? "恢复对局" : "提议暂停"}
          disabled={
            !controlled ||
            busy ||
            pending ||
            !(data.controls.canPause || data.controls.canResume)
          }
          aria-pressed={
            data.phase === "paused"
              ? !!data.resumeVotes[room.you]
              : !!data.pauseVotes[room.you]
          }
          onClick={() =>
            void send(data.phase === "paused" ? "resume" : "pause", {
              enabled:
                data.phase === "paused"
                  ? !data.resumeVotes[room.you]
                  : !data.pauseVotes[room.you],
            })
          }
        >
          {data.phase === "paused" ? <Play size={18} /> : <Pause size={18} />}
        </button>
        <button
          className="cq-submit"
          aria-label={me?.submitted ? "撤回提交" : "提交回合"}
          disabled={
            !controlled ||
            busy ||
            pending ||
            marchQueue.length > 0 ||
            !(data.controls.canSubmit || data.controls.canUnsubmit)
          }
          onClick={() => void send(me?.submitted ? "unsubmit" : "submit")}
        >
          {me?.submitted ? <RotateCcw size={18} /> : <Check size={19} />}
          <span>
            {data.phase === "resolving"
              ? "行动中"
              : data.phase === "paused"
                ? "已暂停"
                : data.phase === "finished"
                  ? "已结束"
                  : me?.submitted
                    ? "撤回"
                    : "完成本轮"}
          </span>
        </button>
      </footer>
      {plansOpen && (
        <Modal title="本轮计划" onClose={() => setPlansOpen(false)}>
          <div className="cq-plan-sheet">
            {plan.length ? (
              plan.map((o) => (
                <div key={o.id}>
                  <span>{orderLabel(o)}</span>
                  <button
                    aria-label={`撤销计划 ${o.id}`}
                    disabled={!canPlan}
                    onClick={() => void send("undo", { id: o.id })}
                  >
                    <Trash2 size={18} />
                  </button>
                </div>
              ))
            ) : (
              <p>地图上的行动会列在这里。</p>
            )}
          </div>
          <button
            className="cq-text-button cq-resign"
            disabled={!controlled || !data.controls.canResign}
            onClick={() => {
              setPlansOpen(false);
              setResignOpen(true);
            }}
          >
            <Flag size={16} />
            认输
          </button>
        </Modal>
      )}
      {financeOpen && (
        <Modal title="国政" onClose={() => setFinanceOpen(false)}>
          <div className="cq-finance">
            <dl>
              <div>
                <dt>预计税收</dt>
                <dd>{number(me?.income ?? 0)}</dd>
              </div>
              <div>
                <dt>军费</dt>
                <dd>{number(me?.upkeep ?? 0)}</dd>
              </div>
              <div>
                <dt>民心</dt>
                <dd>{me?.morale ?? 0}%</dd>
              </div>
            </dl>
            <label>
              税率 {tax}%
              <input
                aria-label="税率"
                type="range"
                min="0"
                max="100"
                step="10"
                value={tax}
                disabled={!canPlan}
                onChange={(e) => setTax(Number(e.target.value))}
              />
            </label>
            <button
              disabled={!canPlan || tax === me?.tax}
              onClick={async () => {
                if (await send("tax", { value: tax })) setFinanceOpen(false);
              }}
            >
              调整税率
            </button>
          </div>
        </Modal>
      )}
      {resignOpen && (
        <Modal title="退出争夺" onClose={() => setResignOpen(false)}>
          <p>认输后领地归为中立，仍可观战。</p>
          <button
            disabled={!controlled || pending || busy}
            onClick={async () => {
              if (await send("resign")) setResignOpen(false);
            }}
          >
            确认认输
          </button>
        </Modal>
      )}
    </section>
  );
}
