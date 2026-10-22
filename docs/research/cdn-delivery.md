# 前端模块 OSS 发布

当前任务同时包含扑克布局稳定、重叠手牌、结算页延迟导入失败和游戏程序 CDN 分发。公共结算组件已从旧版房间大模块中拆出，结算时不再触发该模块下载。

## 实现前的失败场景

1. HTML 引用 CDN，但动态 JS、CSS 或资源清单仍请求应用服务器；带绝对地址的版本检测失效。
2. 模块上传不完整、字节数或校验和错误、错误 MIME、缺少跨域头，构建仍被标记可发布。
3. 同一个不可变地址被覆盖不同内容；旧页面在部署后引用的代码被删除。
4. 新部署先于资源上传，或发布 tag 重新构建导致 SHA 与来源不一致。
5. 去除服务端模块后仍嵌入 JS/CSS；去除时误删入口、清单或其他素材。
6. 凭证写入产物／日志，公开工作流将敏感数据暴露给非受信来源。
7. 本地开发和无 CDN 的验收构建损坏；清单验证把绝对 URL 当本地文件路径。
8. CSP 拦截外部代码或样式；自定义图片域名被错误加入脚本白名单；CDN 未压缩导致程序传输过大。
9. 结算再次延迟加载旧房间模块，完成对局后出现 100% 加载失败；关闭、重开、刷新结果或再来一局失效。

采用提交 SHA 作为 CDN 目录，构建引用 `https://cdn.ichenj.com/bga/web/<SHA>/`。main 完成本次必要检查后上传、完整回读并核对 CORS／MIME／SHA，再从待打包目录移除 `assets/`。入口 HTML、资源清单和 API 保留在应用服务器；既有图片／音效继续使用各自 OSS 版本。tag 仅复用已验证二进制，保留所有已发布代码对象。

JS、CSS 和 SVG 以 gzip 表示上传，保留原 MIME 和 Content-Encoding；校验读取解压后的内容 SHA。CSP 只为固定代码域名开放脚本／样式／字体，不把可配置的图片域名提升为脚本来源。

## 已实现与本地验收

上述流程已接入 main 构建。公共结算组件位于 `web/src/ui/results.tsx`，旧模块仅保留兼容导出。阻止游戏就绪后的新增脚本／样式请求，真实 HTTP 掼蛋对局仍可完成结算、关闭重开、刷新结果和再来一局。

提交 `be9e2f279be454a6a76c18b040b769f5f64950a1` 的测试构建上传并回读了 71 个不可变对象，解压内容共 5,124,390 字节，压缩传输共 1,106,465 字节。测试从本地服务器删除 `assets/` 后，浏览器 12 个代码／样式请求全部来自该提交的 CDN 目录，四个结算流程检查点通过。此数值属于该次测试构建，不承诺以后构建大小不变。

可重复执行：

```sh
. scripts/env.sh
node --test web/scripts/cdn-base.test.mjs web/tests/version-update.test.mjs
python3 -B scripts/test_cdn_delivery.py
go test -count=1 -run '^(TestPublicVersion.*|TestFrontendCDNPolicyHTTP)$' ./internal/server
WIRE_BOARD_ASSET_BASE="https://cdn.ichenj.com/bga/web/$(git rev-parse HEAD)/" npm --prefix web run build
node scripts/game-loading/check-build.mjs
python3 -B scripts/cdn_delivery.py publish
WIRE_BOARD_ASSET_BASE="https://cdn.ichenj.com/bga/web/$(git rev-parse HEAD)/" CDN_TRIM_BEFORE_TEST=1 python3 -B scripts/game-loading/check-results.py
```

本地证据在 `.local/cdn-delivery/verified.json` 与 `.local/poker-layout/results/result.json`。前者逐个记录 URL、MIME、编码、字节数和 SHA；后者包含真实动作数量、CDN 请求和结算结果。main 同样保留 `cdn-assets-verification` 产物。上传凭证只通过 GitHub Secrets 或本地忽略目录读取，不进入前端及发布包。

版本接口继续提供旧页面可识别的 `/assets/` 身份标识，另以 `assetURLs` 给出真实 CDN 地址。新页面归一化地址比较，旧页面仍能收到更新提示；不自动刷新或中断对局。
