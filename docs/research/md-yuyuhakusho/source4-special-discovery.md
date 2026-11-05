# 飞影特殊技的合法固定输入与吞龙生命周期

本轮从原版真实菜单和手柄输入得到两个独立场景。胜拔战场景在 3602 帧内覆盖九个特殊选择器；双真人练习场景自然完成放龙、操龙、吞龙、强化和负能量恢复。没有写入 ROM、RAM、Request、HP、Energy、角色或 AI 状态，也没有通过存档注入中间结果。自适应阶段只选择真实按钮，保存按钮事件后又从冷启动固定复放。

## 九个选择器的胜拔战场景

[固定输入](source4-special-discovery-inputs.json) SHA-256 为 `5ba6b13d2bb0b285c4307981a5e903d75a7936de3d2632a9f1bf449189650424`。[唯一原 core3602](../../../games/yuyuhakusho/engine/testdata/fighter4-attacks-special-reference-manifest.json) 的 frames SHA-256 为 `666c2a9fff6c47b3b0540275beae5a7a1c0f6ada1e1d690d6a67d1596617b22e`。来源 4／channel0 为玩家，来源 9／channel255 为原电脑。

自然成功帧：R1 `1874`、R2 `1925`、Ground R3-B `1999`／A `2051`、Ground R4-B `2190`／A `2388`、Air R3-B `2624`／A `2653`、R6 `3434`。R6 前真实 HP=95、Energy=202、主 child.Size=0，成功扣至 Energy=2，进入 `29/a6`。`19092` 出现在 1897，`191ba` 出现在 3489。

[原版发现记录](source4-special-discovery-first-report.json) 保留自然状态与 PC。该场景 `2a` 只有 1 帧，随后 child 消失，MovementSource 仍为 `12578`；不把它写成吞龙成功。选择器另有 [1225 次完整原 CPU 返回](../../../games/yuyuhakusho/engine/testdata/fighter4-attacks-special-natural-manifest.json)，与未观察原 core 逐帧一致，独立 Go 对照详见 [选择器验证](../../../games/yuyuhakusho/engine/FIGHTER4_ATTACK_VERIFICATION.md)。

## 双真人练习吞龙场景

[双端口固定输入](source4-special-discovery-duo-inputs.json) SHA-256 为 `b303ca8622f90747052902f81b2b7f2bdde39f59e47feb6f36a6bf8287a6ac34`。每个事件保留原始 `port` 和按钮；3598 帧以后没有输入。

真实菜单进入「练习比赛 → Survival → 2 人」。席位与原对象如下：

| 席位 | 原对象 | 来源 | InputChannel | 操作 |
| --- | --- | --- | --- | --- |
| 玩家 1 | `8400` | 3「浦饭幽助」 | 0 | 用真实普通攻击将对方 HP 降至 103，随后换道并停手 |
| 玩家 2 | `9000`，child=`9100` | 4「飞影」 | 1 | 真实输入 R6，随后通过方向键操龙返回 |

该场景是两个真人输入通道，不是电脑或原胜拔战模板。原主对象在 `FF8000`，实际 Behavior=`a01e`／Stage=32；不能把 Stage32 改成旧胜拔战的 Stage20。

[唯一未观察原 core5502](../../../games/yuyuhakusho/engine/testdata/fighter4-attacks-duo-reference-manifest.json) 的 frames SHA-256 为 `9f282f53a016d184d45052b30eaf8c482d4b568132f7979f7eebff4a7faa5917`，入口上限 5500，后 2 帧排空。按新窗口要求，只保留并共享这一份完整 Duo control；先前尚未共享的 3602 预览已由同路径替换，完整 control 的前 3602 帧序列化 SHA 为 `e7215bca68fd73791b233ac3348e0d40a4cbcf116c582230e7ad01fbe6ea0aae`，与预览完全一致。

[完整生命周期记录](source4-special-discovery-duo-lifecycle.json) 的所有 5502 帧 RAM／video 与原 control 一致：

| 事件 | 原版视频帧 |
| --- | --- |
| 吞龙，MovementSource 首次为 `15946`，Energy 回到 223 | 3589 |
| `13fc8` 强化 child 首次活动 | 3598 |
| `15946` 与强化 child 最后活动 | 4298 |
| Energy 首次为负，恢复 `12578`、child 清除 | 4299 |
| 强化结束后首次普通站立 | 4379 |
| 窗口末 | HP103、Energy−1、普通站立、child0 |

`13e9a` 为全局 FX 池对象，所属龙指针存于 `+48`，本场景为 `9100`；不能只用飞影 0x400 家族地址过滤它。[5502 帧原回调发现记录](source4-special-discovery-duo-full-report.json) 保留实际对象位置。组件与完整世界的独立 Go 验收由对应模块进行，本发现报告不代替这些验收。

## 主对象就绪窗口与元数据纠正

生命周期记录同时保存 `retro_run` 前／后主对象及 manager 的连续窗口。视频步后，`a01e` 的 Stage14→16→18→20→22→24→26→30 后，于第 2162 帧进入 Stage32，manager=`f2a4`；视频步前对应第 2163 帧。manager Stage14 在视频步后第 2458 帧起稳定；从 2500 开始验证世界仍早于首次实际受伤第 2563 帧。

最初的探针误把代码名 `9ad0` 当作 RAM 地址，读到无关零值；修正时又遗漏了 `FF8000`，未找到对象。最终按真实固定主对象地址 `FF8000` 读取，得到 `a01e/32`，没有改原状态或将等待阶段规范化。最初误址输出作为 [历史记录](source4-special-discovery-duo-lifecycle-initial.json) 保留，不能用其中的主对象字段作证；最终记录的全部帧仍匹配同一原 control。

## 复现

复用既有规范 control，不重复生成。固定输入观察只调用原版模拟器和读取状态：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/source4-special-discovery.py \
  --inputs docs/research/md-yuyuhakusho/source4-special-discovery-inputs.json \
  --out .local/md-yuyuhakusho/source4-special-discovery-recheck --frames 3602

PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/source4-special-discovery-duo-lifecycle.py
```

第二条命令断言全部 5502 帧与既有 Duo control 相等，并重新导出自然生命周期。Root 的独立世界生产器保持原有 SHA 与失败日志，不使用本探针替换旧生产器。
