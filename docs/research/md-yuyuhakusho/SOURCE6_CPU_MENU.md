# 通过原版菜单进入电脑阵对局

固定的真实手柄输入可以从冷启动进入「练习比赛 → Survival → 2 人」并由原菜单填入电脑阵。最终场上为真人阵和电脑阵，电脑对象 `9000` 的 InputChannel 为 `255`。被动观察记录到 1,110 次完整原版 AI 返回、297 次来源 6 决策分派及 2,835 次共享 RNG 调用，逐帧 RAM／video 与唯一未观察原 core 基线一致。

本输入的 3602 帧内，电脑阵只有 4 次空中特殊选择器调用，未成功选择 `34`。这份数据保留为独立 AI 自然证据，不补入真人特殊输入，也不代表独立 Go 世界或 AI 已验收。

## 实际输入与证据

- [固定输入](../../../games/yuyuhakusho/engine/testdata/fighter6-ai-inputs.json)：SHA-256 `16505f2ee81d6828397785195f2912c554d33ee7647585f1d93e6f76d39ce753`。
- [冷启动选择记录](source6-ai-selection.json) 与 [场上截图](source6-ai-selection.png)：match characters=`[7,0,0,7]`，controllers=`[1,0,0,0]`；角色 1 的 source=6／channel=0，角色 4 的 source=6／channel=255。
- [完整原 CPU AI 证据](../../../games/yuyuhakusho/engine/testdata/fighter6-ai-natural-validation.json)：包含完整 actor／world、RNG、输入、入口及实际返回 SP；每次 `exit_sp=entry_sp+4`。
- [唯一原 core3602 基线](../../../games/yuyuhakusho/engine/testdata/fighter6-attacks-ai-reference-manifest.json)：frames SHA-256 `21bdaf7229acd97b95120deed04e34216a912e4d963ce16e592eaa5353ebbe5f`，原 core SHA-256 `1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da`。

实际 AI 入口为 `1cc8a`，返回点为 `156ee`。观察确认 InputChannel=255 后才收录调用，不将真人阵或展示画面的有形角色计入电脑 AI。原版外部观察 core 的 SHA-256 为 `e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306`。

## 原菜单路径

原 ordinary 模板选择「胜拔战」，首个电脑固定为酎。新输入从模式页向下选择「练习比赛」，然后进入 Survival、确认 2 人、为玩家 1 报名并进入选角。角色页按住右至边界，再左移和上移，选择阵。

原菜单随机填充电脑人选。最终输入在第 1883～1887 帧确认角色、第 2033～2037 帧确认场地、第 2183～2187 帧继续，稳定得到电脑阵。[33 次有界菜单尝试](source6-ai-menu-trials.json) 保留不同确认时刻的原版结果。尚未确认能用菜单直接指定任意电脑角色。

全部尝试从冷启动重放物理按钮，没有写入 ROM、RAM、Request、RNG 或 AI 状态，也没有用存档恢复来制造结果。不同确认时刻对人选的影响来自原程序执行，最终输入不注入挑选出的中间状态。

## 展示模式的有界发现

[零输入 18000 帧发现记录](source6-ai-attract-discovery.json) 显示，第 4340 帧自然出现 source6／InputChannel255 的对象 `8800`，同场还有来源 4 和 7；来源 6 共出现 2,083 帧。较早的展示角色使用 channel 0／1，没有计作这份 AI 证据。

本轮没有为零输入展示另建 control，也未对展示模式导出完整 AI 返回。最终验证使用 3602 帧内可进入的练习模式输入；展示发现只证明该有界执行内出现了来源 6 的电脑通道。

## 八组合法输入的 Request 4 探索

在同一菜单入口后，只改变真人的移动、跳跃、空投、普通攻击和防守，共运行 [8 组有界观察](source6-ai-r4-bounded-search.json)。没有新增未观察 control，也未改变主 AI 输入或其规范基线。全部观察仍为 `34` 成功 0 次。

最后一组 [投技脉冲输入](../../../games/yuyuhakusho/engine/testdata/fighter6-ai-r4-inputs.json) 在第 2950、3502 帧实际执行 `1d212 → 1dc50 → 1e7a2`，将 Request 从 0 改为 4；两个完整返回的 InputChannel 均为 255。原版当时为 Action `05`、Pose `2f`、EnabledStages `80`，没有进入 `34` 构造。[独立 Request 4 记录](../../../games/yuyuhakusho/engine/testdata/fighter6-ai-r4-validation.json) 保留完整原 CPU 调用、实际 SP 和输入。

该次级输入没有单独的未观察 core 对照，也不以主输入的 `21bd...` 基线代替。它证明本次原 CPU 执行中实际产生了 Request 4；空中选择器 `34` 的自然成功仍未证实。

## 复现

已有唯一原 core 基线时复用原产物。以下命令只追加被动观察，不改模拟器状态；输出留在工作区。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter6_natural_ai.py \
  --inputs games/yuyuhakusho/engine/testdata/fighter6-ai-inputs.json \
  --out .local/md-yuyuhakusho/fighter6-ai-recheck
```

预期得到 1,110 次完整 AI 返回、297 次来源 6 决策分派、4 次空中特殊返回、0 次 `34` 成功。将 `observed-frames.json` 与规范原 core frames 逐帧比较；任一差异或不完整返回都不能记为通过。
