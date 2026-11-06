# 音轨终止与声部恢复的先行失败场景

- 将 DAC 的整个 driver 提前返回误作普通 voice return，或重复计算 handler 的栈跳过与 return。
- 使用隔离样本的资源分支标签推断实际 primary／secondary 席位；原版 primary domain 使用负标志及 voice-local instrument bank。
- 恢复目标错位，FM inactive target 被错误清除 claimed／重新加载，PSG inactive／noise target 漏掉强制 rest 或 noise latch。
- 恢复直接进入 instrument seek 中段，却再次计算 claimed guard／bank selection；25 次写入、pan 尾部或 return 重复／遗漏。
- primary track stop 的 priority clear 与 queue-replace 不在唯一 Root effect lease 中执行，重复、遗漏、保存过期回调或以局部副本替代真实 owner。
- 跨 voice restore 仍按结束声部的 channel／owner 校验，或放宽全部 owner／position／sequence 校验；终止后 active 变化未正确处理。
- sibling voice 在扫描前后的状态混用，恢复字段被第二次推进；异常后仍提交 score 或继续使用已变化的外部 owners。
- 把单声部终止／恢复验证扩大为完整 Root 时序、world IRQ、联合 checkpoint 或平台整场播放完成。

本计划、固定原版条件与 E2E 在实现前冻结。既有普通新音符和 held-note 对照继续保留，当前声部恢复必须复用同一临时 clock／refresh／预算与 canonical 子服务。
