// Package service runs the news section: admins draft stories (usually by
// having AI rewrite a source article), publish them, and Pro members are pushed
// and can read them.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"rabhana/db/sqlc"
	"rabhana/lib/ai"
	"rabhana/lib/webfetch"
	"rabhana/news/model"
	"rabhana/pkg/errs"
	settingsSvc "rabhana/settings/service"
)

const (
	maxTitleRunes   = 200
	maxSummaryRunes = 300
	// Column limits. The editor recommends far less (50 and 120): phones
	// cut notifications off well before these.
	maxNotificationTitleRunes = 100
	maxNotificationBodyRunes  = 300
	maxSourceNameRunes        = 200
	maxSourceTextRunes        = 50000
	maxBodyBytes              = 500_000
	maxCoverBytes             = 8 << 20
	maxSourceImages           = 20
	// Smaller than this is an icon, avatar or button, not a news photo.
	minImageWidth  = 200
	minImageHeight = 120
	// Copying the article's photos happens inside the admin's request, which
	// a proxy cuts off at 100 seconds; whatever is not stored by then is left out.
	imagePhaseTimeout = 40 * time.Second
	// Pushes in flight at once. Each is one request to FCM; this keeps a large
	// audience from opening thousands of connections at the same moment.
	pushConcurrency = 8
)

// PushSender delivers a push without adding it to the in-app notification
// list, and reports whether it reached at least one of the member's devices.
type PushSender interface {
	SendPushOnly(ctx context.Context, userID int32, title, body string, data map[string]string) bool
	PushEnabled() bool
}

// ImageUploader stores an image in the public bucket and returns its URL.
type ImageUploader interface {
	UploadFile(ctx context.Context, file []byte, originalName string) (string, error)
}

type Service struct {
	queries  *sqlc.Queries
	pool     *pgxpool.Pool
	settings *settingsSvc.Service
	push     PushSender
	uploader ImageUploader
	fetcher  *webfetch.Fetcher
	// Sent to OpenRouter for attribution.
	appURL string
}

func NewService(queries *sqlc.Queries, pool *pgxpool.Pool, settings *settingsSvc.Service, push PushSender, uploader ImageUploader, appURL string) *Service {
	return &Service{
		queries:  queries,
		pool:     pool,
		settings: settings,
		push:     push,
		uploader: uploader,
		fetcher:  webfetch.New(),
		appURL:   appURL,
	}
}

// DetailError carries a sentinel from pkg/errs plus a detail worth showing the
// admin — the provider's own error message, or why a page could not be read.
type DetailError struct {
	Err    error
	Detail string
}

func (e *DetailError) Error() string { return e.Err.Error() + ": " + e.Detail }
func (e *DetailError) Unwrap() error { return e.Err }

// ============================================================ source + AI

// Extract reads the article at a link and keeps a copy of its main image.
func (s *Service) Extract(ctx context.Context, rawURL string) (*model.ExtractResponse, error) {
	page, err := s.fetcher.FetchArticle(ctx, rawURL)
	if err != nil {
		switch {
		case errors.Is(err, webfetch.ErrInvalidURL), errors.Is(err, webfetch.ErrBlockedHost):
			return nil, errs.ErrInvalidNewsURL
		case errors.Is(err, webfetch.ErrUpstreamCode):
			return nil, &DetailError{Err: errs.ErrSourceFetchFailed, Detail: "الموقع رفض الطلب (" + strings.TrimPrefix(err.Error(), webfetch.ErrUpstreamCode.Error()+": ") + ")"}
		case errors.Is(err, webfetch.ErrNotHTML):
			return nil, &DetailError{Err: errs.ErrSourceFetchFailed, Detail: "الرابط لا يشير إلى صفحة ويب"}
		case errors.Is(err, webfetch.ErrNoArticle):
			return nil, &DetailError{Err: errs.ErrSourceFetchFailed, Detail: "لم يُعثر على نص خبر في الصفحة"}
		case errors.Is(err, webfetch.ErrTooLarge):
			return nil, &DetailError{Err: errs.ErrSourceFetchFailed, Detail: "الصفحة كبيرة جدًا"}
		default:
			slog.Warn("news: source fetch failed", "url", rawURL, "error", err)
			return nil, &DetailError{Err: errs.ErrSourceFetchFailed, Detail: "تعذر الاتصال بالموقع"}
		}
	}

	resp := &model.ExtractResponse{
		SourceURL:   page.URL,
		SourceTitle: page.Title,
		SourceName:  sanitizeText(page.SiteName, maxSourceNameRunes),
		SourceText:  page.Text,
		PublishedAt: page.PublishedAt,
		Images:      []model.SourceImage{},
	}

	// The share image first, then the article's own photos, all copied in
	// parallel. Index 0 is the cover candidate.
	candidates := make([]webfetch.Image, 0, len(page.Images)+1)
	if page.ImageURL != "" {
		candidates = append(candidates, webfetch.Image{URL: page.ImageURL})
	}
	candidates = append(candidates, page.Images...)
	stored := s.copyImages(ctx, candidates)

	rest := stored
	if page.ImageURL != "" {
		rest = stored[1:]
		if stored[0] != nil {
			resp.CoverImageURL = stored[0].URL
		}
	}
	for _, img := range rest {
		if img != nil {
			resp.Images = append(resp.Images, *img)
		}
	}
	// No usable share image: the first real photo makes a better cover than none.
	if resp.CoverImageURL == "" && len(resp.Images) > 0 {
		resp.CoverImageURL = resp.Images[0].URL
		resp.Images = resp.Images[1:]
	}
	if resp.CoverImageURL == "" && len(candidates) > 0 {
		resp.CoverWarning = "تعذر حفظ صورة الغلاف من المصدر — يمكنك رفع صورة بنفسك"
	}
	return resp, nil
}

// copyImages re-hosts each image, in order, leaving nil where one could not
// be fetched, was too small to be a photo, or did not finish in time.
func (s *Service) copyImages(ctx context.Context, images []webfetch.Image) []*model.SourceImage {
	out := make([]*model.SourceImage, len(images))
	ctx, cancel := context.WithTimeout(ctx, imagePhaseTimeout)
	defer cancel()

	var g errgroup.Group
	g.SetLimit(4)
	for i, img := range images {
		g.Go(func() error {
			stored, err := s.copyImage(ctx, img.URL)
			if err != nil {
				slog.Info("news: source image skipped", "image_url", img.URL, "reason", err)
				return nil
			}
			stored.Alt = sanitizeText(img.Alt, 200)
			out[i] = stored
			return nil
		})
	}
	_ = g.Wait()
	return out
}

var imageExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

var errImageTooSmall = errors.New("image too small to be a photo")

// copyImage re-hosts an image on our storage. Hotlinking the source would
// break when the site moves the file and would leak members' visits to it.
func (s *Service) copyImage(ctx context.Context, imageURL string) (*model.SourceImage, error) {
	data, contentType, err := s.fetcher.FetchImage(ctx, imageURL, maxCoverBytes)
	if err != nil {
		return nil, err
	}
	ext, ok := imageExt[contentType]
	if !ok {
		return nil, fmt.Errorf("unsupported image type %s", contentType)
	}
	w, h, measured := webfetch.ImageSize(data, contentType)
	if measured && (w < minImageWidth || h < minImageHeight) {
		return nil, errImageTooSmall
	}
	url, err := s.uploader.UploadFile(ctx, data, "news-image"+ext)
	if err != nil {
		return nil, err
	}
	return &model.SourceImage{URL: url, Width: w, Height: h}, nil
}

// Generate has the AI write a draft from source text. Nothing is saved: the
// admin reviews and edits the draft first.
func (s *Service) Generate(ctx context.Context, req model.GenerateRequest) (*model.GenerateResponse, error) {
	provider, apiKey, modelName, err := s.aiConfig(req.Provider)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(req.SourceText)
	if len([]rune(text)) < 80 {
		return nil, errs.ErrSourceTooShort
	}
	text = truncateRunes(text, 24000)

	interests, err := s.queries.ListInterests(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list interests: %w", err)
	}

	raw, err := ai.CompleteJSON(ctx, s.aiClientConfig(provider, apiKey, modelName), ai.Request{
		System:     systemPrompt,
		User:       userPrompt(interests, req.SourceName, req.SourceURL, req.SourceTitle, text),
		SchemaName: "news_article",
		Schema:     articleSchema,
	})
	if err != nil {
		return nil, aiError(provider, err)
	}

	var out struct {
		Title             string `json:"title"`
		Summary           string `json:"summary"`
		BodyHTML          string `json:"body_html"`
		NotificationTitle string `json:"notification_title"`
		NotificationBody  string `json:"notification_body"`
		InterestIDs       []any  `json:"interest_ids"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		slog.Warn("news: AI returned invalid JSON", "provider", provider, "model", modelName, "error", err)
		return nil, &DetailError{Err: errs.ErrAIRequestFailed, Detail: "رد الموديل ليس بالصيغة المطلوبة — جرّب موديلًا آخر"}
	}

	active := make(map[int32]bool, len(interests))
	for _, i := range interests {
		active[i.ID] = true
	}
	resp := &model.GenerateResponse{
		Title:       sanitizeText(out.Title, maxTitleRunes),
		Summary:     sanitizeText(out.Summary, maxSummaryRunes),
		BodyHTML:    sanitizeBody(out.BodyHTML),
		InterestIDs: []int32{},
		Provider:    provider,
		Model:       modelName,

		NotificationTitle: sanitizeText(out.NotificationTitle, maxNotificationTitleRunes),
		NotificationBody:  sanitizeText(out.NotificationBody, maxNotificationBodyRunes),
	}
	seen := map[int32]bool{}
	for _, v := range out.InterestIDs {
		if id, ok := toInt32(v); ok && active[id] && !seen[id] {
			seen[id] = true
			resp.InterestIDs = append(resp.InterestIDs, id)
		}
	}
	if resp.Title == "" || resp.BodyHTML == "" {
		return nil, &DetailError{Err: errs.ErrAIRequestFailed, Detail: "الموديل لم يكتب عنوانًا أو نصًا"}
	}
	return resp, nil
}

// Models in the API return numbers as float64, but some put them in quotes.
func toInt32(v any) (int32, bool) {
	switch n := v.(type) {
	case float64:
		if n == float64(int32(n)) {
			return int32(n), true
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return int32(i), true
		}
	}
	return 0, false
}

// GenerateNotification writes only the push title and text, from the article
// as it now stands.
func (s *Service) GenerateNotification(ctx context.Context, req model.NotificationTextRequest) (*model.NotificationTextResponse, error) {
	provider, apiKey, modelName, err := s.aiConfig(req.Provider)
	if err != nil {
		return nil, err
	}
	// Space before each tag so words from adjacent paragraphs do not run together.
	bodyText := truncateRunes(sanitizeText(strings.ReplaceAll(req.BodyHTML, "<", " <"), 20000), 6000)
	article := "Headline: " + sanitizeText(req.Title, maxTitleRunes) + "\nSummary: " + sanitizeText(req.Summary, maxSummaryRunes) +
		"\n\nArticle (between the markers):\n<<<ARTICLE\n" + bodyText + "\nARTICLE>>>\n"

	raw, err := ai.CompleteJSON(ctx, s.aiClientConfig(provider, apiKey, modelName), ai.Request{
		System:     notificationSystemPrompt,
		User:       article,
		SchemaName: "news_notification",
		Schema:     notificationSchema,
	})
	if err != nil {
		return nil, aiError(provider, err)
	}
	var out struct {
		NotificationTitle string `json:"notification_title"`
		NotificationBody  string `json:"notification_body"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, &DetailError{Err: errs.ErrAIRequestFailed, Detail: "رد الموديل ليس بالصيغة المطلوبة — جرّب موديلًا آخر"}
	}
	resp := &model.NotificationTextResponse{
		NotificationTitle: sanitizeText(out.NotificationTitle, maxNotificationTitleRunes),
		NotificationBody:  sanitizeText(out.NotificationBody, maxNotificationBodyRunes),
		Provider:          provider,
		Model:             modelName,
	}
	if resp.NotificationTitle == "" || resp.NotificationBody == "" {
		return nil, &DetailError{Err: errs.ErrAIRequestFailed, Detail: "الموديل لم يكتب نص الإشعار"}
	}
	return resp, nil
}

// TestAI makes the smallest possible structured call with a provider's saved
// key and model, so the admin finds out at once whether they work.
func (s *Service) TestAI(ctx context.Context, provider string) (*model.AITestResponse, error) {
	provider, apiKey, modelName, err := s.aiConfig(provider)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	raw, err := ai.CompleteJSON(ctx, s.aiClientConfig(provider, apiKey, modelName), ai.Request{
		System:     `Reply with the JSON object {"ok": true} and nothing else.`,
		User:       "ping",
		SchemaName: "ping",
		Schema:     pingSchema,
	})
	if err != nil {
		return nil, aiError(provider, err)
	}
	var out struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, &DetailError{Err: errs.ErrAIRequestFailed, Detail: "الموديل لا يدعم الردود المنظمة (JSON) — اختر موديلًا آخر"}
	}
	return &model.AITestResponse{Provider: provider, Model: modelName, LatencyMS: time.Since(start).Milliseconds()}, nil
}

// Models lists what the admin can choose for a provider, with its saved key.
func (s *Service) Models(ctx context.Context, provider string) ([]string, error) {
	if !settingsSvc.IsAIProvider(provider) {
		return nil, errs.ErrInvalidAIProvider
	}
	apiKey, _ := s.settings.AICredentials(provider)
	ids, err := ai.ListModels(ctx, provider, apiKey)
	if err != nil {
		return nil, aiError(provider, err)
	}
	return ids, nil
}

func (s *Service) aiConfig(requested string) (provider, apiKey, modelName string, err error) {
	provider = requested
	if provider == "" {
		provider = s.settings.AIProvider()
	}
	if !settingsSvc.IsAIProvider(provider) {
		return "", "", "", errs.ErrInvalidAIProvider
	}
	apiKey, modelName = s.settings.AICredentials(provider)
	if apiKey == "" || modelName == "" {
		return "", "", "", &DetailError{Err: errs.ErrAINotConfigured, Detail: provider}
	}
	return provider, apiKey, modelName, nil
}

func (s *Service) aiClientConfig(provider, apiKey, modelName string) ai.Config {
	return ai.Config{Provider: provider, APIKey: apiKey, Model: modelName, AppURL: s.appURL, AppName: "Rabhanaa"}
}

func aiError(provider string, err error) error {
	var pe *ai.ProviderError
	switch {
	case errors.As(err, &pe):
		return &DetailError{Err: errs.ErrAIRequestFailed, Detail: pe.Message}
	case errors.Is(err, ai.ErrNotConfigured):
		return &DetailError{Err: errs.ErrAINotConfigured, Detail: provider}
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(err.Error(), "Client.Timeout"):
		return &DetailError{Err: errs.ErrAIRequestFailed, Detail: "انتهت مهلة الانتظار — جرّب مرة أخرى أو اختر موديلًا أسرع"}
	case errors.Is(err, ai.ErrEmptyResponse):
		return &DetailError{Err: errs.ErrAIRequestFailed, Detail: "رد فارغ من الموديل"}
	default:
		slog.Error("news: AI call failed", "provider", provider, "error", err)
		return &DetailError{Err: errs.ErrAIRequestFailed, Detail: "تعذر الاتصال بمزود الذكاء الاصطناعي"}
	}
}

// ============================================================ admin CRUD

type cleanedNews struct {
	params      sqlc.CreateNewsParams
	interestIDs []int32
}

func (s *Service) clean(ctx context.Context, req model.SaveRequest) (*cleanedNews, error) {
	title := sanitizeText(req.Title, maxTitleRunes)
	if title == "" {
		return nil, errs.ErrNewsTitleRequired
	}
	body := sanitizeBody(req.BodyHTML)
	if len(body) > maxBodyBytes {
		return nil, errs.ErrNewsTooLarge
	}
	coverURL, err := optionalURL(req.CoverImageURL)
	if err != nil {
		return nil, err
	}
	sourceURL, err := optionalURL(req.SourceURL)
	if err != nil {
		return nil, err
	}
	provider := ""
	if settingsSvc.IsAIProvider(req.AIProvider) {
		provider = req.AIProvider
	}

	ids, err := s.activeInterestIDs(ctx, req.InterestIDs)
	if err != nil {
		return nil, err
	}
	images, err := json.Marshal(cleanSourceImages(req.SourceImages))
	if err != nil {
		return nil, err
	}
	return &cleanedNews{
		params: sqlc.CreateNewsParams{
			Title:         title,
			Summary:       sanitizeText(req.Summary, maxSummaryRunes),
			BodyHtml:      body,
			CoverImageUrl: text(coverURL),
			SourceUrl:     text(sourceURL),
			SourceName:    text(sanitizeText(req.SourceName, maxSourceNameRunes)),
			SourceText:    text(truncateRunes(strings.TrimSpace(req.SourceText), maxSourceTextRunes)),
			SourceImages:  images,
			AiProvider:    text(provider),
			AiModel:       text(sanitizeText(req.AIModel, 128)),

			NotificationTitle: sanitizeText(req.NotificationTitle, maxNotificationTitleRunes),
			NotificationBody:  sanitizeText(req.NotificationBody, maxNotificationBodyRunes),
		},
		interestIDs: ids,
	}, nil
}

// cleanSourceImages keeps only well-formed http(s) entries. The list comes back
// from the admin's browser, so it is checked like any other input.
func cleanSourceImages(in []model.SourceImage) []model.SourceImage {
	out := make([]model.SourceImage, 0, len(in))
	seen := map[string]bool{}
	for _, img := range in {
		u, err := webfetch.ParseURL(img.URL)
		if err != nil || seen[u.String()] || len(out) >= maxSourceImages {
			continue
		}
		seen[u.String()] = true
		out = append(out, model.SourceImage{
			URL:    u.String(),
			Alt:    sanitizeText(img.Alt, 200),
			Width:  max(img.Width, 0),
			Height: max(img.Height, 0),
		})
	}
	return out
}

func optionalURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := webfetch.ParseURL(raw)
	if err != nil {
		return "", errs.ErrInvalidNewsURL
	}
	return u.String(), nil
}

// activeInterestIDs drops unknown and retired interests rather than failing:
// the list comes from a picker or from the AI, and either may be a little stale.
func (s *Service) activeInterestIDs(ctx context.Context, ids []int32) ([]int32, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.queries.ListActiveInterestsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to check interests: %w", err)
	}
	out := make([]int32, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, req model.SaveRequest, adminID int32) (*model.AdminNews, error) {
	c, err := s.clean(ctx, req)
	if err != nil {
		return nil, err
	}
	c.params.CreatedByAdminID = pgtype.Int4{Int32: adminID, Valid: adminID > 0}

	var created sqlc.News
	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		n, err := q.CreateNews(ctx, c.params)
		if err != nil {
			return fmt.Errorf("failed to create news: %w", err)
		}
		created = n
		return replaceInterests(ctx, q, n.ID, c.interestIDs)
	})
	if err != nil {
		return nil, err
	}
	return s.toAdmin(ctx, created, true)
}

// Update edits a draft or a live story; edits to a live story show at once.
func (s *Service) Update(ctx context.Context, publicID uuid.UUID, req model.SaveRequest) (*model.AdminNews, error) {
	c, err := s.clean(ctx, req)
	if err != nil {
		return nil, err
	}
	var updated sqlc.News
	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		n, err := q.UpdateNews(ctx, sqlc.UpdateNewsParams{
			Title:         c.params.Title,
			Summary:       c.params.Summary,
			BodyHtml:      c.params.BodyHtml,
			CoverImageUrl: c.params.CoverImageUrl,
			SourceUrl:     c.params.SourceUrl,
			SourceName:    c.params.SourceName,
			SourceText:    c.params.SourceText,
			SourceImages:  c.params.SourceImages,
			AiProvider:    c.params.AiProvider,
			AiModel:       c.params.AiModel,
			PublicID:      pgUUID(publicID),

			NotificationTitle: c.params.NotificationTitle,
			NotificationBody:  c.params.NotificationBody,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNewsNotFound
		}
		if err != nil {
			return fmt.Errorf("failed to update news: %w", err)
		}
		updated = n
		return replaceInterests(ctx, q, n.ID, c.interestIDs)
	})
	if err != nil {
		return nil, err
	}
	return s.toAdmin(ctx, updated, true)
}

func replaceInterests(ctx context.Context, q *sqlc.Queries, newsID int32, ids []int32) error {
	if err := q.DeleteNewsInterests(ctx, newsID); err != nil {
		return fmt.Errorf("failed to clear news interests: %w", err)
	}
	for _, id := range ids {
		if err := q.AddNewsInterest(ctx, sqlc.AddNewsInterestParams{NewsID: newsID, InterestID: id}); err != nil {
			return fmt.Errorf("failed to add news interest %d: %w", id, err)
		}
	}
	return nil
}

func (s *Service) inTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed
	if err := fn(s.queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) AdminGet(ctx context.Context, publicID uuid.UUID) (*model.AdminNews, error) {
	n, err := s.queries.GetNewsByPublicID(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNewsNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.toAdmin(ctx, n, true)
}

func (s *Service) AdminList(ctx context.Context, status string, page, pageSize int32) (*model.AdminNewsList, error) {
	if status != "draft" && status != "published" {
		status = ""
	}
	rows, err := s.queries.AdminListNews(ctx, sqlc.AdminListNewsParams{Status: status, Lim: pageSize, Off: (page - 1) * pageSize})
	if err != nil {
		return nil, err
	}
	total, err := s.queries.AdminCountNews(ctx, status)
	if err != nil {
		return nil, err
	}
	interests, err := s.interestsFor(ctx, rows)
	if err != nil {
		return nil, err
	}
	viewers, err := s.viewerCounts(ctx, rows)
	if err != nil {
		return nil, err
	}
	out := &model.AdminNewsList{News: make([]model.AdminNews, 0, len(rows)), Total: total, Page: page}
	for _, n := range rows {
		item := adminFromRow(n, false)
		item.Interests = nonNil(interests[n.ID])
		item.Viewers = viewers[n.ID]
		out.News = append(out.News, item)
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, publicID uuid.UUID) error {
	n, err := s.queries.DeleteNews(ctx, pgUUID(publicID))
	if err != nil {
		return err
	}
	if n == 0 {
		return errs.ErrNewsNotFound
	}
	return nil
}

func (s *Service) Unpublish(ctx context.Context, publicID uuid.UUID) (*model.AdminNews, error) {
	n, err := s.queries.UnpublishNews(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNewsNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.toAdmin(ctx, n, true)
}

// ============================================================ publishing

// Publish makes a story visible and, when asked and the notification setting
// allows it, pushes it to members — Pro and non-Pro alike; a non-Pro member
// who taps it gets the teaser and an upgrade prompt. A story is pushed at
// most once in its life, whatever happens to it afterwards.
func (s *Service) Publish(ctx context.Context, publicID uuid.UUID, notify bool) (*model.PublishResponse, error) {
	current, err := s.queries.GetNewsByPublicID(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNewsNotFound
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(current.BodyHtml) == "" {
		return nil, errs.ErrNewsBodyRequired
	}

	n, err := s.queries.PublishNews(ctx, pgUUID(publicID))
	if err != nil {
		return nil, fmt.Errorf("failed to publish news: %w", err)
	}

	mode := s.settings.NewsNotifyMode()
	resp := &model.PublishResponse{NotifyMode: mode, AlreadyNotified: n.NotifiedAt.Valid}

	if notify && mode != settingsSvc.NewsNotifyOff && !n.NotifiedAt.Valid {
		recipients, err := s.recipients(ctx, n.ID, mode)
		if err != nil {
			return nil, err
		}
		// Nobody to tell (no matching interests, say): leave the story
		// un-notified so fixing its interests and publishing again still works.
		if len(recipients) > 0 {
			if _, err := s.queries.ClaimNewsNotification(ctx, n.ID); err == nil {
				if err := s.queries.SetNewsNotifiedCount(ctx, sqlc.SetNewsNotifiedCountParams{ID: n.ID, NotifiedCount: int32(len(recipients))}); err != nil {
					slog.Error("news: failed to record notified count", "news_id", n.ID, "error", err)
				}
				// Recorded before sending, so the analytics show the audience at
				// once and deliveries fill in as the pushes go out.
				if err := s.recordRecipients(ctx, n.ID, recipients); err != nil {
					slog.Error("news: failed to record recipients", "news_id", n.ID, "error", err)
				}
				resp.NotifiedCount = len(recipients)
				go s.fanOut(context.WithoutCancel(ctx), n, recipients)
			} else if errors.Is(err, pgx.ErrNoRows) {
				// Someone else's publish claimed it first.
				resp.AlreadyNotified = true
			} else {
				return nil, fmt.Errorf("failed to claim notification: %w", err)
			}
		}
	}

	refreshed, err := s.queries.GetNewsByPublicID(ctx, pgUUID(publicID))
	if err != nil {
		return nil, err
	}
	article, err := s.toAdmin(ctx, refreshed, true)
	if err != nil {
		return nil, err
	}
	resp.News = *article
	return resp, nil
}

func (s *Service) recipients(ctx context.Context, newsID int32, mode string) ([]sqlc.ListNewsRecipientsRow, error) {
	if mode == settingsSvc.NewsNotifyOff {
		return nil, nil
	}
	rows, err := s.queries.ListNewsRecipients(ctx, sqlc.ListNewsRecipientsParams{
		ByInterest: mode == settingsSvc.NewsNotifyByInterest,
		NewsID:     newsID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list news recipients: %w", err)
	}
	return rows, nil
}

func (s *Service) recordRecipients(ctx context.Context, newsID int32, recipients []sqlc.ListNewsRecipientsRow) error {
	ids := make([]int32, len(recipients))
	pro := make([]bool, len(recipients))
	for i, r := range recipients {
		ids[i], pro[i] = r.ID, r.IsPro
	}
	return s.queries.InsertNewsDeliveries(ctx, sqlc.InsertNewsDeliveriesParams{NewsID: newsID, UserIds: ids, WasPro: pro})
}

// notificationText is what the push says: the admin's notification text, or
// the title and summary when it was left empty.
func notificationText(n sqlc.News) (title, body string) {
	title, body = n.NotificationTitle, n.NotificationBody
	if title == "" {
		title = n.Title
	}
	if body == "" {
		body = n.Summary
	}
	if body == "" {
		// Space before each tag so words from adjacent paragraphs do not run together.
		body = truncateRunes(sanitizeText(strings.ReplaceAll(n.BodyHtml, "<", " <"), 400), 140)
	}
	return title, body
}

func (s *Service) fanOut(ctx context.Context, n sqlc.News, recipients []sqlc.ListNewsRecipientsRow) {
	title, body := notificationText(n)
	data := map[string]string{"type": "news_published", "news_id": uuid.UUID(n.PublicID.Bytes).String()}

	start := time.Now()
	var delivered atomic.Int64
	var g errgroup.Group
	g.SetLimit(pushConcurrency)
	for _, r := range recipients {
		g.Go(func() error {
			if s.push.SendPushOnly(ctx, r.ID, title, body, data) {
				delivered.Add(1)
				if err := s.queries.MarkNewsDelivered(ctx, sqlc.MarkNewsDeliveredParams{NewsID: n.ID, UserID: r.ID}); err != nil {
					slog.Error("news: failed to record delivery", "news_id", n.ID, "user_id", r.ID, "error", err)
				}
			}
			return nil
		})
	}
	_ = g.Wait()
	slog.Info("news: push fan-out finished", "news_id", n.ID, "recipients", len(recipients),
		"delivered", delivered.Load(), "took", time.Since(start).String())
}

// SendTestNotification pushes a story's notification to the admin's own
// devices only, so they can see how it looks and that tapping it opens the
// story before every member gets it. Nothing is recorded: tests never show up
// in the story's analytics and do not count as its one notification.
func (s *Service) SendTestNotification(ctx context.Context, publicID uuid.UUID, adminID int32, req model.TestNotificationRequest) (*model.TestNotificationResponse, error) {
	n, err := s.queries.GetNewsByPublicID(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNewsNotFound
	}
	if err != nil {
		return nil, err
	}
	if !s.push.PushEnabled() {
		return nil, errs.ErrPushUnavailable
	}
	devices, err := s.queries.CountUsersWithActiveDevice(ctx, []int32{adminID})
	if err != nil {
		return nil, err
	}
	if devices == 0 {
		return nil, errs.ErrNoTestDevice
	}

	// What the editor shows now, which may not be saved yet.
	if t := sanitizeText(req.NotificationTitle, maxNotificationTitleRunes); t != "" {
		n.NotificationTitle = t
	}
	if b := sanitizeText(req.NotificationBody, maxNotificationBodyRunes); b != "" {
		n.NotificationBody = b
	}
	title, body := notificationText(n)
	data := map[string]string{"type": "news_published", "news_id": uuid.UUID(n.PublicID.Bytes).String(), "test": "1"}
	if !s.push.SendPushOnly(ctx, adminID, title, body, data) {
		return nil, errs.ErrTestPushFailed
	}
	return &model.TestNotificationResponse{Devices: devices}, nil
}

// Audience is what the publish dialog shows before anything is sent.
func (s *Service) Audience(ctx context.Context, publicID uuid.UUID) (*model.AudienceResponse, error) {
	n, err := s.queries.GetNewsByPublicID(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNewsNotFound
	}
	if err != nil {
		return nil, err
	}
	mode := s.settings.NewsNotifyMode()
	recipients, err := s.recipients(ctx, n.ID, mode)
	if err != nil {
		return nil, err
	}
	resp := &model.AudienceResponse{NotifyMode: mode, Recipients: len(recipients), AlreadyNotified: n.NotifiedAt.Valid}
	ids := make([]int32, len(recipients))
	for i, r := range recipients {
		ids[i] = r.ID
		if r.IsPro {
			resp.RecipientsPro++
		}
	}
	resp.RecipientsFree = resp.Recipients - resp.RecipientsPro
	if len(ids) > 0 {
		if resp.WithPush, err = s.queries.CountUsersWithActiveDevice(ctx, ids); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// ============================================================ members

func (s *Service) IsPro(ctx context.Context, userID int32) (bool, error) {
	return s.queries.IsProUser(ctx, userID)
}

func (s *Service) Status(ctx context.Context, userID int32) (*model.NewsStatus, error) {
	isPro, err := s.IsPro(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := &model.NewsStatus{IsPro: isPro}
	if isPro {
		if resp.Unread, err = s.queries.CountUnreadNews(ctx, userID); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func (s *Service) List(ctx context.Context, userID, page, pageSize int32) (*model.NewsList, error) {
	rows, err := s.queries.ListPublishedNews(ctx, sqlc.ListPublishedNewsParams{UserID: userID, Lim: pageSize, Off: (page - 1) * pageSize})
	if err != nil {
		return nil, err
	}
	total, err := s.queries.CountPublishedNews(ctx)
	if err != nil {
		return nil, err
	}
	out := &model.NewsList{News: make([]model.NewsListItem, 0, len(rows)), Total: total, Page: page}
	for _, r := range rows {
		out.News = append(out.News, model.NewsListItem{
			PublicID:      uuid.UUID(r.PublicID.Bytes).String(),
			Title:         r.Title,
			Summary:       r.Summary,
			CoverImageURL: r.CoverImageUrl.String,
			PublishedAt:   r.PublishedAt.Time,
			IsNew:         r.IsNew,
		})
	}
	return out, nil
}

// Get opens a story for a member and records the visit. A Pro member gets the
// article; anyone else gets its teaser and ErrProRequired, which the app
// shows as the upgrade prompt. fromPush marks a tap on the notification.
func (s *Service) Get(ctx context.Context, publicID uuid.UUID, userID int32, fromPush bool) (*model.NewsArticle, *model.NewsTeaser, error) {
	n, err := s.queries.GetPublishedNews(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, errs.ErrNewsNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	isPro, err := s.IsPro(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	// Tracking never blocks reading: a failed write is logged, not returned.
	if err := s.queries.RecordNewsView(ctx, sqlc.RecordNewsViewParams{
		NewsID: n.ID, UserID: userID, FromPush: fromPush, ReadAsPro: isPro, Gated: !isPro,
	}); err != nil {
		slog.Error("news: failed to record view", "news_id", n.ID, "user_id", userID, "error", err)
	}

	if !isPro {
		return nil, &model.NewsTeaser{
			PublicID:      uuid.UUID(n.PublicID.Bytes).String(),
			Title:         n.Title,
			Summary:       n.Summary,
			CoverImageURL: n.CoverImageUrl.String,
			PublishedAt:   n.PublishedAt.Time,
		}, errs.ErrProRequired
	}

	interests, err := s.interestsFor(ctx, []sqlc.News{n})
	if err != nil {
		return nil, nil, err
	}
	return &model.NewsArticle{
		PublicID:      uuid.UUID(n.PublicID.Bytes).String(),
		Title:         n.Title,
		Summary:       n.Summary,
		BodyHTML:      n.BodyHtml,
		CoverImageURL: n.CoverImageUrl.String,
		Interests:     nonNil(interests[n.ID]),
		PublishedAt:   n.PublishedAt.Time,
	}, nil, nil
}

// Preview is the member view of any story, published or not, for an admin —
// what a test notification opens. Not recorded in the analytics.
func (s *Service) Preview(ctx context.Context, publicID uuid.UUID) (*model.NewsArticle, error) {
	n, err := s.queries.GetNewsByPublicID(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNewsNotFound
	}
	if err != nil {
		return nil, err
	}
	interests, err := s.interestsFor(ctx, []sqlc.News{n})
	if err != nil {
		return nil, err
	}
	published := n.PublishedAt.Time
	if !n.PublishedAt.Valid {
		published = n.UpdatedAt.Time
	}
	return &model.NewsArticle{
		PublicID:      uuid.UUID(n.PublicID.Bytes).String(),
		Title:         n.Title,
		Summary:       n.Summary,
		BodyHTML:      n.BodyHtml,
		CoverImageURL: n.CoverImageUrl.String,
		Interests:     nonNil(interests[n.ID]),
		PublishedAt:   published,
	}, nil
}

// MarkReadToEnd records that a Pro member scrolled to the end of a story.
func (s *Service) MarkReadToEnd(ctx context.Context, publicID uuid.UUID, userID int32) error {
	n, err := s.queries.GetPublishedNews(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrNewsNotFound
	}
	if err != nil {
		return err
	}
	_, err = s.queries.MarkNewsReadToEnd(ctx, sqlc.MarkNewsReadToEndParams{NewsID: n.ID, UserID: userID})
	return err
}

// UpgradeClicked records that a non-Pro member tapped "subscribe" on a
// story's upgrade prompt.
func (s *Service) UpgradeClicked(ctx context.Context, publicID uuid.UUID, userID int32) error {
	n, err := s.queries.GetPublishedNews(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrNewsNotFound
	}
	if err != nil {
		return err
	}
	_, err = s.queries.MarkNewsUpgradeClicked(ctx, sqlc.MarkNewsUpgradeClickedParams{NewsID: n.ID, UserID: userID})
	return err
}

// ============================================================ analytics

func (s *Service) Analytics(ctx context.Context, publicID uuid.UUID) (*model.NewsAnalytics, error) {
	n, err := s.queries.GetNewsByPublicID(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNewsNotFound
	}
	if err != nil {
		return nil, err
	}
	d, err := s.queries.NewsDeliveryStats(ctx, n.ID)
	if err != nil {
		return nil, err
	}
	v, err := s.queries.NewsViewStats(ctx, n.ID)
	if err != nil {
		return nil, err
	}
	return &model.NewsAnalytics{
		NotifiedAt:     timePtr(n.NotifiedAt),
		Recipients:     d.Recipients,
		RecipientsPro:  d.RecipientsPro,
		RecipientsFree: d.Recipients - d.RecipientsPro,
		Delivered:      d.Delivered,
		DeliveredPro:   d.DeliveredPro,
		DeliveredFree:  d.Delivered - d.DeliveredPro,
		Viewers:        v.Viewers,
		ViewersPro:     v.ViewersPro,
		ViewersFree:    v.ViewersFree,
		PushOpens:      v.PushOpens,
		PushOpensPro:   v.PushOpensPro,
		PushOpensFree:  v.PushOpensFree,
		ReadToEnd:      v.ReadToEnd,
		UpgradeClicks:  v.UpgradeClicks,
		Converted:      v.Converted,
		TotalViews:     v.TotalViews,
	}, nil
}

var viewerFilters = map[string]bool{"": true, "pro": true, "free": true, "push": true, "upgrade": true}

// Viewers lists the members who opened a story, most recent first.
func (s *Service) Viewers(ctx context.Context, publicID uuid.UUID, filter string, page, pageSize int32) (*model.NewsViewerList, error) {
	n, err := s.queries.GetNewsByPublicID(ctx, pgUUID(publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNewsNotFound
	}
	if err != nil {
		return nil, err
	}
	if !viewerFilters[filter] {
		filter = ""
	}
	rows, err := s.queries.ListNewsViewers(ctx, sqlc.ListNewsViewersParams{NewsID: n.ID, Filter: filter, Lim: pageSize, Off: (page - 1) * pageSize})
	if err != nil {
		return nil, err
	}
	total, err := s.queries.CountNewsViewers(ctx, sqlc.CountNewsViewersParams{NewsID: n.ID, Filter: filter})
	if err != nil {
		return nil, err
	}
	out := &model.NewsViewerList{Viewers: make([]model.NewsViewer, 0, len(rows)), Total: total, Page: page}
	for _, r := range rows {
		out.Viewers = append(out.Viewers, model.NewsViewer{
			PublicID:       uuid.UUID(r.PublicID.Bytes).String(),
			Name:           r.Name,
			Phone:          r.Phone.String,
			Email:          r.Email,
			FromPush:       r.FromPush,
			ReadAsPro:      r.ReadAsPro,
			Gated:          r.Gated,
			ReadToEnd:      r.ReadToEnd,
			UpgradeClicked: r.UpgradeClicked,
			ViewCount:      r.ViewCount,
			FirstViewedAt:  r.FirstViewedAt.Time,
			LastViewedAt:   r.LastViewedAt.Time,
		})
	}
	return out, nil
}

func (s *Service) MarkSeen(ctx context.Context, userID int32) error {
	return s.queries.MarkNewsSeen(ctx, userID)
}

// ============================================================ helpers

func (s *Service) toAdmin(ctx context.Context, n sqlc.News, full bool) (*model.AdminNews, error) {
	interests, err := s.interestsFor(ctx, []sqlc.News{n})
	if err != nil {
		return nil, err
	}
	viewers, err := s.viewerCounts(ctx, []sqlc.News{n})
	if err != nil {
		return nil, err
	}
	out := adminFromRow(n, full)
	out.Interests = nonNil(interests[n.ID])
	out.Viewers = viewers[n.ID]
	return &out, nil
}

func (s *Service) viewerCounts(ctx context.Context, rows []sqlc.News) (map[int32]int64, error) {
	out := map[int32]int64{}
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]int32, len(rows))
	for i, n := range rows {
		ids[i] = n.ID
	}
	counts, err := s.queries.CountNewsViewersByNews(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to count viewers: %w", err)
	}
	for _, c := range counts {
		out[c.NewsID] = c.Viewers
	}
	return out, nil
}

func adminFromRow(n sqlc.News, full bool) model.AdminNews {
	out := model.AdminNews{
		PublicID:      uuid.UUID(n.PublicID.Bytes).String(),
		Title:         n.Title,
		Summary:       n.Summary,
		CoverImageURL: n.CoverImageUrl.String,
		SourceURL:     n.SourceUrl.String,
		SourceName:    n.SourceName.String,
		Status:        n.Status,
		AIProvider:    n.AiProvider.String,
		AIModel:       n.AiModel.String,
		Interests:     []model.InterestRef{},
		SourceImages:  []model.SourceImage{},
		PublishedAt:   timePtr(n.PublishedAt),
		NotifiedAt:    timePtr(n.NotifiedAt),
		NotifiedCount: n.NotifiedCount,
		CreatedAt:     n.CreatedAt.Time,
		UpdatedAt:     n.UpdatedAt.Time,

		NotificationTitle: n.NotificationTitle,
		NotificationBody:  n.NotificationBody,
	}
	if full {
		out.BodyHTML = n.BodyHtml
		out.SourceText = n.SourceText.String
		if err := json.Unmarshal(n.SourceImages, &out.SourceImages); err != nil || out.SourceImages == nil {
			out.SourceImages = []model.SourceImage{}
		}
	}
	return out
}

func (s *Service) interestsFor(ctx context.Context, rows []sqlc.News) (map[int32][]model.InterestRef, error) {
	out := map[int32][]model.InterestRef{}
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]int32, len(rows))
	for i, n := range rows {
		ids[i] = n.ID
	}
	links, err := s.queries.ListNewsInterests(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to list news interests: %w", err)
	}
	for _, l := range links {
		out[l.NewsID] = append(out[l.NewsID], model.InterestRef{ID: l.ID, NameAr: l.NameAr})
	}
	return out, nil
}

func nonNil(v []model.InterestRef) []model.InterestRef {
	if v == nil {
		return []model.InterestRef{}
	}
	return v
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
