# PC 初代场景音乐恢复

从已认证 `main.pak` 中的 `mainmusic.mo3` 和 `mainmusic_hihats.mo3` 恢复经典主题、分轨、前奏和循环区间。仅离线解码模块数据，未执行原版 Windows 程序。发布资产为 FLAC，原始 MO3 和解码库只保留在研究缓存。

## 原始程序依据

| 依据 | 地址 | Evidence ID |
| --- | --- | --- |
| 场景通道静音配置 | `0x45A6C0` | `ev_8344638e4d53ac63b893680a4fc1e0c1c22fcfc46f258e359d3c13daaf293c8d` |
| 原始起始 order 和角色音量 | `0x45ADB0` | `ev_6e6d9ef1435e122ae803c42485febc4dce09ca0747b9ea1f5a8ac3f0f6c8199a` |
| 定位、循环和重置标记 | `0x45AC70` | `ev_bc1ca0f7f02d585378d2eeb7cfaff6cd1bc28ba03fe4f53da7805e2107dcbba2` |
| 动态音乐状态与夜晚 76／77 入口 | `0x45B260` | `ev_4e30c38ed63d7607a9387bfe8f281ccb30053fac985b78ae8d528e35cc6aef86` |

原始程序实际遍历通道 `0..29`，与历史参考源码的 `<29` 不同。泳池打击层实际启用 `18..28`，不是参考源码中的 `25..28`。草地启用主轨 `0..23`、打击 `24..26`、hihat `27`；浓雾为 `0..15`、`16..22`、`23`；屋顶为 `0..17`、`18..20`、`21`。夜晚和菜单／Boss 主轨使用全部 30 通道。

## 循环边界

在 44,100 Hz、立体声 PCM16LE 下，以 BASS `MUSICPOS | MIXTIME` 回调内的 `BASS_POS_BYTE | BASS_POS_DECODE` 记录边界。普通 BYTE 查询会受 GetData 块起点影响，不能用于本验收。65,536 与 1,024 帧两种块大小均得到相同边界；另外用单采样帧拉取验证了位置方法。完整时间线见 [theme-loops-v2.json](evidence/music/theme-loops-v2.json)。

| 主题 | 起始 order | 循环 order | 前奏帧数 | 首轮结束帧数 |
| --- | --- | --- | --- | --- |
| 草地 | 0 | 12 | 2,207,232 | 8,369,088 |
| 夜晚 | 48 | 48 | 0 | 4,935,168 |
| 泳池 | 94 | 94 | 0 | 4,469,760 |
| 浓雾 | 125 | 126 | 96,320 | 4,912,320 |
| 屋顶 | 184 | 185 | 176,384 | 4,762,368 |
| 菜单 | 152 | 152 | 0 | 3,852,800 |
| Boss／夜屋顶 | 158 | 158 | 0 | 4,409,600 |
| 选卡 | 122 | 123 | 158,688 | 952,128 |

夜晚原始打击句柄先定位到静音 order 92，循环为 176,256 帧。它不是丢失音频或解码失败：两种块大小和多种寻址设置均得到相同的全零 PCM。原控制器在高潮将其跳至同一模块的 76 或 77；这两个有声变奏循环到 78，稳定循环均为 2,467,584 帧。详见 [night-active-loops.json](evidence/music/night-active-loops.json) 和 [night-seek-state.json](evidence/music/night-seek-state.json)。

## 解码与无损验收

工具来自 [Un4seen 官方 macOS BASS 包](https://www.un4seen.com/files/bass24-osx.zip)，ZIP SHA-256 为 `dfadd6238896b02b144b2870655fb9ee2445fc84d5c00f7e1c56faf9343ce59c`。实际库为含 arm64 的 BASS 2.4.18.3，SHA-256 为 `e81fb7b4d0009ba6343fbfcd840620704dcb840686d405b9734cd37150d31974`。库、头文件、许可和下载记录均位于 `.local/pvz-music/tools/`；不安装到外部目录，不进入发布包。

解码使用无输出设备 `BASS_Init(0, 44100, ...)`，flags 为 `0x228204`（DECODE、PRESCAN，以及原版 POSRESET、RAMP、LOOP 低位标记）。每个正式轨道分别用两种块大小完整渲染，PCM 必须逐字节相同。FFmpeg 编码 FLAC 后再次解码为 PCM，核对采样率、声道、位深、帧数和 SHA-256。验收记录见 [music-render.json](evidence/music-render.json) 和 [music-validation.json](evidence/music-validation.json)。

原程序使用旧版 BASS 接口，包含旧 API 的高位定位标记；当前解码器按现代 ORDER 模式恢复相同模块起点。这里确认主题、源段、分轨和循环位置，不声称与旧 Windows 解码器或随敌人数变化的原动态混音逐采样等价。

## 重复执行

先在工作区内准备上述固定摘要的官方库及认证 MO3。已验证的库位置为 `.local/pvz-music/tools/bass24-osx/libbass.dylib`。下面的脚本均从仓库根目录执行，不创建外部配置或音频设备：

```sh
# 可选：重新测量边界并与保留数据比较。
.local/pvz-research/venv/bin/python -B games/pvz/build/music_tools/scan_themes.py
.local/pvz-research/venv/bin/python -B games/pvz/build/music_tools/scan_layers.py
.local/pvz-research/venv/bin/python -B games/pvz/build/music_tools/scan_night_active.py

# 渲染及无损回解；默认使用仓库内保留的边界记录。
.local/pvz-research/venv/bin/python -B games/pvz/build/music_classic.py
.local/pvz-research/venv/bin/python -B games/pvz/build/verify_music.py \
  --output docs/research/pvz/evidence/music-validation.json
```

平台通过 `audio.track` 播放，主轨按场景懒加载，原本静音的附加层标为可选。后台、静音和恢复继续使用平台音频生命周期；音乐恢复从有效对局时间定位，不补播旧请求。FLAC 使用 `audio/flac`、identity 编码和 Range 回读校验。
