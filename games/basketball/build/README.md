# 浏览器运行时构建

`npm --prefix web run build` 和 Vite 开发服务会调用本目录的 `prepareBasketballRuntime`，把篮球的独立 Go 规则运行时编译成 Wasm。它只编译 `games/basketball/cmd/browser-wasm` 的实际依赖，不运行其他游戏的规则或完整对局测试。

生成的公开目录只有三个文件，随后由公共素材汇集器复制到 `web/dist/basketball/v1/runtime/`：

- `basketball.wasm`：使用 `GOOS=js`、`GOARCH=wasm` 编译的规则代码。
- `wasm_exec.js`：来自同一次编译所用 Go 工具链的浏览器支持代码。
- `build.json`：Go 版本、构建参数、构建脚本摘要、实际源码清单和两个执行文件的 SHA-256。

源 ROM、原图提取目录、研究日志和源码文件不会复制到公开目录。图片和音频不由此构建器发布；本步骤不改变游戏上架设置。

## 缓存与失败处理

缓存位于工作区 `.local/basketball/runtime-build/`，按源码、工具链、编译器支持代码和构建脚本的摘要分目录。复用前核对来源、文件校验和及 Wasm 有效性。源码或工具链改变后生成新目录；文件或来源记录损坏则重建；编译失败直接停止，不能返回另一份旧源码产物。

使用 `-trimpath`、`-buildvcs=false` 和固定编译参数，清除环境中的隐式 `GOFLAGS`、Go workspace 和实验开关。未显式提供的 Go 缓存与临时目录默认放入工作区 `.local`。相同源码和工具链在不同检出路径下生成相同 Wasm；来源记录不含绝对工作区路径或构建时间。

## main 与 tag

main 前端作业按 `go.mod` 安装 Go，然后构建网页并验证 `web/dist` 中的运行时。Wasm 随网页进入应用的嵌入文件系统，再进入已有的已验证 Linux 构建产物。tag 发布工作流保持原流程，只复用该提交的已验证应用产物，不重新编译 Wasm 或运行游戏规则测试。

从源码构建的 Dockerfile 已让前端阶段使用与后端阶段相同的 Go 基础镜像工具链。本轮验证了本机真实应用构建、嵌入和 HTTP／浏览器执行，尚未执行容器构建或远端 GitHub Actions，不将本地结果表述为远端发布成功。

## 验证

```sh
source scripts/env.sh
node scripts/basketball/accept_runtime_build.mjs
python3 scripts/basketball/accept_runtime_package.py
node games/basketball/build/verify-runtime.mjs web/dist
```

第一项验证真实 Go 编译、不同检出路径的产物一致性、缓存复用、文件损坏、来源记录损坏、编译失败和源码变化。失败场景只修改工作区内单独复制的验收源码，不破坏实际游戏实现。

第二项执行完整网页构建和应用编译，启动真实应用后核对嵌入文件、MIME、生产 CSP 和私有路径的 404。随后在真实浏览器中加载该应用提供的 Wasm，执行 210 视频帧并进入原版比赛阶段；此项不证明画面／OSS 素材发布或完整游戏已完成。

结果保留在：

- `.local/basketball/runtime-build-acceptance/result.json`
- `.local/basketball/runtime-package/result.json`
- `.local/basketball/engine/runtime-dist-verified.json`

实现前失败记录为 `.local/basketball/engine/runtime-build-before.log`、`runtime-manifest-before.log` 和 `runtime-package-before.log`。CI 范围、游戏边界和发布来源检查也已通过，记录在同目录的 `runtime-ci-scope.log`、`runtime-boundaries.log` 和 `runtime-release-guards.log`。
