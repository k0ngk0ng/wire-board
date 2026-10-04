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
      <label className="sg-pack-option">
        <input
          type="checkbox"
          checked={value?.packs?.includes("wind") || false}
          onChange={(e) =>
            onChange({
              ...value,
              packs: e.target.checked ? ["standard", "wind"] : ["standard"],
            })
          }
        />
        风包 · 经典八将
      </label>
      <p>
        武将采用经典技能版本。
        {deck === "military" &&
          "军争包含火焰与雷电伤害，铁索会将属性伤害传给其他横置角色。"}
      </p>
    </fieldset>
  );
}
