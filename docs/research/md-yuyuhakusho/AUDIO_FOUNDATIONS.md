# 音频目录、调度阶段与平台声道

本阶段完成有限媒体的完整性验证、四个可重入调度阶段，以及平台有限声道接口。实际对局尚未播放声音；完整驱动、回调内调度、原版混音和独立音乐合成继续实现。机器索引见 [audio-foundations-session.json](../../../games/yuyuhakusho/verification/audio-foundations-session.json)，整体进度仍为 81%。

## 有限媒体目录

新增 [audio 包](../../../games/yuyuhakusho/audio/README.md)和 37,134-byte 紧凑目录，包含 121 个 PCM 命令、12 个有限 FM／PSG 序列与有限结算音乐 `8f`。原生竞态、Wasm 和独立 CLI 对全部 134 个原 WAV 的格式、样本量、完整文件／PCM SHA、首个非零采样位置和未裁切内容均通过。两轮导出逐 byte 相同，旧目录和全部 137 个 WAV 保持原样。

控制命令、静音诊断 `7f` 和限长音乐 `82/87` 不进入有限目录。11 人物的 139 个动画引用覆盖 78 个唯一声音编号，仅证明动画数据绑定，不表示全部玩法可达或完整预加载集合。目录自身必须由调用者按可信不可变 manifest SHA 验证，再调用 `ValidateWAV` 验证实际媒体；ROM 身份字段不能认证任意修改后的 metadata。

媒体保留隔离采集的起始原点及前导延迟。首个非零采样位置只是观察值，不能直接裁掉，也不能在实际分派时再叠加一次队列延迟。原始采集、时基和浏览器媒体位置是三个不同层次。

## 调度阶段

[SoundSchedulerState](../../../games/yuyuhakusho/engine/sound_scheduler.go)只持有延迟 word、延迟声音 byte、重入积压 byte 与 busy byte。现有世界队列和 `Clock.AudioPause` 继续分别拥有自己的状态，helper 只接收／返回队列临时视图。

开始与结束分开，允许驱动执行期间出现嵌套 IRQ；补执行结束时递减当前积压，不用进入前副本覆盖新积压。原生竞态与 Wasm 共 7 项检查通过，包含 1,592 个原私有阶段、16,974 条自然阶段投影及 185 个真实消费者调用。

两条自然流的 9,102 个视频步骤在完整 RAM、视频和 PCM 上一致，并有 145 个回调内消费反例。因此没有把消费者接到世界帧尾。原 PC 时间表、任意删除这些调用或引入 CPU／RAM VM 都不能解决独立调度问题。详细范围见 [AUDIO_TIMING.md](AUDIO_TIMING.md)。

## PCM 选择与混音边界

新增 `SelectPCMCommand` 在原生竞态与 Wasm 各 4 项检查中通过，覆盖 13,036 次原私有调用与 33 个真实队列分派。它只返回接收／拒绝、单 DAC／A／B、自动偏好和 duck 请求意图；不创建第二份 Z80 状态，也不宣称声音已开始、结束或混音完成。

23 组输入的原／观察内核完整 RAM、视频与 PCM 相同。成对采样同时保留两个单音及其原混音：相同输入对齐下，直接饱和叠放仍有 161,234 个样本值不同，没有发生相加饱和；固定双增益也不能重建。没有做任意移位搜索，也没有把差异只归于一个原因。平台有限 lane 的并发能力因此仍不能代替原 DSP。

控制 `02/03` 操作的是主／副 FM／PSG 行组，本批实测没有停止活动 PCM；`04` 也不能直接映射为停止所有 PCM。`F0` 的额外 descriptor 不等于 `7a` WAV。完整控制、声部恢复、duck 渐变和 Z80 确认仍待恢复，详见 [AUDIO_DISPATCH_RESEARCH.md](AUDIO_DISPATCH_RESEARCH.md)。

## 平台有限声道接口

`BoardProps.audio` 新增：

```ts
effect(lane, sources, { playbackID, startOffset? }): Promise<boolean>
stopEffect(lane, expectedPlaybackID?): void
```

`lane` 是稳定的规则通路，`playbackID` 是单次播放实例，`sources` 是同一有限媒体的最多 4 个替代地址。媒体不循环。平台只执行媒体生命周期，游戏驱动决定路由、优先级和抢占。原 `play`／`playWithFallback` 的单任务语义保留；新接口不把两个独立 WAV 的叠放称为原版混音。

每个 owner 最多保留 16 条有限 lane，平台最多 32 条。每条 lane 只保留最近实例；相同实例刷新不倒退，来源冲突拒绝，新实例即使使用相同声音也重新播放。结束后的最近实例仍阻止重复快照重播。实例 ID 不能当作 lane 名不断增长；游戏适配器仍须按有界事件游标拒绝旧事件。

迟到的 `stopEffect` 可带预期实例 ID，避免停止后继。停止、静音、离桌和 dispose 清理媒体与 token；取消静音不自动恢复历史请求。game pause 则保留当前媒体位置，恢复不重新加载。有限起始偏移超过实际时长时拒绝，不从头补播。

实现复用已有原生媒体生命周期，每个有限通路使用独立的平台子播放器；游戏没有新增 `Audio`／`AudioContext` 或专用静音开关。75 项受控 Promise／生命周期测试通过，包含原有 67 项兼容与 8 项新增竞态、实例和容量测试；这些是低层异常验证，不是扬声器采样证据。

真实 Chromium 对新接口的 8 项 E2E 通过，包括双有限声道与 BGM 同时播放、实例身份、定向停止、fallback／offset／过期恢复、迟到来源、自然 ended、暂停恢复、静音及真实标签页隐藏。当前实现的旧接口另有 10 项真实媒体兼容检查。原生后缀回跳的事件观测与无损 PCM 接缝分别验收；早期 captureStream 在首次 ended 后移除轨道，只录得 3 秒，不能据此证明跨循环 PCM 无间隙。

一次重复验收在首次媒体请求前失败。受控复现证明页面实际起初为 hidden，真实背景点击同样产生零媒体请求；修复仅激活精确测试页并等待实际可见，冻结的 TS 媒体断言保持不变。没有覆盖 `document.hidden` 或放宽生产的后台静音门槛。初始失败、恢复后的正式结果及脚本谱系见 [visibility-setup-validation.json](../../../games/yuyuhakusho/verification/audio-platform-media-api/visibility-setup-validation.json)。

## 连续音乐的可行路径

新增八首音乐共 130,480 个原／观察步骤逐步一致，72 条轨道、21,147 条原序列命令与研究模型匹配。`83/85/8d/95` 已确认完整 driver 共同周期，PCM 仍不重复；其余四首的长共同周期保留为结构候选，未导出循环成品。

Chrome 152 的实际原生播放已验证：原 `82` 前 12 秒 PCM 通过无损 FLAC/fMP4 编码与重组后逐 byte 相等；单 HTMLAudio／MSE 在播放中追加 12 个分片，跨 11 个边界没有播放后 waiting、stalled 或换源。没有创建 AudioContext。它证明有限原 PCM 的原生无损承载可行，不证明独立合成、无限 BGM、扬声器逐样本一致或 Safari／手机支持。

后续路径是完整 typed 音符／音色、持续推进的独立 DSP、Go／Wasm 无损编码与平台流接口，再验证低延迟动态混音及其他浏览器。不能以旧 WAV 首尾相接、裁切或交叉淡化代替原相位。详见 [AUDIO_MUSIC_RESULTS.md](AUDIO_MUSIC_RESULTS.md)。

## 复跑

在 worktree 根目录，所有工具目录均使用工作区内路径。已完成的 Go 全局遥测误改经用户明确授权恢复为原先的 `local`；正常 Go 工具使用工作区的 `TEST_TELEMETRY_DIR`，不再用全局配置命令限定本地验收。

```sh
node --experimental-strip-types --test web/tests/platform-audio*.test.mjs
node web/node_modules/typescript/bin/tsc -p web/tsconfig.json --noEmit --incremental false
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_audio_media_api.py \
  --proposed-finite-lanes --label finite-review
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_audio_media_api.py \
  --label legacy-review
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/summarize_audio_foundations.py
```

两个浏览器命令串行运行，避免争用同一自有 profile。目录、调度与音乐各自的精确命令见对应验证说明。没有上传媒体、推送、发布或部署。
