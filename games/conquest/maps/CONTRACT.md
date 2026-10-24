# 征服时代地图契约

`world.json`、`europe.json`、`usa.json` 与 `china.json` 是后端规则与前端绘制共用的单一源。地图只定义几何、地区连接与开局位置，不决定收入、战斗或胜负。

```ts
type Map = {
  id: "world" | "europe" | "usa" | "china";
  version: number;
  sha256: string;
  name: string;
  description: string;
  width: number;
  height: number;
  provinces: {
    id: string;
    name: string;
    path: string;
    label: {x: number; y: number};
    neighbors: string[];
    seaNeighbors: string[];
  }[];
  starts: Record<"2"|"3"|"4"|"5"|"6"|"7"|"8", string[]>;
  seaRoutes: {from: string; to: string; path: string}[];
  insets?: {title: string; x: number; y: number; width: number; height: number}[];
};
```

- 地区 ID 稳定且不用于人物身份标注。前端显示中文 `name` 和服务端兵力。
- `path` 是同一地图逻辑坐标中的 SVG `M/L/Z` 路径，多个闭环表示岛屿或分离陆块。标签点位于主要陆块内。地理细节经低分辨率地图简化，不代表精确行政边界。
- `neighbors` 仅包含具有共享边界的陆上邻区；`seaNeighbors` 对应显式的双向海路。两者各自对称，互不重复。服务端只能通过这两类连接判断可达，不能通过像素距离推断。
- `seaRoutes` 的路径用于显示海路；攻击仍按区域 ID 和连接关系校验。
- `starts[n]` 保留旧版预置配置及制图校验用途，不再决定新局出生点。规则引擎从全部地区按私有种子随机选择互不陆地／海路相邻的位置；地图 JSON、版本与摘要保持不变，旧存档按已保存位置恢复。
- 摘要算法：删除顶层 `sha256` 字段，对剩余对象递归按键排序，以不转义 Unicode、无多余空白的 JSON 序列化为 UTF-8，再取 SHA-256。Python 表达式为 `json.dumps(unsigned, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()`。
- 地图修改必须增加 `version`，并更新摘要。已经开始的对局保存地图 ID、版本和摘要，不能在旧局中静默替换连接或地区。

Go 包 `games/conquest/maps` 提供 `Load(id string) (Map, error)` 与 `Catalog() []Summary`。每次 `Load` 返回独立数据，调用者不得依靠共享可变状态。前端通过动态导入当前地图 JSON，目录页不加载完整几何。

地图来源、许可与生成方式见 `README.md` 和 `sources.json`；验证命令为 `python3 -B games/conquest/tests/map_contract_test.py`。
