# 星垒高频音效与 iPhone 卡帧

用户在同一线上对局反馈：电脑流畅，iPhone 画面约每秒卡顿；关闭平台音效后明显改善。观战时网络 RTT 约 10～15 毫秒，房间消息约 30～40 KiB，不能用桌面观战的平均帧率否定手机上的问题。

星垒原先通过平台单次播放接口发出每个声音。每次请求会停止旧任务、移除媒体源、调用 `load()`，再设置音源并播放；高频战斗会反复重建原生媒体播放链路。

## 修复与边界

- 星垒按有限的音效名称使用固定通道，显式开启 `reuseLoaded`。同一通道的新事件只重新定位并播放，保留已载入的媒体源；已成功加载的本地备用源也可复用。
- 每次实际提示具有独立播放标识，不使用事件 ID 创建通道；持续保留每位所有者 16 条、全平台 32 条的通道上限。
- 重复播放标识不重播；旧播放、metadata 和 ended 回调不能复活过期声音或停止新事件。换源、失败、暂停、全局静音和离桌仍按原有所有权清理。
- 不重复设置已经为 `playback` 的原生音频会话。未显式开启复用的调用保留原先的替换行为。
- 继续使用平台原生媒体接口；未增加游戏内独立播放器或音频开关。

实现前失败场景和异步边界见 `web/tests/platform-audio-reuse.test.mjs` 的文件头；新增隔离测试先于实现编写，并保留失败基线。

## 验收

```sh
. scripts/env.sh
node --experimental-strip-types --test web/tests/platform-audio*.test.mjs web/tests/audio-preferences.test.mjs
npm --prefix web test -- starfort
npm --prefix web run build
python3 -B games/starfort/tests/accept_audio_reuse_ui.py --skip-build
```

`accept_audio_reuse_ui.py` 使用真实 HTTP／WebSocket 房间、生产战场和原生媒体播放器，检查预热后的音源设置、`load()`、实际播放、通道数量、全局静音和离桌清理。它只计数，不替换播放行为。结果在 `.local/starfort-audio-reuse/browser/`。

另有 WebKit 原生媒体对照，连续播放 24 次四类声音；前后均实际成功播放全部请求。旧有限音效通道为每个新实例创建播放器，预热后继续出现 20 次音源设置、40 次 `load()`；开启复用后两项均为 0。该对照验证原生媒体生命周期，不把 macOS WebKit 当作物理 iPhone 的帧率证明。原始结果、调用计数和复现脚本保存在 `.local/starfort-live-lag/native-audio/`。

本修复纳入本轮已授权的星垒修订。上述自动验收验证媒体生命周期与实际播放；物理 iPhone 的改善仍需发布后实机回归，不能用桌面或模拟手机帧率替代。
