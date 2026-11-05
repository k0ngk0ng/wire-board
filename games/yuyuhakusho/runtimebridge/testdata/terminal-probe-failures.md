# 终局预览证据采集范围

先于临时 probe 编写，不修改规则、呈现状态或终局语义。

- 49 条冻结轨迹中，完整终局与固定 pan 非终局混淆；以非终局 F+1 的正常像素差异宣称终局恢复错误。
- 终止 tick 的 manager `0x8040` 阶段、`clear-world`、draw／arena-sprite 事件数量或有效对象数未采集，仅由提前返回空事件推断丢失角色。
- 重放输入／场地／角色不对应冻结记录，未核验最终 world 和完整 pixel trace 摘要。
- completed checkpoint 预览修改权威状态、误标 F+1，或未真正经 `RestoreCheckpoint` 重建会话。
- 实际像素有差异却只记录推断；不保存首个差异的 checkpoint、前一帧状态、事件、原始 RGBA、PNG 与 SHA。

若没有终局像素差异，同样保留真实终局样例与逐项比较记录，并纠正先前的条件性推断。
