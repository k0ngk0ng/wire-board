# 魂斗罗实现与验收进度

更新日期：2026-10-28。按用户最新要求，本次交付范围为单人挑战和双人真人合作，暂不开放电脑玩家或托管，也不以电脑从头打通八关作为交付门槛。魂斗罗已接入主程序，单人实际开局、双人真人操作、控制器设置、PC／320px、全屏和恢复已有真实浏览器验证；服务端拒绝添加电脑及开启托管。

八关规则、素材与服务器判定保留。已保存的辅助通关及独立回放是研发证据，不标记为电脑稳定通关或正常生命数胜利。此前 v11 从头辅助测试的单人／双人失败结果也保留；后续电脑策略研发与本次真人交付分开。正式上线按已验证构建和版本 tag 流程执行，线上状态以 `/api/version` 与 Release 记录为准。

## 已实现及已有证据

| 范围 | 当前实现与证据 | 验证限制 |
| --- | --- | --- |
| 原版基准 | 固定日版 ROM SHA-256；使用 REA/Ghidra 静态分析和原版执行对照，见 [初始调研](README.md) | 原版模拟器仅用于开发对照 |
| 独立规则 | Go 引擎覆盖移动、武器、敌人、八关、场景切换、暂停、终局和音频控制，见 [完整 World 对照](evidence/active-world-verification.json) 和 [八关对照](evidence/world-eight-original-verification.json) | 逐关受控初态对照不等于自然连续通关 |
| 联机模块 | 权威输入、序号、租约、加载屏障、回放和存档恢复，见 [服务端模块](server-module.md) | 正式浏览器宿主仍需绑定最新视觉电脑资源和源码版本 |
| 合作积分 | 全队胜利每位真人 +20、共同失败每位真人 −10；显式电脑席位不计分，真人主动托管保留开局冻结的计分资格，见 [模块验证](evidence/server-module-verification.json) | 受控终局账本验证不证明八关玩法完成 |
| 自然共同失败 | 标准双人局经合法移动耗尽生命，原版继续菜单选择退出后完成共同失败终局；保留 1,663 帧流程和 227 条输入动作记录，见 [自然失败验证](evidence/natural-shared-failure-verification.json) | 模块时钟加速验收，不包含浏览器或自然通关；积分账本另有 HTTP／SQLite 检查 |
| 控制器与浏览器 | 复用平台「控制器设置」、`classic-2`、公共 FC 手柄及音频入口；真实双人房间覆盖 PC、320px、观战、重连、托管、设置和刷新，见 [运行层验收](runtime-platform.md) | 证据绑定该次规则与前端构建；电脑后续改动仍需最终构建复验 |
| 主程序接入 | `main.go` 使用 `internal/production.Modules()`；缺少真实构建身份时在数据库初始化和监听前拒绝启动。真实主程序验证默认可见、双人房间、PC／320px、改键、全屏、网络恢复，见 [主程序证据](evidence/main-integrated-source-v11/result.json) 和 [浏览器证据](evidence/main-integrated-source-v11/browser/result.json) | 私有本地数据库与环回网络；尚未部署或完成从头通关验收 |
| 干净构建与交付 | 项目内保存按摘要固定的约 23.5 MB 无损音频源码包，默认构建不读取研究音频缓存。Vite 只汇入清单引用的运行资源；原生链接前保存并核验同源运行清单 | 真实构建已本地执行；尚未上传真实 CDN |
| CI 与 CDN 校验 | 长战役按模式和人数拆成四项独立任务，仅规则范围触发；CDN 流程覆盖清单、摘要、MIME、CORS 和音频 206 Range，裁剪后再链接原生身份。见 [CI 接线证据](evidence/ci-final-wiring-verification.json) 和 [本地协议验证](evidence/contra-cdn-verification.json) | 选择器、脚本和环回端到端验证；尚未执行 GitHub Actions 或 Docker 源构建 |
| 正常生命数电脑终局 | v11 单人、双人分别在第 17,195、18,353 帧自然失败并完成结算，存档恢复和独立回放通过，见 [模式验收](evidence/fresh-standard-motion-v11/verification.json) | 自然失败证明终局流程，不计作八关胜利 |
| 60Hz 呈现与恢复 | PC／手机采样步长均为 1，画面间隔中位数 16.7ms；650ms 字节延迟、后台恢复与公开生命同步通过，见 [运行层验证记录](evidence/platform-runtime-verification.json) | 环回与受控延迟结果，不是公网或并发容量验收 |
| 全屏与触摸 | 独立单人房间验证全屏范围、生命显示、控制器分区、键盘及触摸 Start，见 [单人检查](evidence/platform-solo-root-check.json) | 局部检查，不替代双人完整对局验收 |
| 托管输入释放 | 公共平台显式 `IsInputRelease` 能力及 HTTP/WebSocket/SQLite 边界通过，见 [释放验证](evidence/server-host-release-verification.json) | 普通行动仍须取消托管后才能执行 |
| 电脑可见性 | 服务端仅向观察器提供最终 RGBA 和实际公开 HUD；视觉资源在构造前校验并冻结，见 [服务端视觉边界](evidence/server-bot-visual-verification.json) 和 [原版像素复验](evidence/bot-visual-provider-root-verification.json) | 服务端边界夹具使用合成图集和观察器替身，不是正式电脑策略验收 |
| 室内场景记忆 | 修复短暂无法识别画面后丢失室内场景的问题；13 项 race 回归通过，双人 8,500 帧合法前缀接管后于第 13,486 帧进入第三关，独立重放一致，见 [场景记忆验证](evidence/coop-scene-memory-verification.json) | 测试补命、带前缀的局部续跑；不代表新局连续八关通过 |
| 双人角色追踪 | 完整角色识别纠正旧身份、联合候选消歧及旧位置回退冲突修复；8 项针对性 race 检查与存档恢复／取消托管检查通过，见 [追踪验证](evidence/coop-identity-repair-verification.json) | 新双人连续流程仍在运行，不能据此宣称第三关卡点或八关通关已解决 |
| 生命显示时序 | 生命值随已提交画面保存，覆盖自然伤亡、恢复、暂停和终局，见 [生命显示验证](evidence/session-hud-verification.json) | 最终浏览器构建仍需复验 |
| 自然失败结算界面 | 真实单人自然失败显示 −10 积分；结算浮层等待已提交终局，关闭后保留最终画面，可重新查看，见 [浏览器终局验证](evidence/terminal-browser-verification.json) | 单人失败路径，不是八关成功或双人终局完整验收 |
| 音频集成 | 慢加载、暂停恢复、后台持续音乐和终局完整音效通过针对性与原生媒体验证，见 [音频集成审查](audio-integration-review.md) | 实际设备听感和采样级多轨同步尚未验收 |

## 本次交付与后续范围

1. **真人入口。** 保留 1／2 人选项，单人无需添加电脑即可开局。前端隐藏电脑和托管入口，生产模块省略相应能力；不能通过直接调用房间 API 绕过。
2. **真人操作验收。** 真实主程序覆盖单人开局与移动、双人房间、PC／320px、键鼠／触摸、加载屏障、全屏、改键、观战、恢复和刷新。验收记录绑定执行时的规则、运行产物和前端构建；后续修改按影响范围复验。
3. **八关与结算。** 原版八关规则保留，通关由服务器判定，全队真人共同加分或扣分。合法输入的接续通关、正常生命数自然失败和账本幂等分别留证；用户已明确不要求以完整通关测试阻塞本次交付。
4. **电脑研发。** 电脑代码和诊断资料保留，但不对玩家开放。历史 [177,154 帧接续通关](evidence/assisted-prefix-eight-clear-verification.json)、[363,330 帧接续通关](evidence/energy-retry-eight-clear-final/verification.json)及后续钳爪修复记录均不是从新局的稳定电脑验收。CI 电脑战役仅由默认关闭的研发开关启动，结果如实保存，不阻塞真人版交付。
5. **正式发布。** 干净构建、源码素材包、资源摘要与加载范围、CDN 回读校验及原生身份链接已接线并本地验证。实际上传、Actions、版本 tag 和服务器更新尚未执行；正式发布继续遵循已验证构建复用流程。

手机单指原地覆盖 A、B 不能可靠表达两个独立触点，本次不新增组合键或特殊滑动手势。公共控制器保持原有多指输入方式。

## 性能证据的适用范围

早期第 600 帧性能夹具尚停在介绍画面，不能用于战斗容量判断。[修正后的战斗基准](evidence/actual-battle-performance-verification.json) 先通过合法 Start 输入进入战斗，并断言 `LevelRoutine=4`。在 Apple M4 上，双人真人规则与两份视图序列化约 2.48ms／50ms 批次，原始视图约 76.7KB／批次；该轮双电脑约 76.7ms，仍高于预算。后续等价优化及隔离原型需分别记录，不能把原型或单场景结果当成最终生产容量验收。

正式浏览器测试宿主仅在显式启用时运行。长时间验收需给 `go test` 设置足够的超时；宿主自身的运行上限不覆盖 Go 测试进程默认超时。

测试专用 `CONTRA_BOT_INFINITE_LIVES=1` 会在新局补足备用生命；之后等待自然扣命提交到公开画面再补充，保留可见的扣命／复活线索，不提供无敌或自动复活已淘汰角色。实现仅在 `_test.go` 中，正式游戏不读取此开关。辅助记录包含 `infinite_lives`、`test-life-refills.json` 和 `assisted_campaign_cleared`，不标记为标准命数的 `complete_campaign`。
