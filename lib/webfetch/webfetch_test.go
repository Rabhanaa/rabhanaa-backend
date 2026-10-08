package webfetch

import (
	"bytes"
	"context"
	"errors"
	"image"
	pngenc "image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const articleHTML = `<!doctype html>
<html lang="ar" dir="rtl"><head>
<meta charset="utf-8">
<title>ارتفاع أسعار الدواجن - موقع الأخبار</title>
<meta property="og:title" content="ارتفاع أسعار الدواجن في الأسواق">
<meta property="og:site_name" content="موقع الأخبار">
<meta property="og:image" content="/images/cover.png">
<meta property="article:published_time" content="2026-10-01T09:30:00Z">
</head><body>
<nav><a href="/">الرئيسية</a> <a href="/sport">رياضة</a></nav>
<article>
<h1>ارتفاع أسعار الدواجن في الأسواق</h1>
<p>شهدت أسعار الدواجن في الأسواق المصرية ارتفاعًا ملحوظًا خلال الأسبوع الجاري، حيث سجل سعر الكيلو زيادة بنسبة عشرة في المائة مقارنة بالأسبوع الماضي، وفقًا لتجار الجملة في عدة محافظات.</p>
<p>وأرجع التجار هذا الارتفاع إلى زيادة تكلفة الأعلاف المستوردة وتراجع المعروض من المزارع الصغيرة، مؤكدين أن الأسعار قد تستقر خلال الأسابيع المقبلة مع دخول دورات إنتاج جديدة إلى السوق.</p>
<p>ويتوقع خبراء القطاع أن تتخذ الحكومة إجراءات لدعم صغار المربين، من بينها توفير الأعلاف بأسعار مخفضة وتسهيل الحصول على القروض، بهدف الحفاظ على استقرار السوق وحماية المستهلكين.</p>
</article>
<footer>جميع الحقوق محفوظة</footer>
</body></html>`

func TestFetchArticle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/old":
			http.Redirect(w, r, "/news/1", http.StatusMovedPermanently)
		case "/news/1":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(articleHTML))
		case "/file.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF-1.4"))
		case "/missing":
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	f := newFetcher(true)
	ctx := context.Background()

	page, err := f.FetchArticle(ctx, srv.URL+"/old")
	if err != nil {
		t.Fatalf("FetchArticle: %v", err)
	}
	if page.URL != srv.URL+"/news/1" {
		t.Errorf("URL = %q, want the post-redirect address", page.URL)
	}
	if !strings.Contains(page.Title, "الدواجن") {
		t.Errorf("Title = %q", page.Title)
	}
	if page.SiteName != "موقع الأخبار" {
		t.Errorf("SiteName = %q", page.SiteName)
	}
	if page.ImageURL != srv.URL+"/images/cover.png" {
		t.Errorf("ImageURL = %q, want it resolved against the page", page.ImageURL)
	}
	if !strings.Contains(page.Text, "الأعلاف المستوردة") || strings.Contains(page.Text, "رياضة") {
		t.Errorf("Text should hold the article and drop the nav:\n%s", page.Text)
	}
	if !strings.Contains(page.Text, "\n") {
		t.Error("Text lost its paragraph breaks")
	}
	if page.PublishedAt == nil || page.PublishedAt.Year() != 2026 {
		t.Errorf("PublishedAt = %v", page.PublishedAt)
	}

	if _, err := f.FetchArticle(ctx, srv.URL+"/file.pdf"); !errors.Is(err, ErrNotHTML) {
		t.Errorf("pdf: got %v, want ErrNotHTML", err)
	}
	if _, err := f.FetchArticle(ctx, srv.URL+"/missing"); !errors.Is(err, ErrUpstreamCode) {
		t.Errorf("404: got %v, want ErrUpstreamCode", err)
	}
	for _, bad := range []string{"", "ftp://x.com/a", "javascript:alert(1)", "/relative"} {
		if _, err := f.FetchArticle(ctx, bad); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("%q: got %v, want ErrInvalidURL", bad, err)
		}
	}
}

func TestBlocksPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(articleHTML))
	}))
	defer srv.Close()

	// The production fetcher must refuse the loopback test server.
	if _, err := New().FetchArticle(context.Background(), srv.URL); !errors.Is(err, ErrBlockedHost) {
		t.Fatalf("got %v, want ErrBlockedHost", err)
	}

	cases := map[string]bool{
		"8.8.8.8": true, "1.1.1.1": true, "2606:4700:4700::1111": true,
		"127.0.0.1": false, "10.0.0.5": false, "172.16.3.4": false, "192.168.1.1": false,
		"169.254.169.254": false, "100.64.0.1": false, "0.0.0.0": false, "::1": false,
		"fe80::1": false, "fd00::1": false, "198.18.0.1": false, "255.255.255.255": false,
	}
	for addr, want := range cases {
		if got := isPublicIP(net.ParseIP(addr)); got != want {
			t.Errorf("isPublicIP(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestFetchImage(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a.png":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(png)
		case "/page":
			w.Write([]byte("<html>not an image</html>"))
		case "/big.png":
			w.Write(append(png, make([]byte, 2048)...))
		}
	}))
	defer srv.Close()
	f := newFetcher(true)
	ctx := context.Background()

	data, ct, err := f.FetchImage(ctx, srv.URL+"/a.png", 1024)
	if err != nil || ct != "image/png" || len(data) != len(png) {
		t.Fatalf("FetchImage = %d bytes, %q, %v", len(data), ct, err)
	}
	if _, _, err := f.FetchImage(ctx, srv.URL+"/page", 1024); !errors.Is(err, ErrNotImage) {
		t.Errorf("html: got %v, want ErrNotImage", err)
	}
	if _, _, err := f.FetchImage(ctx, srv.URL+"/big.png", 1024); !errors.Is(err, ErrTooLarge) {
		t.Errorf("big: got %v, want ErrTooLarge", err)
	}
}

func TestCollectsArticleImages(t *testing.T) {
	page := `<!doctype html><html><head><meta charset="utf-8">
<meta property="og:image" content="https://cdn.example.com/cover.jpg">
<title>خبر</title></head><body>
<header><img src="/logo.png" alt="logo"></header>
<article>
<h1>ارتفاع أسعار الألبان</h1>
<p>شهدت أسعار الألبان في الأسواق ارتفاعًا ملحوظًا هذا الأسبوع بسبب زيادة تكلفة الأعلاف وتراجع الإنتاج في عدد من المزارع الكبرى في البلاد.</p>
<figure><img src="/photos/farm.jpg" alt="مزرعة ألبان"></figure>
<p>وقال تجار إن الأسعار قد تستمر في الارتفاع خلال الأسابيع المقبلة ما لم تتحسن الظروف، مؤكدين أن الطلب ما زال قويًا في المدن الكبرى رغم ارتفاع الأسعار.</p>
<img src="data:image/gif;base64,R0lGODlhAQABAAAAACw=" data-src="/photos/lazy.webp" alt="lazy">
<img src="https://cdn.example.com/cover.jpg" alt="duplicate of cover">
<img src="/pixel.gif" width="1" height="1">
<img src="/photos/farm.jpg" alt="duplicate">
<p>ويتوقع الخبراء أن تتدخل الجهات المعنية لضبط الأسواق وتوفير الأعلاف بأسعار مناسبة للمربين خلال الفترة القادمة لضمان استقرار الإمدادات.</p>
</article></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(page))
	}))
	defer srv.Close()

	got, err := newFetcher(true).FetchArticle(context.Background(), srv.URL+"/story")
	if err != nil {
		t.Fatal(err)
	}
	var urls []string
	for _, img := range got.Images {
		urls = append(urls, img.URL)
	}
	want := []string{srv.URL + "/photos/farm.jpg", srv.URL + "/photos/lazy.webp"}
	if strings.Join(urls, "|") != strings.Join(want, "|") {
		t.Fatalf("images = %v\nwant %v (cover, logo, pixel and duplicates excluded)", urls, want)
	}
	if got.Images[0].Alt != "مزرعة ألبان" {
		t.Errorf("alt = %q", got.Images[0].Alt)
	}
}

func TestImageSize(t *testing.T) {
	// Minimal headers for each WebP layout, plus a real PNG.
	vp8x := append([]byte("RIFF\x00\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x00\x00\x00\x00"), 0x1f, 0x03, 0x00, 0x57, 0x02, 0x00)  // 800x600
	vp8 := append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 \x00\x00\x00\x00\x00\x00\x00"), 0x9d, 0x01, 0x2a, 0x40, 0x01, 0xf0, 0x00) // 320x240
	var pngBuf bytes.Buffer
	if err := pngenc.Encode(&pngBuf, image.NewGray(image.Rect(0, 0, 1200, 630))); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, ct string
		data     []byte
		w, h     int
	}{
		{"webp extended", "image/webp", vp8x, 800, 600},
		{"webp lossy", "image/webp", vp8, 320, 240},
		{"png", "image/png", pngBuf.Bytes(), 1200, 630},
	}
	for _, c := range cases {
		w, h, ok := ImageSize(c.data, c.ct)
		if !ok || w != c.w || h != c.h {
			t.Errorf("%s: got %dx%d ok=%v, want %dx%d", c.name, w, h, ok, c.w, c.h)
		}
	}
	if _, _, ok := ImageSize([]byte("not an image"), "image/webp"); ok {
		t.Error("garbage reported a size")
	}
}

func TestLogoShareImageReplacedByHeroPhoto(t *testing.T) {
	const title = "Global Food Week Highlights the UAE’s Dairy Demand"
	page := `<!doctype html><html><head><meta charset="utf-8"><title>` + title + `</title>
<meta property="og:title" content="` + title + `">
<meta property="og:image" content="/favicon/android-chrome-512x512.png">
</head><body>
<div class="hero"><img src="/upload/hero.jpg" class="figure-img" alt="Global Food Week Highlights the UAE's Dairy Demand"></div>
<div class="content">
<p>Dubai hosted Global Food Week this month, drawing producers and buyers from across the region to discuss growing demand for dairy products in the Emirates.</p>
<p><img src="/upload/stand.jpg" alt="A dairy stand"></p>
<p>Exhibitors said demand for cheese, yoghurt and plant-based alternatives continues to rise, supported by population growth and a strong hospitality sector across the country.</p>
<p>Organisers expect next year's edition to be larger, with more international pavilions and a dedicated area for dairy technology and packaging innovation.</p>
</div>
<aside><img src="/upload/other-story.jpg" alt="An unrelated story"></aside>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(page))
	}))
	defer srv.Close()

	got, err := newFetcher(true).FetchArticle(context.Background(), srv.URL+"/news/1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ImageURL != srv.URL+"/upload/hero.jpg" {
		t.Errorf("cover = %q, want the hero photo instead of the favicon", got.ImageURL)
	}
	for _, img := range got.Images {
		if strings.Contains(img.URL, "hero.jpg") || strings.Contains(img.URL, "favicon") {
			t.Errorf("cover or logo repeated in images: %v", got.Images)
		}
	}
	if len(got.Images) == 0 || !strings.HasSuffix(got.Images[0].URL, "/upload/stand.jpg") {
		t.Errorf("images = %v, want the in-article photo", got.Images)
	}

	for u, want := range map[string]bool{
		"https://x.com/favicon/android-chrome-512x512.png": true,
		"https://x.com/static/logo.svg":                    true,
		"https://x.com/apple-touch-icon.png":               true,
		"https://x.com/upload/2026/10/cows.jpg":            false,
	} {
		if looksLikeLogo(u) != want {
			t.Errorf("looksLikeLogo(%s) = %v", u, !want)
		}
	}
}
