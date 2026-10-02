package web_test

import "testing"

func TestPagesLoadTimeZoneScript(t *testing.T) {
	env := newTestEnv(t)
	tag := `<script src="/static/tz.js" defer></script>`
	assertContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), tag)
	assertContains(t, do(t, env.H, "GET", "/login", nil, anon).Body.String(), tag)
	assertContains(t, do(t, env.H, "GET", "/signup", nil, anon).Body.String(), tag)

	js := do(t, env.H, "GET", "/static/tz.js", nil, anon).Body.String()
	assertContains(t, js, "resolvedOptions().timeZone", `"todo_tz="`, "SameSite=Lax", "Secure",
		// Reload only when the cookie was saved and the page used another zone,
		// so blocked cookies or an unknown zone cannot cause a reload loop.
		"if (saved && used && used !== zone) {")
}
