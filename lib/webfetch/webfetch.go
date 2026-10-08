// Package webfetch downloads a news page and pulls out the article, for the
// admin's "write a story from this link" flow.
//
// The URL comes from an admin, but it is still a server-side request to an
// arbitrary address, so the client refuses anything that resolves to a private,
// loopback or link-local address — including after redirects and DNS tricks,
// because the check runs on the IP actually being dialled.
package webfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

var (
	ErrInvalidURL   = errors.New("webfetch: invalid url")
	ErrBlockedHost  = errors.New("webfetch: address not allowed")
	ErrTooLarge     = errors.New("webfetch: response too large")
	ErrNotHTML      = errors.New("webfetch: not an html page")
	ErrNotImage     = errors.New("webfetch: not an image")
	ErrNoArticle    = errors.New("webfetch: no article content found")
	ErrUpstreamCode = errors.New("webfetch: unexpected status")
)

const (
	maxPageBytes = 5 << 20
	// Enough for a long article. The AI does not need more, and every extra
	// character is paid for on each generation.
	maxTextRunes = 24000
	// Photos inside the article, beyond the cover. More than this is a
	// gallery page, and every one is downloaded and stored.
	maxArticleImages = 10
	userAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"
)

// Page is what the article extraction found.
type Page struct {
	URL      string
	Title    string
	SiteName string
	Excerpt  string
	Text     string
	// The page's share image (og:image). Often the main photo, sometimes just
	// the site's logo.
	ImageURL string
	// Images inside the article body, in reading order, without ImageURL.
	Images      []Image
	PublishedAt *time.Time
}

type Image struct {
	URL string
	Alt string
}

type Fetcher struct {
	client *http.Client
}

// New returns a fetcher that only connects to public addresses.
func New() *Fetcher {
	return newFetcher(false)
}

// allowPrivate exists for tests, which serve pages from 127.0.0.1.
func newFetcher(allowPrivate bool) *Fetcher {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			if allowPrivate {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return ErrBlockedHost
			}
			if !isPublicIP(net.ParseIP(host)) {
				return ErrBlockedHost
			}
			return nil
		},
	}
	transport := &http.Transport{
		// No proxy: the dial check must see the real destination.
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	}
	return &Fetcher{client: &http.Client{
		Transport: transport,
		Timeout:   25 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("webfetch: too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrInvalidURL
			}
			return nil
		},
	}}
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		// 0.0.0.0/8, carrier-grade NAT 100.64.0.0/10, benchmarking 198.18.0.0/15
		// and the reserved 240.0.0.0/4 are not covered by the checks above.
		switch {
		case v4[0] == 0,
			v4[0] == 100 && v4[1]&0xC0 == 64,
			v4[0] == 198 && (v4[1] == 18 || v4[1] == 19),
			v4[0] >= 240:
			return false
		}
	}
	return true
}

// ParseURL accepts only absolute http(s) URLs.
func ParseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, ErrInvalidURL
	}
	return u, nil
}

func (f *Fetcher) get(ctx context.Context, rawURL, accept string) (*http.Response, error) {
	u, err := ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrInvalidURL
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "ar,en;q=0.8")
	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlockedHost) {
			return nil, ErrBlockedHost
		}
		return nil, fmt.Errorf("webfetch: request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()
		return nil, fmt.Errorf("%w: %d", ErrUpstreamCode, resp.StatusCode)
	}
	return resp, nil
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrTooLarge
	}
	return data, nil
}

// FetchArticle downloads the page and extracts the main article.
func (f *Fetcher) FetchArticle(ctx context.Context, rawURL string) (*Page, error) {
	resp, err := f.get(ctx, rawURL, "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "html") {
		return nil, ErrNotHTML
	}
	raw, err := readLimited(resp.Body, maxPageBytes)
	if err != nil {
		return nil, err
	}
	// Some Arabic sites still serve windows-1256; readability expects UTF-8.
	utf8 := raw
	if r, err := charset.NewReader(bytes.NewReader(raw), contentType); err == nil {
		if converted, err := io.ReadAll(r); err == nil {
			utf8 = converted
		}
	}

	// Relative links and images resolve against where the page actually ended
	// up, not the link the admin pasted.
	finalURL := resp.Request.URL
	article, err := readability.FromReader(bytes.NewReader(utf8), finalURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoArticle, err)
	}

	var text strings.Builder
	if article.Node != nil {
		if err := article.RenderText(&text); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNoArticle, err)
		}
	}
	body := truncateRunes(normaliseText(text.String()), maxTextRunes)
	if body == "" {
		return nil, ErrNoArticle
	}

	page := &Page{
		URL:      finalURL.String(),
		Title:    strings.TrimSpace(article.Title()),
		SiteName: strings.TrimSpace(article.SiteName()),
		Excerpt:  strings.TrimSpace(article.Excerpt()),
		Text:     body,
	}
	if page.SiteName == "" {
		page.SiteName = strings.TrimPrefix(finalURL.Hostname(), "www.")
	}

	// The share image is usually the story's photo, but some sites point it
	// at their logo. The hero photo above the text is often outside what
	// readability keeps, so it is looked for in the whole page.
	share := absoluteURL(finalURL, article.ImageURL())
	if looksLikeLogo(share) {
		share = ""
	}
	var lead string
	if doc, err := html.Parse(bytes.NewReader(utf8)); err == nil {
		lead = leadImage(doc, finalURL, page.Title)
	}
	page.ImageURL = share
	if share == "" {
		page.ImageURL = lead
	}
	var images []Image
	if lead != "" && lead != page.ImageURL {
		images = append(images, Image{URL: lead, Alt: page.Title})
	}
	if article.Node != nil {
		for _, img := range collectImages(article.Node, finalURL, page.ImageURL) {
			if img.URL != lead && len(images) < maxArticleImages {
				images = append(images, img)
			}
		}
	}
	page.Images = images
	if t, err := article.PublishedTime(); err == nil && !t.IsZero() {
		page.PublishedAt = &t
	}
	return page, nil
}

// FetchImage downloads an image, refusing anything that is not one.
func (f *Fetcher) FetchImage(ctx context.Context, rawURL string, maxBytes int64) ([]byte, string, error) {
	resp, err := f.get(ctx, rawURL, "image/avif,image/webp,image/png,image/jpeg,image/*;q=0.8")
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := readLimited(resp.Body, maxBytes)
	if err != nil {
		return nil, "", err
	}
	// Sniff rather than trust the header: plenty of CDNs label everything
	// application/octet-stream.
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", ErrNotImage
	}
	return data, contentType, nil
}

// collectImages lists the article's own images. Readability has already
// dropped the page chrome and resolved lazy-loading attributes into src, so
// what is left is mostly the story's photos; the size check after download
// catches the icons that slip through.
func collectImages(root *html.Node, base *url.URL, cover string) []Image {
	seen := map[string]bool{cover: true}
	var out []Image
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if len(out) >= maxArticleImages {
			return
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			src := attrs["src"]
			if src == "" || strings.HasPrefix(src, "data:") {
				src = firstNonEmpty(attrs["data-src"], attrs["data-lazy-src"], firstSrcset(attrs["srcset"]))
			}
			if u := absoluteURL(base, src); u != "" && !seen[u] && !tinyByAttrs(attrs) {
				seen[u] = true
				out = append(out, Image{URL: u, Alt: strings.TrimSpace(attrs["alt"])})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

// Share images that are the site's branding rather than the story.
var logoHints = []string{"favicon", "logo", "android-chrome", "apple-touch", "/icon", "-icon", "_icon", "placeholder", "default-share", "og-default"}

func looksLikeLogo(u string) bool {
	lower := strings.ToLower(u)
	for _, hint := range logoHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// leadImage finds the story's hero photo anywhere in the page: an image
// marked itemprop="image", or one whose alt text is the headline — the way
// most news templates label the photo above the article.
func leadImage(doc *html.Node, base *url.URL, title string) string {
	want := normaliseCaption(title)
	var found string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			alt := normaliseCaption(attrs["alt"])
			matches := attrs["itemprop"] == "image" ||
				(want != "" && alt != "" && (alt == want || (len([]rune(alt)) >= 20 && strings.Contains(want, alt))))
			if matches {
				src := attrs["src"]
				if src == "" || strings.HasPrefix(src, "data:") {
					src = firstNonEmpty(attrs["data-src"], attrs["data-lazy-src"], firstSrcset(attrs["srcset"]))
				}
				if u := absoluteURL(base, src); u != "" && !looksLikeLogo(u) {
					found = u
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

// normaliseCaption makes "the UAE’s" and "the UAE's" compare equal.
func normaliseCaption(s string) string {
	s = strings.NewReplacer("’", "'", "‘", "'", "“", `"`, "”", `"`).Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

// tinyByAttrs skips tracking pixels and icons that declare their size.
func tinyByAttrs(attrs map[string]string) bool {
	for _, k := range []string{"width", "height"} {
		if v := strings.TrimSuffix(attrs[k], "px"); v != "" {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 && n < 50 {
				return true
			}
		}
	}
	return false
}

func firstSrcset(srcset string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(srcset), ",")
	url, _, _ := strings.Cut(strings.TrimSpace(first), " ")
	return url
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func absoluteURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := base.Parse(ref)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.String()
}

// normaliseText collapses the blank lines readability leaves between blocks,
// keeping paragraph breaks.
func normaliseText(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
			}
			blank = true
			continue
		}
		out = append(out, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
