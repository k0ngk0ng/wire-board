import type { CSSProperties } from "react";
import { Minus, Plus } from "lucide-react";
import { catanCardNames, catanCardColors, catanCardImage } from "./catan-cards";
const total = (a: number[]) => a.reduce((n, x) => n + x, 0);

export function CatanResource({
  color,
  count,
  assets = "",
  small = false,
}: {
  color: number;
  count?: number;
  assets?: string;
  small?: boolean;
}) {
  return (
    <span
      role="img"
      aria-label={`${catanCardNames[color]}${count === undefined ? "" : ` ${count}`}`}
      className={`catan-resource ${small ? "small" : ""}`}
      style={{ "--resource-color": catanCardColors[color] } as CSSProperties}
    >
      <span className="catan-resource-art">
        {assets ? (
          <img src={catanCardImage(assets, color, small)} alt="" />
        ) : (
          <span>{["♣", "▰", "♧", "❧", "◆", "▤", "▱", "●"][color]}</span>
        )}
      </span>
      <span>{catanCardNames[color]}</span>
      {count !== undefined && <b>{count}</b>}
    </span>
  );
}
export function Bundle({
  values,
  assets,
  showZero = false,
}: {
  values: number[];
  assets: string;
  showZero?: boolean;
}) {
  return (
    <div className="catan-bundle">
      {values.map(
        (n, c) =>
          (n > 0 || showZero) && (
            <CatanResource key={c} color={c} count={n} assets={assets} small />
          ),
      )}
    </div>
  );
}
export function ResourcePicker({
  label,
  values,
  onChange,
  limits,
  assets,
  disabled,
}: {
  label: string;
  values: number[];
  onChange: (n: number[]) => void;
  limits: number[];
  assets: string;
  disabled: boolean;
}) {
  return (
    <fieldset className="catan-picker" disabled={disabled}>
      <legend>
        {label} <b>{total(values)}</b>
      </legend>
      {values.map((n, c) => (
        <div key={c}>
          <CatanResource color={c} assets={assets} small />
          <div className="catan-stepper">
            <button
              type="button"
              aria-label={`${label}减少${catanCardNames[c]}`}
              disabled={n === 0}
              onClick={() =>
                onChange(values.map((x, i) => (i === c ? x - 1 : x)))
              }
            >
              <Minus size={13} />
            </button>
            <output>{n}</output>
            <button
              type="button"
              aria-label={`${label}增加${catanCardNames[c]}`}
              disabled={n >= limits[c]}
              onClick={() =>
                onChange(values.map((x, i) => (i === c ? x + 1 : x)))
              }
            >
              <Plus size={13} />
            </button>
          </div>
        </div>
      ))}
    </fieldset>
  );
}
