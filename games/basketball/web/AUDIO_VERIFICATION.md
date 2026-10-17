# 篮球网页音频适配验证

实现前失败场景：

- 原控制 ID 0／253／255 被当作 WAV；暂停继续通过重播曲头伪装。
- 新建 Audio 或 AudioContext，绕过平台统一静音、生命周期和手机媒体通道。
- 27 个已实现效果 ID 缺项；未知或非法 ID 被猜测映射为素材。
- OSS 和本地候选源被当作两段播放列表；本地预览 URL 与正式不可变路径不能分别配置。
- 重复／旧事件在重连或快照刷新时重播；重置、离桌后未取消旧声音。
- 同批控制命令被异步等待重排，尤其 253／255／253／255。
- 原音乐的未验收 133 秒录音被直接设为循环；未经核验的候选接缝被称为原循环。
- 播放失败或浏览器自动播放拒绝阻断游戏帧；静音状态被音频适配器擅自解除。
- 压缩音乐缺失时退回巨大 WAV、CDN 与本地选用不同文件，或编码延迟／填充改变时长和循环边界。

适配器仅负责 ID 到来源以及控制命令映射，使用平台 GameAudioPort。公共 pause/resume 与 music(loop) 由独立平台分支实现，本分支不建立私有播放器。先编写针对性测试，再实现适配器；浏览器真实播放交由完整客户端的后续 E2E 验收。

## 调用接口

```ts
const sounds = new BasketballAudio(boardProps.audio, {
  assetsBase: "https://example.invalid/assets", // 可省略；追加不可变版本路径
  localBase: "/research/basketball/audio", // 研究服务器目录；正式默认 /basketball/v1/audio
});
void sounds.consume(step.soundIDs, step.sequence);
sounds.reset(restored.sequence); // 停止旧会话，设定恢复水位，不重播历史批次
sounds.dispose(); // 离桌，只清理该平台 owner
```

`consume` 接受原事件数组和可选递增序号，返回 `{ignored, unsupported, failed}`；音频失败不会 reject 游戏调用。同批平台调用同步依序发出，不因某个媒体 Promise 等待而推迟后续控制。重复／旧序号不播放；不提供序号时，调用方必须保证只提交新事件。

当前映射包括首批 27 个效果、后续场地／特殊球新增的 13／14／30／46，共 31 个效果；音乐包括各场地独立的 59～66 和有限结局曲 75。76 只保留前奏加循环段的原时间轴预览，尚未注册运行时播放。`0 → stop()`、`253 → pause()`、`255 → resume()`；恢复不会调用 `music()` 重新装载源。音频适配器不读取或修改平台静音偏好。

原 `StartOriginalCourt` 已按场地表产生 66。客户端只消费该增量，不因收到 `StageState` 或刷新画面而重复补发音乐／暂停命令。

## 验证结果

`games/basketball/tests/audio.test.mjs` 的 6 项测试覆盖全部效果映射、OSS／本地候选顺序、控制交错、重复批次、恢复水位、未知 ID、同步抛错／异步拒绝及未完成播放 Promise。实现前失败记录、通过记录分别为 `.local/basketball/audio/adapter-before.log`、`adapter-after.log`。全前端 TypeScript 编译通过，记录为 `adapter-typecheck.log`。

后续增加各场地独立音乐、有限结局曲／76 边界以及新增效果测试。有限曲 75 的真实浏览器到尾测试还发现公共媒体 Task 结束后 owner 信息丢失；先保留失败并写 4 项回归，再补充加载来源的 owner。该轮公共音频及游戏侧 53 项通过，未运行其他游戏完整对局。有限曲结束后 dispose 的真实来源清理也已重验。

压缩分发阶段新增「缺少压缩条目不能回退至巨大 WAV」测试，该轮游戏侧共 10 项通过，记录为 `.local/basketball/audio/compression/adapter-after.log`；实现前失败记录为 `no-wav-fallback-before.log`。这些数量描述对应保存记录，不覆盖后续联机恢复接口的新增测试。

```sh
source scripts/env.sh
node --experimental-strip-types --test games/basketball/tests/audio.test.mjs
web/node_modules/.bin/tsc -p web/tsconfig.json
```

公共平台 pause/resume 的真实媒体时序由其独立测试覆盖；本映射测试不冒充浏览器完整 E2E。

## 持续音乐与派生素材

未加工的原音乐仍保留，原接缝审计由 `scripts/basketball/verify_audio_seam.py` 复现：单循环节拍相位不同，完整驱动状态需经过 16,384 帧（5 次循环）才相等；直接裁剪仍有 PCM 相位跳变。因此没有直接循环原来的 133 秒录音。

播放清单 `games/basketball/assets/audio-playback.json` 为 66 保留独立的 `music-066-loop.wav` 母版，实际 `file` 指向压缩分发的 `music-066-loop.mp3`。适配器通过同一个平台 `music(sources, {loop: true})` 持续播放，暂停／继续仍使用原平台接口。原素材清单 `audio-catalog.json` 不改为循环成品，以免混淆未加工证据和派生输出。

派生文件取两次独立原版 20,000 帧录音中的帧区间 `[3279,19663)`，样本区间 `[2405344,14427786)`，时长 **272.617732 秒**。保持完整 5 次循环长度，只在最后 **1,411 个样本（31.995 毫秒）**，以余弦渐变权重衔接同一音乐位置的两段原 PCM；没有缩短周期、加入静音、变速或生成替代音符。99.9883% 的样本保持原样。具体公式、源片段、工具和 SHA 均保存在 `music-066-loop-source.json`，两次独立渲染及两次派生结果相同。

`verify_music_loop.py` 在实现前已定义素材验收：完整驱动状态相等、周期长度不变、窗口外样本不变、接缝连续、无削波，以及 8 毫秒窗口／2 毫秒步长的局部 RMS 偏差不能超过 ±2 dB。实际结果：边界相对原相邻样本误差为 0，末 16 样本误差为 0，局部 RMS 最大偏差约 −0.308 dB，最大相邻步幅 1,982，低于两段原源的 2,249。没有仅凭首尾两个样本相等就宣布通过。

独立 `agent-browser` 原生媒体验收使用真实循环 WAV、平台 GameAudio 和本适配器，seek 到曲尾前测试两次实际循环，没有缩短素材或提高播放速率。两次都跨尾继续播放、保持同一媒体及来源；253／255 保留播放位置，dispose 清理来源。产物在 `.local/basketball/audio/loop-browser/result.json` 和 `verified.png`。

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/derive_music_loop.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_music_loop.py
# 重新派生母版后，重新建立压缩分发条目；不必重新录音。
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/encode_music_distribution.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_music_distribution.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/accept_audio_loop.py
```

这是明确披露的 PCM 派生文件，不是逐样本原始录音。人工听感、全场混音和设备矩阵仍未验收，`listening_accepted` 与 `accepted_for_release` 均为 `false`，未上传 OSS。

## 压缩分发的真实媒体结果

9 首运行时音乐已改为 MP3 128 kbps，完整解码样本数与 WAV 母版逐项相等，合计从 1,343,940,008 字节降至 243,808,096 字节。CDN 和本地候选都使用压缩文件；没有 WAV 网络回退。完整参数、格式取舍、分发清单约定和复现命令见 [MUSIC_DISTRIBUTION.md](../assets/MUSIC_DISTRIBUTION.md)。

8 首压缩循环曲分别通过两次实际跨尾，75 通过有限曲到尾，另有 66 模拟 CDN 失败后使用本地 MP3 的用例。全部验证 253／255、来源保持和 dispose 清理，并检查未请求 WAV。结果在 `.local/basketball/audio/encoded-loop-browser-NNN/` 与 `encoded-loop-browser-66-cdn-fallback/`；汇总脚本 `scripts/basketball/verify_music_browser.py` 将 10 个结果绑定至素材和实际测试 bundle 的 SHA。

验证使用本地 Chromium，未覆盖真实 Safari／iOS／Android。记录属于对应保存的编译 bundle，不能覆盖公共播放器或适配器后续的联机恢复修改。
