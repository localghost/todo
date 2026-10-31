// Makes the screenshots of docs/user-guide.md. It signs up a demo user on a
// running app, adds sample items, dates and labels, and saves one PNG per
// state. run.sh starts the app on a temporary database first.
//
// Usage: node shots.mjs <app URL> <output folder>
import { join } from "node:path";
import { launch, MOUSE, sleep } from "./cdp.mjs";

const [BASE, OUT] = process.argv.slice(2);
if (!BASE || !OUT) {
  console.error("usage: node shots.mjs <app URL> <output folder>");
  process.exit(2);
}
const out = (name) => join(OUT, name);
const q = JSON.stringify;

// The sample list. "due" is days from today and a time; "done" checks it off.
const ITEMS = [
  { text: "Call the dentist", due: [-1, "09:00"], notes: "Dr Nowak, tel. 600 100 200\nBook online: https://dentist.example.com/booking" },
  { text: "Send the invoice", due: [1, "10:00"], labels: ["work"] },
  { text: "Water the plants", labels: ["home"] },
  { text: "Plan the team meeting", due: [3, ""], labels: ["work", "urgent"] },
  { text: "Buy milk", labels: ["home"], done: true },
];
const COLORS = { work: "#2555da", home: "#38c75d", urgent: "#d1382e" };

// send runs a form request from the page, as the logged-in user.
const send = (p, method, path, values) => p.eval(`fetch(${q(path)}, { method: ${q(method)}, headers: { "HX-Request": "true" },
  body: new URLSearchParams(${q(values)}) }).then((r) => r.status)`);

async function signUp(p) {
  await p.goto(BASE + "/signup");
  await p.click("#signup-username"); await p.type("demo");
  await p.click("#signup-password"); await p.type("demo-password");
  await sleep(3500); // the sign-up form refuses a form sent faster than a person can type
  await p.click("#signup-form button[type=submit]");
  await p.waitFor("location.pathname === '/' && !!document.getElementById('new-item')", 20000);
}

// day is the date in days from today, as a date field wants it.
const day = (p, days) => p.eval(`(() => { const d = new Date(); d.setDate(d.getDate() + ${days});
  return d.getFullYear() + "-" + String(d.getMonth() + 1).padStart(2, "0") + "-" + String(d.getDate()).padStart(2, "0"); })()`);

async function addSamples(p) {
  const zone = await p.eval("Intl.DateTimeFormat().resolvedOptions().timeZone");
  for (const [i, it] of ITEMS.entries()) {
    const id = i + 1;
    await send(p, "POST", "/items", { text: it.text });
    for (const name of it.labels || []) {
      await send(p, "POST", `/items/${id}/labels`, { name, color: COLORS[name] });
    }
    if (it.due) {
      await send(p, "PUT", `/items/${id}/due`, { due_date: await day(p, it.due[0]), due_time: it.due[1], due_tz: zone });
    }
    if (it.notes) {
      await send(p, "PUT", `/items/${id}/notes`, { notes: it.notes });
    }
    if (it.done) {
      await send(p, "POST", `/items/${id}/toggle`, {});
    }
  }
}

// NEW_BROWSER makes the page see the notification state of a browser that
// has not asked yet. Headless Chromium always reports "denied", but a new
// user sees the button "Turn on notifications", as the guide says.
const NEW_BROWSER = `Object.defineProperty(Notification, "permission", { get: () => "default" });`;

// openPicker opens the pop-up of an icon on a row and waits for it.
async function openPicker(p, button, ready) {
  await p.goto(BASE + "/");
  await p.click(button);
  await p.waitFor(ready);
  await sleep(200);
}

async function desktop() {
  const p = await launch(9441, MOUSE);
  try {
    await p.viewport(1000, 900, false);
    await p.init(NEW_BROWSER);

    await p.goto(BASE + "/login");
    await p.shot(out("login.png"), [".auth-wrap"]);

    await signUp(p);
    await addSamples(p);

    await p.goto(BASE + "/");
    await p.move(990, 890);
    await p.eval("document.activeElement.blur()");
    await sleep(200);
    await p.shot(out("list.png"), ["main"], 0);

    await p.shot(out("overdue.png"), ["#item-1"]);

    // The add field with a due date and two labels picked.
    await p.goto(BASE + "/");
    await p.click("#new-item"); await p.type("Pay the rent");
    await p.click("#due-add-0");
    await p.waitFor("!!document.querySelector('#due-picker-0')");
    await p.eval(`document.querySelector('#due-picker-0 [name=due_date]').value = ${q(await day(p, 1))}`);
    await p.click("#due-picker-0 .due-time"); await p.type("1000");
    await p.click("#due-picker-0 .due-save");
    await p.waitFor("!!document.querySelector('.due-set #due-add-0')");
    for (const name of ["home", "urgent"]) {
      await p.click("#label-add-0");
      await p.waitFor("!!document.querySelector('#picker-0')");
      await p.click(`#picker-0 .picker-opt[data-name=${q(name)}] .picker-name`);
      await p.waitFor(`[...document.querySelectorAll('#add-extras > .chip .chip-name')].some((e) => e.textContent === ${q(name)})`);
      await p.key("Escape");
      await p.waitFor("!document.querySelector('#picker-0')");
    }
    await p.click("#new-item");
    await p.shot(out("add-form.png"), ["#add-form", "#add-extras"]);

    await p.goto(BASE + "/");
    await p.click("#item-3 .text");
    await p.waitFor("!!document.querySelector('#item-3.editing input')");
    await p.shot(out("edit.png"), ["#item-2", "#item-3", "#item-4"]);
    await p.key("Escape");

    await p.goto(BASE + "/");
    await p.click(".clear-done");
    await p.waitFor("!!document.querySelector('#clear-confirm .confirm')");
    await p.shot(out("clear-done.png"), ["#toolbar", "#clear-confirm"]);

    await openPicker(p, "#due-add-2", "!!document.querySelector('#due-picker-2 input[type=date]')");
    await p.shot(out("due-popup.png"), ["#item-2", "#due-picker-2"]);

    // The notes of an item (canvas page "21 · Notes", option N2).
    await openPicker(p, "#notes-add-1", "!!document.querySelector('#notes-picker-1 .notes-view')");
    await p.shot(out("notes.png"), ["#item-1", "#notes-picker-1"]);

    const ready = "document.activeElement === document.querySelector('#picker-2 .picker-search')";
    await openPicker(p, "#label-add-2", ready);
    await p.shot(out("label-list.png"), ["#item-2", "#picker-2"]);

    // The Labels page: at rest, the color wheel of "work", the question
    // before "urgent" is deleted.
    await p.goto(BASE + "/labels");
    await p.move(990, 890);
    await sleep(200);
    await p.shot(out("labels-page.png"), ["main"], 0);
    await p.click(`.lp-line[data-name=${q("work")}] .sw`);
    await p.waitFor("document.activeElement.matches('.wheel')");
    await p.shot(out("color-wheel.png"), [`.lp-line[data-name=${q("work")}]`, ".lp-color"]);
    await p.key("Escape");
    await p.click(`.lp-line[data-name=${q("urgent")}] .lp-x`);
    await p.waitFor("!!document.querySelector('.lp-ask [data-ask-delete]')");
    await p.shot(out("label-delete.png"), [`.lp-line[data-name=${q("urgent")}]`, ".lp-ask"]);

    await p.goto(BASE + "/");
    const work = await p.eval(`document.querySelector('#item-2 .chip[data-label] a').getAttribute('href')`);
    await p.goto(BASE + work);
    await p.move(990, 890);
    await sleep(200);
    await p.shot(out("filter.png"), ["main"], 0);

    // The list in due date order, then back to the hand-made order, so the
    // phone shot shows the list as at the start.
    await p.goto(BASE + "/");
    await p.click(".sort-due");
    await p.waitFor("document.getElementById('list-section').dataset.sortDue === 'true'");
    await p.move(990, 890);
    await p.eval("document.activeElement.blur()");
    // The rows slide to their places (app.js, SLIDE_MS): wait until they stop.
    await p.waitFor("document.getAnimations().length === 0");
    await p.shot(out("sort-due.png"),["#toolbar", ".sort-hint", "#items"]);
    await p.click(".sort-due");
    await p.waitFor("document.getElementById('list-section').dataset.sortDue === 'false'");
    await p.waitFor("document.getAnimations().length === 0");

    // The search with "pl" (canvas page "19 · Search").
    await p.click(".search-toggle");
    await p.type("pl");
    await p.move(990, 890);
    await sleep(200);
    await p.shot(out("search.png"), ["#toolbar", "#search-row", "#items"]);
    await p.key("Escape");

    // A repeating item right after its check (canvas page "20 · Repeat"),
    // then its old date and no repeat again, for the phone shot.
    const zone = await p.eval("Intl.DateTimeFormat().resolvedOptions().timeZone");
    const back = { due_date: await day(p, 1), due_time: "10:00", due_tz: zone };
    await send(p, "PUT", "/items/2/due", { ...back, due_repeat: "week" });
    await p.goto(BASE + "/");
    await p.click("#check-2");
    await p.waitFor("!!document.querySelector('#item-2 .repeat-msg')");
    await p.move(990, 890);
    await p.eval("document.activeElement.blur()");
    await sleep(200);
    await p.shot(out("repeat.png"), ["#item-2"]);
    await send(p, "PUT", "/items/2/due", { ...back, due_repeat: "" });
    return p.logs;
  } finally {
    p.close();
  }
}

// phone takes one shot at 375 px with a touch screen (no mouse settings).
async function phone() {
  const p = await launch(9442, ["--hide-scrollbars"]);
  try {
    await p.viewport(375, 740, true);
    await p.init(NEW_BROWSER);
    await p.goto(BASE + "/login");
    await p.click("#login-username"); await p.type("demo");
    await p.click("#login-password"); await p.type("demo-password");
    await p.click(".auth-form button[type=submit]");
    await p.waitFor("location.pathname === '/' && !!document.getElementById('new-item')", 20000);
    await p.eval("document.activeElement.blur()");
    await sleep(300);
    await p.shot(out("phone.png"));
    return p.logs;
  } finally {
    p.close();
  }
}

const logs = [...await desktop(), ...await phone()];
// The empty-text and permission checks of the app log no errors; anything
// here is a problem with the page or the script.
if (logs.length) {
  console.error("browser errors:\n" + logs.join("\n"));
  process.exit(1);
}
console.log("screenshots saved in " + OUT);
