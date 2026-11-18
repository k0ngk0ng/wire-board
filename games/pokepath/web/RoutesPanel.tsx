import { useEffect, useRef, useState, type CSSProperties } from "react";
import { useFact, useLanguage, useLocalize } from "./I18n";
import { ResourceImage, type Panel } from "./Panels";
import { useResourceURL } from "./resource-url";
import type { AssetStore } from "./assets";
import type { Action, Assets, RouteMeta, View } from "./types";

type RouteProps = { view: View; assets: Assets; store?: AssetStore; disabled: boolean; onAct: (action: Action) => void; onClose: () => void; onPanel?: (panel: Panel) => void };
const secretAwards: Record<number, string> = { 20: "phione", 30: "mew", 31: "victini", 32: "marshadow", 33: "farfetchdGalar", 35: "calyrex", 46: "spectrier" };

function RouteTile({ route, store, name, current, completed, locked, active, disabled, position, variant, star, record, ribbons, onSelect }: { route: RouteMeta; store?: AssetStore; name: string; current: boolean; completed: boolean; locked: boolean; active: boolean; disabled: boolean; position?: CSSProperties; variant: "world" | "group" | "secret"; star: string; record: string | number; ribbons: number; onSelect: () => void }) {
  const button = useRef<HTMLButtonElement>(null);
  const [visible, setVisible] = useState(variant !== "group");
  const cached = Boolean(store?.images.has(store.path(route.background)));
  const image = useResourceURL(visible || cached ? route.background : undefined, store);
  useEffect(() => {
    if (!button.current || visible || cached) return;
    if (typeof IntersectionObserver === "undefined") { setVisible(true); return; }
    const observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { setVisible(true); observer.disconnect(); } }, { rootMargin: "90px" });
    observer.observe(button.current);
    return () => observer.disconnect();
  }, [visible, cached]);
  return <button ref={button} type="button" data-route-id={route.id} className={`pokepath-route-tile ${variant}${current ? " selected" : completed ? " completed" : ""}${locked ? " locked" : active ? " active" : ""}${Array.isArray(route.position) ? " wide" : ""}`} style={{ ...position, backgroundImage: image ? `url("${image}")` : undefined }} disabled={disabled || locked || active || current} aria-label={locked ? `${name}，需要${route.unlockStars}星` : name} onClick={onSelect}><span className="pokepath-route-tile-name" data-i18n-literal>{name.toUpperCase()}</span><span className="pokepath-route-record-box"><span className="pokepath-route-record"><span className="pokepath-route-star">{star}</span>{variant === "group" && !route.isSecret ? `${record} ` : record}{variant === "group" && !route.isSecret && <span className="pokepath-route-ribbons">{store && <ResourceImage path="src/assets/images/icons/ribbon1.png" store={store} />}<span>{`${ribbons}/10`}</span></span>}</span></span>{locked && <span className="pokepath-route-requires"><span className="pokepath-route-star">{star}</span>{route.unlockStars}</span>}</button>;
}

export function RoutesPanel({ view, assets, store, disabled, onAct, onPanel, onClose }: RouteProps) {
  const localize = useLocalize();
  const fact = useFact();
  const language = useLanguage();
  const [list, setList] = useState(false);
  const routes = Object.values(assets.catalog?.routes ?? {}).sort((first, second) => (first.order ?? first.id) - (second.order ?? second.id));
  const discovered = (route: RouteMeta) => Boolean(view.progression?.secretMaps?.[String(route.id)] || route.secretKey && view.progression?.secretMaps?.[route.secretKey]);
  const nameFor = (route: RouteMeta) => route.names?.[language] || route.names?.[0] || route.name;
  const maps = routes.filter(route => !route.isSecret);
  const mirage = routes.find(route => route.id === 30 && discovered(route));
  const worldMaps = [...maps, ...(mirage ? [mirage] : [])].filter(route => Array.isArray(route.position) || (route.position ?? -1) >= 0);
  const anchors = new Map(worldMaps.map(route => [Array.isArray(route.position) ? route.position[0] : route.position, route]));
  const reserved = new Set(worldMaps.flatMap(route => Array.isArray(route.position) ? route.position.slice(1) : []));
  const secrets = routes.filter(route => route.isSecret && discovered(route));
  const editor = view.items.some(item => item.id === "revysBook");
  const star = view.progression?.settings.theme === "halloween" ? "🎃" : "⭐";
  const tile = (route: RouteMeta, variant: "world" | "group" | "secret") => {
    let position: CSSProperties | undefined;
    if (variant === "world") {
      const positions = Array.isArray(route.position) ? route.position : [route.position ?? 0];
      const anchor = positions[0];
      const col = anchor % 9;
      const row = Math.floor(anchor / 9);
      const columns = Math.max(...positions.map(position => position % 9)) - col + 1;
      const rows = Math.floor(Math.max(...positions) / 9) - row + 1;
      position = { gridColumn: `${col + 1} / span ${columns}`, gridRow: `${row + 1} / span ${rows}` };
    }
    const record = view.records?.[String(route.id)] ?? 0;
    const ribbons = Object.values(view.progression?.ribbons?.[String(route.id)] ?? {}).reduce((sum, value) => sum + value, 0);
    return <RouteTile key={route.id} route={route} store={store} name={nameFor(route)} current={route.id === view.route} completed={record >= 100 || Boolean(view.progression?.secrets?.[secretAwards[route.id]])} locked={view.stars < route.unlockStars} active={view.waveActive} disabled={disabled} position={position} variant={variant} star={star} record={route.isSecret ? "???" : Math.min(100, record)} ribbons={ribbons} onSelect={() => { onAct({ type: "route", route: route.id }); onClose(); }} />;
  };
  const group = (title: string, routes: RouteMeta[], secret = false) => <section key={secret ? "secret" : routes[0].id} className={`pokepath-route-group${secret ? " secret" : ""}`}><h2 className="pokepath-route-group-title" data-i18n-literal>{title.toUpperCase()}</h2><div>{routes.map(route => tile(route, "group"))}</div></section>;
  return localize(<section className="pokepath-central-panel pokepath-classic-routes" data-pokepath-font={view.progression?.settings.font} data-pokepath-theme={view.progression?.settings.theme} role="dialog" aria-label="路线">{list ? <div key="group" className="pokepath-route-list">{Array.from({ length: Math.ceil(maps.length / 4) }, (_, index) => { const routes = maps.slice(index * 4, index * 4 + 4); return group(nameFor(routes[0]).replace(/-\d+$/, ""), routes); })}{(secrets.length > 0 || routes.some(route => route.id === 30)) && group(fact("text.map.secretRoutes") || "秘密路线", secrets, true)}</div> : <><div key="world" className="pokepath-world-grid">{Array.from({ length: 54 }, (_, position) => { const route = anchors.get(position); return route ? tile(route, "world") : !reserved.has(position) && <div key={`empty-${position}`} data-route-placeholder={position} style={{ gridColumn: position % 9 + 1, gridRow: Math.floor(position / 9) + 1 }}>?</div>; })}</div>{secrets.some(route => route.id !== 30) && <div className="pokepath-secret-routes">{secrets.filter(route => route.id !== 30).map(route => tile(route, "secret"))}</div>}</>}<nav><button type="button" onClick={() => setList(!list)} data-i18n-literal><span>{(fact(list ? "text.map.globalView" : "text.map.routeView") || (list ? "全局视图" : "路线视图")).toUpperCase()}</span></button>{editor && <button type="button" disabled={disabled || view.waveActive} onClick={() => onPanel?.("editor")} data-i18n-literal><span>{(fact("text.editable.title") || "波次创建器").toUpperCase()}</span></button>}</nav><button type="button" data-controller-cancel className="pokepath-central-close" aria-label="关闭路线" onClick={onClose}>×</button></section>);
}
