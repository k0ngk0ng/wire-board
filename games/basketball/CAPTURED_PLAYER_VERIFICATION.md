# 箱子捕获与脱离人物动作

实现前列出的失败场景：将捕获直接当成受伤或解除；把 `$056C=4` 当对象号而非逃脱计数；每轮待机重置计数；电脑和真人使用相同自动跳脱；持球时直接改球动作 11 而非排队请求；球的推出方向误用新朝向而非旧姿态；在物理之后才释放球权或绑定特殊持有人；跳脱覆盖旧竖直冲量；跑跳重新计算横速；落地计数下溢被夹成 0；提前清除特殊持有人；未经过道具动作 7 就直接去掉箱子。

装置接线另列失败场景：人物完成动作时复制物理绑定，导致装置清除所选人物 `JumpPhase` 的写入无法传回调度器；把平台下方的受控前态误当正常起跳落地；在下一次动作才观察到重置，从而漏掉首步差异。

## 恢复范围与来源

人物 `$50..$56` 覆盖捕获待机、行走、跑动、刹车、跳脱、跑跳及落地恢复。电脑由 `$50` 的下一动画动作进入 `$54`；真人待机的下一动作为 `$50`。每次 `$54/$55` 落地递减 `CaptureCounter`，从 4 经五次落地变为 255；负数时人物转普通落地 6，固定道具 4 转动作 7。道具动作 7 的入口负责清除 `SpecialHolder`。JSON 中的 `captureObject` 仅保留为既有回放字段名，不再建立另一份状态。

| 原程序位置 | 恢复行为 |
| --- | --- |
| bank6 `$94F8` | 动作 `$50` 绑定特殊持有人；按旧显示姿态设置球横速 ±2 像素，释放球权并排队请求 11 |
| bank6 `$953F` | 动作 `$51` 行走和捕获姿态序列 |
| bank6 `$8831` | 动作 `$52` 共享跑动、`ShotCycle`、持球跑动的相邻动画字节；腾空转 `$0F` |
| bank6 `$9EC1` | 动作 `$53` 刹车，横速归零转 `$50` |
| bank6 `$9556`、`$EA28` | 动作 `$54/$55` 清附着；在旧竖直速度、重力上分别加 2272、288；落地递减同一计数 |
| bank6 `$9EE2` | 动作 `$56` 停止水平和竖直运动，落地姿态结束后转 `$50` |
| bank6 `$B1BD` | 道具动作 7 清除固定对象 4 对应的特殊持有人 |
| bank6 `$9749`、`$978D` | 普通坠落检查 `$053D..$0546` 的所有权；`$056B` 不在该扫描范围内 |

REA 静态来源保留在 `.local/basketball/rea/captured-player/analysis/`，批量反编译 Evidence ID 为 `ev_aaa1d310233d2f1144674761bc118a019f79f8190ae14cd85d7a1530a20c1e0f`。共享跑动来源另见 `.local/basketball/rea/possessed-idle/shared-run/`。

原指令夹具由 `scripts/basketball/extract_captured_player.py` 生成，保存在 `engine/testdata/captured-player.json.gz`，摘要及 ROM、初始 RAM、夹具的 SHA-256 保存在相邻的 `captured-player-source.json`。原指令执行仅截获 `$FBCC` 作为音效请求出口，不运行目标音频设备。

## World 接线

`engine/world_actions.go` 将捕获动作交给 `engine/world_captured.go`。绑定特殊持有人、球权释放、球请求和球横速先提交，再进入原场地物理；完成物理后提交姿态、附着和逃脱计数。球请求 11 的原接受条件只允许球动作 1 或 2，其余动作保留请求。

捕获跑动进入 `$0F` 后，普通坠落沿用真实所有权扫描。`SpecialHolder` 不再被误判为持有其他对象；仍检查篮筐、两个道具以及携带人物的所有权。

`CompleteCapturedPlayer` 临时切换原物理绑定的附着输出地址，结束时恢复该地址。它保留同一个绑定实例，使装置写入的 `resetJumpPhase` 能在外层调度器提交，而不等下一次动作才生效。

## 验证范围

- 560 次受控人物原指令调用，比较所有可恢复的 World 状态及音效请求。
- 131 组人物与箱子原指令调用，从一个初态连续完成电脑五次跳脱。每组按人物、箱子顺序调用；这不是 131 次完整 World 更新或 131 局比赛。
- 50 条球请求 11 接受或保留用例，覆盖 25 个球动作和 entered 状态。
- 4 条特殊持有人存在时的普通坠落原指令用例。
- 16 次装置原指令调用：平台下方旧前态 1 次，以及从 32 像素表面起跳到落地的连续 15 次。比较每步完整 World 状态及音效请求。
- World 测试先由真实 Kind10 接触搜索与响应进入动作 `$50`，不以强制写入捕获动作作为测试入口。覆盖被捕获者持球、其他人持球、无人持球，以及电脑完整五次跳脱与箱子动作 7。

旧装置测试使用 `(X=280, Depth=76, Height=0)`。原程序在首步就触发装置转 `$3E`，因此等待「捕获跳跃落地」是错误判据。该前态仍作为 `below-surface-original-regression` 保留，断言首步的原动作和跳跃阶段重置；另有 `surface-landing` 从实际 32 像素表面验证跳脱落地。这两项均从已完成真实捕获的 World 状态建立受控装置边界，不声称 Court2 会自然生成 Kind10 箱子。

以上动作验证不代表完整比赛或浏览器验收。下方补充真人逻辑输入与冷启动路线；触屏和控制器映射仍须浏览器验收。特殊持球跑动在原姿态 7 时的音效请求 1 保留在逻辑输出中，本次没有修改平台音效资源；普通捕获到逃脱链不请求该声音。

## 可重复执行

在仓库根目录执行：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_captured_player.py
go test -race ./games/basketball/engine -run '^(TestWorldCaptured|TestWorldKind10CaptureToComputerEscape|TestOriginalCaptured|TestCapturedDropRequest|TestKind10|TestBallDeflectionAndDispatchAgainstOriginal|TestBallRequestRetainsIllegalAndProcessedRequests|TestFalling|TestWorldLoop)' -count=1 -json > .local/basketball/rea/captured-player/final-race.json
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^(TestWorldCaptured|TestWorldKind10CaptureToComputerEscape|TestOriginalCaptured|TestCapturedDropRequest|TestKind10|TestBallDeflectionAndDispatchAgainstOriginal|TestBallRequestRetainsIllegalAndProcessedRequests|TestFalling|TestWorldLoop)' -count=1 -v > .local/basketball/rea/captured-player/final-wasm.log
```

重建原指令夹具需要工作区内已有的授权 ROM、初始 RAM 和 py65；运行 Go 及 Wasm 测试只读取已提交夹具，不需要 ROM。

失败证据保留在同一产物目录：`world-before.log` 为缺少捕获调度分支，`fall-before.log` 为误将特殊持有人纳入普通坠落所有权检查，`device-reset-before.log` 和 `device-diagnostic-state-before.log` 为旧装置判据及逐步状态，`device-contract-before-ready.log` 为修正判据后保留的真实失败。最后一份记录显示旧前态、表面落地和原指令对照均得到 `JumpPhase=37`，原程序要求为 0。`device-contract-after.log` 记录修复后全部通过。中间编译窗口的日志单独保留，不作为行为验收结果。

最终验证：`final-race.json` 中 22 个顶层测试、含子测试共 28 项通过，包耗时 11.564 秒；`final-wasm.log` 中相同范围通过，包耗时 3.514 秒。两次进程均以状态码 0 退出。范围含新增装置回归、原捕获链、Kind10 接触、球请求、坠落和已有 WorldLoop 轨迹；没有运行其他游戏或把受控验证记为完整比赛。


## 真人输入与冷启动路线（2026-10-18）

此次只增加验收，不修改生产输入。`captured_human_input_test.go` 已有的两层证据重新在 race 和 Wasm 下验证：

- 5,264 次原 bank6 `$805C` 输入调用，覆盖四个角色、捕获动作 `$50..$56`、14 组连续按键序列、持球优先级和空中跳跃门槛。夹具 `captured-human-input-source.json` 明确标记 `natural_gameplay:false`：原调用未修改指令，但使用受控前态，未执行物理，不能记作自然对局。
- 两条生产 `StepWorldLoop` 链各 207 次更新，分别覆盖被捕获者持球和未持球。入口由真实 Kind10 搜索产生捕获，但捕获前位置与投影为受控设置；之后真人走、双击跑、按住 A+B、重复空中按键、五次落地与普通步行全部通过。它也不是冷启动自然路线。

`captured_native_route_test.go` 增加从 `NewRuntime` 开始的按键路线。固定场地 4、种子 `0x12345678`、模式 128、控制器 `[0,1,2,3]`，保持原时钟；角色 2、3 为不操作的真人。没有写入位置、动作、球权、接触候选或逃脱计数。第 114 视频帧完成原开场，按下表继续：

| 持续视频帧 | 控制器 0 | 控制器 1 | 结果 |
| --- | --- | --- | --- |
| 21 | Up | Up | 向箱子所在纵深移动 |
| 54 | Left | Up | 角色 1 到深度 70 |
| 15 | Left | 松开 | 角色 0 接近传送带箱子 |
| 6 | A | 松开 | 拾取 Kind10 箱子 |
| 15 | 松开 | 松开 | 等待原拾取动作结束 |
| 6 | Right + A | 松开 | 向右投掷箱子 |
| 60 | 松开 | 松开 | 第 246 视频帧捕获角色 1；至 291 仍等待真人输入 |
| 每轮 6 + 120，共五轮 | 松开 | A+B 持续 6 帧，再全部松开 120 帧 | 五次独立跳脱 |
| 18 | 松开 | Right | 脱离后恢复普通步行 |

五次落地发生在视频帧 `[336,462,588,714,840]`。第五次计数从 0 下溢为 255，箱子动作 7 解除特殊持有人；第 939 帧验证普通步行。测试在捕获后、每次起跳后及落地后进行 JSON 保存恢复，每个后续视频帧比较完整 Runtime 一致性。

这条路线验证的是 Go 视频运行时收到逻辑控制器按键后的自然交互，尚未经过原生浏览器键盘／触屏，也没有证明原 ROM 从同样冷启动按键得出完全相同轨迹。它采用四真人测试配置，不能冒充平台「两人合作 + 两个原版电脑」验收。额外按真实合作配置 `[0,1,-1,-1]` 运行同一键序时，电脑在第一次跳后介入，逃脱链发生变化；第 845 帧遇到 `rider support object 4 requires its original kind`（支撑道具 Kind14）。失败前态与日志原样保留，交由骑乘动作恢复处理。

可重复执行：

```sh
source scripts/env.sh
go test -race ./games/basketball/engine -run '^(TestOriginalCapturedHumanInput|TestWorldCapturedHumanInputThroughEscape)$' -count=1 -v > .local/basketball/rea/captured-player/human-input-resume-race.log
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^(TestOriginalCapturedHumanInput|TestWorldCapturedHumanInputThroughEscape)$' -count=1 -v > .local/basketball/rea/captured-player/human-input-resume-wasm.log
go test -race ./games/basketball/engine -run '^TestRuntimeCourtFourHumanCaptureColdControllerRoute$' -count=2 -v > .local/basketball/rea/captured-player/cold-native-route-race.log
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^TestRuntimeCourtFourHumanCaptureColdControllerRoute$' -count=2 -v > .local/basketball/rea/captured-player/cold-native-route-wasm.log
```

以上四次进程状态码均为 0；输入范围 race 11.000 秒、Wasm 3.242 秒，冷启动路线重复两次分别 2.303 秒和 0.361 秒。`.local/basketball/rea/captured-player/route/result.json` 保存精确按键表、结果与 SHA-256；`cold-route-states.json` 保存各按键段边界的完整 Runtime；`cooperative-failure-state.json` 保存合作模式失败前态和当帧输入。未将受控调用、冷启动逻辑按键或失败的合作路线记为已完成浏览器验收。

## 原生浏览器双控制器验收

`scripts/basketball/accept_captured.py` 使用独立测试页面 `captured_test_page.html` 与 `captured_browser_harness.ts`，没有修改生产输入、运行时或 `accept_live.py`。页面同时创建两个实际 `BasketballInput` 和两个共享 `FCController`；控制器 0 使用平台默认 W/S/A/D、J/K，控制器 1 使用方向键、逗号／句号，其余按键也互不冲突。每视频帧分别 `sample()` 后交给同一 Go/Wasm `advance`，呈现使用现有 `BasketballRenderer`。页面不提供设置坐标、动作、接触或计数的接口。

桌面场景从冷启动开始，全部按键经浏览器原生键盘事件输入；320px 场景的移动、拾取、投掷仍由两套原生键盘输入，角色 1 的五次 A+B 改用真实双指触摸共享手柄。共记录 10 次可信 `pointerdown(pointerType=touch)`。这验证手机尺寸下的真人跳脱触摸链，不声称在一部手机完成双人导航与比赛，也不是网络房间或开放四人产品模式。

两个场景均复现捕获视频帧 246、五次落地 `[336,462,588,714,840]`、939 恢复步行。捕获中保存／恢复完整快照，验证状态和 Canvas 像素逐项保持一致。截图实际检查了被捕获角色位于相机内、画面下方的两个手柄、320px 宽度无溢出及双指按钮的实际坐标。每段快照、逐视频输入和状态、可信事件来源、关键帧截图保存在 `.local/basketball/captured-native-browser/`，`result.json` 列出范围、检查项及 SHA-256。

执行命令：

```sh
source scripts/env.sh
python3 scripts/basketball/accept_captured.py
```

首次运行发现测试页面读取计数 0 时未处理 JSON `omitempty`，第二次发现一次取回整个逐帧记录超过 CDP 超时；两项均仅修正测试采集器，日志 `browser-first.log`、`browser-second.log` 保留在 `.local/basketball/rea/captured-player/`。采集器现按 30 帧分页读取，未缩短游戏时间、跳过动作或改变生产逻辑。

最终 `browser-final.log` 进程状态码为 0，46 项检查通过。桌面与 320px 场景各保留 939 帧记录、34 个可信原生输入事件；手机场景 Canvas 为 256×240，两个手柄下缘分别为 478.77px、614.53px，均位于 760px 验收视口内。已目视检查桌面捕获和手机双指起跳截图。该结果补齐本节限定范围的浏览器输入证据，不替代完整比赛、原 ROM 冷启动轨迹对照或合作模式中的电脑交互验收。

## 两人合作中的原版电脑干预与 Kind14 回归

骑乘支撑及 `$07/$3F` 原链恢复后，重新检查此前视频帧 845 的失败。新增 `captured-cooperative-runtime-before.json.gz` 原样保存此前失败前态（SHA 与来源见相邻 `captured-cooperative-runtime-source.json`）；`TestCapturedCooperativeRecordedKind14Failure` 不修改状态，从同一帧连续推进至 1029，原报错不再出现。

`TestRuntimeCourtFourCooperativeCaptureControllerRoute` 从同一 Court4 固定种子冷启动，将控制器配置改为平台合作对应的 `[0,1,-1,-1]`，另一队两个角色由原版电脑控制。仍执行上述相同按键序列，保留真实 AI 干预：

- 视频帧 246：Kind10 箱子捕获真人角色 1，计数为 4。
- 视频帧 336：真人第一次落地，计数变为 3。
- 视频帧 357：原版电脑接触使真人进入动作 `$48`，特殊持有人清为 -1，计数仍为 3。之后的 A+B 继续按原当前动作处理，不强行留在捕获动作，也不人为补出五次跳脱。
- 视频帧 939 到 1029：继续按住真人控制器 Left，人物 X 从 125 移至 65，骑乘后的方向输入仍可操作。

冷启动测试逐视频对比 JSON 恢复重放。原失败快照回归与整条冷启动路线均通过 native、race（7.880 秒）和 Wasm（2.505 秒），对应 `.local/basketball/rea/captured-player/cooperative-resume-{first,race,wasm}.log`，状态码均为 0。没有改变 AI、随机种子、原时钟或 World 状态以制造五次跳脱。

独立浏览器测试支持以下命令：

```sh
source scripts/env.sh
python3 scripts/basketball/accept_captured.py --cooperative
```

桌面原生双键盘与 320px 场景的原生双指按键均通过 38 项检查，实际捕获、第一次落地及 AI 打断帧与 Go 路线相同，继续至 1029 无异常。关键截图包括 `mobile-320-ai-interruption.png` 和 `desktop-continued-human-input.png`，产物位于 `.local/basketball/captured-native-browser-cooperative/`；`result.json` 保存检查和 SHA-256，`cooperative-browser-first.log` 保存成功进程输出。手机场景仍是键盘导航加触屏 A+B 的组合验收，开发页面没有网络房间，也不宣称完整手机双人模式。这一结果闭合前文所列 Kind14 失败，但不会把 AI 提前打断记为五次跳脱成功。
