import type { SGOptions } from "./types";

export function SanguoshaOptions({
  value,
  onChange,
  disabled = false,
}: {
  value?: SGOptions;
  onChange: (value: SGOptions) => void;
  disabled?: boolean;
}) {
  const deck = value?.deck || "standard";
  return (
    <fieldset className="sg-options" disabled={disabled}>
      <legend>三国杀规则</legend>
      <div className="sg-deck-options">
        {(
          [
            { id: "classic", name: "经典标准将", detail: "25 将 · 原版技能" },
            {
              id: "breakthrough",
              name: "界限突破标准将",
              detail: "21 位界将＋4 位未修订标准将",
            },
          ] as const
        ).map((item) => (
          <button
            key={item.id}
            type="button"
            className={
              (value?.standardVersion || "classic") === item.id
                ? "selected"
                : ""
            }
            aria-pressed={(value?.standardVersion || "classic") === item.id}
            onClick={() => onChange({ ...value, standardVersion: item.id })}
          >
            <strong>{item.name}</strong>
            <small>{item.detail}</small>
          </button>
        ))}
      </div>
      <div className="sg-deck-options">
        {[
          {
            id: "standard",
            name: "经典标准",
            detail: "108 张牌 · 熟悉的身份博弈",
          },
          {
            id: "military",
            name: "标准＋军争",
            detail: "160 张牌 · 属性杀、酒、铁索与新装备",
          },
        ].map((item) => (
          <button
            key={item.id}
            type="button"
            className={deck === item.id ? "selected" : ""}
            aria-pressed={deck === item.id}
            onClick={() =>
              onChange({
                ...value,
                mode: "identity",
                deck: item.id,
                packs: value?.packs || ["standard"],
              })
            }
          >
            <strong>{item.name}</strong>
            <small>{item.detail}</small>
          </button>
        ))}
      </div>
      {[
        { id: "wind", label: "风包 · 经典八将" },
        { id: "fire", label: "火包 · 经典八将" },
        { id: "thicket", label: "林包 · 经典八将" },
        { id: "mountain", label: "山包 · 经典八将" },
        { id: "god", label: "神将 · 经典八将" },
      ].map((pack) => (
        <label className="sg-pack-option" key={pack.id}>
          <input
            type="checkbox"
            checked={value?.packs?.includes(pack.id) || false}
            onChange={(e) =>
              onChange({
                ...value,
                packs: e.target.checked
                  ? [...(value?.packs || ["standard"]), pack.id]
                  : (value?.packs || ["standard"]).filter(
                      (id) => id !== pack.id,
                    ),
              })
            }
          />
          {pack.label}
        </label>
      ))}
      <p>
        {value?.standardVersion === "breakthrough"
          ? "采用旧版界限突破技能；同一人物的经典与界版不会同时入选。"
          : "标准武将采用经典技能版本。"}
        扩展包可独立组合。
        {deck === "military" &&
          "军争包含火焰与雷电伤害，铁索会将属性伤害传给其他横置角色。"}
      </p>
    </fieldset>
  );
}
