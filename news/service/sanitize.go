package service

import (
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// Article bodies come from three places: the admin's editor, raw HTML the admin
// pastes, and AI output written from a scraped page — which can carry hidden
// instructions. Every body is sanitised on every write, so whatever is stored
// is safe to render as HTML in the member app.
//
// Allowed: ordinary article markup, tables, images, links (opened in a new tab,
// nofollow) and YouTube embeds. Not allowed: scripts, styles, event handlers,
// forms, and iframes from anywhere other than YouTube.
var bodyPolicy = newBodyPolicy()

// The only iframe source accepted: YouTube's embed player.
var youtubeEmbed = regexp.MustCompile(`^https://(www\.)?(youtube\.com|youtube-nocookie\.com)/embed/[A-Za-z0-9_-]{6,20}(\?[A-Za-z0-9_=&;%.-]*)?$`)

func newBodyPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowElements("figure", "figcaption", "div", "span", "u", "s", "mark")
	p.AllowAttrs("dir").Matching(regexp.MustCompile(`^(rtl|ltr|auto)$`)).Globally()
	p.AllowStyles("text-align").MatchingEnum("left", "right", "center", "justify").OnElements("p", "h2", "h3", "h4")
	// TipTap's YouTube node wraps the iframe in this div.
	p.AllowAttrs("data-youtube-video").Matching(regexp.MustCompile(`^$`)).OnElements("div")
	p.AllowElements("iframe")
	p.AllowAttrs("src").Matching(youtubeEmbed).OnElements("iframe")
	p.AllowAttrs("width", "height").Matching(bluemonday.Number).OnElements("iframe", "img")
	p.AllowAttrs("allowfullscreen").OnElements("iframe")
	p.AllowAttrs("frameborder").Matching(bluemonday.Number).OnElements("iframe")
	p.AllowAttrs("allow").Matching(regexp.MustCompile(`^[a-z\-; ]*$`)).OnElements("iframe")
	p.AllowAttrs("title").OnElements("iframe")
	p.AllowAttrs("loading").Matching(regexp.MustCompile(`^lazy$`)).OnElements("img", "iframe")
	p.RequireNoFollowOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.AllowURLSchemes("http", "https", "mailto", "tel")
	return p
}

// An iframe whose src was stripped is still an empty frame on the page.
var emptyIframe = regexp.MustCompile(`(?is)<iframe\b[^>]*>\s*</iframe>`)

func sanitizeBody(html string) string {
	clean := bodyPolicy.Sanitize(html)
	clean = removeSrclessIframes(clean)
	return strings.TrimSpace(clean)
}

func removeSrclessIframes(html string) string {
	return emptyIframe.ReplaceAllStringFunc(html, func(tag string) string {
		if strings.Contains(strings.ToLower(tag), " src=") {
			return tag
		}
		return ""
	})
}

var textPolicy = bluemonday.StrictPolicy()

// sanitizeText is for plain-text fields (title, summary): no markup at all,
// whitespace collapsed, and cut to the column's length.
func sanitizeText(s string, maxRunes int) string {
	s = textPolicy.Sanitize(s)
	// StrictPolicy escapes what it keeps; the field is plain text, so undo that.
	s = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&#34;", `"`, "&#39;", "'", "&quot;", `"`).Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > maxRunes {
		s = strings.TrimSpace(string(r[:maxRunes]))
	}
	return s
}
