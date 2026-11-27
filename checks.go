package main

// Checks that run beside the build (.github/workflows):
//
//	-diff OLD   what a change did to the printed pages: every PDF in OLD against the same one in
//	            -out, page by page, as pictures (ci.yaml, on every pull request)
//	-links      every web address the print shows still answers (checks.yaml, monthly)
//	-facts      the checklist of every phone number and web address in the print, for a person
//	            to re-verify before the fall reprint (checks.yaml, each September)

import (
	"bufio"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ---------- -diff ----------

// visualDiff rasterizes every PDF that both oldDir and newDir hold (pdftoppm, 50 dpi), compares
// them page by page, and writes a picture of each page that changed into outDir: the old page,
// the new one, and the new one again with the changed pixels in red. It returns a Markdown
// summary for the pull request's job summary.
func visualDiff(oldDir, newDir, outDir string) (string, error) {
	pdfs, err := filepath.Glob(filepath.Join(newDir, "*.pdf"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "kys-diff-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	var rows []string
	total, changed := 0, 0
	for _, newPDF := range pdfs {
		name := strings.TrimSuffix(filepath.Base(newPDF), ".pdf")
		oldPDF := filepath.Join(oldDir, name+".pdf")
		if _, err := os.Stat(oldPDF); err != nil {
			rows = append(rows, fmt.Sprintf("| %s | new file |", name))
			continue
		}
		oldPages, err := rasterize(oldPDF, filepath.Join(tmp, "old", name))
		if err != nil {
			return "", err
		}
		newPages, err := rasterize(newPDF, filepath.Join(tmp, "new", name))
		if err != nil {
			return "", err
		}
		var pages []string
		for i := 0; i < max(len(oldPages), len(newPages)); i++ {
			total++
			switch {
			case i >= len(oldPages):
				pages = append(pages, fmt.Sprintf("%d (added)", i+1))
				changed++
			case i >= len(newPages):
				pages = append(pages, fmt.Sprintf("%d (removed)", i+1))
				changed++
			default:
				differs, err := comparePages(oldPages[i], newPages[i], filepath.Join(outDir, fmt.Sprintf("%s-p%02d.png", name, i+1)))
				if err != nil {
					return "", err
				}
				if differs {
					pages = append(pages, fmt.Sprint(i+1))
					changed++
				}
			}
		}
		if len(pages) > 0 {
			rows = append(rows, fmt.Sprintf("| %s | %s |", name, strings.Join(pages, ", ")))
		}
	}

	var b strings.Builder
	b.WriteString("### Printed pages\n\n")
	if len(rows) == 0 {
		fmt.Fprintf(&b, "No printed page changed (%d pages compared).\n", total)
		return b.String(), nil
	}
	fmt.Fprintf(&b, "%d of %d pages changed. Each changed page is in the run's `page-diff` artifact: the old page, the new one, and the new one with the changes in red.\n\n", changed, total)
	b.WriteString("| File | Pages |\n|---|---|\n")
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
	return b.String(), nil
}

// rasterize renders every page of a PDF to PNG in its own directory (so one PDF's pages never
// match another's whose name it starts) and returns the files in page order.
func rasterize(pdf, dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	prefix := filepath.Join(dir, "p")
	out, err := exec.Command("pdftoppm", "-r", "50", "-png", pdf, prefix).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pdftoppm %s: %w\n%s", pdf, err, out)
	}
	pages, err := filepath.Glob(prefix + "-*.png")
	sort.Strings(pages) // pdftoppm pads the page numbers, so names sort in page order
	return pages, err
}

// comparePages reports whether two page pictures differ, and if they do writes the side-by-side
// picture to out.
func comparePages(oldPNG, newPNG, out string) (bool, error) {
	a, err := readPNG(oldPNG)
	if err != nil {
		return false, err
	}
	b, err := readPNG(newPNG)
	if err != nil {
		return false, err
	}
	ab, bb := a.Bounds(), b.Bounds()
	w, h := max(ab.Dx(), bb.Dx()), max(ab.Dy(), bb.Dy())
	mark := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(mark, bb, b, bb.Min, draw.Src)
	differs := ab.Size() != bb.Size()
	red := color.RGBA{R: 230, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := image.Pt(x, y)
			if !p.In(ab) || !p.In(bb) || !sameColor(a.At(x, y), b.At(x, y)) {
				differs = true
				mark.Set(x, y, red)
			}
		}
	}
	if !differs {
		return false, nil
	}
	const gap = 12
	sheet := image.NewRGBA(image.Rect(0, 0, 3*w+2*gap, h))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.Gray{Y: 160}), image.Point{}, draw.Src)
	draw.Draw(sheet, ab.Sub(ab.Min), a, ab.Min, draw.Src)
	draw.Draw(sheet, bb.Sub(bb.Min).Add(image.Pt(w+gap, 0)), b, bb.Min, draw.Src)
	draw.Draw(sheet, mark.Bounds().Add(image.Pt(2*(w+gap), 0)), mark, image.Point{}, draw.Src)
	f, err := os.Create(out)
	if err != nil {
		return true, err
	}
	if err := png.Encode(f, sheet); err != nil {
		f.Close()
		return true, err
	}
	return true, f.Close()
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func sameColor(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// ---------- -links ----------

// webRE finds a web address written in the print: a host under .gov, .org or .com, with or
// without a path, as the workbook prints them (vtalert.gov, dcf.vermont.gov/benefits/fuel). An
// address after "@" is an email address and is left out.
var webRE = regexp.MustCompile(`(?i)(?:^|[^@\w.-])((?:[a-z0-9-]+\.)+(?:gov|org|com)(?:/[a-z0-9/_.-]*[a-z0-9/])?)`)

// printedAddresses returns every web address in the print templates (not the website's own), each
// once, in order.
func printedAddresses(tplDir string) ([]string, error) {
	files, err := printTemplates(tplDir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		for _, m := range webRE.FindAllStringSubmatch(string(b), -1) {
			if u := strings.ToLower(m[1]); !seen[u] && !strings.HasPrefix(u, "www.w3.org") {
				seen[u] = true
				out = append(out, m[1])
			}
		}
	}
	return out, nil
}

// printTemplates is every template that prints (all but the website's).
func printTemplates(tplDir string) ([]string, error) {
	all, err := filepath.Glob(filepath.Join(tplDir, "*.html"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range all {
		if filepath.Base(f) != "site.html" {
			out = append(out, f)
		}
	}
	return out, nil
}

// checkLinks fetches every printed web address over https, as a browser would first. A site that
// answers 401, 403 or 429 is up but turns automated checks away, and one that answers only over
// http is still reached by a browser that falls back to it: both are warnings. Anything else at
// 400 or above, or no answer after a retry, fails.
func checkLinks(tplDir string) error {
	addrs, err := printedAddresses(tplDir)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	var failed []string
	for _, a := range addrs {
		url := "https://" + a
		status, err := fetch(client, url)
		if err != nil || status >= 500 {
			time.Sleep(5 * time.Second)
			status, err = fetch(client, url)
		}
		if err != nil {
			if hs, herr := fetch(client, "http://"+a); herr == nil && hs < 400 {
				fmt.Printf("warn  %s: no https (%v); http://%s answers %d\n", url, err, a, hs)
				continue
			}
		}
		switch {
		case err != nil:
			fmt.Printf("FAIL  %s: %v\n", url, err)
			failed = append(failed, url)
		case status == 401 || status == 403 || status == 429:
			fmt.Printf("warn  %s: %d (turns automated checks away; check it by hand)\n", url, status)
		case status >= 400:
			fmt.Printf("FAIL  %s: %d\n", url, status)
			failed = append(failed, url)
		default:
			fmt.Printf("ok    %s: %d\n", url, status)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d printed web addresses failed: %s", len(failed), len(addrs), strings.Join(failed, ", "))
	}
	fmt.Printf("all %d printed web addresses answer\n", len(addrs))
	return nil
}

func fetch(c *http.Client, url string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; knowyoursystems.org link check)")
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

// ---------- -facts ----------

var (
	phoneRE  = regexp.MustCompile(`\b(?:\d{3}-\d{3}-\d{4}|\d-\d-\d)\b`)
	defineRE = regexp.MustCompile(`{{\s*define\s+"([^"]+)"\s*}}`)
	h1RE     = regexp.MustCompile(`(?s)<h1>(.*?)</h1>`)
	tagRE    = regexp.MustCompile(`<[^>]*>|{{[^}]*}}`)
	spaceRE  = regexp.MustCompile(`\s+([.,;:!?)])`)

	// blockTitles names the shared blocks, which have no <h1> of their own, by where they print.
	blockTitles = map[string]string{
		"vt-numbers": "Back cover: Vermont numbers",
		"vt-alert":   "Back cover: VT-Alert",
		"c4":         "Back cover",
	}
)

// factsChecklist returns, as Markdown for a GitHub issue, every printed line that carries a phone
// number or a web address, under the page it is on.
func factsChecklist(tplDir, edition string) (string, error) {
	files, err := printTemplates(tplDir)
	if err != nil {
		return "", err
	}
	type section struct {
		title string
		items []string
	}
	var sections []*section
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			return "", err
		}
		var cur *section
		sc := bufio.NewScanner(file)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if m := defineRE.FindStringSubmatch(line); m != nil {
				cur = &section{title: m[1]}
				if t, ok := blockTitles[m[1]]; ok {
					cur.title = t
				}
				sections = append(sections, cur)
			}
			if m := h1RE.FindStringSubmatch(line); m != nil && cur != nil {
				cur.title = strings.TrimSpace(html.UnescapeString(tagRE.ReplaceAllString(m[1], " ")))
			}
			text := spaceRE.ReplaceAllString(strings.Join(strings.Fields(html.UnescapeString(tagRE.ReplaceAllString(line, " "))), " "), "$1")
			if cur == nil || text == "" || (!phoneRE.MatchString(text) && !webRE.MatchString(text)) {
				continue
			}
			cur.items = append(cur.items, text)
		}
		file.Close()
		if err := sc.Err(); err != nil {
			return "", err
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Before the fall reprint, check every phone number and web address in the workbook and the supply list (%s edition). Call or open each one, and check that the words around it still hold: names, hours, rules, deadlines. Fix what changed in `templates/`, and tick each box as you go.\n\n", edition)
	b.WriteString("- [ ] `EDITION`: the new edition's month and year\n")
	b.WriteString("- [ ] The monthly link check has been passing (Actions → checks)\n")
	b.WriteString("- [ ] Chromium in the `Dockerfile`: move `SNAPSHOT` and `CHROMIUM` to a recent snapshot, and check the visual diff on that pull request\n\n")
	for _, s := range sections {
		if len(s.items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", s.title)
		for _, it := range s.items {
			if len(it) > 400 {
				it = it[:400] + "…"
			}
			fmt.Fprintf(&b, "- [ ] %s\n", it)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

// runCheck runs the check a flag asked for, if any, and reports whether one ran.
func runCheck(diffOld, diffOut, outDir, tplDir, edition string, links, facts bool) (bool, error) {
	switch {
	case diffOld != "":
		summary, err := visualDiff(diffOld, outDir, diffOut)
		if err == nil {
			fmt.Print(summary)
		}
		return true, err
	case links:
		return true, checkLinks(tplDir)
	case facts:
		s, err := factsChecklist(tplDir, edition)
		if err == nil {
			fmt.Print(s)
		}
		return true, err
	}
	return false, nil
}
