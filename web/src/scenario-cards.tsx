import {
  Children,
  isValidElement,
  useId,
  useState,
  type ReactNode,
} from "react";

// Read the existing option definitions so availability and names have one source.
export function ScenarioCards({
  label,
  value,
  onChange,
  disabled,
  children,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  children: ReactNode;
}) {
  const [expanded, setExpanded] = useState(false);
  const id = useId();
  const options = Children.toArray(children).flatMap((child) =>
    isValidElement<{ value: string; disabled?: boolean; children: ReactNode }>(
      child,
    )
      ? [{ ...child.props }]
      : [],
  );
  const common = [
    "",
    "cities-knights",
    "shores",
    "rivers",
    "caravans",
    "fishing",
  ];
  const ordered = [...options].sort((a, b) => {
    const rank = (key: string) =>
      common.includes(key) ? common.indexOf(key) : common.length;
    return rank(a.value) - rank(b.value);
  });
  const shortlist = ordered.slice(0, 6);
  if (!shortlist.some((o) => o.value === value)) {
    const selected = ordered.find((o) => o.value === value);
    if (selected) shortlist.splice(5, 1, selected);
  }
  return (
    <fieldset className="scenario-card-picker" disabled={disabled}>
      <legend>{label}</legend>
      <div
        id={id}
        className={`scenario-card-grid ${expanded ? "is-expanded" : ""}`}
      >
        {(expanded ? ordered : shortlist).map((option) => (
          <button
            key={option.value}
            type="button"
            data-scenario-value={option.value}
            aria-pressed={value === option.value}
            disabled={option.disabled}
            onClick={() => onChange(option.value)}
          >
            {option.children}
          </button>
        ))}
      </div>
      {ordered.length > 6 && (
        <button
          type="button"
          className="scenario-more"
          aria-expanded={expanded}
          aria-controls={id}
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? "收起剧本" : `查看全部 ${ordered.length} 种剧本`}
        </button>
      )}
    </fieldset>
  );
}
