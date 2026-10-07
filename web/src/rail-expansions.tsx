import type { RailMapSpec, RailPayment, Route } from "./types";
import "./rail-expansions.css";

export const railMapNames: Record<string, string> = {
  usa: "美国",
  europe: "欧洲",
  india: "印度",
  switzerland: "瑞士",
  nordiccountries: "北欧",
  legendaryasia: "传奇亚洲",
};
export const routePoints = (r: Route) =>
  [0, 1, 2, 4, 7, 10, 15, 18, 21, 27][r.length] + 2 * (r.mountain || 0);
export const railColors = [
  "#a35e91",
  "#eee6d1",
  "#357bac",
  "#e3bc38",
  "#dd8540",
  "#39434a",
  "#c84b43",
  "#568757",
  "#e4ba55",
];
export const railColorNames = [
  "紫",
  "白",
  "蓝",
  "黄",
  "橙",
  "黑",
  "红",
  "绿",
  "万能",
];
export function paymentText(pay: number[]) {
  return pay
    .map((n, c) => (n ? `${railColorNames[c]}×${n}` : ""))
    .filter(Boolean)
    .join("、");
}
export function railPaymentValid(
  r: Route,
  info: RailMapSpec,
  hand: number[],
  color: number,
  pay: number[],
) {
  if (
    color < 0 ||
    color > 7 ||
    (r.color >= 0 && r.color !== color) ||
    pay.length !== 9 ||
    pay.some((n, c) => !Number.isInteger(n) || n < 0 || n > hand[c])
  )
    return false;
  const total = pay.reduce((a, b) => a + b, 0);
  if (r.substitute === 4)
    return Array.from({ length: r.length + 1 }, (_, n) => n).some(
      (n) => total === r.length + 3 * n && pay[color] >= r.length - n,
    );
  if (r.substitute === 3 && r.ferry)
    return Array.from({ length: r.ferry + 1 }, (_, n) => n).some(
      (n) =>
        total === r.length + 2 * n &&
        pay[8] >= r.ferry! - n &&
        pay[color] + pay[8] >= r.length - n,
    );
  const wildAllowed =
    info.wildRule === "any" ||
    r.tunnel ||
    (info.wildRule === "special" && !!r.ferry);
  return (
    total === r.length &&
    pay[color] + pay[8] === total &&
    pay[8] >= (r.ferry || 0) &&
    (pay[8] === 0 || wildAllowed)
  );
}
export function RailPaymentEditor({
  hand,
  value,
  onChange,
  suggestions = [],
}: {
  hand: number[];
  value: RailPayment;
  onChange: (p: RailPayment) => void;
  suggestions?: RailPayment[];
}) {
  return (
    <div className="rail-payment-editor">
      {suggestions.length > 0 && (
        <label>
          建议组合
          <select
            aria-label="建议支付组合"
            value=""
            onChange={(e) => {
              const i = Number(e.target.value);
              onChange({
                color: suggestions[i].color,
                tokens: [...suggestions[i].tokens],
              });
            }}
          >
            <option value="" disabled>
              选择组合，或在下方调整
            </option>
            {suggestions.map((p, i) => (
              <option key={i} value={i}>
                {railColorNames[p.color]}为主 · {paymentText(p.tokens)}
              </option>
            ))}
          </select>
        </label>
      )}
      <div className="rail-payment-cards">
        {railColorNames.map((name, c) => (
          <label key={c}>
            <span>
              <i style={{ background: railColors[c] }} />
              {name}
              <small>持有 {hand[c]}</small>
            </span>
            <input
              aria-label={`支付${name}牌`}
              type="number"
              min={0}
              max={hand[c]}
              value={value.tokens[c]}
              onChange={(e) => {
                const tokens = [...value.tokens];
                tokens[c] = Math.max(
                  0,
                  Math.min(hand[c], Number(e.target.value) || 0),
                );
                onChange({ ...value, tokens });
              }}
            />
          </label>
        ))}
      </div>
      <p className="muted small">
        已选 {value.tokens.reduce((a, b) => a + b, 0)} 张：
        {paymentText(value.tokens) || "尚未选择"}
      </p>
    </div>
  );
}
export function RailMapPicker({
  maps,
  value,
  assets,
  disabled,
  onChange,
}: {
  maps: RailMapSpec[];
  value: string;
  assets: string;
  disabled?: boolean;
  onChange: (id: string) => void;
}) {
  const selected = maps.find((m) => m.id === value);
  return (
    <section className="rail-map-picker" aria-label="铁路地图选择">
      <div className="rail-map-options">
        {maps.map((m) => (
          <button
            type="button"
            key={m.id}
            disabled={disabled}
            aria-pressed={value === m.id}
            className={value === m.id ? "selected" : ""}
            onClick={() => onChange(m.id)}
          >
            {assets && (
              <img
                src={`${assets}/${m.art}/${m.id === "usa" ? "map-unlabeled-v4.webp" : "preview.webp"}`}
                alt=""
              />
            )}
            <span>
              <strong>{m.name}</strong>
              <small>
                {m.minPlayers}–{m.maxPlayers} 人
              </small>
            </span>
          </button>
        ))}
      </div>
      {selected && (
        <div className="rail-map-summary">
          <strong>
            {selected.name} · {selected.trains} 节车厢
          </strong>
          <ul>
            {selected.rules.map((rule) => (
              <li key={rule}>{rule}</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
export function RailMapRules({ map }: { map: RailMapSpec }) {
  return (
    <>
      <p>
        {map.name}地图，{map.minPlayers}–{map.maxPlayers} 人。每人四张列车牌、
        {map.trains} 节车厢；同时从 {map.setupTickets}{" "}
        张初始目的地中至少保留两张，120 秒后由电脑选牌并开启托管。
      </p>
      <ul>
        {map.rules.map((rule) => (
          <li key={rule}>{rule}</li>
        ))}
      </ul>
      <ol>
        <li>
          每回合选择：摸两张列车牌、占领一条路线、或抽取最多三张目的地并至少保留一张。
          {map.stations > 0 && "也可以建造一座车站。"}
        </li>
        <li>
          {map.wildSingle
            ? "公开万能牌计一次摸牌，可以连续拿两张。"
            : "拿公开万能牌占用整个行动；第二次不能拿公开万能牌。"}
          暗抽万能牌均计一次摸牌。公开市场出现至少三张万能牌时会自动换牌。
        </li>
        <li>
          同一玩家不能占领双线；至少 {map.doubleMin}{" "}
          人时才开放双线的两条。灰线可选任意一种颜色；特殊支付以本地图规则为准。
        </li>
        <li>
          某玩家回合结束时剩余车厢不超过两节，每人（包括触发者）再行动一次。完成目的地加分，未完成扣分。路线长
          1–9 节分别计 1、2、4、7、10、15、18、21、27 分。
        </li>
        {map.stations > 0 && (
          <li>
            第 1/2/3 座车站各花费 1/2/3
            张同色牌，可用万能牌补足；每城最多一座。每座借用一条相邻对手路线，所有任务共用同一借路方案，不计入最长路线。系统按任务总得分最高的方案自动结算；仅本人可查看借路预览。
          </li>
        )}
        <li>
          同分先比较完成任务数量
          {map.id === "europe"
            ? "，再比较未建车站数量，最后比较最长路线奖励"
            : map.id === "legendaryasia"
              ? "，再比较已占领的山地路线数量"
              : map.bonus === "longest"
                ? "，再比较最长路线奖励"
                : ""}
          ；仍相同则共同获胜。
        </li>
      </ol>
    </>
  );
}
