# 电脑视觉观察边界

实现前失败清单。电脑观察只包含玩家实际看到的最终 RGBA 和界面确实显示的 HUD；不再调用 `ObserveBot(World)`，也不提供旧接口回退。

| 边界 | 失败情形 |
| --- | --- |
| 配置 | 未验证或缺少的视觉资源进入模块；资源 SHA／尺寸／像素范围错误；可变外部数组修改已构造资源；回调读取文件或网络 |
| 观察 | 直接将 World、实体槽、敌人种类、碰撞窗口、屏外背景、原始 OAM、随机数、HP 或时序交给策略 |
| HUD | 把未显示的关号、场景、Continue 选项、阶段、存活或武器从规则补为 Known；把零生命当未知 |
| 同画面 | 相同最终画面与公开 HUD 因隐藏实体、地形或规则变化产生不同策略输入 |
| 席位 | 双电脑每席重复渲染或识别整帧；误给对方 actor；一个策略修改观察结果后污染另一席；真人席位推进策略记忆 |
| 阶段 | 暂停、加载屏障或终局仍运行策略；取消托管重置 Session 或其他席位输入 |
| 错误 | 渲染／观察失败后仍提交部分按键或帧；缺失资源时退回规则捷径或猜测 HUD |
| 存档 | 视觉身份或共享规则构建不同仍恢复；恢复后遗漏策略记忆或改变玩家物理状态 |

当前 UI 对照：`web/Board.tsx` 明确显示各席位的备用生命（包括 0）与暂停状态；创建房间人数公开。其余信息仅由原版 Canvas 像素呈现。`BotPublicHUD` 因此只填 Seats、Occupied、Lives 和 Paused；Phase 留空，Stage／Scene／Choice／Alive／Weapon 保持未知。

配置资源在模块构造之前读取、验证并冻结为内存资源。每个权威规则帧最多生成一次 `BotVisualFrame`，以规范 `Actor=0` 识别一次公共画面。服务器随后为各自动席位复制观察结果和敌人切片，设置该席位的 actor，再调用各自策略。即使只有第二席托管，视觉识别也使用规范身份；这不授权第一席操作，也不推进其策略记忆。全局不可变视觉模板在配置阶段绑定纯 observer，观察函数只接收 `BotVisualFrame`，不能按隐藏当前关卡选择模板。

策略接口由引擎组提供：`ObserveBotFrame(frame BotVisualFrame, vision *BotVision) (BotObservation, error)`；服务端先通过注入纯 observer 的边界 E2E 验证权限和数据流，再接入正式视觉分类器。边界夹具不证明电脑能够自然通关。

## 已落地接口

`NewBotVisualResources(manifest, rgb, pages)` 在模块构造外调用，完整验证清单内所有 RGB／CHR SHA、尺寸和索引范围，并转换为只读内存 catalog。`ID()` 返回无部署路径的内容摘要，`Catalog()` 供配置阶段构造全局视觉知识。传入数组、清单或返回像素的后续修改不会改变保留资源。

`ModuleWithConfig(Config{RulesBuildID, Visuals, ObserveBot})` 三项必填。`ObserveBot` 是 `func(engine.BotVisualFrame) (engine.BotObservation, error)`，由正式配置闭包绑定真实 `BotVision`；不接受 World。观察器返回同一公共画面中全部席位的观察，服务器核对规范 actor、tick 和公开席位数，再复制给各席位的 `Bot.Decide`。除身份外，不补填任何观察字段。`State.VisualsID` 与 `View.VisualsID` 绑定视觉资源，恢复不同资源会失败。

初轮边界验收用合成静态 atlas 与纯 observer spy，验证每帧只生成一次像素、双席位共享、未知 HUD、真实公开零生命、隐藏规则不影响输入、取消托管、错误回滚、加载和暂停不调用观察。它只验证接线，不证明视觉分类或电脑策略已验收。正式分类器仍由引擎组提供，不设置规则回退。
