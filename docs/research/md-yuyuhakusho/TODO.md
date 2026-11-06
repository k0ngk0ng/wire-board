# 《魔强统一战》复刻待办

进度使用待办清单，以对应范围的验收证据标记完成，不再使用百分比估算。

- [x] 玩法、11 名角色和 8 个场景。
- [x] 联机同步、平台房间、电脑／托管与 MD 控制器接入。
- [x] FM／PSG／PCM 合成、资源解码及分阶段时序验证。
- [x] 正常 held-note 声部的连续执行器；721 组原版整段调用对照通过，原生竞态与 Wasm 各通过 6,163 条测试记录。见[整段声部验收](../../../games/yuyuhakusho/verification/score-clock-held-voice/)。
- [x] 串联新音符解析、参数、资源及 FM／PSG／DAC 后处理；1,095 组原版整段调用对照通过，原生竞态与 Wasm 各通过 7,262 条测试记录。见[新音符声部验收](../../../games/yuyuhakusho/verification/score-clock-note-voice/)。
- [x] 音轨结束、优先级／队列更新和被占用声部恢复；119 组原版终止调用对照通过，原生竞态与 Wasm 各通过 7,485 条测试记录。见[终止与恢复验收](../../../games/yuyuhakusho/verification/score-clock-track-end/)。
- [x] 初始无新请求的稳定运行整拍，串联 header、tempo、18 个声部和 return；307 组原版整拍对照通过，原生竞态与 Wasm 各通过 7,794 条记录。见[稳定运行整拍验收](../../../games/yuyuhakusho/verification/score-clock-running-tick/)。
- [x] 实际队列消费与普通 PCM 请求整拍；287 组队列和 122 组完整请求原版对照通过，当前原生竞态与 Wasm 各通过 8,206 条 clock／Root 记录。见[请求消费与普通分派验收](../../../games/yuyuhakusho/verification/score-clock-request-queue/)。
- [x] 音乐／primary 技能音轨初始化、切换前停止与恢复；63 组完整原版调用通过，当前原生竞态与 Wasm 各通过 8,273 条 clock／Root 记录。见[分派初始化验收](../../../games/yuyuhakusho/verification/score-clock-dispatch-init/)。
- [x] 无额外 IRQ 的 driver／Root 控制、渐弱与暂停恢复；103 组原版完整调用、实际混音存档继续执行通过，原生竞态与 Wasm 各通过 8,403 条 clock／Root 记录。见[控制整拍验收](../../../games/yuyuhakusho/verification/score-clock-driver-control/)。
- [x] FM／PSG 独立输入时间域；8 组完整原版混音对照通过，两端各通过 184 条芯片／混音检查。见[独立时间域验收](../../../games/yuyuhakusho/verification/fmpsg-independent-lanes/)。
- [ ] 接通 world IRQ、请求生产时刻和联合音频 checkpoint。
- [ ] 接入平台播放器，按需加载并发布无损压缩资源。
- [ ] 完成浏览器整场对局、混合音频、暂停恢复、重连及加载验收。

试玩服务保留现有已验收玩法构建，尚未开启完整音频。分阶段音频通过不代表整场播放已完成。
