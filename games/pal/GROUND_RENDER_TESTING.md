# 地面图块渲染核验

用户指出第 20 段截图的地面贴图可能异常。先核对同次剧情中的水下、余杭入口和王宫场景，再定位绘图差异。参考图必须由已登记的原版 C 地图绘图函数和原始 `map.mkf`／`gop.mkf` 生成，不能由当前浏览器地图实现生成后再自证一致。

## 实现前失败场景

1. 地图高位图块编号、上下层、半格索引或行列地址错误，导致取到其他地砖。
2. RLE 透明段、黑色像素、图块大小或图集边界解码错误，产生空洞、方块或接缝。
3. 镜头偏移、负坐标截断、半格位移或绘图范围错误；某张开场地图通过，却在后续地图错位。
4. 前景瓦片被画到地面或错误覆盖人物；波动误作用于人物、对白或前景遮挡。
5. 调色板、夜色或淡入参数错误，使正确图块呈现异常颜色。
6. 手机／PC 的缩放或预测画面不同于当前权威局面；只比较 DOM 或导出的图片，不比较实际 Canvas。
7. 为修复贴图修改原版地图、资源、碰撞、存档身份或剧情状态；原版 C 参考与游戏共用同一份有问题的浏览器实现。
8. 将只含地面的独立像素比对当成完整剧情画面验收；地面、波动和遮挡须分别注明验证范围。

补充检查使用同三张地图的 9 组镜头偏移和 4 组地图边缘坐标，对照原版 C 的整数截断、半格拼接和边界回退。此项为绘制夹具，不推进剧情，也不代表人物能走到所有边界坐标。另复核已有原版 C 波动参考中的全部相位、预测与画面备份。补充产物写入 `ground-pan/`，不覆盖此前截图和校验清单。

参考生成使用独立的验证核心，只读取实际剧情的公开镜头与调色板参数，调用原版地图绘图函数；不恢复或改写游戏存档。像素、参数、原始资源摘要和浏览器结果保留在 `.local/pal/verification/ground-tiles/`。

当前核对第 20 段同次剧情的水下地图 188、余杭入口地图 5 和王宫地图 184。原版 C 独立绘制两层地图，并按公开绘图指令调用原版 RLE 函数叠加人物和前景；余杭另核对同帧运动预测。PC 与 320px 画布共 14 次逐像素比较，差异均为 0。对照图为 `original-vs-current.png`。

这三张截图对应的绘图指令没有场景波动；本结果不能代替其他波动相位、完整绘图指令投影或未定位场景的核验。用户所指具体位置仍待确认，不能据此断言所有地面渲染均无问题。目前没有改动游戏绘制、原始贴图或碰撞。

```sh
. scripts/env.sh
PAL_NATIVE_WASM=.local/pal/runtime-candidates/scene-wave/reference/rules.wasm PAL_ORIGINAL_PIXELS=1 node games/pal/tests/ground-oracle.mjs
python3 -B games/pal/tests/ground-tiles-browser.py
```

补充检查共 138 帧：三张剧情地图的 39 个镜头／边界夹具，以及原版客栈的 99 个波动、预测、文字和画面备份夹具。波动参考覆盖全部 32 个相位。PC 和 320px 画布共 276 次逐像素比较，差异均为 0；截图已人工检查。结果为 `.local/pal/verification/ground-pan/result.json`，源文件副本和摘要保留在同目录。本次没有修改游戏实现、原始素材或存档；用户所指的具体异常仍未定位。

用户已同意先按现有核对结果继续推进，地面观感留到最后复盘。此决定不扩大上述像素对照的验证范围，也不将未知位置标为已修复。

```sh
. scripts/env.sh
PAL_NATIVE_WASM=.local/pal/runtime-candidates/scene-wave/reference/rules.wasm PAL_ORIGINAL_PIXELS=1 PAL_GROUND_PAN=1 node games/pal/tests/ground-oracle.mjs
PAL_GROUND_PAN=1 python3 -B games/pal/tests/ground-tiles-browser.py
node games/pal/build/lint.mjs games/pal/tests/ground-oracle.mjs games/pal/tests/ground-tiles-browser.py
```
