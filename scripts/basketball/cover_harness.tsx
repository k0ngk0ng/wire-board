import { createRoot } from "react-dom/client";
import { AssetsContext } from "../../web/src/ui/board-host";
import { Cover } from "../../games/basketball/web/Cover";
const cases = [["local", ""], ["fallback", "/missing-cover"], ["cdn", "https://cdn.ichenj.com/bga/v20261002-1"]];
createRoot(document.getElementById("root")!).render(<main>{cases.map(([name, base]) => <figure key={name}><AssetsContext.Provider value={base}><div data-cover-case={name}><Cover /></div></AssetsContext.Provider><figcaption>热血篮球 · {name}</figcaption></figure>)}</main>);
