export function CatanEventPicker({
  value,
  disabled = false,
  onChange,
}: {
  value: boolean;
  disabled?: boolean;
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
      <details>
        <summary>牌组说明</summary>
        <p className="muted small">
          本站采用已交叉核对的旧版点数与事件配比，按2025事件效果执行；尚未核实与2025实体牌表完全相同。关闭后仍用原来的骰子。
        </p>
      </details>
    </fieldset>
  );
}
