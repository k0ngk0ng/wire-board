# 受击阶段与伤害取整

原版目标为 PokéPath TD v1.7.3。独立引擎保留不同阶段的取整与伤害统计，不将所有增伤提前合并为一个倍率。

## 已核对的行为

| 场景 | 原版行为 | 修正前 | 修正后 |
| --- | --- | --- | --- |
| 怪力钳，威力 7，必定暴击，目标不足半血 | 先将暴击伤害取整为 11，再增加向上取整的 50% | 16 | 17 |
| 超级大嘴娃普通攻击，威力 7，必定暴击，目标不足半血 | 先将暴击伤害取整为 11，再增加 11 | 21 | 22 |
| 超级大嘴娃额外攻击，威力 6，必定暴击 | 暴击后为 9，半威力取整为 5，受击时半血增伤为 10 | 9 | 10 |
| 超级大嘴娃额外攻击，威力 7，无暴击 | 半威力取整为 4，受击时半血增伤为 8 | 7 | 8 |
| 技术高手，威力 5，处于自然之怒领域 | 领域先将伤害提升至 6，再由技术高手提升至 9 | 10 | 9 |
| 技术高手，威力 834，处于自然之怒领域 | 领域伤害为 1001，超过技术高手门槛 | 未列入首轮基线 | 1001 |

半血条件严格使用当前生命小于最大生命的 50%。正好半血时，首次普通攻击不获得半血增伤；若命中后跌破半血，后续塔更新按原版触发额外攻击。

额外攻击的末阶段倍率显式保存在弹道状态中，常规伤害缩放仍保留原有含义。保存时弹道仍在飞行、恢复后命中、不同毫秒分段以及本机与 Wasm 执行均比较完整状态，不只比较最后扣血。

原版来源为研究目录中的 `Tower.js`、`Projectile.js`、`Enemy.js` 和 `Game.js`。原版程序仅在本地对照浏览器中执行，产品仍由独立 Go 规则和 Go/Wasm 客户端预测执行。

## 验收范围

测试先于实现修改。原版对照共 71 组，其中 47 组新增场景覆盖威力 6／7／15、暴击率 0／100、半血上下边界、普通与额外攻击、领域和技术高手门槛。另有 731 项本机与 Wasm 完整状态和瞬时投影比较，以及真实弹道快照倍率的合法与非法输入检查。

首轮基线的 69 组中有 18 组新增伤害场景不一致。另有一次既有连接光束场景的原版普通弹丸命中时刻不同，后续两轮均一致；首轮未保留发射几何，原因仍未确认。这项观察不计为伤害阶段修复，也不代表冷资源与连续画面已完全一致。夹具现保留每次发射的尺寸、位置、速度和目标几何，便于复查。

本次规则版本为 `d6a01ea5b411fb7c3c2f897823ce338fe74f4850745dcce577aa92a446ba83ac`。当前结果、源码摘要、截图与其他回归结果统一登记在 `.local/pokepath/verification/impact-stages-verification.json`；总体进度仍为工程估算，未宣布完整复刻或正式上线。

## 复现

在仓库根目录执行。先生成并核对规则版本，再产生本机轨迹，最后消费轨迹做 Wasm 比较。

```sh
. scripts/env.sh
node games/pokepath/build/rules-revision.mjs --check
node --test games/pokepath/tests/tower-order-reference.test.mjs
node --test games/pokepath/tests/tower-order-wasm-e2e.test.mjs
go test -race -json ./games/pokepath/engine ./games/pokepath/tests -skip '^TestAICompleteFirstHundredWavesFromActualInitialStateE2E$' -count=1
node --test --test-concurrency=1 games/pokepath/tests/native-architecture.test.mjs games/pokepath/tests/native-runtime-e2e.test.mjs games/pokepath/tests/native-attack-modes-wasm.test.mjs
node games/pokepath/build/lint.mjs
npm --prefix web run build
node games/pokepath/tests/native-platform-browser.mjs --standard-web
node games/pokepath/tests/native-platform-browser.mjs --standard-web --challenge
go test -json ./games/pokepath/engine -run '^TestAICompleteFirstHundredWavesFromActualInitialStateE2E$' -count=1 -timeout=15m
```
