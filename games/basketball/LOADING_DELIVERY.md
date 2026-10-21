# 开局加载与 OSS 分发

热血篮球的图片、音频和 Wasm 从 OSS 不可变地址获取。正式构建不再携带这些文件；源码、研究素材和本地验收缓存仍可保留。`web/dist/basketball/v1/runtime/` 只包含 `wasm_exec.js` 和 `build.json`，合计约 48 KB。构建检查拒绝混入篮球媒体文件或 Wasm。

## 开局流程

1. 玩家在房间准备阶段选好球场和球员。开始游戏后，服务端进入 `loading`，保留原版初始运行状态，不推进视频帧、AI 或比赛时钟。
2. 浏览器并行下载 Wasm 与当前球场资源。Wasm 按收到的字节报告进度，球场按实际完成的资源数报告进度。校验、编译和初始化成功后才显示本机 100%。
3. 真人通过平台现有操作通道提交带对局 `epoch` 的 `loaded` 确认。所有未托管真人就绪后，以当前服务端时间开始比赛。
4. 电脑和观战者不参与等待。加载失败时保留错误提示和重试入口；玩家也可使用平台已有的托管入口。不会因等待超时而静默开始。
5. 已开赛对局的刷新和重连只恢复本机；重复确认不会重新计时或暂停别人。加载中的部分确认写入检查点，服务重启后保留。战役下一场使用新的 `epoch`，重新等待必要资源。

背景音乐通过平台媒体播放器按当前场景播放，不进入开局预加载列表。只加载当前球场及其配色、结算素材；跨场地时按需补齐对应资源。第七个球场的浏览器样例请求了 57 个不同的图片／数据资源，清单大小合计 2,507,240 字节；这是该场地的实例，不代表所有场地或网络耗时。

## Wasm 版本绑定

`assets/runtime-manifest.json` 固定源文件摘要、Wasm 内容摘要、长度和 OSS URL。构建使用 `go.mod` 声明的精确 Go 版本，避免开发机与 CI 的补丁版本不同而产生不同字节。每次应用构建都验证当前源码编译结果与已发布对象相符，不能静默引用旧引擎。

更新引擎时，在仓库根目录执行：

```sh
source scripts/env.sh
python3 -B scripts/basketball/publish-runtime.py
python3 -B scripts/basketball/publish-runtime.py --execute
npm --prefix web run build
node games/basketball/build/verify-runtime.mjs web/dist
node games/basketball/build/verify-assets.mjs web/dist
```

不带 `--execute` 只输出发布计划。执行时核对既有对象，不覆盖不同内容，上传后分别从 OSS 和 CDN 回读校验，再更新源码中的版本清单。凭据使用既有 `config/secrets`，不写入清单、前端或验收报告。旧 OSS 对象保留，以支持回滚版本。

## 验证

```sh
source scripts/env.sh
node --experimental-strip-types --test games/basketball/tests/*.test.mjs
BASKETBALL_BROWSER_E2E=1 go test ./games/basketball/server -run '^TestBasketballBrowser' -count=1
go test -race ./games/basketball/server -run '^TestBasketball(RealtimeLoadingHTTP|Loading)' -count=1
python3 -B scripts/ci/scope.py run --game basketball --backend
```

浏览器结果及截图位于 `.local/basketball/loading-browser/`；原有控制器和完整房间界面回归分别位于 `network-board/`、`platform-rooms/`。加载验收使用生产前端和真实服务端，故意延迟 Wasm、使一张球场图片失败，再验证重试、等待第二位真人及开赛后的重载。测试将校验过的 OSS 字节映射到本地测试服务，以适配回环地址的 CORS；这些结果不代表生产网络速度。OSS 上传工具另行保留远端回读记录。

历史文档中关于“容器内完整回退包”的说明记录的是 v1.26.0 及以前的实现，以本文的分发边界为准。
