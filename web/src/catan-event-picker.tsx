export function CatanEventPicker({
  value,
  disabled = false,
  citiesKnights = false,
  cloth = false,
  onChange,
}: {
  value: boolean;
  disabled?: boolean;
  citiesKnights?: boolean;
  cloth?: boolean;
  onChange: (enabled: boolean) => void;
}) {
  return (
    <fieldset className="catan-helper-options" disabled={disabled}>
      <label>
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
        />
        事件牌 · 本站牌组
      </label>
      <p className="muted small">
        用抽牌代替生产骰，先完成事件，再按牌面生产。36张普通牌与新年重洗；双人每回合抽两张，五六人仅①号抽牌。
      </p>
      {citiesKnights && (
        <p className="muted small">
          城市骑士先执行事件文字，再结算独立红骰／事件骰，最后生产。炼金术替代抽牌；本站补充规则：贸易优势可随机偷资源或商品。
        </p>
      )}
      {cloth && (
        <p className="muted small">
          本站补充规则：强盗逃跑只进入布匹地图的大岛沙漠，没有合法沙漠则移到场外，不偷牌、不移动海盗。
        </p>
      )}
      <details>
        <summary>牌组说明</summary>
        <p className="muted small">
          本站采用已交叉核对的旧版点数与事件配比，按2025事件效果执行；尚未核实与2025实体牌表完全相同。关闭后仍用原来的骰子。
        </p>
      </details>
    </fieldset>
  );
}
