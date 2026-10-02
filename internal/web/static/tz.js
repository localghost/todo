// Sends the browser's time zone to the server in the todo_tz cookie, so due
// dates are read and shown in local time. If the server used another zone for
// this page, reload once after the cookie was saved.
(function () {
  "use strict";
  let zone;
  try {
    zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch (e) {
    return;
  }
  if (!zone) {
    return;
  }
  const want = "todo_tz=" + zone;
  const cookies = () => document.cookie.split("; ");
  if (cookies().includes(want)) {
    return; // the server already knows this zone
  }
  const secure = location.protocol === "https:" ? "; Secure" : "";
  document.cookie = want + "; Path=/; Max-Age=31536000; SameSite=Lax" + secure;
  // Reload only when the cookie was saved and the page used another zone, so
  // blocked cookies or a zone the server does not know cannot cause a loop.
  const saved = cookies().includes(want);
  const used = document.body.dataset.tz;
  if (saved && used && used !== zone) {
    location.reload();
  }
})();
