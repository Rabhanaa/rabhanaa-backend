package model

import "time"

// ------------------------------------------------------------------ admin

type ExtractRequest struct {
	URL string `json:"url" binding:"required"`
}

// SourceImage is a photo from the source article, already copied to our
// storage. Width and height are 0 when the format could not be measured.
type SourceImage struct {
	URL    string `json:"url"`
	Alt    string `json:"alt"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// ExtractResponse is what was pulled from the source page, before any AI.
type ExtractResponse struct {
	SourceURL   string `json:"source_url"`
	SourceTitle string `json:"source_title"`
	SourceName  string `json:"source_name"`
	SourceText  string `json:"source_text"`
	// Our own copy of the page's main image, or "" when there was none or it
	// could not be stored; CoverWarning then says why.
	CoverImageURL string `json:"cover_image_url"`
	CoverWarning  string `json:"cover_warning,omitempty"`
	// The article's other photos, in reading order, without the cover.
	Images      []SourceImage `json:"images"`
	PublishedAt *time.Time    `json:"published_at,omitempty"`
}

type GenerateRequest struct {
	// Empty means the provider selected in settings.
	Provider    string `json:"provider"`
	SourceURL   string `json:"source_url"`
	SourceTitle string `json:"source_title"`
	SourceName  string `json:"source_name"`
	SourceText  string `json:"source_text" binding:"required"`
}

type GenerateResponse struct {
	Title             string  `json:"title"`
	Summary           string  `json:"summary"`
	BodyHTML          string  `json:"body_html"`
	NotificationTitle string  `json:"notification_title"`
	NotificationBody  string  `json:"notification_body"`
	InterestIDs       []int32 `json:"interest_ids"`
	Provider          string  `json:"provider"`
	Model             string  `json:"model"`
}

// NotificationTextRequest asks the AI for just the push text, from the
// article as it stands after the admin's edits.
type NotificationTextRequest struct {
	Provider string `json:"provider"`
	Title    string `json:"title" binding:"required"`
	Summary  string `json:"summary"`
	BodyHTML string `json:"body_html"`
}

type NotificationTextResponse struct {
	NotificationTitle string `json:"notification_title"`
	NotificationBody  string `json:"notification_body"`
	Provider          string `json:"provider"`
	Model             string `json:"model"`
}

type SaveRequest struct {
	Title         string  `json:"title" binding:"required"`
	Summary       string  `json:"summary"`
	BodyHTML      string  `json:"body_html"`
	CoverImageURL string  `json:"cover_image_url"`
	SourceURL     string  `json:"source_url"`
	SourceName    string  `json:"source_name"`
	SourceText    string  `json:"source_text"`
	AIProvider    string  `json:"ai_provider"`
	AIModel       string  `json:"ai_model"`
	InterestIDs   []int32 `json:"interest_ids"`
	// Offered again in the editor's gallery when the draft is reopened.
	SourceImages []SourceImage `json:"source_images"`
	// The push. Empty falls back to the title and summary.
	NotificationTitle string `json:"notification_title"`
	NotificationBody  string `json:"notification_body"`
}

type InterestRef struct {
	ID     int32  `json:"id"`
	NameAr string `json:"name_ar"`
}

type AdminNews struct {
	PublicID      string        `json:"public_id"`
	Title         string        `json:"title"`
	Summary       string        `json:"summary"`
	BodyHTML      string        `json:"body_html,omitempty"`
	CoverImageURL string        `json:"cover_image_url"`
	SourceURL     string        `json:"source_url"`
	SourceName    string        `json:"source_name"`
	SourceText    string        `json:"source_text,omitempty"`
	SourceImages  []SourceImage `json:"source_images"`
	Status        string        `json:"status"`
	AIProvider    string        `json:"ai_provider"`
	AIModel       string        `json:"ai_model"`
	Interests     []InterestRef `json:"interests"`
	PublishedAt   *time.Time    `json:"published_at"`
	NotifiedAt    *time.Time    `json:"notified_at"`
	NotifiedCount int32         `json:"notified_count"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`

	NotificationTitle string `json:"notification_title"`
	NotificationBody  string `json:"notification_body"`
	// Members who opened it, Pro or not.
	Viewers int64 `json:"viewers"`
}

type AdminNewsList struct {
	News  []AdminNews `json:"news"`
	Total int64       `json:"total"`
	Page  int32       `json:"page"`
}

type PublishRequest struct {
	Notify bool `json:"notify"`
}

type PublishResponse struct {
	News AdminNews `json:"news"`
	// The setting in force: off, all_pro or pro_by_interest.
	NotifyMode string `json:"notify_mode"`
	// How many members the push is going out to. Zero when nobody was notified.
	NotifiedCount int `json:"notified_count"`
	// True when this story had already been pushed, so nothing was sent again.
	AlreadyNotified bool `json:"already_notified"`
}

// AudienceResponse is shown in the publish dialog before anything is sent.
type AudienceResponse struct {
	NotifyMode      string `json:"notify_mode"`
	Recipients      int    `json:"recipients"`
	RecipientsPro   int    `json:"recipients_pro"`
	RecipientsFree  int    `json:"recipients_free"`
	WithPush        int64  `json:"with_push"`
	AlreadyNotified bool   `json:"already_notified"`
}

// NewsAnalytics is one story's reach. "Pro" counts members who read it as Pro
// and never hit the upgrade prompt; "Free" counts members who hit the prompt,
// including those who then upgraded (Converted).
type NewsAnalytics struct {
	NotifiedAt *time.Time `json:"notified_at"`

	Recipients     int64 `json:"recipients"`
	RecipientsPro  int64 `json:"recipients_pro"`
	RecipientsFree int64 `json:"recipients_free"`
	Delivered      int64 `json:"delivered"`
	DeliveredPro   int64 `json:"delivered_pro"`
	DeliveredFree  int64 `json:"delivered_free"`

	Viewers       int64 `json:"viewers"`
	ViewersPro    int64 `json:"viewers_pro"`
	ViewersFree   int64 `json:"viewers_free"`
	PushOpens     int64 `json:"push_opens"`
	PushOpensPro  int64 `json:"push_opens_pro"`
	PushOpensFree int64 `json:"push_opens_free"`
	ReadToEnd     int64 `json:"read_to_end"`
	UpgradeClicks int64 `json:"upgrade_clicks"`
	Converted     int64 `json:"converted"`
	TotalViews    int64 `json:"total_views"`
}

type NewsViewer struct {
	PublicID       string    `json:"public_id"`
	Name           string    `json:"name"`
	Phone          string    `json:"phone"`
	Email          string    `json:"email"`
	FromPush       bool      `json:"from_push"`
	ReadAsPro      bool      `json:"read_as_pro"`
	Gated          bool      `json:"gated"`
	ReadToEnd      bool      `json:"read_to_end"`
	UpgradeClicked bool      `json:"upgrade_clicked"`
	ViewCount      int32     `json:"view_count"`
	FirstViewedAt  time.Time `json:"first_viewed_at"`
	LastViewedAt   time.Time `json:"last_viewed_at"`
}

type NewsViewerList struct {
	Viewers []NewsViewer `json:"viewers"`
	Total   int64        `json:"total"`
	Page    int32        `json:"page"`
}

// TestNotificationRequest carries the notification as it is in the editor,
// saved or not; empty fields fall back to what is saved.
type TestNotificationRequest struct {
	NotificationTitle string `json:"notification_title"`
	NotificationBody  string `json:"notification_body"`
}

type TestNotificationResponse struct {
	// The admin's devices the push went to.
	Devices int64 `json:"devices"`
}

type AITestResponse struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	LatencyMS int64  `json:"latency_ms"`
}

// ---------------------------------------------------------------- members

type NewsListItem struct {
	PublicID      string    `json:"public_id"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary"`
	CoverImageURL string    `json:"cover_image_url"`
	PublishedAt   time.Time `json:"published_at"`
	IsNew         bool      `json:"is_new"`
}

type NewsList struct {
	News  []NewsListItem `json:"news"`
	Total int64          `json:"total"`
	Page  int32          `json:"page"`
}

// NewsArticle is the member view. It carries no source: the admin credits the
// source in the body, in their own words, where and how they choose.
type NewsArticle struct {
	PublicID      string        `json:"public_id"`
	Title         string        `json:"title"`
	Summary       string        `json:"summary"`
	BodyHTML      string        `json:"body_html"`
	CoverImageURL string        `json:"cover_image_url"`
	Interests     []InterestRef `json:"interests"`
	PublishedAt   time.Time     `json:"published_at"`
}

// NewsTeaser is what a non-Pro member sees of a story: enough to know what
// they are missing, next to the upgrade prompt.
type NewsTeaser struct {
	PublicID      string    `json:"public_id"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary"`
	CoverImageURL string    `json:"cover_image_url"`
	PublishedAt   time.Time `json:"published_at"`
}

type NewsStatus struct {
	IsPro  bool  `json:"is_pro"`
	Unread int64 `json:"unread"`
}
