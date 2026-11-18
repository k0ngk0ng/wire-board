# 原版视觉资源发布与页面验收

当前发布索引为 `997f43b90cbcb6432137d46dfa29ad5c5cfcfbf8a2d151aff933a3a9cfae2c1c`。清单包含原版图集、地图、RNG 动画、调色板及三组字体，共 3485 个逻辑文件、3480 个不同对象。来源绑定 12 个原版 MKF 和固定版本的 3 个字体头文件；完整映射保存在 `assets/visual-release.json`。

3485 个文件已在本地完成 CDN 回读，校验大小、SHA-256、MIME、CORS、PNG 尺寸及 gzip 还原内容。首次上传 32,536,658 字节；相同输入重复发布复用全部 3480 个对象，新增对象和上传字节均为 0。音源及结局的已发布地址继续保留。

`SceneRenderer` 按逻辑路径读取发布映射，不通过拼接当前集合目录重新生成所有资源地址。页面校验固定索引摘要，逐文件匹配发布记录和原索引，并限制响应大小、检查 MIME 与实际字节摘要。下载失败可重试；启用发布映射时没有本地素材回退。`Board` 固定使用当前已验证映射，独立像素研究夹具仍可显式使用本地素材。

## 验收范围

渲染器下载边界先复现本地回退失败，再通过 9 项针对性检查：保留地址、错误索引身份、缺项、记录不符、外部地址、错误 MIME、超限流、内容损坏和失败重试。结果为 `.local/pal/visual-client-e2e/result.json`；它不代替浏览器页面验证。

真实平台开场夹具将 `/pal/v1/` 和本地音源入口设置为 404。PC 和 320px 原生触摸分别通过 CDN 加载、13 次画面点按阅读、保存退出、本人续玩、音乐／进行中音效恢复以及前后台／静音边界。画布分别为 1280×800、320×200，没有虚拟操作按钮。证据为 `.local/pal/cdn-visual-board-browser/result.json` 和同目录两张截图。

后期祈雨另沿用已验收的场景波动核心及其真实第 23 段入口。关闭本地视觉入口后，PC／手机均显示第 228、231、244、245、246、232 场景，完成最后一颗灵珠的原版输入及庆典前交代；手机在第 244 场景保存恢复，完整公开呈现边界保持一致。结果为 `.local/pal/candidate-scene-wave/cdn-solo-prayer-ritual-browser/result.json`，原始入口摘要与此前主线验收一致。

首次后期回归暴露测试操作早于页面收到新对白的问题。失败时页面已就绪、无资源错误，停在第 228 场景的正常阅读边界；测试等待真实页面显示同一段文字后再操作，PC／手机完整重跑通过。未因此修改游戏输入规则。

本轮覆盖开场、后期六场景及完整资源回读。后期加载峰值、全剧情浏览器复盘、实际手机和正式服务器／镜像剥离仍须分别验证。上述后期夹具使用保留核心，不代表新音频核心的真实震屏或完整主线已通过。

结局与谢幕另在本地视觉、结局媒体均为 404 时通过真实 `Board` 复测，包含 PC／手机影片、进行中保存与私人继续、手动回退、附加署名、旧跳过手势失效及原生结束边界，共 6 项结果。证据为 `.local/pal/visual-cdn-checkpoint-ending-board-browser/result.json`；这是独立结局夹具，不计入第 24 段主线完成数。

## 复现

先加载工作区环境，再执行离线检查。以下发布命令只上传尚未发布的不同对象；重复执行仍先校验并复用原地址。

```sh
. scripts/env.sh
python3 -B games/pal/tests/visual-publication-e2e.py
python3 -B games/pal/assets/publish-visuals.py
node games/pal/tests/visual-client-e2e.mjs
python3 -B games/pal/assets/publish-visuals.py --publish
python3 -B games/pal/assets/publish-visuals.py --publish --report .local/pal/verification/visual-publication-repeat.json
npm --prefix web run build
PAL_AUDIO_BROWSER=1 PAL_AUDIO_CDN=1 PAL_VISUAL_CDN=1 go test -v ./internal/server -run '^TestPALOriginalAudioBoardBrowserFixture$' -count=1 -timeout=20m
```

夹具启动后，在另一终端加载同一工作区环境，执行 `PAL_VISUAL_CDN=1 python3 -B games/pal/tests/audio-board-browser.py`。脚本结束时关闭自己的浏览器、辅助连接和夹具，删除浏览器 profile，保留结果与截图。

后期夹具使用下列命令启动；启动后在另一终端加载工作区环境，执行 `PAL_BOARD_VARIANT=scene-wave PAL_VISUAL_CDN=1 python3 -B games/pal/tests/solo-board-browser.py --prayer-ritual`。

```sh
. scripts/env.sh
PAL_BOARD_BROWSER=1 PAL_BOARD_VARIANT=scene-wave PAL_BOARD_ENTRY=solo-prayer-ritual PAL_VISUAL_CDN=1 go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=20m
```

结局夹具使用 `PAL_ENDING_BOARD_BROWSER=1 PAL_ENDING_BOARD_CREDITS=1 PAL_ENDING_BOARD_CHECKPOINT=1 PAL_ENDING_CDN=1 PAL_VISUAL_CDN=1 go test -v ./internal/server -run '^TestPALEndingBoardBrowserFixture$' -count=1 -timeout=22m` 启动。在另一终端执行 `PAL_VISUAL_CDN=1 python3 -B games/pal/tests/ending-board-browser.py --credits --checkpoint --cdn`。

发布回读证据为 `.local/pal/verification/visual-publication.json`；零重复上传证据为 `visual-publication-repeat.json`。资源导出测试的工作区隔离证据为 `visual-export-isolation.json`，防止回归测试覆盖带字体的发布索引。
