# 原生媒体接口与有限声道接入

平台基线只有一条有限效果通路，旧接口继续保留这个语义。Root 新增的 `effect/stopEffect` 已通过 8 项真实 Chromium 检查，能同时播放两条有限音效与 BGM；基线能力缺口的 red 仍保留。非零循环点经过两次真实回跳，但事件连续不等于输出样本无间隙。

本分工没有修改共享播放器或游戏代码。基线为 `aa617e23` 的 `web/src/audio.ts`，SHA-256 为 `0901dad46773da863c3889788dd5f4ce8bc215360d384f83d3a32bf9f347dfb7`。Root 后续新增接口的验证与这个基线分别记录。

## 当前接口的实际合约

| 入口 | 实际行为 | 接入边界 |
| --- | --- | --- |
| `play(sources)` | 一个有限任务，数组是最多四段顺序播放；新任务替换旧任务 | 不表示来源替代或独立声音实例 |
| `playWithFallback(sources)` | 一个有限任务，前两项是同一片段的主来源和 fallback | 第二个有限请求会截断第一个；不同 owner 也不产生第二条有限通路 |
| `music(sources, options)` | 一个独立背景媒体；有 `loop/startOffset/playbackID/loopStartSeconds` | 与有限效果共存；原版音乐的媒体和抢占规则仍须另证 |
| `loop(key, sources, options)` | 同 owner 可有多条媒体；固定 `loop=true`，可调音量和速率 | 是连续效果；没有有限实例、起点和自然结束语义，不能冒作双 PCM |
| `pause/resume` | owner 范围，保留当前媒体位置 | 规则暂停与页面隐藏不同；隐藏由宿主调用 `stop()` 清除 |
| `stop/stopMusic` | owner 范围清理；后者只清理背景 | 旧回调和 Promise 不得影响新 owner |
| 平台静音／音量 0 | 清理现有声音；恢复开关不会自动重播 | 游戏适配器只能按当前状态显式恢复，不能补播旧事件 |

`music` 的去重 key 包括来源、循环选项、实例 ID 和循环起点；同一仍在播放的实例不会因更新 `startOffset` 重播。原生 metadata 负责时长校验和累计位置取模。非零起点关闭 `media.loop`，使用 `ended → currentTime → play` 返回后缀；该路径不能称为原生整段循环。

宿主通过 `audio.channel("room:<id>")` 提供 `BoardProps.audio`，离桌、用户切换、隐藏、全局静音继续由宿主管理。原 `GameAudio.play(Sound)` 中 preview／finish／turn 的优先级及其他游戏现有 `play/music/loop` 行为不能因 MD 通路扩展而改变；原 MD 优先级值不能直接传给平台代判。

## 最小扩展提案

```ts
type FiniteEffectOptions = {
  playbackID: string;
  startOffset?: number; // 已经过的媒体秒数；不是未来起播时刻。
};

effect(
  lane: string,
  sources: readonly string[], // 同一有限片段的 fallback。
  options: FiniteEffectOptions,
): Promise<boolean>;

stopEffect(lane: string, expectedPlaybackID?: string): void;
```

`lane` 表示少量稳定的媒体通路，实例 ID 表示该通路的一次声音。两者不能混用，以免为每个事件无限创建媒体。平台不判断 MD 的请求优先级、分派和硬件通路；独立驱动输出已经决定的媒体动作。

同 lane 的同一实例刷新不能重播；同实例更换来源应拒绝且不打断原声音；新实例即使使用相同声音，也应正常重新起播。最近一个已结束的实例需要有限的去重记录，避免快照刷新补播。停止指定旧实例不得停止其继任者。实例和对应状态在 owner 清理、停止与静音后取消；游戏适配器仍需自己的有界事件游标，恢复只依据当前活跃状态。

有限声道不循环。起始 offset 只用于当前有效实例的恢复；超出真实媒体时长时保持静音。来源失败、迟到 metadata 和播放 Promise 必须受 owner／实例／代次约束。新能力复用全局静音、原生媒体、`playback` 音频会话和 owner 生命周期，不另建游戏播放器或 `AudioContext`。

这只是媒体执行能力。一个混合 BGM WAV 不能单独静音其中某条 FM／PSG 轨道；多条隔离 WAV 叠放也未被证明等价于原驱动的双 PCM、音色抢占和混音。原版有序调度、前导延迟起点与分轨／承载方案仍是独立依赖。

## 已执行的真实浏览器验收

使用真实 `HTMLAudioElement`、生产 `GameAudio` 和 Chromium 原生按钮手势。`1f/3f/8f` 在提供给浏览器前逐一核对原 WAV 和 PCM SHA；时长分别为约 2.119／2.036／4.172 秒。媒体以原速播放，没有裁短原曲。

基线观察包含 10 项检查，两次执行均通过：有限请求之间的实际替换；有限效果与背景同时播放；`stopMusic` 不停效果；有限 `8f` 与 offset／实例刷新；暂停 250 ms 位置冻结和恢复；过期有限恢复不补播；真实 404 fallback 保留 offset；两次完整经过的后缀回跳；静音和旧 `ended`；owner 替换、真实 tab 隐藏和 dispose。两个有限请求期待两个通路而实际只有一个，因此 `--require-finite-lanes` 真正退出 1，保留能力 red，并不把既有语义修成另一种含义。

后缀观察使用明确标记的合成诊断 WAV：44,100 Hz 双声道，前缀 1 秒 882 Hz，后缀 2 秒 441 Hz，共 3 秒；不代表任何原曲。首轮完整播放后再经过完整后缀，实际两次 `ended` 后均回到 1 秒，原生 `playing` 通知比 `ended` 晚约 1.4～3.4 ms。首轮记录从开始播放至第二次回跳约 5.123 秒，期间不 seek 加速、不提高速率。这个差值是媒体事件延迟，不能换算为可听间隙或输出样本误差。

新接口另有冻结 E2E：双有限加 BGM、同实例去重／来源冲突、新实例、按实例停止、offset／过期、真实 350 ms 延迟来源替换、已结束实例、暂停／恢复、静音、owner 与宿主清理。基线实际因缺少 `effect/stopEffect` 方法退出 1。Root 实现后的当前源码通过记录为 `finite-restored-final1/result.json`，来源摘要逐项匹配当前共享代码；不能用旧 `play` 的 red 替代新接口验收。

67 项既有平台音频单元测试通过，全部采用假媒体对象；它们证明 JavaScript 生命周期分支，不是原生媒体输出证据。

## 播放前的页面可见性

一次新接口复跑在最初 BGM 请求前失败，既没有音频请求，也没有媒体事件。独立复现确认：工具已执行 open／snapshot 时，目标文档仍可能实际处于 hidden，profile 中还保留多个先前验收页。对真实背景页发送原生可信点击，可得到相同的 `independent music fixture failed`、空媒体和零音频请求；这符合平台禁止隐藏页新播声音的规则。

修复只改变浏览器 setup：按当前验收 URL 找到目标，通过原生 `Target.activateTarget/Page.bringToFront` 置前；用 agent-browser 重新绑定该 target，等待实际 `visibilityState=visible` 后再发送可信点击，并保存前后状态。没有覆盖 `document.hidden`，也没有修改媒体断言或生产代码。受控背景复现先成功记录，再激活页面的同次完整验收通过；正式新 label `finite-restored-final1` 全部 8 项通过，源码摘要匹配当前文件，后续真实 tab 隐藏清理仍通过。

辅助负向控制曾通过外部 CDP reload 导航，导致 agent-browser 的缓存执行上下文失效；该终止失败保留。现在导航统一经 agent-browser `open`，激活 helper 只做原生目标激活。全部源码 lineage 和当前结果见 [visibility-setup-validation.json](../../../games/yuyuhakusho/verification/audio-platform-media-api/visibility-setup-validation.json)。两个冻结 TypeScript harness 的 SHA 始终未变。

## 捕获边界的实际限制

原生 `captureStream + MediaRecorder` 可以录下诊断，但首次得到的 WebM 解码后只有 3 秒、144,000 个 48,000 Hz 立体声帧，而网页已通过两个回跳。后续独立观察固定使用 `aa617e23` 的原始共享源码，并记录流、轨道和录制器事件：首个 `ended` 后流变成 inactive，唯一音频轨道被移除，录制器自动停止；后续媒体在 1 秒继续播放时，原捕获流没有恢复轨道。当前记录不能验证跨尾后的 PCM。

独立观察器初版 HTTP 夹具未实现 Range，导致 seek 后回到 0；宽泛的两次 `ended` 检查曾错误通过。原始结果保留为无效后缀证据，随后先加起点和完整周期断言取得真实 red，再只补 HTTP 206／Range。最终严格观察确认两个回跳均为 1 秒，并仍复现捕获只保留首 3 秒。此修复只涉及测试夹具。

Opus 捕获还把源 44,100 Hz 转为 48,000 Hz。描述性的 RMS／零值统计不能恢复源样本时钟，也不能证明扬声器输出无间隙。报告因此保持 `sample_gapless_verified: false`、`speaker_output_verified: false` 和 `original_mix_equivalent: false`。

本次可用的 agent-browser 引擎为 Chromium 等引擎，未配置 Safari provider 插件，工作区不能解析 Playwright／WebKit 包。没有安装工具或启动工作区外 profile；WebKit、iOS 与物理设备声音输出未实测，不作兼容结论。

## 证据和复跑

机器索引为 [validation.json](../../../games/yuyuhakusho/verification/audio-platform-media-api/validation.json)。所有浏览器 profile、临时构建和原生捕获工作文件位于当前 worktree。

```sh
# 基线兼容和真实媒体事件观察；capability 字段仍报告旧有限通路只有一条。
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_audio_media_api.py --label native-review

# 新有限通路接口验收；基线应缺 method 退出 1，扩展实现后应通过。
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_audio_media_api.py \
  --proposed-finite-lanes --label finite-review

# 固定基线的原生捕获轨道／录制器边界观察。
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_audio_capture_boundary.py
```

同一 `check_audio_media_api.py` 的两个模式共享自有浏览器 profile／会话，必须串行执行。Root 的后续新实现验证使用独立 label，保留既有 red 和基线报告。
