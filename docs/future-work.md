# Future work

Ideas that are not built. Each one says what it gives and what it costs.

## User guide with MkDocs Material

Today the app renders one Markdown file, `docs/user-guide.md`, with goldmark and serves it at
`/guide`. This is enough for one page. If the guide grows to many pages, MkDocs Material can
build it as a small static site.

What it gives:

- A search box over all pages.
- A side menu with the pages and their sections.
- Light and dark mode, and a good layout on a phone.
- Notes, tabs and other blocks for longer text.

How to build it in the same app (no second Fly app is needed):

1. Move the guide into `docs/guide/` as several Markdown files, with a `mkdocs.yml` next to them.
   The current text and the images in `docs/guide-images/` move over as they are.
2. Add a first stage to the `Dockerfile` that uses a Python image, runs
   `pip install mkdocs-material` and `mkdocs build`, and makes the folder `site/`.
3. Copy `site/` into the final image, for example to `/guide`.
4. Give the app a flag, for example `-guide-dir /guide`, and serve that folder at `/guide/`
   with `http.FileServer`, with no login. If the folder is missing (for example with `go run`),
   the app does not serve the guide.

What it costs:

- Content Security Policy: the app sends `default-src 'self'` with every response. MkDocs
  Material puts small inline scripts in its pages (for example for the color scheme), and this
  policy blocks them. `/guide/` needs its own policy: hashes of those scripts, or
  `'unsafe-inline'` for scripts on `/guide/` only.
- Caching: the guide pages and the search index are public, so `/guide/` should send
  `Cache-Control: public` like `/guide` does today.
- Tools: each deploy runs the Python stage, so the build takes longer. To see the guide on a
  computer, you need Python and `mkdocs serve`.
- Deploys: a change to the guide only is still an app deploy, so the machine restarts (a few
  seconds with one machine).
- The screenshot script (`mise run guide:shots`) stays the same.
