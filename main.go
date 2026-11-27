// Command kys builds the Emergency Preparedness Workbook ("Know Your Systems") booklet PDFs.
//
// It renders the page templates in ./templates to HTML, prints them with
// headless Chrome, and lays the pages out as printer spreads for a
// half-letter, saddle-stitched booklet:
//
//	out/know-your-systems-cover.pdf     1 letter sheet, 2 sides (one ink, for light coloured stock)
//	out/know-your-systems-interior.pdf  N/4 letter sheets, 2 sides each (duplex, flip on short edge)
//	out/know-your-systems-proof.pdf     every page in reading order, for review on screen
//	out/know-your-systems-supply-list.pdf   the letter-size two-week supply list: two sheets, 4 sides
//	                                        (duplex, flip on long edge), for laminating
//
// and the home-print editions for the website, which anyone can print on a desk printer:
//
//	out/know-your-systems-workbook-letter.pdf     every booklet page in reading order, enlarged to fill a
//	                                              portrait letter page; no folding, no stapling
//	out/know-your-systems-supply-list-letter.pdf  the supply list worded for pencil on plain paper
//
// With -site it also writes the static website.
//
// Only the Go standard library is used. Chrome or Chromium must be installed.
package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"html"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// page is one finished 5.5 x 8.5 in booklet page, or one side of a letter-size laminated sheet.
type page struct {
	ID    string // template name
	Num   int    // interior page number; 0 for cover pages and sheet sides
	Cover bool
	Sheet bool
	Left  bool // sits on the left of a spread (even pages; gutter on the right)
}

// doc is one PDF to build.
type doc struct {
	file, pageSize string
	single         bool     // one page per PDF page; otherwise two side by side on a letter sheet
	tint           bool     // print the cover stock colour behind the cover pages
	check          bool     // measure this one: fail the build if any page's content does not fit
	zoom           bool     // each half-letter page enlarged onto its own portrait letter page
	sheets         bool     // sheets mode (see pageData.Sheets)
	sides          [][]page // one entry per printed side (or per page, when single)
}

type pageData struct {
	Page    page
	Edition string
	// Sheets is the mode of the home-print editions on the website: loose letter pages from a desk
	// printer, written on in pencil. Without it (workbook mode) the text is for the printed kit: a
	// folded, stapled booklet with a cover, and a laminated supply list for a wet-erase marker.
	// Templates choose wording with {{if .Sheets}}...{{else}}...{{end}}.
	Sheets bool
}

type builder struct {
	tplDir  string
	tpl     *template.Template
	css     string
	assets  string
	edition string
	pgnum   map[string]int // interior page numbers by template name, for cross references
	ccBySA  bool           // print the CC BY-SA 4.0 notice (-cc-by-sa)
}

func main() {
	var (
		tplDir  = flag.String("templates", "templates", "directory holding the page templates and style.css")
		assets  = flag.String("assets", "assets", "directory holding fonts and artwork")
		outDir  = flag.String("out", "out", "output directory")
		name    = flag.String("name", "know-your-systems", "base name for the PDF files")
		edition = flag.String("edition", defaultEdition(), "edition line printed in the back-cover credit (default: the EDITION file, else this month)")
		chrome  = flag.String("chrome", "", "path to Chrome or Chromium (default: $CHROME, then search PATH)")
		keep    = flag.Bool("keep-html", false, "keep the intermediate HTML files next to the PDFs")
		lenient = flag.Bool("allow-overflow", false, "build even if a page's content is too tall (it will be cut off); for use while editing")
		siteDir = flag.String("site", "", "also write the static website to this directory (e.g. -site site)")
		commit  = flag.String("commit", "", "the source's commit, shown on the print-kit page (./build passes it)")
		diffOld = flag.String("diff", "", "check: compare every PDF in this directory with the same one in -out, page by page, instead of building")
		diffOut = flag.String("diff-out", "diff", "with -diff: where the pictures of changed pages go")
		links   = flag.Bool("links", false, "check: fetch every web address in the print instead of building")
		facts   = flag.Bool("facts", false, "check: print the checklist of every phone number and web address in the print instead of building")
		ccBySA  = flag.Bool("cc-by-sa", false, "print the CC BY-SA 4.0 notice on the back cover and the website, and link the website to the source (off until every author and organization agrees)")
		siteURL = flag.String("site-url", "https://knowyoursystems.org/", "the website's public address, e.g. https://example.org/ (for link previews on Facebook and the like)")
	)
	flag.Parse()
	log.SetFlags(0)

	if ran, err := runCheck(*diffOld, *diffOut, *outDir, *tplDir, *edition, *links, *facts); ran {
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	bin, err := findChrome(*chrome)
	if err != nil {
		log.Fatal(err)
	}
	b := &builder{tplDir: *tplDir, assets: *assets, edition: *edition, ccBySA: *ccBySA, pgnum: map[string]int{}}
	if err := b.load(*tplDir); err != nil {
		log.Fatal(err)
	}

	interior, err := b.interiorPages()
	if err != nil {
		log.Fatal(err)
	}
	c1 := page{ID: "c1", Cover: true}
	c2 := page{ID: "c2", Cover: true, Left: true}
	c3 := page{ID: "c3", Cover: true}
	c4 := page{ID: "c4", Cover: true, Left: true}

	// Reading order, for the proof.
	reading := append([]page{c1, c2}, interior...)
	reading = append(reading, c3, c4)

	coverSides := [][]page{{c4, c1}, {c2, c3}}
	// The laminated supply list: two sheets, four sides. A side's ID is "supply-<side>"; the part
	// before the dash is also its CSS class, so every side is styled by .supply.
	supply := []page{{ID: "supply-a", Sheet: true}, {ID: "supply-b", Sheet: true}, {ID: "supply-c", Sheet: true}, {ID: "supply-d", Sheet: true}}
	const letter, half, upright = "11in 8.5in", "5.5in 8.5in", "8.5in 11in"
	// The two documents that hold each page exactly once come first: they carry the fit check.
	docs := []doc{
		{file: *name + "-proof", pageSize: half, single: true, tint: true, check: true, sides: singles(reading)},
		{file: *name + "-supply-list", pageSize: upright, single: true, check: true, sides: singles(supply)},
		{file: *name + "-cover", pageSize: letter, sides: coverSides},
		{file: *name + "-interior", pageSize: letter, sides: impose(interior)},
		// Home-print editions, for the website. The workbook is the proof's pages enlarged, so it
		// needs no fit check of its own beyond the wording; the supply list is re-measured because
		// its wording differs.
		{file: *name + "-workbook-letter", pageSize: upright, single: true, zoom: true, sheets: true, check: true, sides: singles(reading)},
		{file: *name + "-supply-list-letter", pageSize: upright, single: true, sheets: true, check: true, sides: singles(supply)},
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	sides := map[string]int{} // PDF file name to its pages (single) or printed sides
	for _, d := range docs {
		sides[d.file+".pdf"] = len(d.sides)
		html, err := b.document(d)
		if err != nil {
			log.Fatalf("%s: %v", d.file, err)
		}
		htmlPath, _ := filepath.Abs(filepath.Join(*outDir, d.file+".html"))
		pdfPath, _ := filepath.Abs(filepath.Join(*outDir, d.file+".pdf"))
		if err := os.WriteFile(htmlPath, html, 0o644); err != nil {
			log.Fatal(err)
		}
		if d.check {
			if err := checkFit(bin, htmlPath); err != nil {
				if !*lenient {
					log.Fatal(err)
				}
				log.Printf("warning: %v", err)
			}
		}
		if err := printPDF(bin, htmlPath, pdfPath); err != nil {
			log.Fatalf("%s: %v", d.file, err)
		}
		if !*keep {
			os.Remove(htmlPath)
		}
		log.Printf("wrote %s (%d %s)", pdfPath, len(d.sides), map[bool]string{true: "pages", false: "sides"}[d.single])
	}

	if *siteDir != "" {
		u := *siteURL
		if u != "" && !strings.HasSuffix(u, "/") {
			u += "/"
		}
		if err := b.writeSite(*siteDir, *outDir, *name, u, *commit, sides); err != nil {
			log.Fatalf("site: %v", err)
		}
		if err := b.shareImage(bin, *siteDir); err != nil {
			log.Fatalf("site: %v", err)
		}
		log.Printf("wrote site to %s", *siteDir)
	}
}

// load parses the templates and stylesheet.
func (b *builder) load(dir string) error {
	css, err := os.ReadFile(filepath.Join(dir, "style.css"))
	if err != nil {
		return err
	}
	b.css = string(css)
	funcs := template.FuncMap{
		"img":     b.img,
		"dotgrid": dotgrid,
		"kit":     kitCardOf,
		"ccbysa":  func() bool { return b.ccBySA },
		// commitURL: the commit can be linked on GitHub (a clean commit, not "abc1234-dirty").
		"commitURL": func(c string) bool { return c != "" && !strings.HasSuffix(c, "-dirty") },
		"pg": func(id string) (int, error) {
			n, ok := b.pgnum[id]
			if !ok {
				return 0, fmt.Errorf("pg: no interior page named %q", id)
			}
			return n, nil
		},
	}
	b.tpl, err = template.New("booklet").Funcs(funcs).ParseGlob(filepath.Join(dir, "*.html"))
	return err
}

// interiorPages reads the page order from the "interior-order" template.
func (b *builder) interiorPages() ([]page, error) {
	var buf bytes.Buffer
	if err := b.tpl.ExecuteTemplate(&buf, "interior-order", nil); err != nil {
		return nil, err
	}
	ids := strings.Fields(buf.String())
	if len(ids) == 0 || len(ids)%4 != 0 {
		return nil, fmt.Errorf("interior has %d pages; a folded booklet needs a multiple of 4", len(ids))
	}
	pages := make([]page, len(ids))
	for i, id := range ids {
		if b.tpl.Lookup(id) == nil {
			return nil, fmt.Errorf("interior-order names %q but no such template is defined", id)
		}
		pages[i] = page{ID: id, Num: i + 1, Left: (i+1)%2 == 0}
		b.pgnum[id] = i + 1
	}
	return pages, nil
}

// impose arranges reading-order pages into printer spreads for saddle stitching.
// Sheet i carries pages [N-2i | 2i+1] on its front and [2i+2 | N-2i-1] on its back.
func impose(p []page) [][]page {
	n := len(p)
	var sides [][]page
	for i := 0; i < n/4; i++ {
		sides = append(sides,
			[]page{p[n-2*i-1], p[2*i]},
			[]page{p[2*i+1], p[n-2*i-2]},
		)
	}
	return sides
}

func singles(p []page) [][]page {
	out := make([][]page, len(p))
	for i := range p {
		out[i] = []page{p[i]}
	}
	return out
}

// document assembles one complete, self-contained HTML file.
func (b *builder) document(d doc) ([]byte, error) {
	fonts, err := b.fontFaces()
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><title>Emergency Preparedness Workbook</title>\n<style>\n@page{size:%s;margin:0}\n%s\n%s</style></head>\n", d.pageSize, fonts, b.css)
	var classes []string
	switch {
	case d.zoom:
		classes = append(classes, "zoom")
	case d.single:
		classes = append(classes, "single")
	}
	if d.tint {
		classes = append(classes, "tint")
	}
	fmt.Fprintf(&out, "<body class=\"%s\">\n", strings.Join(classes, " "))
	for _, side := range d.sides {
		wrap := "sheet"
		if d.zoom {
			wrap = "lp"
		}
		if !d.single || d.zoom {
			fmt.Fprintf(&out, "<div class=\"%s\">\n", wrap)
		}
		for _, p := range side {
			if err := b.page(&out, p, d.sheets); err != nil {
				return nil, fmt.Errorf("page %s: %w", p.ID, err)
			}
		}
		if !d.single || d.zoom {
			out.WriteString("</div>\n")
		}
	}
	if d.check {
		out.WriteString(fitScript)
	}
	out.WriteString("</body></html>\n")
	return out.Bytes(), nil
}

func (b *builder) page(out *bytes.Buffer, p page, sheets bool) error {
	class := "pg"
	if p.Left {
		class += " left"
	}
	if p.Cover {
		class += " cv " + p.ID
	}
	if p.Sheet {
		class += " ls " + strings.SplitN(p.ID, "-", 2)[0] + " " + p.ID
	}
	fmt.Fprintf(out, "<section class=\"%s\" data-id=\"%s\"><div class=\"body\">", class, p.ID)
	if err := b.tpl.ExecuteTemplate(out, p.ID, pageData{Page: p, Edition: b.edition, Sheets: sheets}); err != nil {
		return err
	}
	out.WriteString("</div>")
	if !p.Cover && !p.Sheet {
		fmt.Fprintf(out, "<footer class=\"ft\"><span>Emergency preparedness workbook</span><span class=\"pn\">%d</span></footer>", p.Num)
	}
	out.WriteString("</section>\n")
	return nil
}

// img returns a PNG from the assets directory as a data URI, so the HTML stays self-contained.
func (b *builder) img(name string) (template.URL, error) {
	data, err := os.ReadFile(filepath.Join(b.assets, name))
	if err != nil {
		return "", err
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(data)), nil
}

// dotgrid draws a cols x rows grid of dots, pitch units apart, as plain SVG circles.
func dotgrid(cols, rows, pitch int) template.HTML {
	var sb strings.Builder
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			fmt.Fprintf(&sb, `<circle cx="%d" cy="%d" r=".8"/>`, c*pitch+pitch/2, r*pitch+pitch/2)
		}
	}
	return template.HTML(sb.String())
}

// fontFaces embeds the bundled fonts so the HTML has no outside dependencies.
func (b *builder) fontFaces() (string, error) {
	faces := []struct{ family, weight, style, file string }{
		{"Atkinson Hyperlegible", "400", "normal", "atkinson-hyperlegible-latin-400-normal.woff2"},
		{"Atkinson Hyperlegible", "400", "italic", "atkinson-hyperlegible-latin-400-italic.woff2"},
		{"Atkinson Hyperlegible", "700", "normal", "atkinson-hyperlegible-latin-700-normal.woff2"},
		{"Atkinson Hyperlegible", "700", "italic", "atkinson-hyperlegible-latin-700-italic.woff2"},
		{"Barlow Condensed", "500", "normal", "barlow-condensed-latin-500-normal.woff2"},
		{"Barlow Condensed", "600", "normal", "barlow-condensed-latin-600-normal.woff2"},
		{"Barlow Condensed", "700", "normal", "barlow-condensed-latin-700-normal.woff2"},
		{"Barlow Condensed", "800", "normal", "barlow-condensed-latin-800-normal.woff2"},
	}
	var sb strings.Builder
	for _, f := range faces {
		data, err := os.ReadFile(filepath.Join(b.assets, "fonts", f.file))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&sb, "@font-face{font-family:%q;font-weight:%s;font-style:%s;src:url(data:font/woff2;base64,%s) format(\"woff2\")}\n",
			f.family, f.weight, f.style, base64.StdEncoding.EncodeToString(data))
	}
	return sb.String(), nil
}

// fitScript measures every page after fonts load and leaves the result in the DOM,
// where checkFit reads it back through Chrome's --dump-dom.
const fitScript = `<script>
document.fonts.ready.then(function(){
  var out=[];
  document.querySelectorAll('.pg').forEach(function(p){
    var b=p.querySelector('.body'), used=0, z=p.currentCSSZoom||1;  // the home edition enlarges each page: boxes come back enlarged, styles and clientHeight do not
    p.classList.add('measure');  // rows that stretch to fill a sheet are measured at their natural height
    Array.prototype.forEach.call(b.children,function(c){
      if(c.classList.contains('grow'))return;
      var s=getComputedStyle(c);
      used+=c.getBoundingClientRect().height/z+parseFloat(s.marginTop)+parseFloat(s.marginBottom);
    });
    out.push(p.dataset.id+':'+Math.round(used)+':'+Math.round(b.clientHeight));
    p.classList.remove('measure');
  });
  var wide=[];
  document.querySelectorAll('.f,.o,.tr,.th,.bar,.big,.made,.title,.band,.calc,.lh,.toc span').forEach(function(el){
    var w=el.scrollWidth-el.clientWidth;
    if(w>1){wide.push(el.closest('.pg').dataset.id+'|'+Math.round(w)+'|'+el.textContent.trim().replace(/\s+/g,' ').slice(0,48));}
  });
  var pre=document.createElement('pre');pre.id='fit';pre.textContent='FIT '+out.join(' ')+' END';
  document.body.appendChild(pre);
  var pre2=document.createElement('pre');pre2.id='wide';pre2.textContent='WIDE '+wide.join(';;')+' END';
  document.body.appendChild(pre2);
});
</script>
`

var (
	fitRE  = regexp.MustCompile(`<pre id="fit">FIT (.*?) END</pre>`)
	wideRE = regexp.MustCompile(`(?s)<pre id="wide">WIDE (.*?) END</pre>`)
)

// checkFit fails the build if any page's content is taller than the page.
func checkFit(chrome, htmlPath string) error {
	out, err := runChrome(chrome, "--dump-dom", "file://"+htmlPath)
	if err != nil {
		return err
	}
	m := fitRE.FindSubmatch(out)
	if m == nil {
		return errors.New("fit check: no measurements came back from Chrome")
	}
	var over []string
	log.Printf("%-10s %8s", "page", "spare")
	for _, f := range strings.Fields(string(m[1])) {
		parts := strings.Split(f, ":")
		used, _ := strconv.Atoi(parts[1])
		room, _ := strconv.Atoi(parts[2])
		spare := float64(room-used) / 96 // CSS px to inches
		log.Printf("%-10s %6.2fin", parts[0], spare)
		if used > room {
			over = append(over, fmt.Sprintf("%s: too tall by %.2fin", parts[0], -spare))
		}
	}
	if w := wideRE.FindSubmatch(out); w != nil && len(bytes.TrimSpace(w[1])) > 0 {
		for _, row := range strings.Split(string(w[1]), ";;") {
			f := strings.SplitN(row, "|", 3)
			if len(f) == 3 {
				px, _ := strconv.Atoi(f[1])
				over = append(over, fmt.Sprintf("%s: a row runs %.2fin off the side (%q)", f[0], float64(px)/96, html.UnescapeString(f[2])))
			}
		}
	}
	if len(over) > 0 {
		return fmt.Errorf("content does not fit:\n  %s", strings.Join(over, "\n  "))
	}
	return nil
}

func printPDF(chrome, htmlPath, pdfPath string) error {
	_, err := runChrome(chrome, "--no-pdf-header-footer", "--print-to-pdf="+pdfPath, "file://"+htmlPath)
	if err != nil {
		return err
	}
	if st, err := os.Stat(pdfPath); err != nil || st.Size() == 0 {
		return fmt.Errorf("chrome did not produce %s", pdfPath)
	}
	return nil
}

func runChrome(chrome string, args ...string) ([]byte, error) {
	base := []string{"--headless=new", "--disable-gpu", "--hide-scrollbars", "--virtual-time-budget=4000"}
	// Chrome refuses to run sandboxed as root, and in the build container (Dockerfile) the
	// container is the sandbox.
	if os.Geteuid() == 0 || os.Getenv("KYS_CONTAINER") != "" {
		base = append(base, "--no-sandbox")
	}
	cmd := exec.Command(chrome, append(base, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("chrome: %w\n%s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func findChrome(explicit string) (string, error) {
	candidates := []string{explicit, os.Getenv("CHROME"), "google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium")
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", errors.New("could not find Chrome or Chromium; install one, or pass -chrome /path/to/chrome")
}

// defaultEdition is the EDITION file's line, so local builds and the website's build print the same
// credit; without the file, this month.
func defaultEdition() string {
	if b, err := os.ReadFile("EDITION"); err == nil {
		if e := strings.TrimSpace(string(b)); e != "" {
			return e
		}
	}
	return time.Now().Format("January 2006")
}
