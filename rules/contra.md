# 魂斗罗开发规范

本游戏同时遵循[通用规范](common.md)、[即时游戏开发规范](realtime.md)、[复刻游戏开发规范](recreation.md)、[控制器开发规范](controllers.md)。界面、加载、音效等改动按 [AGENTS.md](../AGENTS.md) 的专题索引读取对应规范。

## 交付范围

- 用户已确认的当前交付范围为单人或双人真人合作；电脑未完成验收前，暂不开放电脑玩家和托管，不以电脑从头打通八关作为当前交付门槛。
- 真人合作通关时所有参与者得分，失败时一起扣分；资格判定和结算由平台执行。

## 原版分析与资源

- 原版玩法恢复须结合 ROM 与 REA 分析证据。允许随机序列与 FC 空转及中断时序产生的序列不同，同时保留原版随机运算和敌人决策条件，保证服务器存档与回放可重复。
- 客户端运行时与素材统一遵循[通用加载与资源发布规范](loading.md#发布资产与镜像边界)，音频源包也不得进入正式服务器镜像。

## 相关资料

- [模块目录](../games/contra/)
- [服务端接入契约](../games/contra/server/PLATFORM_CONTRACT.md)
- [原版恢复与平台接入记录](../docs/research/nes-contra/runtime-platform.md)
- [本地输入预测与响应验收](../docs/research/nes-contra/input-prediction.md)
