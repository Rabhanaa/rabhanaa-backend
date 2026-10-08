package service

import (
	"strings"
	"testing"
)

func TestSanitizeBodyKeepsArticleMarkup(t *testing.T) {
	in := `<h2>عنوان</h2><p style="text-align: center">فقرة <strong>مهمة</strong> و<em>مائلة</em></p>` +
		`<ul><li>بند</li></ul><blockquote>اقتباس</blockquote>` +
		`<figure><img src="https://storage.rabhanaa.com/a.png" alt="صورة" width="600"><figcaption>تعليق</figcaption></figure>` +
		`<table><tr><td>خلية</td></tr></table>` +
		`<p><a href="https://example.com/x">رابط</a></p>`
	out := sanitizeBody(in)
	for _, want := range []string{
		"<h2>عنوان</h2>", `style="text-align: center"`, "<strong>مهمة</strong>", "<em>مائلة</em>",
		"<li>بند</li>", "<blockquote>", `src="https://storage.rabhanaa.com/a.png"`, `alt="صورة"`,
		"<figcaption>", "<td>خلية</td>", `href="https://example.com/x"`, `rel="nofollow noopener"`, `target="_blank"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q in:\n%s", want, out)
		}
	}
}

func TestSanitizeBodyYouTube(t *testing.T) {
	in := `<div data-youtube-video=""><iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ?start=10" width="640" height="480" allowfullscreen="true"></iframe></div>`
	out := sanitizeBody(in)
	if !strings.Contains(out, `src="https://www.youtube.com/embed/dQw4w9WgXcQ?start=10"`) || !strings.Contains(out, "data-youtube-video") {
		t.Errorf("YouTube embed was not kept:\n%s", out)
	}
	nocookie := sanitizeBody(`<iframe src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"></iframe>`)
	if !strings.Contains(nocookie, "youtube-nocookie.com/embed/") {
		t.Errorf("youtube-nocookie embed was dropped: %s", nocookie)
	}
}

func TestSanitizeBodyStripsDangerousMarkup(t *testing.T) {
	cases := map[string]string{
		"script":            `<p>ok</p><script>alert(1)</script>`,
		"event handler":     `<img src="https://x.com/a.png" onerror="alert(1)">`,
		"javascript link":   `<a href="javascript:alert(1)">x</a>`,
		"other iframe":      `<iframe src="https://evil.example/embed/abcdefgh"></iframe>`,
		"youtube lookalike": `<iframe src="https://www.youtube.com.evil.example/embed/abcdefgh"></iframe>`,
		"non-embed youtube": `<iframe src="https://www.youtube.com/watch?v=dQw4w9WgXcQ"></iframe>`,
		"style tag":         `<style>body{display:none}</style><p>ok</p>`,
		"arbitrary style":   `<p style="position:fixed;top:0">ok</p>`,
		"form":              `<form action="https://x.com"><input name="pw"></form>`,
		"data uri image":    `<img src="data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=">`,
		"object":            `<object data="https://x.com/a.swf"></object>`,
	}
	for name, in := range cases {
		out := strings.ToLower(sanitizeBody(in))
		for _, bad := range []string{"<script", "onerror", "javascript:", "evil.example", "watch?v=", "<style", "position:fixed", "<form", "<input", "data:image", "<object", "<iframe"} {
			if strings.Contains(out, bad) {
				t.Errorf("%s: %q survived in %q", name, bad, out)
			}
		}
	}
}

func TestSanitizeText(t *testing.T) {
	if got := sanitizeText("  <b>ارتفاع</b>   الأسعار & <script>x</script> ", 200); got != "ارتفاع الأسعار &" {
		t.Errorf("got %q", got)
	}
	if got := sanitizeText("أبجدهوز", 3); got != "أبج" {
		t.Errorf("rune truncation: got %q", got)
	}
}
