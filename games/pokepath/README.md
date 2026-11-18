# 宝可梦：巡径守卫

参照 PokéPath TD v1.7.3 独立重写的合作塔防，支持 1～4 人守同一张地图，共享队伍、金币与波次。规则由本项目 Go 实现，界面使用原生 React／Canvas；不嵌入原网页，也不运行上游游戏程序。

## 目录

| 目录 | 职责 |
| --- | --- |
| `engine/` | 自行实现的规则、战斗、成长、能力与电脑策略 |
| `data/` | 归一的地图、波次、属性和名称等规则事实 |
| `server/` | 平台鉴权、多人共享状态、同步与保存恢复 |
| `web/` | 自行实现的原生界面、Canvas、地图操作与预测 |
| `cmd/browser/` | 同一份自写 Go 规则的浏览器 Wasm 接口 |
| `assets/` | 参考美术、音频的来源、摘要与尺寸 |
| `tests/` | 测试先行的独立实现验收 |
| `build/` | 数据整理、运行时构建与 lint |

原版对照范围为 47 个地图、4007 个波次、517 个形态、257 个收集编号、347 个敌人，以及能力、物品、挑战和长期成长。内容完整性与行为验收分别记录，不以资料导入代替机制实现。

## 验证

从仓库根目录执行。所有缓存与验收产物位于仓库内。

```sh
. scripts/env.sh
node --test games/pokepath/tests/native-architecture.test.mjs games/pokepath/tests/lint.test.mjs
node games/pokepath/build/lint.mjs
node --experimental-strip-types --test --test-concurrency=1 games/pokepath/tests/*.test.mjs
go test ./games/pokepath/engine ./games/pokepath/tests -count=1
python3 -B scripts/check_game_boundaries.py
```

独立实现的测试与截图使用 `native` 命名，旧原版运行器方案已移出产品目录，旧测试结果不能作为本实现通过的证据。实际浏览器、电脑完整路线、多人保存与冷加载的复现命令随对应验收文件维护。

包含 Wasm 成本预算的前端套件按文件串行执行，避免多个独立测试同时编译和测量造成资源争用；保持原性能阈值不变。

训练家面板的固定状态、完全加载资源对照使用 `node games/pokepath/tests/profile-render-oracle.mjs`，保留 26 组文字、几何、源码摘要和逐像素截图结果。范围与显示约定见[训练家面板显示对照](../../docs/research/pokepath/profile-render-contract.md)。

背包与路线页使用[固定状态显示对照](../../docs/research/pokepath/collection-render-contract.md)，同时记录排序、筛选、拖动、装备和路线选择的真实输入收据。

[图鉴对照](../../docs/research/pokepath/dex-render-contract.md)记录塔和敌人排序、教程与秘密入口、多人终身统计、续玩及 320 px 触控验收。

## 来源

规则事实、图形与音频参考来源见 [原版研究](../../docs/research/pokepath/README.md)和[资源清单](assets/manifest.json)。上游程序只作研究对照，产品不包含其运行代码。角色、名称与素材权利归各自权利人。
