过关、失败与续关的运行时清理已恢复为 `ApplyCampaignRuntimeReset`。菜单显示由 `PrepareCampaignGameOver` 和 `StepCampaignContinueDisplay` 提供。实现只消费 `Campaign` 已经作出的决定，不重算胜负、不推进关号、不重复扣续关次数，也不重复发出音效。

原版为 `Contra (J).nes`，SHA-256 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。用户 ROM 只读，私有 REA 会话使用工作区内的固定 PRG 切片。完整 8 份分析记录见 [campaign-reset-rea.json.gz](evidence/campaign-reset-rea.json.gz)，源切片和光标坐标来源见 [campaign-reset-source.json](evidence/campaign-reset-source.json)。

| 原版位置 | 已实现行为 | 保留边界 |
| --- | --- | --- |
| `CD1F → C309` | 清 `40..EF`、`0300..06FF` 对应的世界、玩家、敌人、弹池、地图、碰撞和场景临时状态；另清 HUD | 生命、GameOver、分数、增命阈值和显示状态不在清理范围；过关武器由命令恢复 |
| `0190..0196`、`FB` | 不参与上述清理 | 出口计数、出口角色标志、生成器 Screen／Generated 和出口淡出步数保留 |
| 调色板／图形 | 清 `74` 淡入计时及 `72/78/7D` 循环计时 | `36` 请求数、待提交／已提交颜色、CHR 页、图形队列、OAM、雪粒子保留 |
| `C2CF` | 按命令恢复活跃角色生命、GameOver 和增命阈值；清两个局内分数 | 高分、当前关、武器和其他玩家运行态不额外重置；单人模式保留未启用 P2 的生命字节 |
| `CD71` 非 demo 分支 | 清雪粒子 Flags；即时清背景；选择 CHR 10；排入调色板、GAME OVER、分数和可选续关文字 | `6E` 音效与续关次数仍由 `CampaignPlan` 负责 |
| `CDAF` | 按已更新的 Choice 显示光标；Start 继续后隐藏光标 | Start 优先于 Select；只改 P1 的 X、Y、显示码，保留属性与 Y 高字节／小数 |
| `CDF1` | 无单独显示写入 | 耗尽续关后的结束动作由既有 `Campaign` 决策处理 |

父级 `World` 的接入顺序：

1. 保存当前阶段，再调用 `Campaign.BeginTick`。
2. 对 `resetRuntime`／`restartLevel` 命令调用 `ApplyCampaignRuntimeReset`。
3. 原阶段为 5 且进入失败分支时，调用 `PrepareCampaignGameOver(active, levelRoutine == 6)`，统一消费返回的背景和显示效果。原阶段为 6 时，在资源命令执行后调用 `StepCampaignContinueDisplay`。
4. 音效只消费 `CampaignPlan` 一次。两个显示函数不返回音频命令，也不修改 `Campaign`。

`PrepareCampaignGameOver` 要求 Text 队列；它不负责把矩形队列解释成文字。原版进入过渡阶段前的 `CD0E`，或过关插曲的结束阶段，已经切换该模式。控制器中的 `graphics-mode` 是精灵闪烁控制，队列模式另称 `queue-mode`；本组没有新增模式切换效果。

测试先于实现。原版边界录制使用既有普通双人存档，在真正调用入口前声明一次受控输入；返回后才恢复场景供下一条原版调用使用。没有写入返回结果。74 组输入产生 138 条完整入口／返回记录，包含完整 RAM、名称表和音效顺序；独立重录逐字节一致。受控关号／Boss 状态只用于边界验证，不代表自然打通关卡。

限定 race 验收通过 105 个测试条目：40 个过关／失败清理对照、8 个续关资源对照、8 个失败画面对照、32 个续关动作对照、19 个错误原子性子例及父测试。完整 engine Go／JSON 源码在验收期间未变更，摘要与结果见 [campaign-reset-verification.json](evidence/campaign-reset-verification.json)。这些结果限定于本组阶段；完整连续战役、网络对局和平台积分仍由上层验收负责。

复验命令：

```sh
source scripts/env.sh
go test -race -json ./games/contra/engine -run '^TestCampaignReset' -count=1
```

原版重录脚本是 `scripts/contra/campaign_reset_capture.py`，每次使用新的 `--name`，并可指定 `--verify-against campaign-reset-original.json.gz`。REA 种子与源绑定由 `campaign_reset_prepare.py` 生成，之后运行 `node scripts/contra/static_rea.mjs .local/contra-campaign-reset/fixed-spec.json`。`campaign_reset_accept.py` 生成带源码摘要的限定验收记录。
