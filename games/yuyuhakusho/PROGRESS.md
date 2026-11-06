# 《幽游白书：魔强统一战》复刻进度

更新：2026-11-06。进度改用[待办清单](../../docs/research/md-yuyuhakusho/TODO.md)，按验收范围勾选。独立 worktree 为 `worktrees/yuyuhakusho`，分支为 `codex/yuyuhakusho`。全 11 人物／8 场地已有独立 Go／Wasm 玩法；游戏已注册到本地平台。逐连接输入增量、真实主机端到端与竞态正确性已验收；正式认证双人房间正常约 60 帧／秒，单端每次 TCP 写入延迟 120 ms 时慢端约 58 帧／秒。改键、虚拟手柄、320px 竖屏／740×360 横屏原生触摸、全屏及重连通过。完整对局音频继续完成；未上传资源、发布或部署。Main 仅按用户要求回馈即时游戏规范、补游戏规范和索引，其他游戏的未提交工作未覆盖。

最新音频进展：正式 catalog 的音乐与 primary 技能音轨初始化已接入同一个整拍执行器，包含音乐切换前停止旧声部、patch／pan 恢复、header 与 claim。63 组完整原版调用通过；音乐初始化后结束当前拍，primary 继续本拍声部扫描。原版对照修正了 all-primary stop 的第三路 PSG 恢复目标，普通 track-end 保持原路径。当前原生竞态与实际 Node/Wasm 各通过 122 个顶层测试、8,273 条 clock／Root 记录，相关 score 契约和 15 条旧音轨的普通／observed 接口检查也通过，无跳过，见[分派初始化验收](verification/score-clock-dispatch-init/)。既有[队列／普通 PCM](verification/score-clock-request-queue/)、[稳定运行整拍](verification/score-clock-running-tick/)和声部检查继续通过。01／05／04 控制指令、非 idle fade／pause、world IRQ、请求生产时刻、联合音频 checkpoint 和平台播放仍待完成。

## 当前可运行范围

- 从空对象和明确配置创建对局，支持 2／3／4 人混战、4 人组队，以及真人与显式电脑混合。当前开放来源 ID 0「幻海」、1「藏马」、2「桑原」、3「幽助」、4「飞影」、5「仙水」、6「阵」、7「树」、8「户愚吕兄」、9「酎」和 10「户愚吕弟」，场地开放默认场地、灵界、亚空间 1、魔界、洞窟、魔性之森、亚空间 2 和山顶。
- 独立 Go／Wasm 规则驱动输入、真实原版 AI、动作、子对象、特效、碰撞、退场、判胜和清场；运行时不读取 ROM、不执行模拟器，不逐帧载入参考状态。
- 独立索引素材呈现人物、特效、动态背景、色盘、滚动、原版姓名／头像／生命与能量条。浏览器页复用公共 MD 手柄，支持键盘和触屏；当前页面没有接入声音播放。
- 资源解码、Wasm 和 Canvas 就绪后才开始推进。存档保留规则状态、图块映射与呈现缓存；出错时不提交半帧状态。

正式房间的加载死锁已修复。原约 8.8 帧／秒的客户端开销来自完整 JSON 分叉、重放中间像素和未使用的预测摘要；改为直接复制状态、仅推进中间规则状态后，恢复正常双人约 60 帧／秒。随后对照篮球修正本地显示时钟，保持 12 帧预测上限，确认后重放到旧显示目标。最终真实双人可见更新为 59.94／59.93 次／秒；单端每次 TCP 写入延迟 120 ms 时，慢端 55.91 次／秒、帧间隔 p95 为 31.7 ms，原生按键到画布变化 27.8 ms，先于 226.4 ms 的 ACK。完整证据见 [native-clock-network04](verification/platform-app-browser/native-clock-network04/result.json)。此为明确网络条件下的本机实测，不代表所有丢包、设备或网络环境。

同步机制已对照热血篮球、泡泡堂与魂斗罗的实际源码，结论和规则回馈见[同步对照](../../docs/research/md-yuyuhakusho/SYNC_REFERENCE.md)。普通 WS 只发该连接新增输入与最多六帧冗余，120 帧恢复环单独保存；不携完整世界、呈现缓存或像素。原生密集 View 峰值从 2513 B 降为 997 B；最终 [cursor-network06](verification/platform-app-browser/cursor-network06/result.json) 正常 59.95／59.92 帧／秒、延迟慢端 57.97 帧／秒，慢端原生按键约 22.1 ms 响应，早于约 210.3 ms ACK。横屏控制器下置和真实触摸见 [landscape-app08](verification/platform-app-browser/landscape-app08/result.json)。

启动方式和截图见[本地研究页](../../docs/research/md-yuyuhakusho/LOCAL_PLAYER.md)。本轮完整摘要见[八场地整体验收](verification/additional-arenas-session.json)，接口、证据边界和复跑见[七场地接入](../../docs/research/md-yuyuhakusho/ADDITIONAL_ARENA_SESSIONS.md)。上一检查点的[全十一角色对局验收](verification/itsuki-session.json)；既有[十角色验收](verification/sensui-session.json)、[九角色验收](verification/hiei-session.json)、[八角色验收](verification/jin-session.json)、[七角色验收](verification/kurama-session.json)、[六角色验收](verification/elder-session.json)、[五角色验收](verification/kuwabara-session.json)、[四角色验收](verification/yusuke-session.json)、[三角色验收](verification/genkai-session.json)、[双角色验收](verification/fighter10-session.json)和[独立对局／素材验收](verification/independent-session.json)保留各自提交时的绑定。

姓名订正：此前把来源 8／10 的兄弟标注读反。重新放大原 HUD 字形并核对造型后，确认为 **8「兄」、10「弟」**；当前界面、[姓名映射](../../docs/research/md-yuyuhakusho/CHARACTER_IDS.md)和本页已修正，旧阶段报告按历史提交保留。角色编号、规则、原始 CPU 证据与素材均未变，名称修复另有先行界面失败与通过结果。

## 已验证证据

| 项目 | 证据 | 当前边界 |
| --- | --- | --- |
| 原版运行与恢复 | 固定日版 ROM、内核和输入；3,600 帧冷启动基线；恢复 sidecar 补齐原内核未保存的 `m68k.refresh_cycles` 后，1,200 帧 RAM／寄存器／视频一致 | 原版工具仅用于开发对照；不把模拟器源码许可视为游戏素材许可 |
| 输入、共享规则与完整人物 | 11 个人物共用 input／action／actor 驱动；树的四 selector、五专属 MAIN、三个子回调及跨父体停顿／生命视图已恢复，14 个玩家可达特殊根自然成功 | 人物已全部开放；人物本身的原版对照范围与八场地新建对局矩阵分别验收；R4 无玩家命令绑定仅私有证明，自然回血分支未产生生命增量，见[树世界](engine/FIGHTER7_WORLD_VERIFICATION.md)和[组合](engine/FIGHTER7_COMPOSITION_VERIFICATION.md) |
| 幻海完整人物与父子对象 | 四 selector、12 特殊构造、五持续槽、散射弹／释放光效；完整三层各 4,133 次自然调用；原 CPU 私有与真实动画流通过 | `15/16/19` 自然仍未覆盖，私有／连续已覆盖；该阶段自然 AI 仅为酎，见[幻海世界与完整组合](engine/FIGHTER0_WORLD_VERIFICATION.md) |
| 全角色准备、AI 与素材目录 | 11 个来源角色的命令准备、动画／形状目录及真实 AI；AI 新增 37,056 次隔离调用、1,794 次酎自然调用和 33 条连续流 | 自然原 AI 调用与完整独立世界现覆盖酎／阵；其余来源仍有隔离／全新对局证据，全角色素材不能代替招式验证，见 [AI 验证](engine/FIGHTER_AI_VERIFICATION.md) |
| 开局、覆盖层与场地 | 八场 camera／滚动／色盘规则；ID 4 三色循环、ID a 子对象六条即时精灵事件恢复；七场合法菜单各 3,602 帧完整复验 | 原菜单其他赛制未自动开放；场地单回调和完整轮次分别对照，见[场地世界](engine/ARENA_WORLD_SESSIONS_VERIFICATION.md)与[合法菜单](../../docs/research/md-yuyuhakusho/ARENA_MENU_CONTROLS.md) |
| 唯一对象状态与完整轮次 | 二十三条原版流合计 58,739 轮、768,187 回调逐轮及单初态连续一致；树新增 6,844 轮、91,803 回调，特殊场景另比较 24,968 个有序 Draw | 普通树场景证明4944清场，特殊场景只证明保留区间；真实世界4个状态及三类 callback 各8个状态恢复通过，见[树世界](engine/FIGHTER7_WORLD_VERIFICATION.md) |
| 帧前后阶段与 IRQ | 6,576 次自然前后调用、1,440 次隔离、4 流 320 次连续调用；另从冷启动复跑 2,400 视频步取得相同 RAM IRQ | 视频标签不能当作逻辑 tick；硬件采样的 F554 游标明确在边界外，见[帧验证](engine/WORLD_FRAME_VERIFICATION.md) |
| 全新独立对局 | 新增七场每环境 35 组规则对局、28 组完整呈现，覆盖每场全部 11 人物及 2／3／4 人、组队；原生／Wasm 完整状态与像素轨迹一致 | 165 份默认规则及 14 份默认呈现报告在两端保持既有世界／事件／生命周期与像素，共 358 份，见[整体验收](verification/additional-arenas-session.json) |
| 世界精灵 | 1,500 图形键、34,520 去重图块、12,238 原 CPU 提交；83 个自然帧的 355,914 个高优先级精灵像素零差异 | 有界普通显示模式；逐扫描线溢出与影亮模式未模拟，见[精灵呈现](../../docs/research/md-yuyuhakusho/WORLD_PRESENTATION.md) |
| 八场背景与 HUD | 新增 140 原样本、9,535,530 个背景像素零差异；魔性之森最终合成 1,368,854 像素及 29,063 个 aux 可见像素逐位置相等；共享 HUD 独立重建沿用原验证 | ID 4 空白行未使用颜色 9～11 的相位不同，实际可见色与像素相等；aux 原样本窗口仍是参考 adapter，独立 HUD 由新建对局另验，见[呈现](presentation/ARENA_PROFILES_VERIFICATION.md)和[附加精灵](presentation/ARENA_A_AUX_PIXELS_VERIFICATION.md) |
| 完整呈现与存档 | 七场普通对局恢复后各 30 帧全等；合法真人输入触发每场 232 帧非零 Pan，非零存档恢复后 360 帧规则和画面全等；默认六份旧 JSON 字节兼容 | schema 1／2／3 无场地扩展仍只开放默认；新增成对扩展严格校验；背景与 aux 摘要绑定存档，见[动态 Pan](verification/ARENA_PAN_VERIFICATION.md) |
| 浏览器与 MD 控制器 | 七个新增场地逐一实际呈现；魔性之森原生键盘移动、320 px 三指触屏／取消通过；场地按需加载，选择草稿不改变正在运行的对局 | 本地页未接平台房间／网络；见[当前浏览器结果](verification/additional-arena-player/final/result.json)与[控制器记录](../../docs/verification/md-controller/README.md) |
| 音频目录与平台通路 | 134 个有限 WAV 全量 Go／Wasm 校验；四个可重入调度阶段与 PCM 选择通过原调用；平台有限 lane 的 75 项低层检查、8 项新接口和 10 项旧接口真实媒体 E2E 通过；Chrome 无损分片承载跨 11 个边界 | 实际对局仍未接声音；回调内调度、原双 PCM／FM／PSG 混音、独立 DSP 与平台连续流仍待完成，见[音频阶段](../../docs/research/md-yuyuhakusho/AUDIO_FOUNDATIONS.md) |

本轮原生竞态和 Wasm 各完成 **445 项顶层检查**：404 项引擎、22 项呈现、3 项 bridge 与 16 项目录／素材命令检查。引擎按 8 片运行，所有分片退出码为 0。`go vet` 和本地页 TypeScript 检查通过。首轮每端 63 个普通完整对局均通过，但未触发非零 Pan，最后专项断言真实失败；保留该日志后，新增每端 7 个合法双真人输入专项，在非零反馈、暂停和恢复全部通过后仅续汇总，未删除动态断言或重采 original。原生／Wasm 的 126 个完整对局、14 个 Pan 专项和 358 份历史报告由[整体验收](verification/additional-arenas-session.json)绑定，没有运行其他游戏矩阵。

2026-11-06 客户端共享状态验收：精确 raw／gzip 身份、唯一 `renderstate.State`、schema 1 字节兼容、原样 checkpoint 和可丢弃预览已通过；原生／Wasm 各 49 条冻结轨迹共各 203,428 帧、选定完整竞态和 PC／320px 原生输入验证通过。见[客户端说明](../../docs/research/md-yuyuhakusho/CLIENT_RUNTIME_SCHEMA2.md)、[绑定摘要](runtimebridge/testdata/schema2-verification.json)和[浏览器结果](verification/client-bridge-schema2/result.json)。本条仅更新客户端范围，不替代平台网络及音频验收。

终局订正：独立重放 49 个端点，其中 42 个自然终局全部在 `0x8040` 阶段 26 清场、淡出后结束；恢复 `Preview` 与真实终止帧像素差异为 0，帧号保持 F。7 个非终局 pan 端点仍预测 F+1。此前由空事件推断「恢复后丢失角色」已撤回，未新增持久绘制历史、像素或近似状态。见[终态实证](verification/client-terminal-preview/result.json)与[订正记录](../../docs/research/md-yuyuhakusho/CLIENT_TERMINAL_PREVIEW_CORRECTION.md)。

## 后续工作

当前已完成项和剩余项统一维护在[待办清单](../../docs/research/md-yuyuhakusho/TODO.md)。下一项为待消费新请求、dispatch／header 与非 idle fade／pause 的完整 Root 时序，随后处理 world IRQ、联合音频 checkpoint 和平台播放器。稳定运行拍按明确入口条件验收，实际对局音频仍需整场验证。

其余赛制继续按平台最终拟开放范围研究。不可变资源与应用发布须在对应的人物／场地／模式、托管、PC／320 px 触屏及网络验收完成后进行。

## 复跑与工作区

从本 worktree 根目录运行[研究工具说明](../../scripts/yuyuhakusho/README.md)中的命令。ROM、模拟器、原始内存、临时构建和音频媒体保存在 `.local/md-yuyuhakusho/`；交付保留必要来源、脚本、摘要、夹具、截图与验收报告。不得清理仍在使用的 worktree、原工具／素材或 main 的其他工作。

上一检查点清理了 27,572,183 bytes 的早期对局报告、CLI 构建／浏览器 profile 及过期日志，见[清理记录](verification/independent-session-cleanup.json)。本轮另清理父任务重复探针、验收构建、浏览器 profile 和被最终结果替代的日志共 28,076,393 bytes，见[双角色清理记录](verification/fighter10-cleanup.json)；各子任务的清理范围独立保留。运行中的本地页、原版工具／素材、下一角色原版发现和共享 Go 缓存继续供未完成工作使用。

幻海阶段另清理父任务 10 个已完成探针／构建／浏览器缓存，共 33,490,827 bytes，见[本轮清理记录](verification/genkai-cleanup.json)。保留规则与原世界夹具、24 份实际旧角色报告的压缩归档、最终日志、截图、原工具和运行中的试玩页。

幽助阶段清理父任务 12 个已完成探针／CLI 构建／浏览器缓存及过期初跑，共 33,156,043 bytes，见[幽助清理记录](verification/yusuke-cleanup.json)。保留 42 份实际旧角色报告、原世界／字节夹具、原始红灯、最终日志、截图和正在运行的试玩页；共享 Go 缓存继续供完整目标使用。

桑原阶段清理父任务 15 个重复发现、CLI 构建、已关闭的浏览器 profile 和未采用输入，共 42,482,438 bytes，见[桑原清理记录](verification/kuwabara-cleanup.json)。四个子任务的清理记录各自保存。保留 64 份实际旧角色报告、完整原 CPU 诊断与规范夹具、最终日志、截图、固定原版工具和运行中的试玩页。

来源 8 阶段清理父任务 21 个菜单／取证副本、临时构建、已关闭的浏览器 profile、被新姓名结果替代的界面输出及私有 Ghidra 项目，共 52,414,434 bytes，见[来源 8 清理记录](verification/elder-cleanup.json)。保留 90 份旧对局实际报告、48 份重命名但内容未变的新对局报告、姓名订正的原字形／红灯／最终截图、规范 CPU 夹具与固定原工具；共享 Go 缓存和 8799 试玩继续使用。

藏马阶段清理父任务 14 个发现／采集副本、旧实现回放 clone 及其只读链接、临时 CLI 构建和已关闭的浏览器 profile，共 43,687,757 bytes，见[藏马清理记录](verification/kurama-cleanup.json)。保留 120 份旧对局实际报告、规范 CPU 数据、固定输入、首帧边界纠正与旧版本回归证据、最终日志／截图、原工具及活动试玩页；共享缓存继续供完整目标使用。

阵阶段清理父任务 14 个发现、原世界副本、临时 CLI 构建及已关闭浏览器 profile，共 45,673,802 bytes，见[阵清理记录](verification/jin-cleanup.json)。保留 154 份旧对局实际报告、三条原世界规范数据、真正电脑阵及 Request4 的有界原版证据、静态等待字节、最终日志和截图；固定原工具、共享缓存与活动 8799 试玩保留。

飞影阶段另清理父任务 230 个临时／重复文件，共 60,568,341 bytes，见[飞影清理记录](verification/hiei-cleanup.json)。保留三条原世界的规范夹具与控制组、全部初始失败和最终结果、192 份旧对局结果、截图与当前试玩页。子任务各自清理记录继续保存；共享 Go 缓存与原版工具保留供仙水、树及后续平台接入使用。

仙水阶段另清理父任务 232 个临时／重复文件，共 48,711,665 bytes，见[仙水清理记录](verification/sensui-cleanup.json)。保留原世界／控制组、组件冻结证据、全量与续跑日志、234 份旧对局结果和截图。活动试玩页、原工具、共享 Go 缓存与树的后续工作继续保留。

树与共享存储修复阶段另清理父任务283个临时／重复文件，共57,585,144 bytes，见[本轮清理记录](verification/itsuki-cleanup.json)。保留原CPU／输入控制、音频v1/v2真实失败链、光栅owner先行测试与兼容JSON、全部最终分片／续跑日志、280份旧对局结果、截图及当前试玩页。各子任务清理范围独立保留；原工具、共享Go缓存和活动worktree继续用于后续七场地与平台接入。

七场地阶段另清理父任务 12 个临时路径，共 51,579,236 bytes，见[场地清理记录](verification/additional-arena-cleanup.json)。保留原版控制、完整 CPU 与像素夹具、原始失败／最终日志、新场地截图、非零震动存档和活动 8799 研究页；共享 Go 缓存、工具与 worktree 继续用于音频和平台接入。

音频基础阶段的当前范围与机器摘要见[音频阶段验收](verification/audio-foundations-session.json)。本地页仍无对局播放；整体进度保持 81%，未以有限声道或原生载体原型替代完整音频目标。

音频基础阶段另清理父任务已关闭的媒体验收 profile、临时文件和可再生成 JS，共 10,370,690 bytes，见[清理记录](verification/audio-foundations-cleanup.json)。所有原 WAV、原版对照、真实 red／最终结果和活动研究页保留；正在进行的调度预算与混音研究未清理。

持续音频承载阶段新增纯 Go FLAC/fMP4 编码器、平台 `openStream` 和 DPCM 算术。原生／Wasm 编码逐 byte 相同；平台 83 项低层检查、5 组真实连续流验收及 18 项有限／旧接口兼容通过。12 秒共 517 片，真实跨越 516 个边界；首播需要约 209 ms 数据，主流前瞻维持约 260 ms。DPCM 算术与原 41,345 次 DAC 输出值相同，但自主事件时间、完整滤波／FM／PSG 合成和对局接入仍未完成。见[持续流验收](verification/audio-stream-session.json)及[平台流说明](../../docs/research/md-yuyuhakusho/AUDIO_PLATFORM_STREAM.md)。整体保持 81%。

持续流阶段父任务另清理已关闭的连续流／有限媒体 profile、临时目录和可再生成 bundle，共 19,011,443 bytes，见[持续流清理记录](verification/audio-stream-cleanup.json)。完整音频包 16 项原生竞态／Wasm 检查（包含实际 134 WAV）均无跳过；早先未设置媒体目录的一次 15 项加 skip 日志仍保留，最终摘要只绑定全量结果。
