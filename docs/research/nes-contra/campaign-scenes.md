# 日版关前、入场、出口与关间演出

`campaign_scene*.go` 实现日版八关的 Intro、Entrance、EndSequence 和 Interlude。实现使用独立的场景状态、现有角色／图形状态和从 ROM 静态数据解码的资源；运行时不执行 6502 指令，不加载 ROM，也不把录制结果回填到规则状态。

本项验证范围是原版函数、完整场景阶段和直接背景资源。平台房间、完整 World 调度、浏览器呈现以及自然八关通关由对应集成验收负责，不能用本项记录代替。

## 原版证据与覆盖

固定原版 SHA-256 为 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。源 ZIP 只读；提取、录制和 REA 的独立会话都保存在当前工作区内。

| 场景 | 日版入口 | 对照范围 |
| --- | --- | --- |
| Intro | bank 10/11 `$82D7` | 八关 4,893 次正常入口调用，加 332 组生命、分数、人数与跳过输入，共 5,225 次 |
| Entrance | 固定区 `$DB1C` | 八关 334 次连续调用；每关只初始化一次 before，之后连续运行，不回填 after |
| EndSequence | bank 10/11 `$888B` | 首关 674 次连续调用，加八关 736 组输入边界，共 1,410 次 |
| Interlude | bank 8/9 `$9E22` | 首关出口后 701 次调用，加八关入口录制，共 6,623 次 |
| 直接背景 | 固定区 `$C93A` | 4 份静态背景、15 次真实 PPU 写入结果逐字节一致 |

首关 Intro／Entrance 从开机和真实控制器按键开始，没有 RAM 修改。其他关的同类录制只在 `$CC5F` 前改变一次关号，随后不干预。专用出口／关间录制使用明确记录的场景输入起点；它们不是自然击败 Boss 的证据。主 World 的真实战斗录制另行维护。

9 组重复录制的完整 RAM 流逐字节相同。REA 的 80 个函数证据保存在 [native 证据归档](evidence/campaign-scene-native.json.gz)，完整索引、摘要、测试名称和 SHA-256 见 [验证记录](evidence/campaign-scene-verification.json)。静态分析只能说明代码结构；行为结论同时由真实原版调用和 Go 对照约束。

## 调度契约

| 接口 | 前置阶段 | 方法负责 | 调用方随后执行 |
| --- | --- | --- | --- |
| `StepIntro` | `LevelRoutine=1` | 调色、关卡／分数／生命文字、地图滚动与揭示、提示动画；真正结束时 1→2 | 应用场景结果，再调用 `Campaign.AfterPresentation` |
| `StepCampaignEntrance` | `LevelRoutine=3` | 实际地形列／行、碰撞与图形队列写入，返回真实完成条件 | 完成后交给 `Campaign.AfterPresentation` 做 3→4 |
| `StepEndSequence` | `LevelRoutine=9` | 清除输入并产生自动移动／跳跃，维护角色出口步骤及等待，真正结束时 9→10 | 继续真实 Combat 和出口敌人阶段 |
| `StepInterlude` | `LevelRoutine=10` | 说话／文字／粒子／CHR 动画与渐隐，真正结束时 10→5 | 应用场景结果，再交给 Campaign 的阶段 5 推进 |

这些方法不增加 World 帧号，也不提交下一次显示边界。Intro／Interlude 通过 `CampaignSceneResult` 返回即时背景、按顺序排列的声音和显示控制。共享 `Palette`、`Patterns`、`GraphicsQueue`、角色与逻辑对象仍由 `ActiveWorld` 唯一持有。Intro／Interlude 的失败回滚覆盖整个场景及可变队列载荷。

`StepInterlude` 读取 `CampaignSceneEnvironment{Busy, DisplayMask}`，这两个值来自 World 的显示状态；场景不另存副本。Intro 跳过使用 P1 的 **Pressed Start**，Interlude 跳过使用 **Held Start**。

## 显示控制的含义

| Control Kind | 原版效果 | 接线要求 |
| --- | --- | --- |
| `graphics-mode` | `$07EB` | 更新唯一的图形模式；兼容现有 `Flicker` 投影 |
| `display-busy` | `$20` | 由显示边界维护忙碌期限，不能由客户端动画完成回调清零 |
| `display-disable` | 当前 `$2001=0` | 关闭当前呈现；不把它误写成期望的 `$FE` 掩码 |
| `ppu-control` | `$FF` 与 `$2000` | 使用现有 `World.Scroll.NameTable` 完整控制字节 |
| `display-mask` | `$FE` | 改变后续显示边界使用的掩码 |
| `split-scroll` | `$F968` 中段滚动 | 使用 Intro 的独立滚动值；不覆盖普通镜头 `Scroll.X` |
| `raster-mask` | 关间演出中段 `$2001` | 只作用于该次分段呈现，不修改 `$FE` |
| `raster-control` | 关间演出中段 `$2000` | 只作用于该次分段呈现，不修改 `$FF` |

`ImmediateBackground` 对应原版关闭显示后直接写入的完整背景，不能塞进容量 192 字节的普通延迟队列。静态数据解码器的输出已与原版 `$C93A` 返回时的 PPU 内容对照，见 [背景证据](evidence/campaign-scene-background-original.json.gz)。Canvas／浏览器如何应用分段呈现仍须通过 World 和前端截图验证。

## 两种图形队列格式

原版 `$CB30` 根据 `$23` 选择整个提交的解析器：零值进入 `$CB37` 字串格式，非零进入 `$CBC0` 矩形格式。它们不能混在同一次队列提交中。

`GraphicsTextPacket` 的格式为 `[kind, addressHi, addressLo, tile..., FF]`；kind 1／2 的地址步长分别为 1／32。`GraphicsQueue.Mode` 使用语义枚举：`GraphicsText` 对应原版 `$23=0`，Go 零值 `GraphicsRectangular` 对应普通战斗的非零 `$23`。两者共用 `Used`、容量、终止符和调色板提交门槛。

FF 并不总是终止符。FF 后的字节至少为 6 时，FF 是图块数据；后续字节小于 6 时才进入下一条命令。编码保留这一成本差异，拒绝「图块 FF 后紧跟 0..5」这种字串格式无法无歧义表达的序列。末尾图块 FF 与终止 FF 会形成两个连续 FF。

## 状态所有权与清理

出口的 `$FB` 是 `Scene.ExitFadeSteps`，不是调色板加载计时。真正的 `Palette.FadeTimer` 对应 `$74`。源码对照发现并修复了这两者混用的问题，保留了双值 canary 的 red／green；Interlude 每帧在 `$9E3B` 清除的也是 `$74`。

Interlude 临时占用原版已经退出战斗的 `$40..$4B` 头部空间。独立代码将其中的渐隐阶段、说话者、文字地址等放入 `Scene.Interlude`。例如 `$41` 在该阶段是 `FadeDelay`，不是游戏地图的 `ScrollType`。原始字节对照必须按当前阶段的语义投影，不能把这些别名写入另一套玩法字段。方法返回阶段 5 的当帧仍保留这些局部值；下一帧的原版清理才改变归属。

| 原版清理 | 应清理 | 应保留 |
| --- | --- | --- |
| `$C307`／关卡推进 | `$40..$EF` 和 `$0300..$06FF`；包括 IntroScroll、Reveal、Interlude 局部字段 | ExitDelay／Budget／Actors（`$0190..$0193`）、ExitFadeSteps（`$FB`），以及既定生命／分数资源 |
| `$C328`／关间初始化 | `$0300..$05FF` 中的临时对象、弹丸和敌人字段 | 地图窗口 `$0600..$067F`、碰撞 `$0680..$06FF`、生命和分数 |
| `$C32E` | 先清 `$40..$4F`，再执行 `$C328` | 同上；调色板组随后由当前演出显式加载 |

Intro 的地图擦除还保留了一个原版进位细节：前半程第二次模板选择受 `$CAC5` 返回进位影响；后半程地址调整 `$8511` 会清掉该进位。不能统一按「连续两行」重写，否则原版队列字节会不同。

## 复查

原版输入矩阵、REA 与测试均保留脚本。运行名称必须唯一，录制拒绝覆盖旧目录。准备和分析只写当前工作区：

```sh
python3 -B scripts/contra/campaign_scene_prepare.py
node scripts/contra/campaign_scene_rea.mjs .local/contra-campaign-scene/exit-spec.json
node scripts/contra/campaign_scene_rea.mjs .local/contra-campaign-scene/interlude-spec.json
python3 -B scripts/contra/campaign_scene_assets.py
.local/board-art-env/bin/python -B scripts/contra/campaign_scene_capture.py --name scene-review-natural --kind natural
.local/board-art-env/bin/python -B scripts/contra/campaign_scene_cases.py --name scene-review-cases --kind intro
.local/board-art-env/bin/python -B scripts/contra/campaign_scene_background_capture.py --label review
source scripts/env.sh
go test -race ./games/contra/engine -run '^(TestCampaignScene|TestGraphics.*Text)' -count=1 -json
```

清理仅限本任务的失败分析、缓存、过时录制和可由压缩证据精确恢复的原始调用流；保留 ROM、状态输入、关键截图、完整原版 RAM、最终 REA dossier 和验收证据。具体路径及回收量记录在本项清理清单中。
