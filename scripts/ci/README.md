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
