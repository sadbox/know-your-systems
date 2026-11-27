# Emergency Preparedness Workbook

"Know your systems": a household workbook for getting through a two-week power outage. Made in Tunbridge,
Vermont. A half-letter booklet, folded and stapled, that replaces the "Be Prepared" tri-fold pamphlet.
Twenty-four inside pages on six letter sheets, set in 11 pt type, a cover sheet printed in one ink on light
orange stock, and a two-week supply list on two letter-size sheets to laminate.

A collaboration of the towns of Tunbridge and Royalton and the Two Rivers-Ottauquechee Regional
Commission (TRORC). Written by James McGuire, Brenda Field, Ari Lattanzi and Brad Salzmann.

## Build

    ./build                           # the PDFs to ./out and the website to ./site, in the pinned container

Needs Docker. `./build` builds the `Dockerfile` (the generator) on the base image (`base/Dockerfile`: Debian
with a pinned Chromium) and runs it on this checkout, exactly as CI and the website's deploy do, so a page
that fits here fits there. Arguments pass through, e.g. `./build -edition "March 2027"`.

The base image is `ghcr.io/sadbox/know-your-systems-base` (public), tagged from `base/Dockerfile` itself
(`base/tag`: the Chromium version and a hash of the file). `./kys` pulls it from GHCR, and that is the
only way it gets it. No login is needed.

To update Chromium: in one pull request, move `SNAPSHOT` in `base/Dockerfile` to a later
snapshot.debian.org date and `CHROMIUM`, `POPPLER` and `CA_CERTIFICATES` to the versions on it, and build
with `KYS_BUILD_BASE=1 ./build` (the new base isn't published until the merge). CI builds it the same way
for a change to `base/`, the visual diff shows what it changed, and on merge `base.yaml` publishes it.

`go run .` also works, with Go (the version in `go.mod`) and your own Chrome or Chromium, for quick
edits. Its fit check can differ from the container's by a few hundredths of an inch, so check with
`./build` before a pull request. No Go dependencies outside the standard library.

    go run .                          # writes the five PDFs to ./out
    go run . -edition "March 2027"    # edition line in the back-cover credit (default: the EDITION file)
    go run . -chrome /path/to/chrome  # if Chrome is not on PATH (or set $CHROME)
    go run . -allow-overflow          # while editing: build even if a page does not fit
    go run . -cc-by-sa                # print the CC BY-SA 4.0 notice (back cover, website footer); off until every author agrees

| File | What it is |
|---|---|
| `out/know-your-systems-cover.pdf` | Cover. One letter sheet, two sides: outside (back, front) and inside (Know your town, Home map). Black only. |
| `out/know-your-systems-interior.pdf` | Inside pages laid out as printer spreads. Six letter sheets, twelve sides. Full color. |
| `out/know-your-systems-supply-list.pdf` | The laminated two-week supply list: two sheets, four sides. Sheet 1: food, water and fuel, then the Medical and Household lists. Sheet 2: Hardware list, then the outage bucket, safety notes and "Anything else". A date box for each item, and blank rows at the end of every section. |
| `out/know-your-systems-proof.pdf` | Every booklet page in reading order at 5.5 x 8.5 in, for review on screen. Cover pages are tinted light orange here to suggest the stock; the cover PDF itself has no tint. |
| `out/know-your-systems-cover-mockup.pdf` | The cover again, with the light orange printed behind it. For a test copy on white stock only; pair it with the interior file. Do not print it on orange stock. |
| `out/know-your-systems-booklet.pdf` | Cover and interior in one file, for printing the whole booklet on plain white paper: seven letter sheets, landscape, two-sided, **flip on short edge**, actual size. The light orange is printed in on the cover sheet. Stack in print order, fold, staple. |

## Checks

- **Every page fits:** every build fails if a page's content is too tall or a row runs off the side.
- **What a change does to the pages:** on every pull request, ci.yaml's `visual-diff` job builds the base
  branch and the PR, compares every PDF page by page, and lists the changed pages in the job summary,
  with pictures (old, new, changes in red) in the run's `page-diff` artifact. Locally:
  `./kys -diff <other checkout>/out` after `./build`.
- **The printed web addresses:** `.github/workflows/checks.yaml` fetches every one on the 1st of the month
  (`./kys -links`); a site that turns automated checks away, or answers only over http, is a warning.
- **The facts, yearly:** on September 1st the same workflow opens an issue listing every printed phone
  number and web address as a checklist for a person to re-verify before the fall reprint (`./kys -facts`).
- **The site:** knowyoursystems.org is a Synthetic Monitoring website check in homelab-configs
  (`tofu/monitoring/synthetics.tf`), with the other sites' "website down" alert.

`./kys` runs any of the generator's commands in the same container as `./build`.

## Home-print editions and the website

    go run . -site site                                   # also writes the website to ./site, for https://knowyoursystems.org/
    go run . -site site -site-url https://example.org/    # the same, for another address (used in the link-preview tags)

Two more PDFs are built for anyone to print on a desk printer. The website offers only these:

| File | What it is |
|---|---|
| `out/know-your-systems-workbook-letter.pdf` | Every booklet page in reading order, enlarged 1.28x onto its own portrait letter page (about 14 pt type). Print single- or double-sided; nothing to fold or staple. |
| `out/know-your-systems-supply-list-letter.pdf` | The supply list worded for pencil on plain paper instead of a wet-erase marker on a laminated sheet. |

The wording differences are sheets-mode blocks, `{{if .Sheets}}…{{else}}…{{end}}`: pencil instead of a wet-erase marker on the supply list, the supply list's name on the Food page, and page names ("the Know your town page") where the booklet says "inside the front cover". Without `.Sheets` (workbook mode) the text is for the printed kit.
The build fit-checks both home editions too.

`-site` writes a static website to the directory you name: `index.html` (the landing page and downloads), `downloads/`
and `assets/`. The site is only a place to get the PDFs. Nothing is filled in online: the workbook has to work when the
power and the internet are out.
`.github/workflows/pages.yaml` builds it on GitHub's runner and publishes it to https://knowyoursystems.org/
on every push to `main`; `site/` and `out/` are not committed. For a new edition, change `EDITION`. Its text comes from the same templates as the print, through
named blocks shared with the booklet (`kys-intro`, `kys-how`, `kys-toc`, `vt-numbers`, `vt-alert`,
`credit-text`); only the surrounding web wording is in
`templates/site.html`. Styles are `templates/site.css`.
The team's print files (the proof, cover, interior, booklet, cover mock-up and laminated list) are on an
unlisted print-kit page, `site/kit-8db13109/` (`kitPath` in site.go), with how to print each and the commit
they were built from. It is linked from nowhere and asks search engines not to index it; anyone with the
link can open it, and nothing on it is private.

## Print

1. Cover: black ink on light orange letter stock, landscape, two-sided, **flip on short edge**, actual size (no "fit to page").
2. Interior: plain letter paper, landscape, two-sided, **flip on short edge**, actual size.
3. Stack the six interior sheets in the order they print (page 24 | 1 on the outside, 12 | 13 in the middle),
   lay the cover on the outside, fold in half and staple twice on the fold.

All content sits at least 0.3 in from the sheet edges, so an ordinary office printer will not clip it.

## The laminated supply list

Two letter sheets travel with the booklet. The supply list is on these sheets only; the booklet does not
repeat it. The check-up lists are in the booklet only (pages 12, 20 and 21).

1. Print `know-your-systems-supply-list.pdf` on white stock, portrait, two-sided, **flip on long edge**,
   actual size. Pages 1 and 2 make sheet 1; pages 3 and 4 make sheet 2.
2. Laminate each in a letter pouch (9 x 11 1/2 in). Nothing needs trimming.
3. Hand them out with a wet-erase marker. People tick a box and write the date they bought or replaced
   the item; a damp cloth clears it.

Each list side is three columns: tick boxes, words, date boxes. The rules run under the words only, so no
rule comes near a box. The values at the top of the `.supply` block in `style.css` set the sizes:

| Value | What it sets | Now |
|---|---|---|
| `--dw`, `--dbox` | outside width and height of a date box | 0.98 x 0.39 in |
| `--dgap` | clear space between one date box and the next; a one-line row is `--dbox` + `--dgap` tall | 0.11 in |
| `--tbox` | outside size of a tick box | 0.22 in |
| `--tick`, `--dcol` | width kept for the tick box and for the date box | 0.32 in, 1.06 in |

Every section ends with blank rows for the household's own items. In `templates/sheet.html` each one is
a `{{template "sup-blank"}}` after the section's list; add or remove them there.

The list sides fill the sheet. Both columns run down to the footer, and the rows of a column share
whatever room is left over, so rows end up a little taller than the minimum above and no blank band is
left at the foot. The build still prints the real spare room for each side (it measures with the
stretching switched off). When a side shows 0.5 in or more to spare, one more blank row will fit in its
taller column; when the build says a side is too tall, take a blank row out.

Sections are placed so the columns come out even: fuel sits with food and water on sheet 1 (the things
with a shelf life), and the vehicle kit sits under tools on sheet 2.

## The lists

`templates/lists.html` holds every list item:

- the supply lists (`sup-...`) print on the laminated supply list;
- the check-up lists (`li-...`) print in the booklet, on pages 12, 20 and 21.

In a supply item the `<span>` is the item and the optional `<em>` is how long it keeps. The sheets are laid
out in `templates/sheet.html`.

## Edit

Each page is a named block in `templates/interior.html` or `templates/cover.html`. Plain HTML, styled by
`templates/style.css`.

- Page order is the `interior-order` line at the top of `interior.html`. The count must be a multiple of 4.
- A fill-in row is `<div class="f"><span>Label</span><i></i></div>`. Each `<i>` is a write-in line;
  classes `xs`, `s`, `m`, `l` give it a fixed width. `<b class="cb"></b>` is a checkbox.
- `{{pg "water"}}` prints that page's number, so cross references survive reordering.
- Boxes all share one build, a tinted panel with a bar down the left, and differ only in colour:
  `<div class="tip">` (green, a note), `<div class="warn">` (orange, a danger), and
  `<div class="tw"><b>For two weeks</b><span>...</span></div>` (blue, the recurring two-week note).
  `<div class="calc">` sets a line of worked arithmetic.
  Use a box for a note that sits beside something the reader fills in. Advice that stands on its own is
  set as a section instead (next line).
- A section of plain advice is `<section><h2>Heading</h2><ul class="y">...</ul></section>`. Each `<li>`
  gets a green tick. For a list of things never to do, put `class="dng"` on the `<h2>` (orange heading)
  and `class="no"` on every `<li>` in it (orange cross). Keep the two together, so that orange always
  means "never": a section is all ticks under a green heading or all crosses under an orange one.
  `<small>` inside an `<h2>` sets a short note at the right end of its rule. The `<section>` keeps a
  heading with its list when spare room is shared out. Page 5 is built this way, and so is "What to
  stock" on the Food page.
- A blank tick-box row for the household's own item is `<div class="f own"><b class="cb"></b><i></i></div>`.
- A heading the household finishes itself is `<h3 class="fill"><span>Every</span><i></i></h3>`; put blank
  tick-box rows under it. Page 20 has two.
- Spare room on a page is shared out between its rows automatically. To pack a page from the top instead,
  put `<div class="grow"></div>` where the slack should go.
- Wrap a phone number in running text in `<span class="nb">` so it never breaks across two lines.
- Type size for the whole booklet is one number: `--kk` at the top of `style.css`. 1 is 10 pt body type,
  1.1 is 11 pt (the current setting). Every text size in the booklet scales with it; write-in rows, page
  numbers and page titles do not. Raise it and the build tells you which pages no longer fit.

The build measures every page in Chrome and **fails if anything is too tall for its page or a row runs off
the side**, naming the page. It also prints how much spare room each page has.

## Pieces

    main.go            the generator
    site.go            the website writer
    templates/         page content (interior, cover, the two sheets, shared lists, the website) and stylesheets
    assets/fonts/      Atkinson Hyperlegible (text) and Barlow Condensed (headings), both SIL Open Font License
    assets/*.png       cover illustration, prepared for print
    art/               the source illustration and prep_art.py, which makes the two PNGs in assets/

The cover illustration is from a photograph by James McGuire. To swap it, replace
`art/groundhog-source.png` (black ink on a transparent or white background) and run
`uv run art/prep_art.py`, then rebuild. It also makes the website icons in `assets/site/`.

## License

The generator is MIT-licensed (`LICENSE`): the Go code, the scripts, the Dockerfiles, the workflows, the
stylesheets and the template markup.

The workbook and the supply list themselves are not: their wording (in `templates/`), the cover
illustration and its source photograph (`assets/`, `art/`) and the PDFs built from them belong to their
authors. The fonts in `assets/fonts/` carry their own SIL Open Font License.
