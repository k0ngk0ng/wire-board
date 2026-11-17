# 静态资源复用

应用提交目录用于绑定构建来源；资源正文的地址由内容决定。`scripts/asset_reuse.py` 在前端构建后、产物上传前，为每个逻辑文件选择实际 CDN URL。身份由 SHA-256、大小、MIME 和 Content-Encoding 共同确定，优先复用同一逻辑文件的既有地址，再查询同内容对象；改名和同批别名不会复制正文。

基线来自线上 `cdn-assets-manifest.json`，并核对该提交的 `cdn-local-readback` 成功状态。可选的已验证祖先产物还须匹配状态中记录的同一次成功构建，下载缓存逐文件校验。没有本地 CDN 验证的候选产物不能成为复用依据。

`cdn-assets-manifest.json` 保存逻辑路径到实际 URL 的映射及累计对象记录。`game-load-manifest.json` 仅增加资源入口和必要的程序大小信息，不把完整资源登记表发给每个开局。生成的程序、CSS 和 HTML 引用会随映射更新，重新计算大小、摘要和 gzip 副本；程序不能从历史目录意外加载已经变化的相对依赖。

## 加载与发布

正式页面通过 `/_media/<逻辑路径>` 取得按当前服务器清单生成的 307 跳转，最终地址可以来自不同历史批次。服务器不返回或代理资源正文。Range、方法和浏览器缓存仍由实际 CDN 对象处理；显式本地镜像与独立夹具继续使用各自入口。篮球单独固定发布的资源继续沿用其原始发布空间。

上传按实际 URL 去重。OSS 中已存在的对象必须先核对正文、大小、MIME 和编码；不符或已知地址缺失时失败，不覆盖、不改写地址绕过。PUT 使用禁止覆盖条件，超时后重新 GET 确认，不盲目重复写入。上传收据记录实际复用对象数、PUT 对象数和传输字节数，区别于准备阶段的候选计划。

Actions 核验 OSS 并生成候选包，CDN 回读仍在本地执行。资源登记表摘要与同次构建、回读结果、剥离后的服务器包绑定；正式 tag 继续复用该提交的成功 Linux 构建。旧对象及旧版本引用不删除。

## 复现与验收

先行失败场景见 [资源复用失败清单](../scripts/asset-reuse-failures.md)。所有缓存、下载、临时文件和测试产物保存在仓库内。

```sh
mkdir -p .local/asset-reuse/tmp
export TMPDIR=$PWD/.local/asset-reuse/tmp
export GOTMPDIR=$TMPDIR
export GOCACHE=$PWD/.local/go-build
export GOMODCACHE=$PWD/.local/go-mod
export GOTOOLCHAIN=local
export GOTELEMETRY=off
python3 -B scripts/asset_reuse_e2e_test.py
python3 -B scripts/test_cdn_delivery.py
python3 -B scripts/test_cdn_local_ready.py
node games/racing/build/lint.mjs --config games/racing/build/asset-reuse-lint.json --self-test
go test ./internal/server -run '^TestStaticMedia' -count=1
```

真实发布准备由 CI 在构建后调用 `python3 -B scripts/asset_reuse.py --dist web/dist`。离线复跑可用 `--baseline <仓库内清单文件>` 指定已经验证、留存的清单；这不替代正式发布所需的同次构建和本地 CDN 回读。

E2E 记录保存在 `.local/asset-reuse/result.json`，覆盖原样重跑、只改一个文件、改名、同批重复、响应格式变化、历史地址校验、损坏对象禁止覆盖、相对程序依赖与已验证缓存防篡改。HTTP 验收检查历史／改名目标与危险地址拒绝，原 Range 和包边界检查继续运行。命名、目录与禁用写法的检查范围由 `games/racing/build/asset-reuse-lint.json` 明确登记，不代表全仓库 lint。
