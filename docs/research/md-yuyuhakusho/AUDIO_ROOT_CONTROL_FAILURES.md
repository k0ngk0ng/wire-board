# 比赛 Root 控制恢复的先行失败场景

- 把 isolated score 的 control／fade／pause 硬零当作完整系统状态；新增 root state 又复制 World queue、AudioPause、Scheduler、PCM flags 或 18 voices。
- 01／05 被播放为音效，清 DAC 只改 Active 却留下原应清 flags，或错误停止当前 PCM。
- remaining 从 1 减为 0 的同一 tick 提前 fade；初始32被当成固定后续间隔；最后 steps 归零时继续 level step 而不走04。
- FM／PSG fade 与 duck 的阈值效果混用；byte wrap、signed 判断、deactivate 和 skip-write 被改成统一 clamp。
- 立即 PSG attenuation 走普通 volume()，推进了 envelope／gate／pan／modulation／note／tempo。
- 04 同时清掉 Scheduler 或停止 PCM，遗漏 Root pause／queue／selected 的真实写点，或在被中断的 bulk 清理后用旧副本覆盖新请求。
- priority save／clear／restore 不按原阶段，bulk clear 又额外支付一次 byte-clear 成本。
- duck 在 PCM flag5尚未置位时提前 apply，或 flag仍在时提前 restore；用户 control mode 与自动duck phase相互覆盖。
- 以 Tone／BaseLevel 重算暂停前硬件level，忽略 primary／claimed／carry-skip／最后已写值；常驻缓存整个YM或18voices。
- pause pending restore 载荷没有真正独立生命周期，恢复／reset后残留，或未验证原 cache 与 chip view 在捕获点的可见值关系。
- 外层先推进 tempo／fade后再调用整tick Advance，重复推进；callback错误后提交部分 Root／score／bus／chip owners。
- 用原 PC 时间表、通用 opcode／RAM VM、固定空tick成本或零定价填补未恢复分支。
- 将私有 SR2700 和无外部stall的函数成本，写成已解决异步 Z80 steal、完整world→IRQ或产品音频。
- 占用已给 full-chip owner 的原采窗口，写入授权 worktree 外，或修改冻结leaf/golden来制造通过。

本清单先于新的 control API 和实现。原自然 pause／已有调度、leaf、bus、PCM、chip 证明只读复用；新增 native oracle 与独立阶段接口必须保留真实 red 和完整可重复结果。
