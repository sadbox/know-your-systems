package main

// The static website: one landing page with the home-print PDFs. Nothing is filled in online;
// the workbook has to work with the power and the internet out. Text shared with the booklet
// (the intro, the three questions, how to use it, the contents, the Vermont numbers, VT-Alert,
// the credit) comes from the same templates, so the site never drifts from the print.
//
//	<site>/index.html           landing page
//	<site>/downloads/*.pdf      the two home-print PDFs
//	<site>/assets/...           stylesheet, fonts, artwork, favicon, share image
//
// The team's print files (booklet spreads, cover, laminated list) are not published.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type download struct {
	File  string // path relative to the site root
	Size  string // "1.2 MB"
	Pages int
}

type siteData struct {
	Edition  string
	URL      string // absolute site URL with trailing slash, for share-card tags; empty leaves them out
	Workbook download
	Supply   download
}

func (b *builder) writeSite(dir, outDir, name, siteURL string, workbookPages, supplyPages int) error {
	for _, d := range []string{"", "assets", "assets/fonts", "downloads"} {
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
	data := siteData{Edition: b.edition, URL: siteURL}
	for _, p := range []struct {
		dst   *download
		file  string
		pages int
	}{
		{&data.Workbook, name + "-workbook-letter.pdf", workbookPages},
		{&data.Supply, name + "-supply-list-letter.pdf", supplyPages},
	} {
		src := filepath.Join(outDir, p.file)
		st, err := os.Stat(src)
		if err != nil {
			return err
		}
		*p.dst = download{File: "downloads/" + p.file, Size: humanSize(st.Size()), Pages: p.pages}
		copies[src] = p.dst.File
	}
	for src, dst := range copies {
		if err := copyFile(src, filepath.Join(dir, dst)); err != nil {
			return err
		}
	}
	for tpl, file := range map[string]string{"site-index": "index.html"} {
		var buf bytes.Buffer
		if err := b.tpl.ExecuteTemplate(&buf, tpl, data); err != nil {
			return fmt.Errorf("%s: %w", tpl, err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
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
