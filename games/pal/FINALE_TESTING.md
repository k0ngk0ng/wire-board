# 南绍地底、拜月与结局验收准备

第 24 段从第 23 段实际终点 `campaign-rain-prayer-state-1.json` 开始，SHA-256 为 `607b5afddcaccd3cf6674b449809695d5b4b39fc8f8ad7eedc283b9c24798ab5`。范围包含最终洞窟、南绍地底与王宫、拜月决战、水魔兽相关结局、原版结局演出及谢幕。目前未完成。

## 实现前失败场景

1. 没有从实际第 23 段终点开始，或将独立最终战／结局夹具拼成完整第 24 段。
2. 跳过迷宫、机关、必要人物和王宫事件；通过改坐标、资源、碰撞或存档绕过。
3. 灵儿／逍遥／阿奴的原版顺序、聚散或本人控制丢失，必需道具被提前授予或错误消费。
4. 普通战斗、剧情战斗与拜月决战的结果、原版增援和条件分支改变；用加数值或直接改结算获得成功。
5. 原版剧情、已接受行动、关键道具及结局演出中的保存丢失续行或随机状态，离线时长被补播，旧输入继续生效。
6. 最终 BOSS 结束后直接返回「游戏完成」，省略水魔兽、牺牲、回忆、原版图像／影片或谢幕。
7. 把未绑定的结局函数替换成空函数、成功事件或占位画面；缺失资源时宣称已还原 Windows 98 版结局。
8. 新增结局能力时改写旧快照的核心／资源身份来强行恢复，或让开发 HP9999 进入正式运行器与存档。
9. PC／手机不能继续结局、保存或退出，字幕／画面被工具栏挤压；只核对函数存在而没有实际演出证据。
10. 只完成开发加血主线 24/24 就标记整款复刻交付，遗漏音画、隐藏物品、兼容与发布门槛；原版难度托管按用户最新指示不再作为交付条件。

完成第 23 段的旧场景波动核心仍导入尚未绑定的 `PAL_EndingScreen`、`PAL_ShowFBP`、`PAL_ScrollFBP`、`PAL_EndingSetEffectSprite`、`PAL_EndingAnimation` 和 `PAL_PlayAVI`；Go 运行器对未知宿主导入明确报错。此项必须通过真实流程验证并实现，不能以结束标记替代。实现前须核对原版 `ending.c` 与对应资源，并遵守 [核心版本保留](NATIVE_VERSIONS.md) 和 [平台存档规范](../../rules/saved-games.md)。

## 结局媒体导出失败场景

1. 来源文件缺失、大小或摘要不符，却生成可用媒体清单；导出修改原始 AVI 或原版资源。
2. 将 Windows 原版三段结局影片替换为 DOS 回退图、静态截图或其他版本的重制影片。
3. 帧数、帧率、尺寸、顺序或解码后的 RGB 像素改变，丢弃重复帧或使用有损视频转码。
4. 音轨采样率、声道、采样数或 PCM 内容改变，使用有损转码代替 FLAC 等无损压缩。
5. 分块发生截断、重叠、空帧、末尾残片；加载续播时必须能直接定位到对应块。
6. 输出路径或符号链接逃出工作区，允许覆盖原始媒体、源码或既有验收归档。
7. 清单未记录文件的真实大小、SHA-256 和 MIME，或导出过程只检查命令退出码，没有回读全部画面和音轨。
8. 把媒体导出通过当作实际结局、浏览器播放、原版 C 像素一致或平台音频接入已通过。

原版 `PAL_EndingScreen` 在可播放 AVI 时依次调用 `4.avi`、`5.avi`、`6.avi`。三段已缓存于 `.local/pal/original-98-media/`，来源与摘要由 `assets/media-source-manifest.json` 固定。导出验证将全部画面与独立 FFmpeg 解码的 RGB 流逐字节比较，并回读无损音轨；此项先验证资源管线，不增加主线完成数。

## 独立结局能力的当前结果

新候选编译固定提交的原版 `ending.c`，文件原字节及许可证保留在 `native/reference/`，SHA-256 为 `ddaaf5014cbc173cbdf2dadf369f29cecba2244b1eaf0ff30b52f63c6c697971`。Windows 路径实际执行原版三段影片顺序。`ending_media.c` 将可恢复的影片编号、帧号、位置和序列写入原生内存，Go 运行器另校验并导出只读状态。`PAL_Shutdown` 只有原版调用到达后才生成终态；未知宿主导入继续报错。

独立开发候选位于 `.local/pal/runtime-candidates/ending-media/hp-9999/rules.wasm`，SHA-256 为 `c295e7b5bd6130a2b432ecb94d99a177aa27a16ea72e1c1fd0a7e3a0eabe4adb`。该候选尚未从新游戏完成主线重跑，未替换默认核心，也不能直接载入第 23 段旧核心的内存。专门的原版数值结局夹具为 `.local/pal/ending-fixture/rules.wasm`，SHA-256 为 `4cdb49c1e165c0ba224aec3aaa10d804dfa2735af55e98594f918125442ca836`；用途标记为 `ending-fixture`，不能进入剧情运行器目录。

已通过的独立检查：

- 原版三段影片共 4,690 帧，全部 RGB 解码流和 FLAC 还原 PCM 逐字节一致。320 个媒体文件共 84,992,537 字节，包含 314 个最多 15 帧的图像块，以及各段元数据和音轨；不把有损视频作为替代资源。
- 原生和 Go 运行器均验证全部帧及 12 个全新实例续行位置，包含片头、分块边界、片中和末帧。Go 另拒绝非法影片、帧号、帧率、时间位置和提前完成标记。
- 游戏模块通过三段影片中的 6 次保存暂停、离线 30 天、新运行器恢复及旧输入拒绝，最后按平台结果接口生成不计竞技积分的单人完成记录。
- 实际 `SceneRenderer` 和 `PalStage` 在 PC、手机竖屏及横屏完成 63 次像素比较，画布分别保持视口内最大原比例。媒体资源大小与摘要由结局清单核验，图像按当前位置加载，仅保留两个解压块。
- 保留旧核心的续档目录及编译身份伪装拒绝通过 race 回归；构建归档的 6 项原有检查通过。新增 C lint 正负例与现有规则共 25 项通过。

上述结果分别为 `.local/pal/verification/ending-media-export.json`、`ending-native.json`、`ending-engine.json`、`ending-authority.json` 和 `.local/pal/ending-browser/result.json`。上述像素比较基于独立 FFmpeg 解码，没有宣称 AVI 解码器与原版 C 逐像素一致；逻辑帧测试也不代表全程实际播放性能或物理扬声器验证。

实际平台 Board 的 PC／手机检查另已通过，结果为 `.local/pal/ending-board-browser/result.json`。包含原生平台媒体通道、PC 键盘跳过、手机点击跳过、取消触摸／双指缩放不推进、陌生账号拒绝，以及两次保存退出和本人立即继续。手机在第 6 段影片播放中冻结和恢复，完整公开边界一致，声音从对应位置继续；帧号和实测媒体位置记录在结果中。截图已检查，两端画布为 1280×800／320×200，未显示虚拟按键。该页面仍来自结局夹具，不替代真实最终章。

页面检查期间修正了两处接入问题：初次播放受自动播放策略阻挡时，后续真实手势从当前位置重试；手机画面命中允许结局阶段，并把影片序列加入手势边界，避免旧触摸跨片段生效。音乐和音效的其他主线接入、完整音画及资源发布仍未完成。

复现资源与画布：

```sh
. scripts/env.sh
python3 -B games/pal/tests/ending-media-e2e.py
python3 -B games/pal/tests/ending-browser.py
```

复现原生与平台规则续档：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --ending-fixture --out .local/pal/ending-fixture
PAL_NATIVE_WASM=.local/pal/runtime-candidates/ending-media/hp-9999/rules.wasm node games/pal/tests/ending-native.mjs
PAL_ENDING_E2E=1 go test -v ./games/pal/engine -run '^TestOriginalEndingTimelineAndFreshRuntimeSaves$' -count=1 -timeout=8m
PAL_ENDING_E2E=1 go test -v ./games/pal/tests -run '^TestOriginalEndingAuthorityFreezeResumeAndTerminalBoundary$' -count=1 -timeout=8m
```

实际平台页面先构建并启动夹具：

```sh
. scripts/env.sh
npm --prefix web run build
PAL_ENDING_BOARD_BROWSER=1 go test -v ./internal/server -run '^TestPALEndingBoardBrowserFixture$' -count=1 -timeout=22m
```

另一终端执行：

```sh
. scripts/env.sh
python3 -B games/pal/tests/ending-board-browser.py
```

最终路线目前使用第 23 段的原核心和实际根档进行首次调查：

```sh
. scripts/env.sh
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 PAL_FINALE=1 go test -v ./games/pal/tests -run '^TestOriginalFinaleRoutePalaceBaiyueAndEnding$' -count=1 -timeout=120m
```

该原核心尚不具备结局能力，首次路线调查不能计作第 24 段完成。后续还须使用新候选的真实新游戏链路，验证王宫、拜月、完整结局和原版之后的续行；不得改写旧存档身份接入新能力。

首次调查已到王宫，完成 68 场普通胜利及 99 次续档后，因为测试未允许原版临时 `[0]` 队伍而停止。原始脚本 `35246` 的 `(0x75,1,0,0)` 明确暂时只保留李逍遥；现只修正测试的队伍断言，保留旧失败根档于 `.local/pal/verification/finale-party-before/`。后续调试续跑与最终从根档完整重跑分开记录，不改变原版队伍或存档。

## 原版附加署名页接入前失败场景

1. 原版三段 AVI 结束后直接完成平台对局，略过原始脚本随后调用的 `PAL_AdditionalCredits`。
2. 署名页没有实际绘制原文，或 C 等待按键时 PC／手机无法确认；为继续增加独立「读完」按钮。
3. 影片跳过的旧确认键跨片段或越过署名页，保存后丢失已经接受的跳过动作，或重新播放已跳过的影片。
4. 署名页中的原生等待位置、画面和输入边界在新实例恢复时丢失，或者按下确认后尚未真正调用原版关闭流程就生成成功。
5. 完整结局夹具覆盖已保留的旧核心，或将专门的结局夹具加入正式剧情版本目录。

原版署名页保持原来的绘制和等待实现，只增加菜单观察边界，使游戏画面点击与键盘确认能够通过统一输入继续。影片、署名页和实际关闭均通过前，不能以电影导出或单独三段影片夹具代表整条最终续行。

## 附加署名页的独立结果

原始脚本在 `PAL_EndingScreen` 后调用的 `PAL_AdditionalCredits` 属于 SDLPAL 的附加 GPL 署名页，与 98 版 AVI 中的原作者字幕分开。现保留原文绘制、背景和原生等待，仅增加独立菜单类型 13 的公开观察；类型 12 继续用于原版通用选项。PC 鼠标和手机轻触画面均发送普通确认，键盘保留，无新增「读完」或虚拟按键。

独立完整结局夹具为 `.local/pal/ending-credits-fixture/rules.wasm`，SHA-256 为 `5f5fd32fe43cd6870bf644965d8571166556efe3ddb46e966e24179f31dfba74`，用途仍为 `ending-fixture`，已验证不能进入剧情运行器目录。此前影片夹具和开发候选未被覆盖，正式核心未提升。

实际原生链路通过三次影片跳过后的 500 毫秒等待续档：清理旧输入后，先推进 250 毫秒、再推进 249 毫秒，最后 1 毫秒才进入下一段。署名页保存后在全新运行器中确认，只有实际调用 `PAL_Shutdown(0)` 才产生终态。结果为 `.local/pal/verification/ending-credits-engine.json`。

实际平台 Board 的 PC／手机验收也已通过。包含三次本人保存退出及继续、陌生账号不可见／不可恢复、影片结束后音轨停止、最后一次影片跳过不跨署名页、署名页完整公开边界恢复、取消触摸／缩放不误确认，以及两端点击画面后由原生关闭生成平台终局。画布保持 1280×800／320×200，截图已复核。结果为 `.local/pal/ending-credits-board-browser/result.json`。实现前手机点击署名页失败的日志、截图和源码另保留于 `.local/pal/verification/ending-credits-board-before/`。

旧影片夹具的 4,690 帧与 12 次新实例续档回归通过；构建归档 6 项检查和 lint 25 项正负例通过。这些独立能力不增加主线 `a/24`，仍须新候选从真实新游戏连续重跑到最终章。

复现完整结局夹具：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --ending-fixture --ending-credits --out .local/pal/ending-credits-fixture
PAL_ENDING_CREDITS=1 go test -v ./games/pal/engine -run '^TestOriginalEndingAcceptedSkipsCreditsAndSavedNativeExit$' -count=1 -timeout=8m
npm --prefix web run build
PAL_ENDING_BOARD_BROWSER=1 PAL_ENDING_BOARD_CREDITS=1 go test -v ./internal/server -run '^TestPALEndingBoardBrowserFixture$' -count=1 -timeout=22m
```

另一个终端运行 `python3 -B games/pal/tests/ending-board-browser.py --credits`。构建中原版署名包含编译日期，复现须以新生成的实际核心摘要为准，不能假定不同日期重建具有相同字节。

## 结局资源发布前失败场景

1. 未通过原始三段 AVI 的完整 RGB／PCM 回读，或来源、图像顺序、帧数及声音长度变化，仍允许上传。
2. 结局清单缺文件、包含额外路径、MIME／压缩响应不符、文件被替换，或者符号链接越出源目录。
3. 三段音轨只在本地播放，CDN 缺少 CORS、Range 或解码后的 PCM 与原始 AVI 不符。
4. 图像块回读失败、gzip 解压截断、帧数不符或 RGB 摘要改变，仍登记已发布。
5. 原样重复发布、集合清单变化或同批重复块造成未变资源换地址或重复上传；忽略已有音源中的相同内容。
6. 已发布对象被覆盖、凭据出现在日志、未回读完成就写入正式发布记录，或在 Actions 中执行 CDN 回读。
7. 页面仍使用本地结局音轨；媒体 URL 与发布记录不符；加载或读取存档时使用旧异步结果。
8. 为了通过资源发布验收更改原生存档的核心身份，或把开发加血核心当成正式核心。

结局音轨、无损图像块和清单按文件内容复用，使用逐文件发布映射。只上传未发布的唯一对象，保留已有音源地址；本地执行全量 CDN 回读，音轨另核对 Range 及 PCM，图像块核对解压 RGB。发布元数据不包含媒体正文。开发加血剧情通过按用户最新确认计入剧情进度；原版难度托管通关不再作为门槛。

结局源音轨是 8 位无符号 PCM。验收必须读取 FLAC STREAMINFO 的实际位深，并比较原版 8 位 PCM 字节；统一解码成 16 位后相等不足以证明位深保留。已发现早期结局导出扩成 16 位，此格式须修正，旧发布地址保留，仅为实际变化的音轨、对应元数据和集合清单创建新对象；未变 RGB 块复用原 URL。


## 当前结局 CDN 与页面结果

结局资源已发布，当前集合为 `35ea7fdcb0d19772be93a4dd246e475f8318f993a95270e7f0c6ed3942d15245`，发布映射保存在 `assets/ending-release.json`。314 个 RGB 分块、3 段 FLAC、3 份影片元数据及集合清单共 321 个对象；含清单的规范数据大小为 85,048,583 字节。FLAC 保留原版 22050 Hz、双声道、8 位 PCM；全部原始 8 位 PCM 字节、FLAC STREAMINFO 及 RGB 流均已核对。

早期 16 位导出已修正：只新增 3 段实际变化的音轨、3 份影片元数据及新集合清单，共 7 个对象、2,299,262 个上传字节。314 个未变图像块全部复用原 URL，主线音源未重新上传；早期地址保留。当前集合再次发布已验证 321 个对象全部复用，新增 0、上传 0 字节。结果为 `.local/pal/verification/ending-publication.json`、`ending-publication-repeat.json` 和 `ending-depth-before.json`。

发布后在本地完成全量 CDN 回读：全部对象核对大小、SHA-256、MIME 和 CORS，三段音轨另核对 HTTP Range 及还原的原版 8 位 PCM；314 个图像块另核对 gzip 解压后的 RGB 摘要。只有全部通过后才写入发布记录。程序更新不重建或上传未变资源。

实际 Board 使用固定发布映射，结局清单先校验版本摘要，再校验每份文件；原生媒体解码准备完成后才进入加载就绪。音轨和图像直接从 CDN 获取，本地结局路径在验收夹具中返回 404。已通过的桌面、手机及横屏画面另有 63 项逐像素对照，结果为 `.local/pal/cdn-ending-browser/result.json`。实际 PC／手机保存、跳过、署名页及原生结束验证独立于全剧情；新核心真实最终章仍须完成。

```sh
. scripts/env.sh
python3 -B games/pal/tests/ending-media-e2e.py
python3 -B games/pal/tests/ending-publication-e2e.py
python3 -B games/pal/assets/publish-ending.py
python3 -B games/pal/assets/publish-ending.py --publish
PAL_ENDING_CDN=1 python3 -B games/pal/tests/ending-browser.py
npm --prefix web run build
PAL_ENDING_BOARD_BROWSER=1 PAL_ENDING_BOARD_CREDITS=1 PAL_ENDING_BOARD_CHECKPOINT=1 PAL_ENDING_CDN=1 go test -v ./internal/server -run '^TestPALEndingBoardBrowserFixture$' -count=1 -timeout=22m
```

最后一个夹具启动后，另一终端加载 `scripts/env.sh` 并执行 `python3 -B games/pal/tests/ending-board-browser.py --credits --checkpoint --cdn`。默认发布命令只离线预检，`--publish` 才上传尚未发布的唯一对象；变体测试不上传任何数据。

## 真实剧情结局页面的前置检查

浏览器入口须绑定完整未调试最终章报告、实际核心、已登记完整终点及结束影片入口的摘要，且报告确实保存过该标签。不能用独立影片夹具或给旧档改写核心标识代替。入口导入新验收房间时，只将服务器及音频时间戳按相同差值换到该房间的时间原点，保持相对位置、队伍、道具、原生内存及源文件不变；正式平台续档继续使用平台保存的逻辑时钟。

`PAL_ENDING_CAMPAIGN=1` 使用真实最终章生成的 `ending-first` 登记入口；相应浏览器脚本加 `--campaign`。登记必须在完整最终章测试实际退出成功后生成，缺少登记或报告时拒绝启动。普通独立影片夹具保留原验收用途。

## 最新核心整段与真实页面结果

最新音频核心从其真实第 23 段终点完整通过最终章，`resumedTestRoute: false`：53 次阅读、71 场正常胜利、143 次新实例续档，4 种原版队伍顺序、守卫／假巫王／拜月、三段影片和谢幕均保留。阿奴实际消费 45,000 文施放 9 次「乾坤一掷」，未修改金钱或结算；正常拜月奖励为 12,800 文。第 4、5、6 影片的首／中／末位置均有新实例续档。

同次影片入口通过真实平台 PC／320px 触屏共 6 项检查，含完整边界保存继续、私人权限、手动回退、原生媒体通道、谢幕与最终原生结束；本地视觉及影片入口均为 404，资源使用已验证 CDN URL。入口和完整报告摘要登记于 `tests/browser-campaign-roots.json`，规则汇总为 `campaign-complete.json`。开发构建仍不进入正式注册目录，原版体力和整款性能不因此通过。

当前真实结局浏览器复现：先设置 `PAL_ENDING_BOARD_BROWSER=1 PAL_ENDING_BOARD_CREDITS=1 PAL_ENDING_BOARD_CHECKPOINT=1 PAL_ENDING_CDN=1 PAL_VISUAL_CDN=1 PAL_ENDING_CAMPAIGN=1` 运行 `go test -v ./internal/server -run '^TestPALEndingBoardBrowserFixture$' -count=1 -timeout=22m`；夹具就绪后执行 `PAL_VISUAL_CDN=1 python3 -B games/pal/tests/ending-board-browser.py --credits --checkpoint --cdn --campaign`。结果在 `.local/pal/campaign-visual-cdn-checkpoint-ending-board-browser/`。

正式原版核心与通关开发核心另均在原构建路径复编译为完全相同字节，源文件摘要一致，只有明确的开发体力配置和构建路径不同；结果为 `campaign-core-pair.json`，没有给任何旧存档改写核心标识。
