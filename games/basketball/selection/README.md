# 准备阶段选择资源

场地与人物选择在进入房间后的准备阶段进行；点击开始后直接进入比赛，不重复弹出选场或选人菜单。此目录只维护资源契约与来源，界面和玩法支持门禁由平台接入方维护。

## 实现前失败场景

- 只输出默认纽约场地或默认两个人物，却把清单标记为八场地／32 人物完整。
- 图片使用队伍颜色但错配人物头部槽位，混淆原 team／character 与平台席位。
- 将原头像映射额外表项 4 当作可选人物；将人物槽编号当作原中文名字。
- 默认帧／朝向／绘制顺序错误，人物被裁掉头或脚，透明背景变黑，缩放模糊。
- 场地采用同一张图换标题，或带入 HUD、研究截图／调试字样；裁切范围不明确。
- 导出图片与来源摘要不一致，依赖顺序不稳定导致重复运行改变结果。
- 缺少源图、源摘要错误或相同键重复时仍导出半份“成功”清单。
- 因图片存在就自动开放未通过的玩法配置，或把开发图片标记为可发布／自动上传 OSS。

先编写导出端到端测试，再实现生成器。验证使用已固定原 ROM 的独立静态 sprite／court 解码器重新构造预期像素；运行程序两次比较全部产物摘要，保留可复现日志。测试输入和输出只写入工作区。

## 已生成资源与原来源

- [catalog.json](catalog.json)：8 个场地、8 队 × 4 个人物。实际图片目录是 `.local/basketball/selection-assets/`，其 `manifest.json` 与此文件字节相同。
- [support-gates.json](support-gates.json)：仅记录内容审计时已完成比赛的历史基线，明确 `authority=evidence-only-not-an-allowlist`。它不是选择允许列表；服务端须按当前验收结果验证整组场地／队伍／人物／模式／装备配置。生成器不读取引擎开关，不自动放开玩法。
- 场地 PNG：`courts/court-{id}.png`，512 × 288。来源为 `.local/basketball/courts/{id}/court-{id+1}.png` 的 `[0,144,512,432]` 裁切，不缩放、不加 HUD／人物／文字。预览保留用于识别场地的下方建筑和场上区域；完整纵向场景仍由原球场资源提供。
- 人物头像：`players/team-{team}-character-{character}-portrait.png`，96 × 96。
- 人物全身：`players/team-{team}-character-{character}-body.png`，128 × 160。
- 人物来自 `.local/basketball/static-atlas/atlas.json` 的 `pose=0`、`facing=0`、`priority=forward`。原 128 × 128 帧中，全身裁切 `[48,61,80,101]`、头像裁切 `[52,61,76,85]`，统一最近邻放大 4 倍。全身没有剪掉任何非透明原像素；头像有意保留头部和上身。图片透明背景，不烘焙卡片底色。

使用原比赛中的站立人物图，不冒充已恢复的原选人菜单插图。原 team／character 均为 0 基编号；人物唯一键为 `team-{team}-character-{character}`，不能用平台座位编号替代。`displayName` 的「球队 N／球员 N／球场 N」是明确编号标签，不是原名字；`nameVerified=false`。只有已核定的纽约场地保留中文名称。没有将头部表的额外索引 4 算作第 5 名可选人物。

源 ROM SHA-256：`0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`。源图与源 manifest 的 SHA-256、sprite 参数、帧矩形、裁切框、缩放和原调色板身份全部写入清单。生成器先验证所有来源和边界，再写输出；无效来源不会生成半份新清单。

人物源表与现有解码器来自 bank2 `$8000` metasprite 表、fixed `$D079/$D0C6` 解码、`$CF30/$D12D` 头部映射、bank3 `$82E5..$8303` CHR 选择和 bank4 `$9526/$97AB` 调色板选择；本任务复用已验证的导出，没有改动这些解码器。[场地源记录](../web/COURTS_GRAPHICS_VERIFICATION.md) 绑定 bank1 `$800C/$801C/$BEC5` 及 `court+1` 调色板，REA 证据 `ev_d65d9412a7dbb3fdcf8f43042defe8e327d525d1e7283954897456d63f8bbf12`。本次裁切不声称解决原运行中的 IRQ 像素残差。

## 准备界面消费契约

```ts
interface SelectionImage {
  file: string; // 相对统一资源根目录，不是仓库绝对路径。
  width: number;
  height: number;
  sha256: string;
}
interface CourtSelectionArt {
  id: number; // 原 Court ID，0..7。
  displayName: string;
  nameVerified: boolean;
  preview: SelectionImage;
}
interface PlayerSelectionArt {
  id: string; // team-N-character-M。
  team: number; // 原 Team ID，0..7。
  character: number; // 原人物槽，0..3。
  displayName: string;
  nameVerified: boolean;
  portrait: SelectionImage;
  fullBody: SelectionImage;
}
// catalog.courts: CourtSelectionArt[]
// catalog.teams: { id, displayName, nameVerified, characters: PlayerSelectionArt[] }[]
```

拼接方式是 `assetBase + image.file`。界面应按 `id`／`team`／`character` 绑定，不用数组下标作为权威身份。PNG 使用 `image-rendering: pixelated`，按宽高比展示，不拉伸。可在准备阶段选择卡显示头像，已选席位展示全身；场地预览保持 16:9。选择图片不替代服务端选项校验、房间准备状态和开始前的完整配置确认。

## 验证与复现

在导出器存在前先执行测试，保留红测 `tests-before.log`。实现后 7 项导出 E2E 通过：完整库存与身份、32 人物独立原解码像素、8 场独立原解码像素、两次全部产物字节一致、错误 ROM 来源、重复人物键、缺失源图。共核对 2,129,920 个 RGBA 像素。该测试只验证准备素材，不计为比赛、网络或玩法验收。

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/selection_assets/test_export.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/selection_assets/export.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/selection_assets/contact_sheet.py
```

测试的临时目录固定在工作区 `.local/basketball/selection-assets-evidence/` 下。导出器仅接受工作区内的输出路径。重新生成并核验后，将 `manifest.json`／`support-gates.json` 同步到本目录同名契约文件（`manifest.json` 在此保存为 `catalog.json`）。本任务没有修改现有 renderer、球场导出器、引擎、服务端或前端。

可复查产物位于 `.local/basketball/selection-assets-evidence/`：`tests-before.log`、`tests-after.log`、`export.log`，以及 `players-contact-sheet.png`、`portraits-contact-sheet.png`、`courts-contact-sheet.png`。已查看总览，人物头脚完整、头像槽位有区别、场地独立；总览中的编号仅供开发核对，正式 PNG 不含文字。

72 张图片均为开发资源，`accepted_for_release=false`。未上传 OSS，也未据此注册或上架游戏；发布许可、正式素材分发及玩法支持仍按整体任务验收。
