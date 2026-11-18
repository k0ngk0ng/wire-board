# 设置、音频与鸣谢对照

设置菜单按原版顺序展示官方链接、设置、音频、快捷键、数据和鸣谢。设置卡包含三个循环选项、十二项图形设置和七条效果透明度滑块。透明度范围为 0～100，以 5 为步长；四组音量范围为 0～20，以 1 为步长，显示整数。五项分类静音按投射物、范围、光线、瞬移、异色排列，对应规则索引 `0、3、4、1、2`。

设置动作通过平台席位和 Go 规则校验。房间成员共享实际配置，观众只能查看。平台静音是本地偏好，覆盖路线音乐与音效。调整房间音量不能取消平台静音；同一路线调整音量不会重启音乐。快捷键提示沿用原版文案，并显示平台当前键位。

官方链接及鸣谢来源于 PokéPath TD v1.7.3 的 `src/js/game/scenes/MenuScene.js`，SHA-256 为 `d6c5531032369c03cf0f667c1fb6fe95f1ec4416eff43ed9ae03bfd72ad9b805`。产品仅保存贡献者名称、语言文案、链接和来源声明，由 React 生成链接与文字节点。研究界面只进入隔离对照夹具。

数据卡目前显示原版版本号。保存、续玩和账号切换仍走平台统一入口；原版浏览器本机的导入、导出、清除数据和兑换流程未接入平台。显示对照保留这一区域的差异。

```sh
. scripts/env.sh
npm --prefix web run build
node games/pokepath/tests/settings-render-oracle.mjs
node games/pokepath/tests/native-platform-browser.mjs --standard-web
node --test games/pokepath/tests/frontend-classic.test.mjs games/pokepath/tests/frontend-language.test.mjs web/tests/platform-audio-call-volume.test.mjs
```

设置报告位于 `.local/pokepath/verification/settings-render/result.json`。简体中文和法语的置顶、音频与鸣谢首屏共六组画面达到零像素差异。两个快捷键截图下方包含数据卡，仍有差异，整组严格画面对照未通过。内容、操作回执、只读和 320 px 触控验证通过。平台浏览器报告位于 `.local/pokepath/native-browser/run-*/result.json`，记录合作房间十种语言同步、音乐增益和观战权限。
