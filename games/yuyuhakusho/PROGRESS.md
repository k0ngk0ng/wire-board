# 《幽游白书：魔强统一战》复刻进度

更新：2026-11-05。整体约 **74%**（工程估算）。独立 worktree 为 `worktrees/yuyuhakusho`，分支为 `codex/yuyuhakusho`。当前已有可操作的幻海、藏马、桑原、幽助、飞影、仙水、阵、户愚吕兄、酎、户愚吕弟／默认场地本地研究页；游戏尚未注册到平台，未上传资源、发布或部署。main 工作区的其他改动未导入或覆盖。

## 当前可运行范围

- 从空对象和明确配置创建对局，支持 2／3／4 人混战、4 人组队，以及真人与显式电脑混合。当前开放来源 ID 0「幻海」、1「藏马」、2「桑原」、3「幽助」、4「飞影」、5「仙水」、6「阵」、8「户愚吕兄」、9「酎」和 10「户愚吕弟」，场地只开放默认场地。
- 独立 Go／Wasm 规则驱动输入、真实原版 AI、动作、子对象、特效、碰撞、退场、判胜和清场；运行时不读取 ROM、不执行模拟器，不逐帧载入参考状态。
- 独立索引素材呈现人物、特效、动态背景、色盘、滚动、原版姓名／头像／生命与能量条。浏览器页复用公共 MD 手柄，支持键盘和触屏；当前页面没有接入声音播放。
- 资源解码、Wasm 和 Canvas 就绪后才开始推进。存档保留规则状态、图块映射与呈现缓存；出错时不提交半帧状态。

启动方式和截图见[本地研究页](../../docs/research/md-yuyuhakusho/LOCAL_PLAYER.md)。本轮完整摘要见[十角色对局验收](verification/sensui-session.json)；既有[九角色验收](verification/hiei-session.json)、[八角色验收](verification/jin-session.json)、[七角色验收](verification/kurama-session.json)、[六角色验收](verification/elder-session.json)、[五角色验收](verification/kuwabara-session.json)、[四角色验收](verification/yusuke-session.json)、[三角色验收](verification/genkai-session.json)、[双角色验收](verification/fighter10-session.json)和[独立对局／素材验收](verification/independent-session.json)保留各自提交时的绑定。

姓名订正：此前把来源 8／10 的兄弟标注读反。重新放大原 HUD 字形并核对造型后，确认为 **8「兄」、10「弟」**；当前界面、[姓名映射](../../docs/research/md-yuyuhakusho/CHARACTER_IDS.md)和本页已修正，旧阶段报告按历史提交保留。角色编号、规则、原始 CPU 证据与素材均未变，名称修复另有先行界面失败与通过结果。

## 已验证证据

| 项目 | 证据 | 当前边界 |
| --- | --- | --- |
| 原版运行与恢复 | 固定日版 ROM、内核和输入；3,600 帧冷启动基线；恢复 sidecar 补齐原内核未保存的 `m68k.refresh_cycles` 后，1,200 帧 RAM／寄存器／视频一致 | 原版工具仅用于开发对照；不把模拟器源码许可视为游戏素材许可 |
| 输入、共享规则与完整人物 | 十个已开放人物共用 input／action／actor 驱动；仙水新增四 selector、七个 MAIN／支持槽、三个子对象，完整三层各 4,134 次自然返回通过 | 剩余树尚未完成。自然 `31→32` 仅见 variant2，其他变体保留原 CPU／真实时钟边界，见[仙水世界](engine/FIGHTER5_WORLD_VERIFICATION.md)、[组合](engine/FIGHTER5_COMPOSITION_VERIFICATION.md)、[MAIN](engine/FIGHTER5_MOTION_VERIFICATION.md) |
| 幻海完整人物与父子对象 | 四 selector、12 特殊构造、五持续槽、散射弹／释放光效；完整三层各 4,133 次自然调用；原 CPU 私有与真实动画流通过 | `15/16/19` 自然仍未覆盖，私有／连续已覆盖；该阶段自然 AI 仅为酎，见[幻海世界与完整组合](engine/FIGHTER0_WORLD_VERIFICATION.md) |
| 全角色准备、AI 与素材目录 | 11 个来源角色的命令准备、动画／形状目录及真实 AI；AI 新增 37,056 次隔离调用、1,794 次酎自然调用和 33 条连续流 | 自然原 AI 调用与完整独立世界现覆盖酎／阵；其余来源仍有隔离／全新对局证据，全角色素材不能代替招式验证，见 [AI 验证](engine/FIGHTER_AI_VERIFICATION.md) |
| 开局、覆盖层与场地 | 开局阶段 0／2、14 类覆盖层、默认场地 camera／滚动／色盘／天空对象恢复；自然、隔离与连续流通过 | 其他七个场地及原菜单／全部赛制没有开放，见[开局](engine/MATCH_START_VERIFICATION.md)、[覆盖层](engine/MATCH_OVERLAYS_VERIFICATION.md)和[场地](engine/DEFAULT_ARENA_VERIFICATION.md) |
| 唯一对象状态与完整轮次 | 二十一条原版流合计 51,895 轮、676,384 回调逐轮及单初态连续一致；仙水新增 4,917 轮、65,195 回调，特殊场景另比较 12,058 个有序 Draw | 普通仙水对战证明清场，特殊场景只证明保留区间；三个实际世界状态及三类 callback 各八个状态保存恢复通过，见[仙水世界](engine/FIGHTER5_WORLD_VERIFICATION.md) |
| 帧前后阶段与 IRQ | 6,576 次自然前后调用、1,440 次隔离、4 流 320 次连续调用；另从冷启动复跑 2,400 视频步取得相同 RAM IRQ | 视频标签不能当作逻辑 tick；硬件采样的 F554 游标明确在边界外，见[帧验证](engine/WORLD_FRAME_VERIFICATION.md) |
| 全新独立对局 | 新增 23 组仙水／十角色规则配置及 13 组完整呈现，原生／Wasm 全状态／像素轨迹一致；旧九角色 234 份报告保持世界／事件／生命周期 | schema3 仅增加 16 条去重 AN；索引媒体及已有曲线不变。代表性 2／3／4 人、组队和真人静止输入不证明平台网络，见[十角色验收](verification/sensui-session.json) |
| 世界精灵 | 1,500 图形键、34,520 去重图块、12,238 原 CPU 提交；83 个自然帧的 355,914 个高优先级精灵像素零差异 | 有界普通显示模式；逐扫描线溢出与影亮模式未模拟，见[精灵呈现](../../docs/research/md-yuyuhakusho/WORLD_PRESENTATION.md) |
| 默认背景与 HUD | 20 原帧背景 1,354,388 像素、完整图层合成 1,433,600 像素零差异；空 HUD 加原资源和有序事件重建的 91,756 窗口像素零差异 | 这些原帧对照与全新对局的跨运行时一致性分别验收；三人使用原槽 0／1／3，见[背景](presentation/ARENA_VERIFICATION.md)和[HUD](presentation/HUD_VERIFICATION.md) |
| 完整呈现与存档 | 仙水单角色、混合、三人、四人、组队十三组完整呈现；原生／Wasm 状态／像素一致，恢复后 30 帧全等；反射、跟随与父子 owner 另有原版状态恢复 | 十个人物／默认场地，schema1/2/3 继续可读；规则包共 4,813 条 AN，见[完整矩阵](verification/sensui-session.json) |
| 浏览器与 MD 控制器 | 可选十个已开放人物；实际 tag5 与前向攻击 `03/83`、键盘、三指触屏／取消、后台释放、桌面／320px 通过 | 本地研究页没有平台房间／网络；公共配置的既有验收见[控制器记录](../../docs/verification/md-controller/README.md) |
| 音频采集 | 134 个有限 WAV，覆盖动画使用的 78 个声音编号，双次完整 PCM 一致；另确认 `82/87` 的完整音序周期 | 精确 PCM 循环、原版混音和平台播放未完成，见[音频采集](../../docs/research/md-yuyuhakusho/AUDIO_RESEARCH.md)与[循环研究](../../docs/research/md-yuyuhakusho/AUDIO_LOOPS.md) |

本轮原生竞态和 Wasm 分别完成 **354 项规则／目录顶层检查**与 **27 项素材／呈现检查**。原生 351 项引擎测试按 88／88／88／87 四组执行，另有 3 项动画目录检查。Wasm 首次运行在 279 项通过后触及 20 分钟时限，剩余 75 项使用同一冻结源码与精确测试集合续跑通过；完整覆盖并集与原生一致，原始超时及启动参数限制记录保留。`go vet`、Wasm 检查和本地页 TypeScript 检查通过。源码、原资料、前置失败、完整结果与分段执行由[十角色验收摘要](verification/sensui-session.json)绑定，没有运行其他游戏矩阵。

## 后续工作

1. 继续恢复最后的来源 7「树」。仙水已完成全部动作与三类子对象；树的 selector／MAIN 静态依赖已整理，三个 callback 正文及跨人物停顿改写仍待原 CPU 与独立规则验证，见[恢复起点](../../docs/research/md-yuyuhakusho/SOURCE7_PLAN.md)。
2. 恢复其他场地及拟开放赛制，补齐其动态背景、资源与完整对局证据。
3. 恢复声音播放时基、BGM 循环和原版混音语义，再接入平台 `BoardProps.audio`。
4. 接入统一房间准备、席位／角色映射、显式电脑和主动托管、公共加载、MD 控制器与全屏。
5. 完成权威同步、本地预测、回滚、输入确认、慢连接隔离及真实操作延迟验收；常规网络仅传必要输入和确认，存档与网络负载分离。
6. 验收所有开放人物／场地／模式、恢复存档、托管、PC／320px 触屏和网络干扰，再准备不可变资源及应用发布。

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
