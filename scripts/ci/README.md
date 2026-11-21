# CI 检查范围

默认按最近一次成功验证的祖先提交计算变更范围，包含被取消或失败的前序提交。检查计划保存为 `verification-plan`，列出变更文件、快速检查组件、公共检查和游戏任务。

## 自动选择

| 变更 | 执行范围 |
| --- | --- |
| 许愿、聊天、玩家资料、网络状态、游戏排序、账号密码等已映射的独立模块 | 对应模块的定向测试，不启动整套公共竞态检查 |
| 公共前端 | 公共前端测试和应用构建，不启动服务端竞态检查 |
| 公共服务端核心、模块接口或未映射的公共代码 | 一个公共检查任务 |
| 游戏代码 | 每个受影响游戏一个任务；改到三个游戏，就并行执行这三个任务 |
| 游戏前端或素材 | 对应游戏的前端、素材检查，不启动该游戏的服务端完整对局 |
| 独立封面 `games/<game-id>/web/Cover.tsx` 及对应 `accept_cover.py`、`cover_harness.tsx` | 公共前端检查和应用构建，不启动 game CI；混合修改和手动指定游戏仍执行对应游戏检查 |
| CI 配置和检查选择脚本 | 流程自身的测试，跳过应用构建、CDN 上传和二进制打包 |
| 仅 Markdown 文档 | 记录计划，跳过功能检查和应用构建 |

快速检查独立于前端构建启动。模块与测试文件的对应关系在 `scope.py` 的 `FAST_COMPONENTS` 中维护；新模块应先补充范围重放用例，再加入映射。缺少映射的公共服务端代码保留公共检查，不能仅凭改动行数跳过测试。

## 手动入口

```sh
# 平台独立模块的快速验证，不构建发布包、不上传 CDN
gh workflow run build.yml --ref main -f validation=quick

# 自动选择范围；也可以额外指定一款游戏
gh workflow run build.yml --ref main -f validation=auto -f game=rail

# 明确要求全量验收
gh workflow run build.yml --ref main -f validation=full
```

`quick` 遇到公共核心或游戏改动时会报错，要求使用 `auto`，不会静默缩小验证范围。涉及前端的平台小改动同时进行 TypeScript 检查。游戏在 `auto` 下已经按前端／服务端改动区分检查范围。

普通源码变更仍需构建当前应用。自动模式的构建与快速结果分别显示；快速结果不等待资源上传和二进制打包。仅修改 CI 或文档时没有发布包，这类成功验证也会推进增量基线，避免下一次把旧改动重复计算进来。

资源发布、恢复与复用工具及其测试归 CI 工具验收，同时要求重新构建候选包；它们不改变游戏规则或游戏检查输入。OSS 发布使用 HEAD 验证当前构建对象，并复用已通过本地回读的外部对象身份记录，不 GET 对象正文。本地 CDN 回读继续执行正文、MIME、编码、CORS 和音频 Range 校验。真实 HTTPS 验收覆盖去重、签名、并发冲突和损坏导致整批失败。

## 零付费下载与资源缓存

普通 CI 的资源正文下载预算为 0。前端、共享、服务器和单游戏分别通过 `.github/actions/restore-resources` 恢复 GitHub 缓存；缺失或损坏时失败，并上传 `resource-traffic-*` 报告，不自动访问 OSS／CDN。主工作流、全量游戏入口和魂斗罗研究入口使用同一约束。

| 用途 | 恢复范围 |
| --- | --- |
| `frontend` | 公开素材、前端导入的图片与字体、声明的客户端运行时资源、魂斗罗音频包和必要生产嵌入输入 |
| `server` | 生产嵌入输入、PAL 服务端构建头文件和已登记服务端发行包 |
| `shared` | 平台测试资源和生产嵌入输入 |
| `game` | 指定游戏、公共测试和生产嵌入输入 |

缓存键只取本用途选中的对象身份和映射。应用代码变化或无关游戏资源变化不会改变缓存键；旧键恢复后仍按当前清单验证每个对象。每个对象按内容地址去重，正文验证通过后才恢复逻辑路径。

游戏缓存与报告分别保存在 `.local/<game-id>/cache/ci-resources/` 和 `.local/<game-id>/verification/ci-traffic/`；平台缓存与报告使用 `.local/ci-resource-cache/` 和 `.local/ci-traffic/`。手动准备发现精确命中的缓存键已损坏时，校验修复成功后仅替换该键；完整缓存不重新上传。

首次启用、增加资源或缓存被 GitHub 淘汰后，需要显式准备对应用途。先查看计划，再以明确字节预算运行手动工作流。单次上限为 2 GiB；预算为 0 时拒绝缺失对象下载。预算按缺失对象的最大响应大小预留，包含 gzip 上界和超限探测字节；失败不自动重试，后续准备保留并复用已校验对象。

```sh
python3 -B scripts/ci/resources.py plan --scope frontend
python3 -B scripts/ci/resources.py plan --scope game --game gomoku
gh workflow run resource-cache.yml --ref main -f scope=frontend -f max_download_bytes=536870912
gh workflow run resource-cache.yml --ref main -f scope=server -f max_download_bytes=16777216
gh workflow run resource-cache.yml --ref main -f scope=game -f game=gomoku -f max_download_bytes=67108864
```

示例预算不是自动授权；实际缺失量以本次计划和缓存报告为准。准备工作流只读取私有桶，不发布版本、不更新服务器。普通资源恢复步骤没有 OSS 密钥。资源缓存只保存经过摘要验证的内容，不保存凭据或旧的 `wire-board` 服务端二进制。

复跑以下验收只访问本机服务，不消耗线上资源流量。结果保存在 `.local/ci-traffic/`；并发发布结果仍保存在 `.local/resource-cleanup/cdn-concurrency/`。

```sh
python3 -B scripts/ci/resources_e2e_test.py
python3 -B scripts/ci/publication_traffic_e2e_test.py
python3 -B scripts/ci/traffic_lint_e2e_test.py
python3 -B scripts/ci/traffic_lint.py --self-test
```

## 魂斗罗与资源绑定

`contra` 是魂斗罗的游戏 ID。初始化、错误注入和竞态验收归魂斗罗任务，只有改到该游戏时才执行。电脑通关研究迁至独立的 `Optional Contra computer research` 工作流，只能手动触发，不出现在普通主流程中。

应用打包仍须保留客户端运行时与服务端的构建标识绑定。这是编译约束，不包含魂斗罗对局、错误注入或竞态测试。Go 构建和 Actions 缓存统一使用 `.local/go-build`、`.local/go-mod`，避免缓存目录与实际编译目录不一致。

## 可重复验证

```sh
python3 -B scripts/ci/check-lanes.py
python3 -B scripts/ci/check-plan.py
python3 -B scripts/ci/test_scope.py
node --test scripts/ci/baseline.test.cjs scripts/release-source.test.cjs
actionlint .github/workflows/build.yml .github/workflows/contra-research.yml
```

`check-lanes.py` 创建临时 Git 仓库、生成真实提交、调用计划 CLI，并实际执行选中的 Go 测试。它还运行工作流的最终汇总脚本，验证失败、漏跑和跳过的任务不会被标记为通过。结果保存在 `.local/ci-lanes/result.json`；临时仓库在结束时删除。CI 的 `quick-component-acceptance` 保留执行日志和重放结果。

发布仍复用同一提交的已验证二进制，不重复运行功能测试。构建流水线本身不会创建版本、发布镜像或更新服务器。

## 卡坦岛验收范围与超时复盘

构建 `44591113156` 的卡坦岛任务触及串行测试的 150 分钟总时限，退出前没有断言失败。首个引擎测试包已完成的 1,392 个顶层测试合计约 149.5 分钟，其中 118 个超过 10 秒的测试合计约 141.4 分钟。该记录没有覆盖尚未执行的部分，不能视为整套测试的完整耗时基准。

最初保留全部用例并限制为两段并发，解决串行进程总超时，但仍需让常规检查的覆盖与成本匹配。经过逐项区分，93 个引擎自然完局／扩展组合批次，以及逐项核对的 115 个 HTTP 批量完局／嵌套验收进入显式全量范围；较慢的配置、守恒、私有牌堆、隐私、时钟和恢复契约继续作为常规必需检查。上述已完成部分中，93 个批次约占 129.1 分钟，其余约 20.4 分钟。旧记录尚未进入 HTTP 测试包。另查明 `verification_env` 对普通服务端检查无条件设置 `WIRE_BOARD_NESTING_ACCEPTANCE=1`，误启原本要求手动执行的 15 场海图嵌套完整对局。该开关现在先从继承环境清除，仅显式全量模式设置为 `1`。真实流程复核发现，该包又对同一扩展组合按人数、事件和地图逐次执行数千至上万次 HTTP 行动，因此需要单独区分批量完局与协议契约。实际墙钟时间以各次验收产物为准。

| 入口 | 卡坦岛服务端范围 | 单个任务内执行 |
| --- | --- | --- |
| 自动检查、手动 `validation=auto` | 全部未登记延后的普通测试、代表性完整对局、并发与观战竞态 | 8 份，最多 2 份并发 |
| 手动 `validation=full` | 全部原有测试，包括全部 218 个登记批次 | 20 份，最多 2 份并发 |
| `Extended Catan acceptance` | 仅卡坦岛的完整验收，不生成应用发布包 | 20 份，最多 2 份并发 |

延后清单在 `scope.py` 的 `CATAN_EXTENDED_TESTS` 中逐项登记，不在 CI 中按耗时或名称猜测过滤。未知新测试默认进入常规范围；没有删除测试实现。常规保留基础、探索者、助手、双人和骑士等代表性完整对局，以及基础完整 HTTP 对局、真实托管完局和全部未登记延后的 HTTP、权限、保存恢复和超时检查。前端／素材变更仍只运行对应前端／素材检查。

普通时钟、恢复和超时断言仍然必需。常规竞态选择聚焦真实并发与观战读取，避免为相同断言再次执行全部耗时前置。`TestCatanConcurrentHTTPActionsAndSnapshots` 使用四个真实客户端并发读取快照，按合法行动推进对局，并检查观战操作拒绝且不改变版本；完整竞态组合保留在显式全量范围。

运行 `44655131330` 暴露了十个遗漏的批量完局：部分名称没有 `Natural`，例如运输航海会按两张地图、2～6 人跑十场终局。已逐项加入完整批次，独立时钟、恢复、配置和权限用例继续必需；常规分段改为八份、并发仍为二，减少末段负载不均。不能根据测试名称猜测其实际成本。

每份结果记录 `suite`、实际命令和 `deferredTests`；聚合 `.local/ci/catan/partitions.json` 记录范围、并发数和全部完成／失败分片。任一必需测试失败会使整个任务失败。全量任务只通过明确手动入口触发，失败仍显示失败并保留日志，不设置 `continue-on-error`。

```sh
python3 -B scripts/ci/catan_suite_e2e_test.py
python3 -B scripts/ci/partition_e2e_test.py
python3 -B scripts/ci/scope.py run --game catan --backend
gh workflow run catan-extended.yml --ref main
```

选择范围测试核对常规与完整集合的精确关系、关键契约、未知新测试、真实子进程、清单和失败传播；全量分片重放验证全部测试恰好覆盖一次与最多两段并发。不能用扩大超时、增加任务数量或静默移除断言代替范围复核。

## 超过 30 分钟的游戏任务

已核对运行 `44620418167` 的成功任务，并与 `44591113156` 交叉比较。以下是旧范围的实际任务总耗时，包含准备步骤；新范围的用时以新一轮产物为准。

| 游戏 | 旧任务耗时 | 常规必需范围 | 显式全量范围 |
| --- | ---: | --- | --- |
| 幽游白书 | 76.1 分钟 | 规则、联机、运行时绑定、完整人数／模式对局；两套轨迹各选五条完整路径，覆盖双人、三人、四人、组队、平移和特殊场地 | 全部 49 条轨迹及五个耗时精确音频／原版整段对照批次 |
| 星垒 | 37.4 分钟 | 当前公共规则的自然完局、40 波无尽及全部未登记延后的规则／HTTP／恢复检查 | 12 个历史版本或组合的自然完局批次 |
| 热血篮球 | 37.3 分钟 | 正常时钟完整 HTTP 比赛、原生引擎、输入、托管、恢复和呈现竞态 | 三个阵容／选人／场地矩阵父用例，共 208 组配置 |

篮球的完整 HTTP 比赛仍在常规服务端范围开启。`BASKETBALL_PREPARATION_MATRIX` 与 `BASKETBALL_SELECTION_ADMISSION` 从继承环境清除，仅显式全量模式启用，防止本机或上层环境误启矩阵。现有源码与客户端运行时绑定检查保持必需。

幽游白书常规与全量分别在单个游戏任务内分成 4／16 份，最多两份并发；前端和运行时绑定仅执行一次。轨迹延后名单逐项登记，未知新增轨迹默认进入常规范围。星垒和篮球继续使用一个游戏任务，不扩散为 Actions 矩阵。

四款长任务的常规任务总预算为 30 分钟；预算包含资源恢复和工具准备。达到预算即失败，必须再次分析检查范围或实现成本，不能静默跳过失败。显式全量入口独立于常规发版，仍保留所有原测试和失败日志。

```sh
python3 -B scripts/ci/game_suites_e2e_test.py
gh workflow run games-extended.yml --ref main -f game=yuyuhakusho
python3 -B scripts/ci/scope.py run --game basketball --backend --extended
```

`--catan-extended` 仍作为 `--extended` 的兼容别名。各份结果中的 `suite`、`deferredTests`、`deferredSubtests` 和实际命令共同记录检查范围；延后清单属于整个套件，不应把各分片重复记录的清单相加。范围与真实进程重放检查同时验证失败传播、未知新用例纳入和完整入口恢复。

## 未变化输入的检查结果

`resume.py` 检查已结束的 main push 祖先运行；即使已有完整成功基线，也会复用失败运行中已经通过且输入未变的游戏。已有基线时只核对从该基线起实际需要验收的游戏，不把其他未变游戏加入待运行集合。公共与前端检查必须有成功来源；全局环境、前端、公共、游戏和汇总作业的定义必须相同。候选构建的依赖、条件和时间预算也必须相同，其发布步骤变化会要求重新构建候选包。计划器按既有归属识别实际改动；公共代码或公共前端变化不能使用这条恢复路径。手动全量或显式指定游戏时不自动复用。

每款游戏的普通、竞态、前端、运行时绑定、环境与内部并发定义逐项比较。可以从不同祖先运行组合成功结果，但每款游戏都记录实际提交、运行、任务和输入摘要；从该来源到当前提交不得再有该游戏改动。较新的同输入失败不能被更旧的成功覆盖，缺少有效成功来源的游戏继续必需。重复任务名或不明确的证明不能用于复用。

证明记录祖先提交、当前提交、运行与任务编号、选择摘要、复用和待运行游戏；计划器核对证明与当前 SHA、基线和完整游戏集合。新一轮仍从当前源码重新构建前端、绑定和二进制，发布仍要求同一提交的成功构建和本地 CDN 回读。完整手动验收不使用部分结果恢复。

候选二进制的启动冒烟使用仓库 OSS 桶、端点变量及配套的两个 secret，并将端点主机名补全为程序要求的 HTTPS 地址。四项必须完整传入，不能让全局桶与端点配置单独进入启动环境；发布容器的独立冒烟不继承宿主环境。OSS 核对每完成 1,000 个对象及最后一批输出实际完成数量和整数秒耗时，失败批次不会输出全部完成。

## 游戏命令边界

根 `cmd/` 只放平台命令。直接导入 `games/<game-id>/...` 的游戏专属验证器应放在 `games/<game-id>/cmd/`。Go lint 将该目录识别为 `package main`；`game_tools.py` 在 CI 中扫描根命令目录，拒绝游戏实现导入，并通过正负例重放。任何根命令变更都会启动该边界检查。

幽游白书像素验证器的当前入口是 `go run ./games/yuyuhakusho/cmd/verify-arena-aux`。旧记录保留原路径与原始摘要用于追溯；新的检查结果写入 `.local/yuyuhakusho/verification/arena-aux-tool/`，不改写历史通过结论。
