# 来源 7「树」恢复起点

本页是现有静态资料的只读整理，没有新增原 CPU 执行或 Go 实现。仙水阶段的完整检查仍由独立摘要记录；本页不能用来宣称树已可玩。

命令准备 `17722`、四个 selector `17790/178ea/17960/17a08`。命令及全角色 AI 已有独立目录和验证，不重做。普通前冲 `0f`、后退 `11`，额外地面根 `195dc` 进入 `13/87`；`11/13/14/45` 的公共执行可参考已有来源 10、3、6 和酎实现，仍需原 CPU 调用上下文验证。

| MAIN | 分派入口 | 主要已见副作用 |
| --- | --- | --- |
| `39` | `1bd6e` | 直接累加已带方向的 VX、Gravity／VY／Y；不得再次按 Facing 镜像 |
| `3a` | `1be08` | 由 Aux48 最高 byte 选择 Camera.Left 加 `20/a0/120`，保留 X 小数 |
| `3b` | `1be64` | 独立尝试子一 `136e6`、子二 `1391e`；Attributes 需要 OR `6000`，不能原样套固定 OR `2000` 的构造 |
| `3c` | `1bfce` | 完整 Energy.long 每次减 `4000`；有条件传 Held／Pressed 到两个子对象，C 可退出 |
| `3d` | `1c032` | 第三子对象 `15094`；同时按 Team 遍历四个父体，将对方 HitStopTicks.byte 写 `ff`，没有 Size／Health 过滤 |

地面请求 1 的 B／A 根成本各 16；请求 2 必须主子槽空，按 C→B→A 选择后续偏移；请求 3 地面／空中均按 C→B→A 选择 Aux48 最高 byte，成本 16。弱生命根要求 Health≤111、Held.Down、成本 200，请求 4／6 共用。空中请求 1 为 RTS，空中请求 2 的 B／A 成本为 16／20。这些都只是静态条件，后续需要合法固定输入与真实扣能量成功记录。

目前只确定三个子回调的创建上下文，现有静态正文和 `fighter-action-flow.json.gz` 未覆盖 `136e6/1391e/15094`。其资源、碰撞、父子关系、清除及其他派生回调必须补证；不能从地址邻近天空对象或全动画目录推断已恢复。

已见 MAIN 使用现有 VX／VY／Gravity、Aux48、Energy、TargetMask 与 HitStopTicks owners，没有证明新的 FE 语义。跨人物 `+69` 改写需要明确临时传递和原子合并，不得误用 `+84` 的 StunTicks 或复制另一份计数。物理 FE 继续保持唯一 slot word，只有 callback 正文证明需要时才增加对应临时解释。

来源为 [选择器静态](static/fighter-action-selectors-focused.txt)、[构造静态](static/fighter-action-constructors-focused.txt)、[动作静态](static/fighter-action-slots-focused.txt)、[分派覆盖](static/fighter-action-coverage.json)及当前 `engine` 公共实现。下一步先补三个 callback 正文并确认最小字段，再沿失败清单、原 CPU、先行冻结测试与真实 red 的顺序恢复。

本页静态起点现已完成后续原CPU与独立规则验证：所有人物包括树均已开放。实际实现与限制以[树世界验收](../../../games/yuyuhakusho/engine/FIGHTER7_WORLD_VERIFICATION.md)为准；这里保留当时未知callback、跨人字段与恢复顺序，不能把这些静态描述替代后续执行证据。
