# 来源 7 子对象失败场景与恢复边界

本清单先于 `fighter7_children.go` 与冻结测试编写。范围为父体真实创建的 `136e6`、`1391e`、`15094`，以及由其正文和原 CPU 证明确认可达的派生入口。静态记录不代表已执行或已通过验收。

## 失败场景

1. 三个入口识别错误、错误席位／对象 ID、初始化与后续 stage 混用，或未知 callback 静默通过。
2. 把 `+10/+14/+4c` 的完整 long、`+48` 的上下 word／byte、`+69` 的命中停顿，误建为第二份持久状态；未经正文证明添加 `+fe` owner。
3. 创建时已有脏槽、创建失败、第三子槽独立占用，或初始化错误覆盖未拥有字段；Size 为零／非整 16／越界时清除范围处理错误。
4. 暂停、慢速门槛与初始化顺序错误；动画 index／duration／ticks／flags／completed 的 byte 溢出、零值和结束边界错误。
5. 方向 bit 与完整方向 byte 混用；带方向的速度再次镜像；坐标 long 小数、signed word 比较、整数溢出或边界等号错误。
6. 父体输入、位置、动作、生命周期与子对象的关系恢复错误；跨父体、对手或全局写入遗漏、错误过滤 Size／Health，或合并顺序错误。
7. 动画记录、图形缓存、坐标投影、图层／深度／属性更新顺序错误；多次 draw 重读最终对象而丢失各次 draw 当时的值。
8. 碰撞、接触标志、攻击／伤害／效果／目标掩码、副作用或命中停顿恢复错误；仅凭 `HitStopTicks > 0` 调用不可达 slave。
9. 派生 child／FX 分配失败、游标回绕、满池、脏槽遗留、额外 callback 或父子清除时机遗漏。
10. 声音排队或 `37c56/37cac` 清槽遗漏；IRQ 24／32 位入口、真实 return、随机数或未知跨域写被统一遮蔽。
11. 以逐步独立 reset 代替单初态 creator→clock→child→clear；捕获入口／返回 SP 错误、restore 未恢复 refresh cycles 或采样改变自然运行。
12. P1 一家四槽的 codec 越权拥有另三家；原 CPU 真正跨域副作用未进入 typed 数据；对象 clear 后低位字节残留或别名持久化造成世界状态不一致。
13. 缺失、错身份、可变或过宽 oracle／动画数据；native 与 Wasm 字节语义、运行结果、重复执行或来源 SHA 无法核验。

## 固定执行顺序

直接 Ghidra 正文和可达链 → 确定最小 canonical 字段与 typed 采集提案 → 由唯一未观察原版基线派生自然／dirty／creator-clock 单初态到 clear 证据 → 冻结 fixture、先行测试、真实 own-symbol red → 独立 Go 规则 → native／Wasm 验证。原 CPU 基线尚未交付时不执行独立原版自然采样、不写 production。
