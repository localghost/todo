// A small Chrome DevTools Protocol driver for the guide screenshots. It needs
// no packages: Node's own fetch and WebSocket talk to headless Chromium.
import { spawn } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, writeFileSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { join } from "node:path";

// findChrome returns $CHROME, or the newest headless Chromium that Playwright
// installed in ~/.cache/ms-playwright.
function findChrome() {
  if (process.env.CHROME) {
    return process.env.CHROME;
  }
  const cache = join(homedir(), ".cache", "ms-playwright");
  const dirs = existsSync(cache) ? readdirSync(cache).filter((d) => d.startsWith("chromium_headless_shell-")).sort().reverse() : [];
  for (const d of dirs) {
    const bin = join(cache, d, "chrome-headless-shell-linux64", "chrome-headless-shell");
    if (existsSync(bin)) {
      return bin;
    }
  }
  throw new Error("no headless Chromium: set CHROME, or run `npx playwright install chromium-headless-shell`");
}

// MOUSE makes the browser report a mouse (hover: hover). Without it, headless
// Chromium reports a touch screen.
export const MOUSE = ["--blink-settings=primaryHoverType=2,availableHoverTypes=2,primaryPointerType=4,availablePointerTypes=4"];

export async function launch(port, extraArgs = []) {
  const dir = mkdtempSync(join(tmpdir(), "guide-shots-"));
  const proc = spawn(findChrome(), ["--headless", "--no-sandbox", `--remote-debugging-port=${port}`, `--user-data-dir=${dir}`, ...extraArgs, "about:blank"], { stdio: "ignore" });
  let info;
  for (let i = 0; i < 50 && !info; i++) {
    await sleep(100);
    try { info = await (await fetch(`http://127.0.0.1:${port}/json/version`)).json(); } catch {}
  }
  if (!info) {
    proc.kill();
    throw new Error("Chromium did not start");
  }
  const ws = new WebSocket(info.webSocketDebuggerUrl);
  await new Promise((r) => ws.addEventListener("open", r, { once: true }));
  let next = 0;
  const pending = new Map();
  const logs = [];
  ws.addEventListener("message", (m) => {
    const msg = JSON.parse(m.data);
    if (msg.id !== undefined) {
      const p = pending.get(msg.id);
      pending.delete(msg.id);
      if (msg.error) p.reject(new Error(msg.error.message));
      else p.resolve(msg.result);
    } else if (msg.method === "Runtime.exceptionThrown") {
      logs.push("exception: " + msg.params.exceptionDetails.exception?.description);
    } else if (msg.method === "Log.entryAdded" && msg.params.entry.level === "error") {
      logs.push("log: " + msg.params.entry.text);
    }
  });
  const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
    const id = ++next;
    pending.set(id, { resolve, reject });
    ws.send(JSON.stringify({ id, method, params, sessionId }));
  });
  const { targetId } = await send("Target.createTarget", { url: "about:blank" });
  const { sessionId } = await send("Target.attachToTarget", { targetId, flatten: true });
  const s = (method, params) => send(method, params, sessionId);
  await s("Page.enable");
  await s("Runtime.enable");
  await s("Log.enable");

  const page = {
    logs,
    send,
    close() { ws.close(); proc.kill(); },
    // init runs source in each new page before the page's own scripts.
    async init(source) {
      await s("Page.addScriptToEvaluateOnNewDocument", { source });
    },
    async goto(url) {
      await s("Page.navigate", { url });
      await page.waitFor("document.readyState === 'complete'");
      await sleep(400);
    },
    async eval(expr) {
      const r = await s("Runtime.evaluate", { expression: expr, awaitPromise: true, returnByValue: true });
      if (r.exceptionDetails) throw new Error("eval failed: " + expr + "\n" + (r.exceptionDetails.exception?.description || r.exceptionDetails.text));
      return r.result.value;
    },
    async waitFor(expr, ms = 5000) {
      const end = Date.now() + ms;
      while (Date.now() < end) {
        try { if (await page.eval(expr)) return; } catch {}
        await sleep(50);
      }
      throw new Error("timeout waiting for: " + expr);
    },
    async center(selector) {
      const r = await page.eval(`(() => { const e = document.querySelector(${JSON.stringify(selector)}); if (!e) return null; e.scrollIntoView({ block: 'nearest' }); const b = e.getBoundingClientRect(); return [b.left + b.width / 2, b.top + b.height / 2]; })()`);
      if (!r) throw new Error("no element: " + selector);
      return r;
    },
    async move(x, y) {
      await s("Input.dispatchMouseEvent", { type: "mouseMoved", x, y });
    },
    async click(selector) {
      const [x, y] = await page.center(selector);
      await page.move(x, y);
      await s("Input.dispatchMouseEvent", { type: "mousePressed", x, y, button: "left", buttons: 1, clickCount: 1 });
      await s("Input.dispatchMouseEvent", { type: "mouseReleased", x, y, button: "left", buttons: 0, clickCount: 1 });
      await sleep(80);
    },
    async hover(selector) {
      const [x, y] = await page.center(selector);
      await page.move(x, y);
      await sleep(80);
    },
    async type(text) {
      await s("Input.insertText", { text });
      await sleep(50);
    },
    async key(key) {
      const codes = { Enter: 13, Escape: 27 };
      const down = { type: "keyDown", key, code: key, windowsVirtualKeyCode: codes[key] };
      if (key === "Enter") down.text = "\r";
      await s("Input.dispatchKeyEvent", down);
      await s("Input.dispatchKeyEvent", { type: "keyUp", key, code: key, windowsVirtualKeyCode: codes[key] });
      await sleep(100);
    },
    async viewport(width, height, touch) {
      await s("Emulation.setDeviceMetricsOverride", { width, height, deviceScaleFactor: 1, mobile: !!touch });
    },
    // shot saves a PNG of the box around the given elements, with pad pixels
    // around it, or of the whole viewport if no selector is given.
    async shot(path, selectors = [], pad = 6) {
      let clip;
      if (selectors.length) {
        const box = await page.eval(`(() => {
          const rects = ${JSON.stringify(selectors)}.map((s) => document.querySelector(s)).filter(Boolean).map((e) => e.getBoundingClientRect());
          if (!rects.length) return null;
          return { x: Math.min(...rects.map((r) => r.left)) + scrollX, y: Math.min(...rects.map((r) => r.top)) + scrollY,
                   r: Math.max(...rects.map((r) => r.right)) + scrollX, b: Math.max(...rects.map((r) => r.bottom)) + scrollY };
        })()`);
        if (!box) throw new Error("no element for the shot: " + selectors.join(", "));
        clip = { x: Math.max(0, box.x - pad), y: Math.max(0, box.y - pad), width: box.r - box.x + 2 * pad, height: box.b - box.y + 2 * pad, scale: 1 };
      }
      const { data } = await s("Page.captureScreenshot", { format: "png", ...(clip ? { clip, captureBeyondViewport: true } : {}) });
      writeFileSync(path, Buffer.from(data, "base64"));
    },
  };
  return page;
}

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
