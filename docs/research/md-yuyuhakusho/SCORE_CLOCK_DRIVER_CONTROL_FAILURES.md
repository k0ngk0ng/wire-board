# 控制、渐弱与暂停整拍的先行失败场景

- 01／05／04 的 dispatcher table、重复分类、调用和 return 成本错位；01／05 启动渐弱时误停 PCM，或 04 漏清字段、漏 key-off／FM／PSG mute。
- active fade 的 remaining／steps 写入顺序错误，零值回绕或 terminal 04 边界错误；header 与后续请求消费、分派和声部扫描没有共用一份预算。
- FM／PSG 的衰减增量、符号边界与 16 级边界错误，claimed、note-off、tie／gate 抑制错误，或者把渐弱的即时 PSG 音量当成包络推进。
- fade 的资源和 loaded algorithm 使用旧投影；重复实现已有 instrument／PSG 计时或在子调用重置 refresh。
- pause gate 预读入口值，未按三次实际读取决定 start／hold／resume；确认／清除写入没有发生在实际 owner 的计时点。
- pause start 未先取得真实总线就捕获音量；捕获另一份缓存、捕获已 mute 的音量，或恢复的 payload 与实际捕获值不一致。
- pause hold 仍推进 tempo、fade、queue、score 或 PCM；resume 重复 PSG mute、漏恢复 FM、提前清 pending payload，或者重复计算 return。
- 把同一 Root／score／PCM／chip 字段存成多个持久 owner；把诊断写入列表再次播放，或从最终结果反推中间效果。
- Root effect/read lease 被遗漏、重复、过期、伪造返回或错误后仍继续；effect 收到的版本／序号／阶段与实际观察不同。
- 总预算或外部 IO／capture 失败后提交 score，或者局部回滚掩盖已发生的 detached owner 访问。
- 旧的 ordinary-only 接口支持边界被悄悄扩大，破坏冻结的失败契约；完整 driver 接口新增能力必须复用同一个内部执行器。
- 将 no-extra-IRQ 的完整调用对照扩大为 world IRQ、请求生产时刻、联合 checkpoint 或平台整场播放完成。

本计划先于输入构造、原版采样、E2E 与生产实现。覆盖空 score、活动音乐／primary、fade 边界、start／hold／resume，以及后续正常请求与声部路径。使用实际 mixed owner，不引入第二份持续更新的寄存器缓存。
