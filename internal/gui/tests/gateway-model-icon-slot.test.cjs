// Run with Node's test runner and Playwright on the module path; see README.md.
// #2498 (塔哥 on Discord): on the Gateway page's model list, a routing
// group's stacked icons made each row's name start at a different x.
// Every row now leads with a fixed-width slot, so a model, a 1-, 2-, 3-
// and 5-provider group share one name column. Chromium and webkit; en;
// narrow width as well as wide.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const providers = {
  providers: [
    { id: "a", name: "Acme", icon: "generic", agents: [], models: [{ id: "solo", name: "Solo", on: true }] },
  ],
  presets: [], excluded: [],
  gateway: {
    running: true, window: true, mine: true, url: "http://127.0.0.1:3999", models: 5, calls: [],
    groups: [
      { id: "group/one", name: "One", icons: ["generic"], providers: ["OpenAI"], members: ["openai/m"] },
      { id: "group/two", name: "Two", icons: ["generic", "openai"], providers: ["OpenAI", "Anthropic"], members: ["openai/m", "anthropic/m"] },
      { id: "group/three", name: "Three", icons: ["generic", "openai", "anthropic"], providers: ["OpenAI", "Anthropic", "Google"], members: ["openai/m", "anthropic/m", "google/m"] },
      { id: "group/five", name: "Five", icons: ["generic", "openai", "anthropic", "deepseek", "grok"], providers: ["OpenAI", "Anthropic", "Google", "DeepSeek", "xAI"], members: ["openai/m", "anthropic/m", "google/m", "deepseek/m", "xai/m"] },
    ],
  },
};

function serve() {
  const state = { agents: [], profiles: [], settings: { lang: "en", theme: "light" } };
  return async (r) => {
    const url = new URL(r.request().url());
    const json = (data) => r.fulfill({ json: data });
    if (url.pathname === "/boot.js") return r.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"en",theme:"light",web:true};` });
    if (url.pathname === "/wails/runtime.js") return r.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(state);
    if (url.pathname === "/api/plugins") return json({ plugins: [] });
    if (url.pathname === "/api/gateway/trace") {
      if (url.searchParams.get("wait")) return new Promise(() => {});
      return json({ mine: true, now: new Date().toISOString(), seq: 1, totals: { requests: 0, rerouted: 0, errors: 0 }, routes: [] });
    }
    if (url.pathname === "/api/gateway/history") return json({ cut: false, days: [], routes: [] });
    if (url.pathname === "/api/groups") return json({ models: [], pools: [], deciders: [], found: true, templates: [], groups: [] });
    if (url.pathname === "/api/providers") return json(providers);
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    try {
      await r.fulfill({ body: await fs.readFile(file), contentType });
    } catch (e) {
      if (contentType === "image/svg+xml") return r.fulfill({ body: "<svg xmlns=\"http://www.w3.org/2000/svg\"/>", contentType });
      throw e;
    }
  };
}

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const width of [1100, 420]) {
    test(`${engine} ${width}: Gateway model names share one leading-slot x (#2498)`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      const page = await (await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: "reduce" })).newPage();
      t.after(() => browser.close());
      page.setDefaultTimeout(5000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await page.route("**/*", serve());
      await page.goto("http://magpie.test/?view=gateway");
      await page.locator("#gwModels .row.model").first().waitFor();
      const xs = await page.evaluate(() => {
        const want = ["a/solo", "group/one", "group/two", "group/three", "group/five"];
        return want.map((id) => {
          const row = [...document.querySelectorAll("#gwModels .row.model")].find((r) => r.querySelector(".name")?.textContent === id);
          if (!row) return { id, missing: true };
          const lead = row.querySelector(":scope > .lead-ic");
          const name = row.querySelector(".name");
          return {
            id,
            hasLead: !!lead,
            leadW: lead ? Math.round(lead.getBoundingClientRect().width) : 0,
            nameX: name ? Math.round(name.getBoundingClientRect().left) : -1,
          };
        });
      });
      assert.deepEqual(errors, []);
      for (const row of xs) {
        assert.equal(row.missing, undefined, JSON.stringify(row));
        assert.equal(row.hasLead, true, JSON.stringify(row));
        assert.equal(row.leadW, 44, JSON.stringify(row));
      }
      const nameXs = xs.map((r) => r.nameX);
      assert.equal(new Set(nameXs).size, 1, `names must share one x: ${JSON.stringify(xs)}`);
    });
  }
}
