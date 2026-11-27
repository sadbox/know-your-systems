package main

// The static website: one landing page with the home-print PDFs, and the print kit: every print
// file, the paper we use and how we print and bind it, for anyone making copies for their town. Nothing is filled in online; the workbook has to work with the power and the
// internet out. Text shared with the booklet (the intro, the three questions, how to use it, the
// contents, the Vermont numbers, VT-Alert, the credit) comes from the same templates, so the site
// never drifts from the print.
//
//	<site>/index.html           landing page
//	<site>/downloads/*.pdf      the two home-print PDFs
//	<site>/<kitPath>/           the print kit: every print file, with how to print and bind it
//	<site>/<oldKitPath>/        a page that sends the kit's old, unlisted address to the new one
//	<site>/assets/...           stylesheet, fonts, artwork, favicon, share image

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	kitPath    = "print-kit"
	oldKitPath = "kit-8db13109" // where the kit was while it was unlisted; the team has the link
	repoURL    = "https://github.com/sadbox/know-your-systems"
)

type download struct {
	File  string // path relative to the page that links it
	Size  string // "1.2 MB"
	Pages int    // pages, or printed sides for a sheet laid out as spreads
}

type siteData struct {
	Root     string // path from the page to the site root: "" or "../"
	Edition  string
	URL      string // absolute site URL with trailing slash, for share-card tags; empty leaves them out
	Commit   string // the source's commit, on the print-kit page
	KitPath  string // the print kit's directory
	Repo     string // the source's GitHub address
	RepoName string // the same, as printed: github.com/sadbox/know-your-systems
	Built    string // the build's date (UTC)
	Workbook download
	Supply   download
	Kit      map[string]download // the print kit's files, by role
}

func (b *builder) writeSite(dir, outDir, name, siteURL, commit string, sides map[string]int) error {
	for _, d := range []string{"", "assets", "assets/fonts", "downloads", kitPath, oldKitPath} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return err
		}
	}
	// Fonts and their licences, artwork, and the stylesheet are copied as they are.
	fonts, err := filepath.Glob(filepath.Join(b.assets, "fonts", "*"))
	if err != nil {
		return err
	}
	copies := map[string]string{
		filepath.Join(b.tplDir, "site.css"):                "assets/site.css",
		filepath.Join(b.assets, "groundhog-cover.png"):     "assets/groundhog-cover.png",
		filepath.Join(b.assets, "groundhog-spot.png"):      "assets/groundhog-spot.png",
		filepath.Join(b.assets, "site", "favicon.png"):     "assets/favicon.png",
		filepath.Join(b.assets, "site", "apple-touch.png"): "assets/apple-touch.png",
	}
	for _, f := range fonts {
		copies[f] = filepath.Join("assets/fonts", filepath.Base(f))
	}
	// pdf copies one PDF from outDir into the site under sub, and describes it for a page in dir
	// (rel is that page's path back to the site root).
	pdf := func(file, sub, rel string) (download, error) {
		src := filepath.Join(outDir, file)
		st, err := os.Stat(src)
		if err != nil {
			return download{}, err
		}
		copies[src] = filepath.Join(sub, file)
		return download{File: rel + sub + "/" + file, Size: humanSize(st.Size()), Pages: sides[file]}, nil
	}
	data := siteData{
		Edition: b.edition, URL: siteURL, Commit: commit, Built: time.Now().UTC().Format("2006-01-02"),
		KitPath: kitPath, Repo: repoURL, RepoName: strings.TrimPrefix(repoURL, "https://"), Kit: map[string]download{},
	}
	if data.Workbook, err = pdf(name+"-workbook-letter.pdf", "downloads", ""); err != nil {
		return err
	}
	if data.Supply, err = pdf(name+"-supply-list-letter.pdf", "downloads", ""); err != nil {
		return err
	}
	for role, file := range map[string]string{
		"proof": "proof", "cover": "cover", "interior": "interior", "laminated": "supply-list",
	} {
		if data.Kit[role], err = pdf(name+"-"+file+".pdf", kitPath, ""); err != nil {
			return err
		}
	}
	for src, dst := range copies {
		if err := copyFile(src, filepath.Join(dir, dst)); err != nil {
			return err
		}
	}
	if err := b.render("site-index", data, filepath.Join(dir, "index.html")); err != nil {
		return err
	}
	// The kit page sits one level down: its links to the site and to every file go through "../".
	kit := data
	kit.Root = "../"
	kit.Workbook.File = "../" + data.Workbook.File
	kit.Supply.File = "../" + data.Supply.File
	kit.Kit = map[string]download{}
	for role, d := range data.Kit {
		d.File = strings.TrimPrefix(d.File, kitPath+"/")
		kit.Kit[role] = d
	}
	if err := b.render("site-kit", kit, filepath.Join(dir, kitPath, "index.html")); err != nil {
		return err
	}
	return b.render("site-kit-moved", kitPath, filepath.Join(dir, oldKitPath, "index.html"))
}

func (b *builder) render(tpl string, data any, path string) error {
	var buf bytes.Buffer
	if err := b.tpl.ExecuteTemplate(&buf, tpl, data); err != nil {
		return fmt.Errorf("%s: %w", tpl, err)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func humanSize(n int64) string {
	if n < 1<<20 {
		return fmt.Sprintf("%d KB", (n+512)>>10)
	}
	return strings.Replace(fmt.Sprintf("%.1f MB", float64(n)/(1<<20)), ".0 ", " ", 1)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// shareImage renders the 1200 x 630 picture that Facebook, Front Porch Forum and text messages show
// when someone shares a link to the site.
func (b *builder) shareImage(chrome, dir string) error {
	fonts, err := b.fontFaces()
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<!doctype html><html><head><meta charset=\"utf-8\"><style>%s</style></head><body>", fonts)
	if err := b.tpl.ExecuteTemplate(&buf, "site-share", nil); err != nil {
		return err
	}
	buf.WriteString("</body></html>")
	tmp, err := os.CreateTemp("", "kys-share-*.html")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		return err
	}
	tmp.Close()
	png, _ := filepath.Abs(filepath.Join(dir, "assets", "share.png"))
	_, err = runChrome(chrome, "--screenshot="+png, "--window-size=1200,630", "--default-background-color=00000000", "file://"+tmp.Name())
	return err
}

// kitCard is one file on the print-kit page (templates/site.html, "kit-card").
type kitCard struct {
	T    string        // title
	D    download      // the file
	Unit string        // "pages" or "sides"
	Body template.HTML // how to print it: a literal written in the template
}

func kitCardOf(title string, d download, unit, body string) kitCard {
	return kitCard{T: title, D: d, Unit: unit, Body: template.HTML(body)} //nolint:gosec // body is a literal in templates/site.html, never outside input
}
