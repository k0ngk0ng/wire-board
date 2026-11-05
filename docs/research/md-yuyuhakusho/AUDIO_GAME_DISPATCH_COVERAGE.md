# 合法声音请求与分派覆盖审计

当前平台比赛流程的 11 个音乐 ID 已形成闭包，没有从现有 23 个 score 定义再请求原菜单音乐的路径。明确发现一个普通 PCM 目录缺口：飞影黑龙合法请求 `F0`，使用独立 descriptor／payload，不能替换成普通 `7A`。基线的 121 个普通 payload 加 6 个音乐 payload 之外还需这 1 个，统一资源至少包含 128 个非诊断 encoded payload。

本次只读审计以已提交的 `92f27271` 音序实现为检查点，未修改 production、冻结 schema 2、原 42 流期望或旧验证结果。机器结果见 [AUDIO_GAME_DISPATCH_COVERAGE_MANIFEST.json](AUDIO_GAME_DISPATCH_COVERAGE_MANIFEST.json)；资源字段索引见 [RESOURCES_INDEX](AUDIO_GAME_DISPATCH_RESOURCES_INDEX.json)，全部 256 个原 selected ID 与静态轨道路径见 [ORIGINAL_IDS](AUDIO_GAME_DISPATCH_ORIGINAL_IDS.json)。审计通过表示来源和闭包断言成立，不表示完整游戏声音已经接线或合成。

## 范围与证据等级

审计覆盖当前 engine 中的声音生产点、28 个已恢复人物／场地／特效／展示资源文件的声音字段、原 dispatcher 全部 256 个 ID、22 个不同音乐 header、12 个 primary header，以及已有原版观察。资源字段共出现 99 个非零 ID；与代码常量、受限计算、比赛音乐和 score 队列意图合并后，得到 139 个平台比赛候选请求 ID。

资源中的声音字段只证明相应已加载规则可以提供该请求，不自动证明每条动画都被真人自然触发。隔离动画原 CPU 的 488 次 queue 写、自然 5,500 帧的 109 次 RequestSound 入口请求／107 次 dispatcher 观察，以及黑龙原 CPU 的两次 `F0` queue 写分别保留，不合并成「全部正常游戏已遍历」。request、selected、实际控制执行与最终声音输出也分别表述。

## 平台比赛音乐闭包

项目使用平台准备流程直接开赛，跳过原版菜单。比赛来源如下：

| 来源 | 可请求 ID | 边界 |
| --- | --- | --- |
| 八场地 `MatchStart` 表 | `82/86/83/95/85/8A/88/8D` | 场地索引 0～7，服务端已限制合法选择 |
| rule 0 对手覆盖 | `87/84` | 原角色槽 4 分别为角色 10／7 |
| 结果／taunt | `8F` | `MatchLifecycle` 的受限角色与结果阶段 |
| 23 个 score 的嵌套队列意图 | 只有 `AB → 7D` | 已完整展开所有前缀／循环事件，未引入另一首音乐 |

合并后的音乐恰为冻结 catalog 的 `82/83/84/85/86/87/88/8A/8D/8F/95`。已查控制 `01/04/05` 是 fade／clear，不生成新的 music ID；其他 sound-control 子程序也没有以队列意图引入原菜单音乐。因此，现有比赛 score 和控制不会再引到 `8E/90/92/93/96`。

所有资源请求中，`80` 以上仅有 `A2/A3/A4/A5/A6/A9/AA`，均由现有 12 个 primary 定义覆盖。代码另有浦饭蓄力 `A6`、命中／落地／KO 和结果语音的受限低位 ID，以及黑龙 `F0`。明细、代码行与全部源 SHA 均在主 manifest 的 `engine_bindings`、`platform_fight_candidate_ids` 和独立资源字段索引中。

## 明确缺口：飞影黑龙 F0

[fighter4_projectile.go](../../../games/yuyuhakusho/engine/fighter4_projectile.go) 在 dragon head 的 `SoundClock == 0` 分支请求 `F0`，该计数按 256 环绕。已有 [原版 projectile fixture](../../../games/yuyuhakusho/engine/testdata/fighter4-projectile-oracle.json.gz) 也实际记录两次：单人声明输入第 3,481 帧、双人声明输入第 3,479 帧，head 回调把 `sound_queue` 从全零写成 `[F0,0,0,0]`。配套 manifest 已绑定原 core／观察 core、完整 RAM／视频基准和 fixture SHA；这不是仅凭 Go 推导的请求。

原 F0 路径先计算 `(code - F0) + 7A`，随后直接进入 descriptor 索引入口，跳过普通 `06..7F` 路径的减 6。因此 `F0` 使用索引 122，普通 `7A` 使用索引 116，两者不同。

F0 的 encoded DPCM4 长度为 6,670 bytes，SHA-256 为 `c6409ebd28c594205970069fbc3233241cf15dc08ce8b847a5fa118b3e1a5382`。它的 native rate 为 20，双槽模式、目标 B、优先级高两位为 128，不请求 duck。它与全部普通 descriptor 及音乐 DAC descriptor 没有 payload 内容重合。新 payload／command owner 已获知该缺口；本次没有补写冻结 catalog，也没有把私有 F0 播放样本冒充正常玩法完整音频验收。

## 原菜单真实音乐与交付边界

既有自然原版流程实际观察到当前 catalog 之外的 5 个音乐 ID：

| ID | 首次 RequestSound 帧 | 首次 selected 帧 |
| --- | ---: | ---: |
| `92` | 375 | 377 |
| `93` | 未见直接 request | 557 |
| `8E` | 835 | 836 |
| `96` | 1,465 | 1,466 |
| `90` | 4,969 | 4,970 |

这些属于原菜单／前端真实流程，不能称作纯诊断音乐。静态轨道有 `92` 的 FM4 在 quanta 142 请求 `93`，以及 `93` 的 FM5 在 quanta 786 请求 `7D`，为没有 RequestSound 入口记录仍出现 selected 提供了来源路径；不把静态关联冒充唯一实时归因。

平台准备页直接进入比赛，不重建这些原菜单，也不为覆盖这些音乐而新增菜单 UI。比赛闭包已独立证明没有引入这 5 个 ID；原版完整菜单与后续未恢复流程仍作为明确边界保留。

整个原音乐表有 31 个可分派 ID、22 个不同 header：`81..96` 各有不同 header，`97..9F` 与 `81` 共用；`81` 为 9 轨 F2 stub，但仍执行初始化。其他未在自然窗口观察的 header 不能仅凭表存在归为诊断，也不能仅凭未出现就断言正常流程永远不可达。

## Group2、E9、FM3 与 pitch envelope

| 项目 | 已证明结论 | 没有据此声称的结论 |
| --- | --- | --- |
| Group2 `AC..EF` | dispatcher 只将该范围送入 secondary handler，首 guard 却要求低 byte 无符号小于 `AC` 才初始化；正常输入交集为空，均在初始化前返回 | 不是仅因 42 流未到达而忽略；也未完整排除 computed／动态绕 guard 的其他入口 |
| Group2 名义表 | guard 后有 F2-only 和 primary 别名结构 | 表存在不等于正常分派执行了 claim／keyoff；不能为本来不进入的结构添加音效 |
| E9 | 22 个不同音乐 header 中，`96` 有 3 处静态 E9 路径；该 ID 已自然 selected | 当前观察未证明菜单生命周期截断前实际执行了 E9，更不等于已有完整 E9 生产验收 |
| FM3 | 22 个音乐与 12 个 primary 的完整静态路径均无 FE，也无 ED／EE 直接 YM 写 | 仅该已查语法域中未见开启路径，不扩展为全 ROM／外部硬件写不可达 |
| Pitch envelope | 已查 header 初始低 7 位均为零，无 F1／F4；F0／FC／FD 只改变 modulation bit 7 | 仅该已查域中未找到 envelope 选择路径，不以静态参数 helper 通过替代任意游戏入口证明 |

`96` 的 E9 位于 FM2／FM3 的 quanta 99、FM5 的 quanta 105。它是原菜单真实音乐内的静态路径，不能说「只诊断所以无关」。其 AM 基础读取还受原条件码、共享字段和 A0／A1 源行为影响，后续若恢复原菜单声音，需要独立实际读取／请求 oracle。

group2 的正常 handler 返回可能发生在 root queue／priority 已消费之后，因此不把它概括成整个系统完全无副作用。原版 byte candidate 检索只找到正常 dispatcher 和 guard 自身的分支，边界及未排除项见 [原分派报告](AUDIO_GAME_DISPATCH_ORIGINAL_RESULTS.md)。

## 控制路径与资源归属

`01/04/05` 由比赛代码实际请求，属于必须接通的 fade／clear 控制，不是可播放 PCM 或 score 资产。`00` 是初始化固件／reset 路径，queue 的零则表示空；不能用普通 `RequestSound(00)` 混同二者。`02/03` 分别停止 primary／secondary；本次比赛候选和自然窗口没有建立它们的显式请求来源，但原 handler 确实存在。

冻结的 42 流证明 score 定义、声部推进和相应有序意图，不覆盖完整 Root 控制／fade、独立 PCM 生命周期和总混音。本次不把 `121 + 23` 的资产数量当作这些控制已经实现或验收的证据。

统一资源 owner 使用 encoded SHA，不使用解码 WAV SHA。当前 121 个普通 payload 与冻结 score 的 6 个音乐 payload 没有重合；加合法 F0 后至少为 128 个非诊断不同 payload。过渡设计由新 `pcmassets` 导入冻结 schema 2 的音乐 payload，并只在新普通资源中保存普通 bytes，构造唯一持久 payload owner；旧 score decoder 仅校验 payload 后释放，不另存采样数据。

导入须区分冻结 score 的 gzip SHA `deaf3a70eac46c199335445d186f77d51f185bdaad4d95443d09b2f1d7054d71` 与解压 JSON SHA `735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830`。未来去内联／资源 revision 需独立前置测试，本次不修改已提交 schema。`7F` 的单字节静默诊断另列，未在本次比赛生产点与资源中发现请求；不因此把它的仲裁／状态行为认定为 no-op。

## 复跑

在授权 worktree 根目录执行只读聚合检查：

```sh
python3 -B docs/research/md-yuyuhakusho/AUDIO_GAME_DISPATCH_COVERAGE_AUDIT.py \
  --out .local/md-yuyuhakusho/audio-game-dispatch-review/coverage.json
```

脚本复核资源 99 ID、比赛音乐与 score 队列闭包、原版两次 F0 queue 写、独立 payload SHA、原 22-header 控制审计，以及冻结 Go／数据 SHA。输出只写入指定工作区路径，不实例化模拟器、不执行新 CPU 捕获、不安装工具或修改生产。资源字段与原 dispatcher 的独立复核命令分别见 [资源报告](AUDIO_GAME_DISPATCH_RESOURCES_RESULTS.md) 和 [原分派报告](AUDIO_GAME_DISPATCH_ORIGINAL_RESULTS.md)。
