# 《幽游白书：魔强统一战》复刻进度

更新：2026-11-05。整体约 **48%**（工程估算）。独立 worktree 为 `worktrees/yuyuhakusho`，分支为 `codex/yuyuhakusho`。当前已有可操作的酎、户愚吕兄／默认场地本地研究页；游戏尚未注册到平台，未上传资源、发布或部署。main 工作区的其他改动未导入或覆盖。

## 当前可运行范围

- 从空对象和明确配置创建对局，支持 2／3／4 人混战、4 人组队，以及真人与显式电脑混合。当前开放来源 ID 9「酎」和 10「户愚吕兄」，场地只开放默认场地。
- 独立 Go／Wasm 规则驱动输入、真实原版 AI、动作、子对象、特效、碰撞、退场、判胜和清场；运行时不读取 ROM、不执行模拟器，不逐帧载入参考状态。
- 独立索引素材呈现人物、特效、动态背景、色盘、滚动、原版姓名／头像／生命与能量条。浏览器页复用公共 MD 手柄，支持键盘和触屏；当前页面没有接入声音播放。
- 资源解码、Wasm 和 Canvas 就绪后才开始推进。存档保留规则状态、图块映射与呈现缓存；出错时不提交半帧状态。

启动方式和截图见[本地研究页](../../docs/research/md-yuyuhakusho/LOCAL_PLAYER.md)。本轮完整摘要见[双角色对局验收](verification/fighter10-session.json)；上一提交的酎研究页与素材验证见[独立对局验收](verification/independent-session.json)。

## 已验证证据

| 项目 | 证据 | 当前边界 |
| --- | --- | --- |
| 原版运行与恢复 | 固定日版 ROM、内核和输入；3,600 帧冷启动基线；恢复 sidecar 补齐原内核未保存的 `m68k.refresh_cycles` 后，1,200 帧 RAM／寄存器／视频一致 | 原版工具仅用于开发对照；不把模拟器源码许可视为游戏素材许可 |
| 输入、共享规则与完整人物 | 酎保留完整验证；户愚吕兄新增四选择器、七构造、五持续槽与共享 input／action／actor，三层各 2,065 次自然返回和私有完整组合通过 | 剩余九个角色未实现完整动作，见[完整组合](engine/FIGHTER10_COMPOSITION_VERIFICATION.md)、[选择器](engine/FIGHTER10_ATTACK_VERIFICATION.md)和[持续动作](engine/FIGHTER10_MOTION_VERIFICATION.md) |
| 全角色准备、AI 与素材目录 | 11 个来源角色的命令准备、动画／形状目录及真实 AI；AI 新增 37,056 次隔离调用、1,794 次酎自然调用和 33 条连续流 | 自然 AI 证据仅覆盖酎；全角色 AI／素材覆盖不代表全角色招式可玩，见 [AI 验证](engine/FIGHTER_AI_VERIFICATION.md) |
| 开局、覆盖层与场地 | 开局阶段 0／2、14 类覆盖层、默认场地 camera／滚动／色盘／天空对象恢复；自然、隔离与连续流通过 | 其他七个场地及原菜单／全部赛制没有开放，见[开局](engine/MATCH_START_VERIFICATION.md)、[覆盖层](engine/MATCH_OVERLAYS_VERIFICATION.md)和[场地](engine/DEFAULT_ARENA_VERIFICATION.md) |
| 唯一对象状态与完整轮次 | 既有两条酎流加新增混合人物流，合计 7,547 轮、99,204 次回调逐轮及单初态连续一致；新流在原视频标签 4,590 清场 | 比较明确排除硬件呈现缓存／DMA 游标；独立音频清槽差异留证，见[双角色世界](engine/FIGHTER_WORLD_VERIFICATION.md) |
| 帧前后阶段与 IRQ | 6,576 次自然前后调用、1,440 次隔离、4 流 320 次连续调用；另从冷启动复跑 2,400 视频步取得相同 RAM IRQ | 视频标签不能当作逻辑 tick；硬件采样的 F554 游标明确在边界外，见[帧验证](engine/WORLD_FRAME_VERIFICATION.md) |
| 全新独立对局 | 新增七种双角色配置均从新世界完成伤害→KO→退场→判胜→清场，原生／Wasm 轨迹一致；旧酎五组双运行时报告逐字节未变 | 覆盖代表性的 2／3／4 人、组队、真人静止输入混合，不证明平台网络，见[世界与完整对局](engine/FIGHTER_WORLD_VERIFICATION.md) |
| 世界精灵 | 1,500 图形键、34,520 去重图块、12,238 原 CPU 提交；83 个自然帧的 355,914 个高优先级精灵像素零差异 | 有界普通显示模式；逐扫描线溢出与影亮模式未模拟，见[精灵呈现](../../docs/research/md-yuyuhakusho/WORLD_PRESENTATION.md) |
| 默认背景与 HUD | 20 原帧背景 1,354,388 像素、完整图层合成 1,433,600 像素零差异；空 HUD 加原资源和有序事件重建的 91,756 窗口像素零差异 | 这些原帧对照与全新对局的跨运行时一致性分别验收；三人使用原槽 0／1／3，见[背景](presentation/ARENA_VERIFICATION.md)和[HUD](presentation/HUD_VERIFICATION.md) |
| 完整呈现与存档 | 新增户愚吕兄单角色、双角色混合、四人混战和组队共四种完整呈现，原生／Wasm 状态和像素轨迹一致，恢复后 30 帧全等 | 两个已开放人物／默认场地，见[完整对局与呈现矩阵](verification/fighter10-session.json) |
| 浏览器与 MD 控制器 | 可选择酎或户愚吕兄；读取真实 tag／输入通道／动作验证后退 `11/0c`，键盘、三指触屏、取消、后台释放、桌面／320px 通过 | 本地研究页没有平台房间／网络；公共 MD 配置的既有验收见[控制器记录](../../docs/verification/md-controller/README.md) |
| 音频采集 | 134 个有限 WAV，覆盖动画使用的 78 个声音编号，双次完整 PCM 一致；另确认 `82/87` 的完整音序周期 | 精确 PCM 循环、原版混音和平台播放未完成，见[音频采集](../../docs/research/md-yuyuhakusho/AUDIO_RESEARCH.md)与[循环研究](../../docs/research/md-yuyuhakusho/AUDIO_LOOPS.md) |

本轮规则与组合原生竞态和 Wasm 各 **158 项**顶层测试通过，`go vet`、Wasm 入口检查及本地页 TypeScript 检查通过。原生全包初次触及默认 10 分钟时限，随后拆成 78／77 项两组并另跑 3 项动画目录检查，覆盖并集已与 Wasm 逐项核对；超时记录不计通过。上一阶段 27 项素材／呈现检查的代码与资源未变。当前源码、原资料、前置失败和结果由[双角色验收摘要](verification/fighter10-session.json)绑定，没有运行其他游戏矩阵。

## 后续工作

1. 继续恢复其余九个角色，下一步优先已由原版 HUD 确认为幻海的来源 ID 0，恢复[四选择器、五个持续槽和两种子对象](../../docs/research/md-yuyuhakusho/FIGHTER0_PLAN.md)。原[动作路线图](../../docs/research/md-yuyuhakusho/FIGHTER_ACTION_ROADMAP.md)保留 `149cc02d` 时的缺口；其中来源 10 已在本轮补齐。
2. 恢复其他场地及拟开放赛制，补齐其动态背景、资源与完整对局证据。
3. 恢复声音播放时基、BGM 循环和原版混音语义，再接入平台 `BoardProps.audio`。
4. 接入统一房间准备、席位／角色映射、显式电脑和主动托管、公共加载、MD 控制器与全屏。
5. 完成权威同步、本地预测、回滚、输入确认、慢连接隔离及真实操作延迟验收；常规网络仅传必要输入和确认，存档与网络负载分离。
6. 验收所有开放人物／场地／模式、恢复存档、托管、PC／320px 触屏和网络干扰，再准备不可变资源及应用发布。

## 复跑与工作区

从本 worktree 根目录运行[研究工具说明](../../scripts/yuyuhakusho/README.md)中的命令。ROM、模拟器、原始内存、临时构建和音频媒体保存在 `.local/md-yuyuhakusho/`；交付保留必要来源、脚本、摘要、夹具、截图与验收报告。不得清理仍在使用的 worktree、原工具／素材或 main 的其他工作。

上一检查点清理了 27,572,183 bytes 的早期对局报告、CLI 构建／浏览器 profile 及过期日志，见[清理记录](verification/independent-session-cleanup.json)。本轮另清理父任务重复探针、验收构建、浏览器 profile 和被最终结果替代的日志共 28,076,393 bytes，见[双角色清理记录](verification/fighter10-cleanup.json)；各子任务的清理范围独立保留。运行中的本地页、原版工具／素材、下一角色原版发现和共享 Go 缓存继续供未完成工作使用。
