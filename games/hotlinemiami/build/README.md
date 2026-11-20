# 构建与检查

所有命令在仓库根目录执行，缓存、临时目录、测试夹具与产物统一位于 `.local/hotlinemiami/`。入口会幂等创建需要的目录，不依赖开发机已有状态。

## 检查

```sh
node --test games/hotlinemiami/tests/lint.test.mjs
node games/hotlinemiami/build/lint.mjs
node --test games/hotlinemiami/tests/runtime-delivery.test.mjs
```

Lint 递归发现本游戏与 `scripts/hotlinemiami/` 下支持语言的全部自编源文件，规则见 `lint-rules.json`。`assets/public/` 为外部恢复载荷，不按自编代码重命名；该子树中每个文件必须在 `assets/resources.json` 登记并通过字节数和 SHA-256 验证，缺失、损坏、未登记或符号链接均失败。`assets/` 中该子树以外的自编元数据仍按语言规则检查。语言正负例、边界与 CI 范围报告位于 `.local/hotlinemiami/verification/lint/`。运行时交付测试使用实际 Go 编译器构建隔离夹具，覆盖源码变化、重复构建缓存、资源损坏、路径与符号链接边界、正式资源恢复，结果位于 `.local/hotlinemiami/verification/delivery/`。

## 浏览器运行时与素材交付

```sh
node games/hotlinemiami/build/runtime-delivery.mjs
```

`prepareHotlineDelivery(root)` 为 Vite 提供公共根目录 `.local/hotlinemiami/delivery/public/`。游戏文件位于该目录的 `hotlinemiami/`，包括素材、`hotline-motion.wasm`、`wasm-exec.js`、`go-runtime-LICENSE.txt` 和合并后的 `manifest.json`。

素材优先从 `.local/hotlinemiami/assets/manifest.json` 读取；研究素材不存在时，可使用 `games/hotlinemiami/assets/public/hotlinemiami/manifest.json` 中已恢复的正式资源。当前扩充交付清单必须包含 6 张地图、8 个角色、12 类武器以及非空图片集合。旧版 3 张地图、5 个角色、7 类武器的清单即使拥有有效运行时和资源摘要，也不能作为当前版本成功交付。缺失、损坏或不完整资源会明确失败，不发布空游戏目录。

每个资源的 `file` 均为清单目录内的相对路径，并提供实际 `bytes` 与 SHA-256。构建校验全部描述，拒绝父目录、绝对路径、反斜杠和符号链接。新交付先在临时目录完整验证，成功后替换；失败保留上一份有效产物。

WASM 从 `games/hotlinemiami/cmd/motion` 以 `GOOS=js`、`GOARCH=wasm` 构建。缓存绑定 `engine/`、`cmd/`、`data/` 全部 Go 源文件、`go.mod`／`go.sum`、构建参数、构建脚本、Go 工具链版本、官方支持脚本与许可证。已有缓存须再次核对大小、摘要和 WASM 格式，不能仅根据文件存在复用。源码构建期间变化时不发布旧绑定。内容扩充不改写未变媒体字节；正式发布继续按内容摘要复用原有资源地址，只新增实际变化的对象。

Go 官方 `wasm_exec.js` 经过 esbuild 移除代码注释并压缩空白；其 BSD 许可证单独保留。游戏清单的 `runtime` 包含源码与构建摘要，以及 `wasm`、`script`、`license` 三个资源描述。字节相同的重复构建复用缓存，不重写有效交付。

`verifyHotlineDelivery(root, publicRoot)` 复核完整素材、当前源码绑定和运行时；`buildHotlineRuntime(root)` 可单独构建共享运动规则，返回运行时缓存路径、清单和 `cached` 状态。单独构建运行时不代表完整素材交付或游戏 E2E 验收通过。
