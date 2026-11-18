import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
const root = fileURLToPath(new URL("../../../", import.meta.url));
const url = process.argv[2] ?? "http://127.0.0.1:8792/";
if (!["localhost", "127.0.0.1"].includes(new URL(url).hostname)) throw Error("A local native fixture is required");
const output = join(root, ".local/pokepath/reference-visual");
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: join(root, ".local/pokepath/classic-fixture/profile"), AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", AGENT_BROWSER_SCREENSHOT_DIR: output, AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const session = execFileSync("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "ppvis"], { cwd: root, env: environment, encoding: "utf8" }).trim();
const browser = (...args) => { const result = JSON.parse(execFileSync("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, encoding: "utf8", timeout: 45000 })); if (!result.success) throw Error(JSON.stringify(result)); return result.data; };
const evaluate = value => browser("eval", "-b", Buffer.from(value).toString("base64")).result;
const catalog = JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/catalog.json")));
const dictionary = JSON.parse(readFileSync(join(root, "games/pokepath/assets/public/pokepath/v1/ui-text.json")));
const normalize = value => value.replace(/\[\/?[A-Z_]+\]/g, "").replace(/\s+/g, " ").trim();
const report = { command: `node games/pokepath/tests/language-classic-ui.mjs ${url}`, muted: true, passed: false, cases: [] };
try {
  browser("open", url);
  browser("set", "viewport", "1238", "674");
  browser("reload");
  browser("wait", "--fn", "document.documentElement.dataset.ready==='true'");
  browser("click", '[data-classic-section="settings"]');
  let current = 8;
  for (let language = 0; language < 10; language++) {
    while (current !== language) { browser("click", '[data-setting-next="language"]'); current = (current + 1) % 10; }
    const values = evaluate("({menu:document.querySelectorAll('.pokepath-settings-card>h2')[1].textContent,pokemon:document.querySelector('.pokepath-card-name').textContent,html:document.querySelector('[data-party-slot]').getAttribute('data-pokemon-id'),text:document.querySelector('.pokepath-classic-settings').innerText})");
    assert.equal(values.menu, dictionary.text.menu.settings.title[language] || dictionary.text.menu.settings.title[0]);
    assert.equal(values.pokemon, catalog.species.charmander.names[language] || catalog.species.charmander.names[0]);
    assert.equal(values.html, "1");
    browser("click", '[data-classic-section="dex"]');
    browser("click", '.pokepath-dex-list.tutorials>article:first-child>button');
    const tutorial = evaluate("document.querySelector('.pokepath-dex-tutorial p').textContent");
    assert.equal(normalize(tutorial), normalize(catalog.tutorials.towers.descriptions[language] || catalog.tutorials.towers.descriptions[0]));
    const path = join(output, `native-language-${language}.png`);
    browser("screenshot", path);
    const untranslated = language < 6 || language === 9 ? [...new Set(values.text.match(/[\u3400-\u9fff]+/g) ?? [])] : [];
    assert.deepEqual(untranslated, [], `Untranslated visible settings in language ${language}`);
    report.cases.push({ language, menu: values.menu, pokemon: values.pokemon, stablePokemonID: values.html, tutorialMatchesOriginal: true, screenshot: path, untranslatedChinese: language < 6 || language === 9 ? [...new Set(values.text.match(/[\u3400-\u9fff]+/g) ?? [])] : [] });
    browser("click", '[data-classic-section="settings"]');
  }
  report.passed = true;
} finally { writeFileSync(join(output, "language.json"), JSON.stringify(report, null, 2) + "\n"); }
process.stdout.write(JSON.stringify(report) + "\n");
