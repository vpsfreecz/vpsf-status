# Themes and responsive layout

Every HTML page has one compact settings gear in the header. Its native `details`
panel contains System, Light and Dark icon radios and two language buttons.
System follows `prefers-color-scheme`, including later OS changes. Light and
Dark override the OS choice. Native radio and link keyboard behavior is
preserved; Escape closes the panel and returns focus to its trigger. Selecting
a theme leaves the panel open; outside interaction or focus leaving closes it.
Each native radio covers its visible icon tile, so a held pointer press keeps
focus inside the panel until release. The timestamp shares the header row with
the brand and settings whenever the available width permits.

The side-by-side language buttons are native links. A selected border and
`aria-current` mark the current language. The links work without JavaScript and
use the server-generated switch URLs verbatim. Entity and group pages preserve
their current route and query parameters; switching language from About returns
to the overview.

Without JavaScript, the page follows the OS theme. Theme controls remain hidden
until their handlers are installed.

Theme choices belong to the browser origin: `localStorage["vpsf-status.theme"]`
contains `light` or `dark`; choosing System removes the key. Invalid values use
System. A guarded script before styles applies an existing override on first
paint. Blocked storage leaves switching available for the current page. Without
JavaScript the theme controls stay hidden and CSS follows the OS. There are no
theme cookies, query parameters or server cache variants.

`public/status.css` owns the semantic palette and overrides the Bootstrap 5.2
components used here. Keep foreground, background, focus and control-border
contrast together when adjusting it. Bump local asset URL versions in
`templates/layout.tmpl` when changing cached assets; static responses are cached
for 24 hours.

The light history palette retains the original green `#198754`, yellow
`#ffc107` and red `#dc3545`, also used for probe coverage stripes. These fills
are separate from accessible status text colors. Dark history colors remain
brighter. Missing or unrecognized history states keep the existing default
operational color; no separate unknown-state style is introduced.

The dark header logo is the official transparent white/orange variant, copied
unchanged from the website's `obrazky/vpsFree-bile.png`. The light logo retains
its original asset. CSS selects the matching image for system or explicit
preferences before first paint; neither image needs a white background plate.

Incident, advisory and probe-event tables stack below 768px, with localized field
labels and retained column headers. Smaller status matrices remain tables.
History scrolls within its own track; coarse pointers get larger day targets and
start at the recent end. Day details open on hover, keyboard focus or tap; Escape
or an outside tap dismisses them. Links within the details remain focusable.

Deploy templates and static assets with the binary's matching translation
catalogs. Theme preferences need no conversion on rollback; older versions ignore
the key. This presentation does not change monitoring, JSON, metrics or history
storage.
