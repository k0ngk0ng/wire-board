import { useEffect, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import history from "./history.json";
import "./style.css";

const pageSize = 20;

export default function ReleaseHistory() {
  const [query, setQuery] = useState("");
  const [limit, setLimit] = useState(pageSize);
  const path = window.location.pathname.replace(/\/$/, "");
  const version = path === "/releases" ? "" : path.slice("/releases/".length);
  const index = history.findIndex((entry) => entry.version === version);
  const entry = history[index];
  const search = query.trim().toLocaleLowerCase();
  const matches = history.filter((item) =>
    `${item.version}\n${item.title}\n${item.body}`
      .toLocaleLowerCase()
      .includes(search),
  );
  useEffect(() => {
    document.title = `${version || "更新日志"} · 围桌`;
  }, [version]);
  return (
    <main className="release-page">
      <header className="release-header">
        <a className="release-home" href="/">
          <img src="/favicon.svg" width="32" height="32" alt="" />
          围桌
        </a>
        {version && <a href="/releases">全部版本</a>}
      </header>
      {version ? (
        entry ? (
          <>
            <article
              className="release-detail"
              data-release-detail={entry.version}
            >
              <div className="release-detail-heading">
                <h1>{entry.version}</h1>
                <time dateTime={entry.date}>{entry.date}</time>
              </div>
              <h2 className="release-title">{entry.title}</h2>
              <div className="release-body">
                <Markdown
                  remarkPlugins={[remarkGfm]}
                  skipHtml
                  components={{
                    a: ({ children, href }) => (
                      <a href={href} target="_blank" rel="noopener noreferrer">
                        {children}
                      </a>
                    ),
                    h1: ({ children }) => <h3>{children}</h3>,
                    h2: ({ children }) => <h3>{children}</h3>,
                    table: ({ children }) => (
                      <div className="release-table">
                        <table>{children}</table>
                      </div>
                    ),
                  }}
                >
                  {entry.body}
                </Markdown>
              </div>
            </article>
            <nav className="release-neighbors" aria-label="相邻版本">
              {history[index - 1] && (
                <a href={`/releases/${history[index - 1].version}`}>
                  ← {history[index - 1].version}
                </a>
              )}
              {history[index + 1] && (
                <a href={`/releases/${history[index + 1].version}`}>
                  {history[index + 1].version} →
                </a>
              )}
            </nav>
          </>
        ) : (
          <section className="release-detail" data-release-missing>
            <h1>未找到版本</h1>
            <p>暂无 {version} 的更新日志。</p>
          </section>
        )
      ) : (
        <section>
          <h1>更新日志</h1>
          <div className="release-search">
            <input
              type="search"
              aria-label="搜索版本或更新内容"
              placeholder="搜索版本或更新内容"
              value={query}
              onChange={(event) => {
                setQuery(event.target.value);
                setLimit(pageSize);
              }}
            />
            {query && (
              <button
                type="button"
                className="outline"
                data-release-clear
                onClick={() => {
                  setQuery("");
                  setLimit(pageSize);
                }}
              >
                清除
              </button>
            )}
          </div>
          <ul className="release-list" data-release-list>
            {matches.slice(0, limit).map((item) => (
              <li key={item.version}>
                <a href={`/releases/${item.version}`}>
                  <span className="release-number">{item.version}</span>
                  <strong>{item.title}</strong>
                  <time dateTime={item.date}>{item.date}</time>
                </a>
              </li>
            ))}
          </ul>
          {!matches.length && (
            <p className="release-empty" data-release-empty role="status">
              没有匹配的版本
            </p>
          )}
          {matches.length > limit && (
            <button
              type="button"
              className="outline release-more"
              data-release-more
              onClick={() => setLimit((value) => value + pageSize)}
            >
              加载更多
            </button>
          )}
        </section>
      )}
    </main>
  );
}
