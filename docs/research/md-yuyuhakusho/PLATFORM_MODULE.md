# 《魔强统一战》服务端接入与共享 tick

此记录覆盖独立规则的服务端模块、计算资源加载、输入边界及服务端／客户端共用的单帧适配器。原版规则与像素恢复的证据由各自研究记录维护。

## 模块行为

`games/yuyuhakusho/server.Module()` 实现平台的配置、准备、开局、输入、普通视图、按需恢复快照、电脑操作、接管、存档恢复、旧版本输入鉴权和定时推进回调。

- 房间支持 2、3、4 人自由对战和 4 人组队；场地编号为 `0,2,4,6,8,10,12,14`，角色编号为 `0..10`。选择由准备阶段保存，开赛直接建立场景。
- 平台席位始终连续；对应原版 slot 分别为 `[0,3]`、`[0,1,3]`、`[0,1,2,3]`。真人输入通道与颜色使用平台席位，胜负从原版 slot／队伍编号反向映射。
- 初始真人全部加载就绪前不推进规则，电脑自动就绪，观战者不参与门槛。运行中的重复 `loaded`、恢复存档和重连保留现有世界。
- 网络帧使用独立的 `uint64`，JSON 输出十进制字符串；规则中原有 16 位帧继续保留其语义。系统每 50 ms 唤醒，按 `715909/11947200` 帧／毫秒累计规则帧和余数；一次最多追赶 250 ms，旧积压不继续扩展。
- 按键采用 MD 12 位逻辑状态。同一时钟内连续按下／松开按顺序进入不同的有效规则帧，保留短按。每席位最多排队 64 次输入，报文最多 1024 字节，序号限制在 JavaScript 安全整数范围内且单次跨度不超过 1024。
- 750 ms 无有效输入会释放 transport 状态；真人身份不变。电脑与主动托管调用原版独立 AI。控制切换写入与按键相同的重放帧，在有效 tick 更改原有控制归属，不清除规则输入历史、动作或 HP。
- 存档恢复或手动接管只清空对应 transport；下一有效规则帧采样已释放状态并产生原版 `Released`，恢复当帧的 world、presentation 和网络帧保持不变。开场提示取自原版 manager 的 stage `0..12`，不使用固定经过帧数猜测。
- 曾含电脑或主动托管的对局保持不计分；本作没有房间席位以外的固定 AI，因此 `Match.HasComputerPlayers` 为 `false`。和局使用平台显式 `Draw`，不将所有席位标成赢家。

`sessiontick.Advance` 统一服务端推进和客户端重放的校验、控制切换、规则步进与无像素呈现元数据更新。任一后续错误返回原始 world／presentation，不部分修改调用者状态。

## 网络与资源

普通视图只包含 epoch、帧号、规则选项、角色公开资源、输入确认、有界重放窗口和两种摘要，不带完整世界或图片。`TestModuleWireAndRecoveryBoundaries` 测量双人房间的 360 次连续变键，并在原始日志记录实测最大字节数；该数字不含平台消息信封或传输层开销，也不代表所有人数与玩法的理论上界。

`Options` 与 `View` 的 JSON 输出显式将 `characters`、`seatSlots` 编码为数字数组，避免 `encoding/json` 将 Go `[]uint8` 当作字节载荷编码成 base64。内存类型和 Go 读取方式保持原样。原始 JSON 类型断言覆盖 2、3、4 人配置、准备结果、保存的选项、玩家／观战视图和恢复快照。

首次接入和失步恢复按需取得 `{view,checkpoint}`。`checkpoint` 是未经解析的完整 JSON 字符串，客户端必须原样交给 Go／Wasm，避免 JavaScript 对 sprite `Present` 的 64 位整数进行舍入。checkpoint 保存原始 world、presentation cache／sprite slot／HUD ref、规则选项、帧号和资源版本身份。

服务端 `data` 包仅显式嵌入以下计算资料，初始化时解压、验证并解码；回调不读取文件或访问网络。

| 文件 | 用途 |
| --- | --- |
| `chu-world-rules.json.gz` | 独立规则数据 |
| `renderstate-metadata.json.gz` | 无像素呈现记录与继承关系 |
| `runtime-resource-descriptor.json` | 13 项逻辑资源的摘要与身份 |

初始化同时核对 rules、render metadata、sprite catalog 和 HUD catalog 的身份。存档保存精确 descriptor SHA-256，缺失、空值或与当前资源不符时拒绝恢复、投影视图和导出 checkpoint。浏览器 Wasm、JS、图片和音频未由该服务端资源包嵌入。

## 测试来源与重现

先行失败范围为 `server/MODULE_FAILURES.md` 和 `sessiontick/FAILURES.md`。`verification/platform-module-test-first.json` 保存冻结测试 SHA、真实缺少符号的红灯日志，以及实现前两次平台契约修订的前后来源：`HasComputerPlayers` 的含义和 opaque checkpoint 格式。旧测试源码归档与旧 manifest 均保留。

资源持久化身份验证是已有模块上的追加功能。`platform-module-resource-test-first.json` 单独记录新增测试和修正前源码 SHA；真实红灯为 `platform-module-resource-red.jsonl`，通过后的定向 race 日志为 `platform-module-resource-native.jsonl`。它不声称最初完整模块尚未存在。

后续复核的三个独立扩展也各自保留先行测试、修正前源码 SHA 和真实红灯：`platform-module-transport-test-first.json`（恢复释放与原版阶段提示）、`platform-module-controller-test-first.json`（MD 控制器声明）、`platform-module-numeric-json-test-first.json`（原始 JSON 数字数组）。它们未改写最初冻结的测试。控制器使用公共 `md-6` preset，不重复定义平台键盘／实体手柄映射。

在 worktree 根目录执行：

```sh
python3 scripts/yuyuhakusho/verify_platform_module.py native
python3 scripts/yuyuhakusho/verify_platform_module.py wasm
python3 scripts/yuyuhakusho/verify_platform_module.py race
python3 scripts/yuyuhakusho/verify_platform_module.py vet
```

脚本将 Go 编译缓存、模块缓存、临时目录和遥测目录限定在 worktree 的 `.local/`，不调用 `go telemetry`。每项保留原始输出、命令、环境覆盖、Go／Node 版本、冻结测试 SHA、依赖源码／计算资源 SHA 和退出状态；执行前后比较源码是否变化。

默认结果名为 `platform-module-*-wire-final.*`，对应数字数组修正后的源码。较早 `*-final.*` 和 `*-review-final.*` 记录单独保留。`wasm-review-final` 的测试二进制全部通过，但执行过程中新增了后续测试文件，因此其 `sources_unchanged`、汇总 `pass` 为 `false`；该日志不能当作当前源码的最终通过记录。

最终 `wire-final` 验收均通过，执行前后源码未变化；四项记录绑定同一份源码清单。计数合并 server 和 sessiontick 两个包，耗时列为 server 包的测试耗时。

| 检查 | 顶层测试 | 子场景 | server 耗时 | 结果 |
| --- | ---: | ---: | ---: | --- |
| Native，完整矩阵 | 18 | 39 | 74.823 s | 通过 |
| Wasm，完整矩阵 | 18 | 39 | 307.831 s | 通过 |
| Race，输入／恢复等边界 | 17 | 7 | 40.897 s | 通过 |
| `go vet` | — | — | — | 通过 |

39 个子场景包括 32 场完整 AI 对局、4 个输入恢复／接管场景和 3 种人数的原始 JSON 输出。密集双人视图最终实测最大值为 2513 字节。完整记录、每场帧号／胜方／摘要、源文件 SHA、先行测试 SHA 及原始日志摘要保存在 `verification/platform-module-final-validation.json`，不以进程退出码代替源码一致性检查。

完整 AI 对局覆盖 8 个场地 × 双人／三人／四人／组队，共 32 场。各环境使用本地新建种子，逐场验证真实生命周期结束、原版胜方到席位的映射，并在第 100 次定时推进后恢复副本，对照后续 10 次完整 checkpoint。不能据此声称 native 和 Wasm 两次独立随机比赛轨迹相同。共享 tick 的固定初态测试另行验证 800 帧、控制切换和 JSON 恢复后的逐帧完整规则、呈现与事件相同。

## 验收边界

服务端回调测试和 Wasm 执行不代表实际浏览器预测延迟、HTTP／WebSocket 慢连接隔离、手机触屏、资源发布剥离或线上部署已经完成；这些结果由平台宿主与前端验收记录补充。本次范围不负责注册游戏、发布资源、上传或部署。

`platform-module-resource-boundary.json` 另保存服务端实际 Go 依赖与 3 个计算嵌入文件的大小、SHA；它属于源码构建边界检查，不代替最终二进制或镜像的素材剥离验收。`platform-module-cleanup.json` 记录已删除的测试草稿语法错误日志；真实先红证据和验收记录保留。共享 Go 缓存仍由其他正在运行的验收使用，未在此范围删除。
