# 魂斗罗生产初始化与构建准备

本目录只准备本地可验证的构建。生产注册、OSS 上传、main／tag 发布和服务器更新均未接入；完整战役及电脑验收仍是上架前置条件。

## 原生初始化

`server.NewProductionModule()` 在注册模块前完成以下工作：

1. 要求已链接的 `CompiledRulesBuildID` 和 `CompiledRuntimeBuildID` 均为规范 SHA-256，缺失时返回错误。
2. 校验 `server/data/visual-catalog-v1.json.gz` 的固定 SHA、压缩边界、来源 ROM SHA、视觉目录 ID、RGB 长度及每页索引像素摘要，随后复制到不可变内存。
3. 预载并校验内置八关规则资源，以真实 `NewBuiltinBotVision` 构造视觉知识，再把仅接收最终像素与公开 HUD 的观察器交给模块。

归一化视觉包只包含 100 页索引像素和 192 字节 RGB 表，压缩后约 91 KB。它不是 ROM、运行存档、CPU 解释器或外部模型。生产初始化不读取 `.local`、`testdata` 或原始 ROM；回调不保留文件系统读取器，也不访问网络。服务端需要这份数据执行离线视觉策略，前端图形和 Wasm 仍是单独的不可变资源。

视觉包的来源与生成工具摘要在 `server/data/visual-source.json`。只有明确更新归一化来源时才重新生成，不能在服务器启动或房间回调里生成：

```sh
python3 -B scripts/contra/prepare_visual_bundle.py \
  --input games/contra/engine/testdata/bot-visual-catalog.json.gz
```

该命令的输入是已经校验的归一化研究数据，仅用于构建准备；生成后生产代码只嵌入 `server/data` 中的包。既有 testdata 保留原位，没有被移动或作为生产回退目录。

## 三种身份

| 字段 | 绑定对象 | 用途 |
| --- | --- | --- |
| `RulesBuildID` | 独立规则源码和嵌入规则数据 | 规则状态、重放摘要与存档兼容边界 |
| `VisualsID` | RGB 与每页索引像素摘要 | 玩家画面和原生视觉策略使用相同内容 |
| `RuntimeBuildID` | `manifest.json` 的精确字节 SHA-256 | 浏览器可执行程序、Go 加载器、工具链、实际输入清单、关卡／图形／音频描述的不可变版本 |

浏览器地址为 `/contra/runtime-${RuntimeBuildID}/manifest.json`。规则源码不变而桥接代码、工具链、加载器或媒体描述变化时，运行产物身份仍会变化。前端在解析清单或执行 Wasm 前校验原始清单字节，并核对规则及视觉身份；恢复时拒绝另一运行产物的 View。

`RuntimeBuildID` 在 Wasm 产生之后计算，只注入原生宿主；不能再注入参与该摘要计算的 Wasm，否则会形成循环。Wasm 仍注入相同的规则 ID，加载器和 Wasm 文件各自保留内容摘要。

换关确认携带两种构建 ID。只更换运行产物时，既存换关屏障的真人确认需要重新完成；规则 Session、revision 和输入序号保持不变，已正常运行的对局不会因此新建全员屏障。规则 ID 或视觉目录 ID 变化仍遵循原有严格存档校验，本准备工作没有新增跨规则版本迁移。

## 本地构建

```sh
source scripts/env.sh
python3 -B scripts/contra/browser_build.py --out .local/contra-production/build
python3 -B games/contra/build/native.py \
  --runtime .local/contra-production/build/public/manifest.json \
  --out .local/contra-production/wire-board
```

运行资源构建使用已提交的归一化视觉包，不再依赖 Pillow 或研究 PNG 目录。当前浏览器音频打包仍要求已校验的本地 WAV 来源可读；本任务没有伪造发布地址或上传媒体。最终发布阶段须先提供并验证相应不可变对象。

原生构建入口核对当前源码、工具链、视觉目录、关卡 bundle 和实际资源字节，再统一生成两个 `-X` 身份参数。编译前后再次核对来源；来源或清单改变、资源缺失／损坏时停止，不复用旧包。输出旁的 `.build.json` 记录目标平台、编译参数、实际输入及可执行文件 SHA-256。

需要 Linux 产物时显式传入 `--goos linux --goarch amd64`。构建整个应用仍使用现有注册列表，因此这个命令不会使魂斗罗在大厅出现。最终接入时，组合入口须显式调用并处理 `NewProductionModule()` 的错误；不能用空身份、固定占位 SHA 或读取工作区的回退方式绕过初始化。

本地构建不会修改 Vite 的自动素材发布、CI 或 tag 流程。现有公共 CDN 发布器只处理 `web/dist/assets`；魂斗罗独立运行资源的上传、校验和服务器包排除必须在最终发布接线时明确完成，不能把整个私有构建目录直接复制进容器。tag 仍只复用同一提交已经验证的构建。

## 复验

```sh
source scripts/env.sh
python3 -B games/contra/build/identity_test.py
go test -race ./games/contra/server ./games/contra/runtime
node web/scripts/test.mjs contra
python3 -B scripts/contra/accept_production.py \
  --runtime .local/contra-production/build/public/manifest.json \
  --out .local/contra-production/accept-recheck
```

最后一项使用真实 Go 编译：携带链接身份的程序在空目录中完成生产初始化并推进真实电脑回调；未链接身份的程序拒绝启动，规则／来源／工具链／视觉身份错配以及损坏的 Wasm、越界资源路径均拒绝进入原生编译。它只验证初始化与构建边界，不声称电脑已经完成八关。

实际 Wasm 构建同时检查八份关卡 JSON 及其独有片段未被嵌入，并检查原生归一化视觉包未进入 Wasm。浏览器 fixture 已复用正式资源读取器，对照嵌入视觉目录，保留真实观察器和前端清单摘要；使用 `scripts/contra/platform_accept.py` 可重新执行真实 PC／320px 房间验收。

本轮结果保存在 [`production-initialization-verification.json`](../../../docs/research/nes-contra/evidence/production-initialization-verification.json) 及其相邻的 `production-initialization/` 目录：原生初始化十个命令检查通过，离线真实策略推进 12 帧；服务端 67 个 race 条目、运行层七项、前端 58 项及构建身份两项通过。真实房间已通过新运行产物地址下的 PC／320px、配置隔离、托管、650 毫秒传输停顿和隐藏期间检查点恢复。本轮原生规则、运行产物与视觉身份分别以结果记录中的三个摘要为准，后续源码修改需重新构建验证。
