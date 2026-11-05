# 输入恢复与阶段提示的追加失败范围

本清单在完整服务端模块已有绿色验收后追加，先于对应测试和修正。原有冻结测试保持不变。

- 真人已按住按键时执行 `Restore`，虽然 transport 的 `Buttons`、`Changes` 已清空，下一 tick 仍继承 `World.Controllers.Current`，导致恢复后继续移动或攻击。
- `Reclaim` 清空自身 transport 后未在下一 tick 释放按键；清空操作误改另一席位的按键。
- 输入队列在恢复或接管时被清空，却在后续 tick 重新播放；用直接修改 `World.Controllers` 的方式释放，丢失原版 `Released` 和扩展按键历史。
- `Restore`、`Reclaim` 在调用当帧修改 world、呈现、位置、HP、RNG 或网络帧；新真人输入不能在后续正常生效。
- 使用固定 `Frame < 240` 代替原版 manager 的开场阶段，原版仍处于 stage 10 或 12 时提前显示比赛进行中；暂停、结果阶段的提示优先级被破坏。

修正边界为现有 transport desired 状态与规则 tick 的衔接；不新增一份规则输入历史，不改原版物理或 AI。
