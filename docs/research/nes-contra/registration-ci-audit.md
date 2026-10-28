# 魂斗罗正式接入与增量 CI 审计

本文记录 2026 年 10 月 28 日的只读审计及其后获授权的 CI 选择器修复。后续只修改了 `scripts/ci/scope.py`、`test_scope.py`、`check-plan.py` 和本文；注册、运行时、引擎、工作流及部署配置未改，也没有上传或发布。最新单人／双人新局、标准命数自然终局与显式辅助八关覆盖仍需分别完成验收，当前不得把魂斗罗加入生产可用列表。

## 已具备的边界

- `games/contra/server/production_native.go` 提供 `NewProductionModule()`，要求真实链接的规则 ID 与运行产物 ID；校验、冻结归一化像素并构造真实视觉观察器后才返回模块。
- `games/contra/build/identity.py`、`native.py` 和 `scripts/contra/browser_build.py` 已把规则、视觉与运行产物身份分开，并校验实际构建输入和资源。Wasm 不注入自身清单摘要，避免循环；八关大数据和原生视觉包不进入 Wasm。
- `games/contra/web/catalog.tsx`／`module.tsx` 已存在，公共目录通过 glob 自动发现；大厅和创建列表仍受服务端 `availableGames` 约束。无需再写一份前端硬编码游戏列表。
- 已有独立原生初始化、PC／320px 房间、恢复、托管、结算和辅助胜利边界证据。这些不替代最新新局流程及后关覆盖，但也不意味着必须增加「常规三命 AI 八关全胜」门槛。

## 当前阻塞及源码依据

| 阻塞 | 当前事实 | 接入要求 |
| --- | --- | --- |
| 注册 API 没有错误通道 | `games/registry.go:Modules()` 返回 slice；`main.go` 直接把它传给 `server.New` | 生产组合应先处理初始化错误，再打开服务；不能 panic、假 ID 或静默忽略初始化失败的已注册游戏 |
| 普通测试会被牵连 | 注册测试、公共模块测试及多个游戏浏览器 fixture 调用同一个 `Modules()` | 普通单元／契约测试不应隐式构建 Wasm、读研究缓存或启动整套生产模块 |
| 原生构建没有两 ID | `.github/workflows/build.yml`、`Dockerfile` 仍直接执行 `go build -ldflags="-s -w"` | 必须从同一份已核验运行清单产生真实链接参数；不得重新手拼占位值 |
| 运行资源没有正式交付入口 | Vite 只接入篮球 delivery；公共 `cdn_delivery.py` 只收集 `dist/assets` 与 favicon | 明确运行清单和负载的交付位置，复用现有不可变写入／回读／裁剪流程 |
| MIME 和寻址未覆盖 | 公共 CDN inventory 没有 `.json`、`.idx`、`.wav` 类型，也未验证媒体 Range | 新增窄范围资源类型校验，音频须实际支持 206 寻址；不能只断言 URL 存在 |
| 干净检出缺少声音材料 | 当前浏览器打包按 `audio-source.json`／`audio-loops.json` 读取 `.local/contra-audio` 的 WAV | 先形成有实际 SHA、字节数和来源的媒体清单；发布构建不能依赖作者机器的研究目录 |
| 资源根地址尚未统一 | `Board.tsx` 以公共 `assets` 根构造 `contra/runtime-${RuntimeBuildID}/manifest.json`，为空时指向应用源 | 生产与开发要明确各自的资源地址；不能无条件套用平台图片根或让缺失资源返回首页 |
| CI 依赖安装遗漏 | game job 的 `npm ci` 条件只有 battlecity／conquest／tabletennis；Contra 的 renderer/client/input 测试使用 esbuild | 选中 Contra 前端检查时安装其实际测试依赖 |

只读审计得到以下结果；其中规则数据、共享注册测试与明确竞态范围已在后续授权修复中解决，构建工具检查接线仍待进行：

| 输入或检查项 | 审计时结果 | 当前处理 |
| --- | --- | --- |
| `games/contra/engine/leveldata/stage-8.json` | `backend: false` | 已修复：只选择 Contra 后端 |
| `games/contra/engine/testdata/fixture.json.gz` | `backend: false` | 已修复：压缩／图像形式的规则和 AI 夹具同样属于 Contra 后端 |
| `games/contra/web/Board.tsx` | `backend: false` | 保持前端／本地 E2E 范围 |
| `games/contra/build/identity.py` | 前端范围，但没有对应 Python 构建测试命令 | 增加构建身份／打包契约，不据此机械运行所有规则矩阵 |
| `games/registry.go`、`main.go` | 仅共享检查 | 保留共享范围，同时补实际注册／启动检查 |
| `games/registry_test.go` | shared 的扫描根不包含它，未生成 `./games` 测试命令 | 已修复：纳入共享发现，仍按函数归属区分游戏专属测试 |
| `TestProductionEmbeddedVisualsAndOfflineCallbacks`、`TestRuntimeArtifactStageConfirmationAndRestore`、`TestSessionBattleReviewIsCommittedAndRestorable`、`TestRuntimeTerminalUsesPersistedBattleReview` | 不在 Contra race 名称规则中 | 已修复：按所属包选择，避免误选 engine 中同名 Production 性能原型 |

`TestBotStandardCampaignE2E` 当前默认执行单人、双人各 120,000 帧，并未使用显式 opt-in 开关。`verification_env('contra', True)` 目前不设置 Contra 专用环境，也不清理补命、输入前缀、人数或目标关卡的诊断变量。不能把「没有设置开关」误认为没有运行全局，也不能让诊断环境改变 main 的验收含义。

## 推荐的注册与开发入口

生产继续保持严格初始化，不放宽 `NewProductionModule()` 的身份要求。增加明确返回错误的生产组合入口，例如 `games.ProductionModules() ([]gameapi.Module, error)`；`main.run()` 在 `server.New` 和数据库初始化之前调用它。注册激活提交才把 Contra 构造器加入该入口；所有前置验收和真实资源交付通过之前不加入。

不要立即把现有零参数 `Modules()` 改成内部吞错的生产函数。它有多个既有测试消费者。最小迁移可保留它作为明确标注的兼容测试集合，并把生产入口与「完整已注册列表」的测试迁到新入口；后续再把游戏专属 fixture 改为显式选择所需模块。生产代码不得继续使用兼容集合，相关断言应防止以后误用。这样不需要为了 Contra 的初始化改动而跑其他游戏完整规则。

普通 `go test` 分为两类：

- 普通源码／模块契约测试可直接运行，不需要生产链接 ID；现有明确标注的协议边界 fixture 不代表真实产物验证。
- 生产注册、实际运行身份与发布打包测试使用单独的可重复进程入口，显式接收真实准备产物并执行带真实 `-X` 的原生程序。缺少准备产物时，普通测试明确跳过该可选入口；main 必须显式启用，禁止把 skip 当作成功证据。

长测入口及环境清理本轮暂不修改，待 root 冻结后接线。用户已经允许无限命覆盖后关，后续应设置两个明确模式，不能额外要求常规三命 AI 八关全胜：

| 拟接模式 | 输入与判定 | 产物边界 |
| --- | --- | --- |
| 标准命数自然终局 | 真实新局、原版备用生命与续关规则、不补命、不使用中途状态或输入前缀；单／双人数都应能合法推进至真实终局，成功或自然失败均可按各自规则结束 | 保存输入、恢复证据、实际 outcome 和计分资格；不能把「合法自然失败」判作缺少八关全胜 |
| 显式辅助八关覆盖 | 明确启用 `CONTRA_BOT_INFINITE_LIVES=1`，记录所有补命；验证八关、特殊阶段、真实 `CampaignCleared` 与终局提交。若采用已验证按键前缀，必须记录并重放该前缀，不隐瞒来源 | 保留 `assisted`、补命清单、`prefix_frames`／策略开始帧及实际来源；不标为标准命数通关，不替代前一模式 |

具体后续接线建议：为长测加显式 opt-in（建议独立模式值，而非仅一个模糊布尔开关）；普通 `go test` 明确 skip 长测，main 只在对应规则／策略／规则夹具受影响时显式执行两种模式。每次从清理过的环境分别构造命令，清除继承的 `CONTRA_BOT_PREFIX`、`CONTRA_BOT_SEATS`、`CONTRA_BOT_TARGET`、`CONTRA_BOT_FRAMES`、`CONTRA_BOT_INFINITE_LIVES` 和证据目录，再填入该模式确认过的参数及工作区输出目录。辅助模式显式设为补命，标准模式显式设为不补命，不能把一条全局 `INFINITE_LIVES=0` 套到所有检查。帧预算和退出条件随模式明确记录，不能通过缩短目标关卡获得伪通过。

为本地开发提供显式启动器，例如 `go run ./cmd/dev` 或 `python3 -B scripts/dev.py`，完成「构建当前运行产物→验证来源→生成两 ID→启动 `go run -ldflags=... .`」。开发服务只挂载显式传入的工作区资源目录，并使用独立开发数据目录。该入口可以使用工作区缓存，但这是用户明确选择的开发流程，不是生产初始化的隐式 fallback。

裸 `go run .` 在没有真实链接身份时仍应返回可执行的准备命令说明，不能假装已经注册成功。若要求保留这种字面命令，先由显式准备命令输出真实链接参数，再执行带参数的 `go run`；不通过判断临时可执行文件路径或偷偷读取 `.local` 猜测运行模式，也不提交会随源码变化失效的固定 ID 常量。

## 推荐的资源与 main 构建顺序

1. 将现有 `browser_build.py` 拆为可复用的纯运行资源准备和开发页打包两个入口。正式准备产物不包含验收 `index.html/app.js`、测试账户或开发服务器。
2. 准备真实媒体清单，记录每个 WAV 的 SHA、字节数、MIME、来源和不可变地址。清单是数据来源，不运行 ROM、不生成伪音频。干净构建通过显式、可校验的材料入口取得这些对象；缺少对象就停止。
3. Vite 调用一个明确的 Contra delivery 适配器。建议运行清单仍由应用源提供，Wasm、加载器和关卡／图形负载放入 `dist/assets/contra/`，使用已有 `WIRE_BOARD_ASSET_BASE` 的提交 SHA 目录。稳定音频也可引用独立的不可变媒体清单，避免每个提交重复传输全部音轨。
4. 负载地址只依赖提交 SHA／负载 SHA，不包含尚未算出的运行清单 SHA；最后编码清单并计算 `RuntimeBuildID`。这样应用可以保留 `/contra/runtime-${RuntimeBuildID}/manifest.json`，清单中负载指向 CDN，不形成自引用摘要。
5. `Board.tsx` 对运行清单采用明确的应用源地址或受控入口，不再把公共图片根当作运行程序根。开发与生产清单允许使用不同负载地址，因而可以有不同 `RuntimeBuildID`；各自必须从真实字节计算并链接，不能把开发 ID 冒充生产 ID。
6. main frontend job 产出应用页面、Contra 清单、运行绑定描述和待交付负载；共享及所选游戏检查消费这批同提交产物。源码、工具链、Go 加载器或资产描述变化时必须失效重建。
7. 所有必需检查通过后，在现有 verify job 内复用 CDN 的不可变写入、已存在对象一致性检查、回读 SHA／MIME／CORS；对音频补真实 Range 检查。缺失或冲突均停止，不能覆盖旧对象。
8. 根据与源码／提交／清单绑定的验证回执裁剪服务器包。保留入口、公共加载清单和 Contra 运行清单；移除已经验证的前端执行负载，原生离线视觉包继续以 native-only 数据保留。
9. 原生构建读取同一清单／验证回执产生链接参数，再编译 Linux 候选程序。当前 `native.py` 假设所有负载仍在清单附近，因此需要支持显式 staging 与经过验证的发布回执，不能在裁剪之后直接沿用其本地路径解析。
10. 对候选程序做真实注册、健康、资源引用、加载屏障和最小可玩性 smoke，再标记为可发布构建。正式 tag 继续只复用该 SHA 的已验证二进制，不编译、不重复规则或完整战役，不再次上传运行资源。

`scripts/cdn_delivery.py` 的签名、禁止覆盖和回读逻辑可复用。资源类型扩展应由受限的 Contra inventory／描述校验驱动，不应放开任意目录扫描。已有篮球不可变发布脚本可参考封装模式，但不能照抄其游戏特定规则，也不能把篮球专属镜像资源条款不加区分地套给原生视觉数据。

## 文件与先行测试清单

除上文已明确完成的 CI 范围、共享注册发现和竞态选择外，下表其余内容仍为后续设计；长测 opt-in、环境清理、注册、发布和部署本轮未实施。

| 文件 | 拟改职责 | 应先写的失败测试 |
| --- | --- | --- |
| `games/registry.go`、`games/registry_test.go` | 错误返回的生产组合、显式测试集合、激活后的唯一注册 | 缺身份不能静默少注册／panic；重复 ID；构造失败不返回部分生产列表；真实准备产物下 bot/reclaim/暂停/释放契约存在 |
| `main.go`、`main_test.go` | 先准备模块再打开数据库和监听；清楚报告缺少构建信息 | 无身份／旧身份启动失败；失败前没有监听或初始化数据；生产未调用兼容测试集合 |
| 新开发启动器及其独立入口测试 | 显式资源准备、真实链接参数、隔离开发目录和本地素材挂载 | 缺 WAV、源变化、错路径和失败构建不启动服务；不猜 `.local`；工作区外路径拒绝；退出无残留进程 |
| `games/contra/build/identity.py`、`native.py`、新 delivery／verify 入口 | staging、生产清单、回执、两 ID 和目标平台绑定 | 同规则不同桥接／工具链／loader 必须变地址；源码／提交不符、损坏文件、裁剪后假回执和错目标平台拒绝；双目录构建身份可重复 |
| `scripts/contra/browser_build.py` | 提取可复用准备逻辑，开发页不进入正式负载 | 正式 inventory 出现开发 HTML、测试服务器、ROM／研究文件即失败；仍不把八关和原生视觉包放入 Wasm |
| 新 `games/contra/assets` 媒体交付清单及验证工具 | 替代研究目录依赖，固定 WAV 来源和实际摘要 | 缺对象、错 SHA／长度／MIME、非不可变 URL、错误 Range；不读取凭据或上传即可生成审核计划 |
| `web/vite.config.ts`、`web/scripts/public-assets.mjs`、`web/scripts/game-load-manifest.mjs` | 汇集明确公开目录，首页仍只含目录／封面，负载按关卡加载 | 首页引入引擎／八关数据；重复公开路径；私有文件泄漏；加载清单与实际产物不一致 |
| `games/contra/web/Board.tsx`、`runtime-bridge.ts`、相关资产测试 | 明确运行清单入口，保持精确字节身份和失败重试 | 全局图片根干扰运行地址；清单来源／SHA 不符；stale cache；生产地址与开发 ID 串用 |
| `scripts/cdn_delivery.py`、`scripts/test_cdn_delivery.py` | 受限类型／inventory、回执绑定及裁剪 | 既有对象冲突不写；SHA/MIME/CORS/Range 不合格不得裁剪；不删除旧对象；所有引用都在本次已验证范围 |
| `scripts/ci/scope.py`、`test_scope.py`、`check-plan.py` | 正确归属规则数据、初始化／运行层／前端，注册测试和明确 race 选择 | 上述只读反例先变红；前端-only 不触发完整战役；规则数据必须触发；取消的祖先提交／重命名不漏选；其他游戏不被扩散 |
| `.github/workflows/build.yml` | 依赖安装、真实产物在作业间传递、标准终局／辅助覆盖的独立环境、身份链接 | Contra 没有 esbuild；Python 身份检查漏跑；两个模式串用参数或标签；候选包先于必需检查；绑定描述丢失／换提交 |
| `Dockerfile`（后续源构建路径） | 使用同样的身份与资源准备／裁剪规则 | 无两 ID 的镜像错误启动；把研究材料、Wasm 或未验证负载打入正式镜像；工具链不一致 |
| `.github/workflows/publish-verified.yml`、`scripts/release-source*.cjs` | 原则上保持流程，只补回归断言 | tag 重新编译／跑战役／上传资产；非同 SHA 产物；手动 main 代替 tag；移动已发布 tag |

## 增量范围与验收顺序

建议把现有单个 `backend` 判定细化为该游戏的规则、运行层／初始化、前端／素材和构建工具检查标志。至少先修复 engine JSON／testdata 的错误归属；不要让目录位置或文件扩展名成为唯一依据。

规则、策略、规则夹具变动只运行 Contra 的规则、单／双标准命数自然终局、显式辅助八关覆盖和相关 race。运行绑定／初始化变动运行构建、恢复、HTTP 和真实启动；前端操作变动运行对应前端／资源检查及 PC／320px E2E。共享注册或部署工具变动覆盖公共契约，不固定执行其他游戏完整矩阵。现有「从最近已验证祖先累计范围」及首次完整基线保持不变。

实施顺序为：先把现状反例变为失败测试；再完成纯构建／资源清单与开发入口；再接 main 的同提交准备、检查、验证和链接；等待最新单／双新局自然终局、辅助八关覆盖、恢复／托管与最终来源一致性验收完成；最后才在同一可审阅接入中激活生产注册。缺任何真实身份、必要媒体或验收时，不通过假 ID、静默旧缓存或先注册再隐藏来绕过。

## 本轮选择器复验

先行红测保留了三类遗漏：engine 数据被判为前端、根注册测试缺失、必要 race 未选中。修复后 19 项选择器测试与 14 项 CLI 场景通过；CLI 只在自动清理的工作区临时 Git 仓库中创建测试提交，没有修改工作区提交历史。另实际运行九个新增选中的顶层 Go race 契约，覆盖共享注册与 Contra 初始化、运行产物恢复及回看，没有运行完整战役或其他游戏规则。

```sh
python3 -B scripts/ci/test_scope.py
python3 -B scripts/ci/check-plan.py
```

实际选中的 Go 命令、来源摘要和红绿日志见 `evidence/ci-scope-verification.json` 及相邻的 `ci-scope/` 记录。后续长测接线须保持上方两模式的区别，并在 root 统一冻结后再实现；本轮没有修改 `engine/bot_e2e_test.go` 或 `verification_env`。
