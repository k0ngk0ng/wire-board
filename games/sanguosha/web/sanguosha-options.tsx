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
  const hegemony = value?.mode === "hegemony";
  const three = value?.mode === "3v3";
  const threeVariant =
    value?.threeVariant === "extended" ? "extended" : "official";
  const deck = value?.deck || "standard";
  const mode = three ? "3v3" : "identity";
  return (
    <fieldset className="sg-options" disabled={disabled}>
      <legend>三国杀规则</legend>
      <div className="sg-deck-options">
        <button
          type="button"
          className={!hegemony && !three ? "selected" : ""}
          aria-pressed={!hegemony && !three}
          onClick={() =>
            (hegemony || three) &&
            onChange({
              mode: "identity",
              deck: "standard",
              standardVersion: "classic",
              packs: ["standard"],
            })
          }
        >
          <strong>身份局</strong>
          <small>主公、忠臣、反贼、内奸</small>
        </button>
        <button
          type="button"
          className={hegemony ? "selected" : ""}
          aria-pressed={hegemony}
          onClick={() =>
            !hegemony &&
            onChange({
              mode: "hegemony",
              deck: "hegemony",
              packs: ["hegemony"],
            })
          }
        >
          <strong>国战</strong>
          <small>同势力双将 · 暗将与明置</small>
        </button>
        <button
          type="button"
          className={three ? "selected" : ""}
          aria-pressed={three}
          onClick={() =>
            !three &&
            onChange({
              mode: "3v3",
              deck: hegemony ? "standard" : deck,
              standardVersion: value?.standardVersion || "classic",
              packs: hegemony ? ["standard"] : value?.packs || ["standard"],
            })
          }
        >
          <strong>3v3 竞技</strong>
          <small>6 人 · 冷暖双阵营 · 主帅阵亡即判负</small>
        </button>
      </div>
      {three ? (
        <>
          <div className="sg-deck-options">
            {[
              {
                id: "official",
                name: "官方竞技",
                detail: "32 将分 4 堆 · 106 张牌（无闪电）",
              },
              {
                id: "extended",
                name: "本站扩展",
                detail: "沿用所选武将包与牌堆",
              },
            ].map((item) => (
              <button
                key={item.id}
                type="button"
                className={threeVariant === item.id ? "selected" : ""}
                aria-pressed={threeVariant === item.id}
                onClick={() =>
                  item.id === "official"
                    ? onChange({
                        ...value,
                        mode: "3v3",
                        threeVariant: "official",
                        standardVersion: "classic",
                        deck: "standard",
                        packs: ["standard", "wind"],
                      })
                    : onChange({
                        ...value,
                        mode: "3v3",
                        threeVariant: "extended",
                      })
                }
              >
                <strong>{item.name}</strong>
                <small>{item.detail}</small>
              </button>
            ))}
          </div>
          {threeVariant === "extended" && (
            <div className="sg-deck-options">
              {[
                { id: "standard", name: "经典标准", detail: "108 张牌" },
                { id: "military", name: "标准＋军争", detail: "160 张牌" },
              ].map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className={deck === item.id ? "selected" : ""}
                  aria-pressed={deck === item.id}
                  onClick={() =>
                    onChange({ ...value, mode: "3v3", deck: item.id })
                  }
                >
                  <strong>{item.name}</strong>
                  <small>{item.detail}</small>
                </button>
              ))}
            </div>
          )}
          {threeVariant === "extended" && (
            <div className="sg-pack-options">
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
                        mode: "3v3",
                        packs: e.target.checked
                          ? [...(value?.packs || ["standard"]), pack.id]
                          : (value?.packs || ["standard"]).filter(
                              (id) => id !== pack.id,
                            ),
                      })
                    }
                  />
                  <span>
                    <strong>{pack.label.split(" · ")[0]}</strong>
                    <small>{pack.label.split(" · ")[1]}</small>
                  </span>
                </label>
              ))}
            </div>
          )}
          <p>
            旧版竞技 3v3：需要 6 人牌桌。官方配置为 32 名武将（标准 25 ＋ 风
            7，不含于吉与神将）分成 4 堆各 8 名，双方主帅各取一堆，共 16
            名平摊后按 1-2-2-2-2-2-2-2-1 抓将，布阵后同时亮将；牌堆为标准 106
            张（无闪电）。主帅体力上限 +1，第一回合由主帅开始；每轮每人
            行动一次，主帅可指定前锋先后顺序；禁用主公技。击杀任意角色摸三张
            牌（含同阵营）；任意一方主帅阵亡，对方阵营获胜。
          </p>
        </>
      ) : hegemony ? (
        <p>
          基础国战：60名专用武将、108张国战牌。每人七名私有候选，选择两名同势力武将；不混入身份局的武将包与技能版本。
        </p>
      ) : (
        <>
          <div className="sg-deck-options">
            {(
              [
                {
                  id: "classic",
                  name: "经典标准将",
                  detail: "25 将 · 原版技能",
                },
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
                    mode,
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
          <div className="sg-pack-options">
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
                <span>
                  <strong>{pack.label.split(" · ")[0]}</strong>
                  <small>{pack.label.split(" · ")[1]}</small>
                </span>
              </label>
            ))}
          </div>
          <p>
            {value?.standardVersion === "breakthrough"
              ? "采用旧版界限突破技能；同一人物的经典与界版不会同时入选。"
              : "标准武将采用经典技能版本。"}
            扩展包可独立组合。
            {deck === "military" &&
              "军争包含火焰与雷电伤害，铁索会将属性伤害传给其他横置角色。"}
          </p>
        </>
      )}
    </fieldset>
  );
}
