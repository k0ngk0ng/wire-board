# 魂斗罗分关资源清单

[runtime-stages.json](../../../games/contra/assets/runtime-stages.json) 按零基 level 0..7 声明 Intro、battle、baseBoss、Interlude、continue 和 final 的 CHR 页及音频 ID。清单约 24 KB，只含资源身份与集合，不含动画规则、媒体内容或 ROM。

battle 页数分别为 14、16、15、16、13、11、9、21，包含公共玩家／弹丸页、本关 alternate 与本关基地 Boss。关内进入 Boss 不依赖下一关资源。Intro／continue 使用 preset 10 的 6 页；Interlude 使用 preset 18／19，后两段加入动态页 47／48。第 8 关不播放 Interlude。

全战役实际可达并集为 100 页。浏览器只取当前关的场景集合，不遍历并集或预载全部 128 页。服务端纯视觉 Bot 可以在回调外校验缓存静态并集，回调只访问内存。

恢复或切关还须保证快照当前 Applied Patterns 的 8 槽可用。例如第 5→6 关刚推进时，可能仍显示前一段 preset 18，而下一关 Interlude 使用 preset 19。不能仅按新 level 假设显示已经换页，也不必为此加载所有历史关卡。

音频数组是按事件加载的描述。music 只列场景 BGM；sfx 包含即时音效、打击乐及源驱动回调，例如 119 完成后可能触发 98。不据列表预下载全部音轨。continue 使用 110；final 保留当前 Session 真正输出的终局资源，不添加 credits 音乐或下一周目。

生成器只读取现有规范化数据与恢复后的源码，不打开或运行 ROM。来源路径及 SHA-256 保存在清单中；来源修改后须重新生成。

    python3 -B scripts/contra/runtime_stages.py
    node --test games/contra/tests/runtime-stages.test.mjs

检查覆盖 preset、有效 CHR 动画、lair 第 12 屏专用行、Interlude 动态页、音频依赖闭包、来源摘要和场景上限；另验证 2,048 条原版 CHR 输出及 372 条原版敌人阶段声音记录。禁用的动画行没有被误当成需求。逐关计数见[资源来源记录](evidence/runtime-stages-sources.json)。
