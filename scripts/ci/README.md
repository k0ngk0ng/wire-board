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
| 自动检查、手动 `validation=auto` | 全部未登记延后的测试、代表性完整对局及原有竞态选择 | 4 份，最多 2 份并发 |
| 手动 `validation=full` | 全部原有测试，包括全部 208 个登记批次 | 20 份，最多 2 份并发 |
| `Extended Catan acceptance` | 仅卡坦岛的完整验收，不生成应用发布包 | 20 份，最多 2 份并发 |

延后清单在 `scope.py` 的 `CATAN_EXTENDED_TESTS` 中逐项登记，不在 CI 中按耗时或名称猜测过滤。未知新测试默认进入常规范围；没有删除测试实现。常规保留基础、探索者、助手、双人和骑士等代表性完整对局，以及基础完整 HTTP 对局、真实托管完局和全部未登记延后的 HTTP、权限、保存恢复和超时检查。前端／素材变更仍只运行对应前端／素材检查。

每份结果记录 `suite`、实际命令和 `deferredTests`；聚合 `.local/ci/catan/partitions.json` 记录范围、并发数和全部完成／失败分片。任一必需测试失败会使整个任务失败。全量任务只通过明确手动入口触发，失败仍显示失败并保留日志，不设置 `continue-on-error`。

```sh
python3 -B scripts/ci/catan_suite_e2e_test.py
python3 -B scripts/ci/partition_e2e_test.py
python3 -B scripts/ci/scope.py run --game catan --backend
gh workflow run catan-extended.yml --ref main
```

选择范围测试核对常规与完整集合的精确关系、关键契约、未知新测试、真实子进程、清单和失败传播；全量分片重放验证全部测试恰好覆盖一次与最多两段并发。不能用扩大超时、增加任务数量或静默移除断言代替范围复核。
