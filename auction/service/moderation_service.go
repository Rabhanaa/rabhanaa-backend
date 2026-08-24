package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/auction/model"
	"rabhana/db/sqlc"
	notifModel "rabhana/notification/model"
	"rabhana/pkg/errs"
)

// PostType distinguishes the two post tables, which are structurally parallel
// but separate everywhere else in the codebase.
type PostType string

const (
	PostTypeSell PostType = "sell"
	PostTypeBuy  PostType = "buy"
)

func ParsePostType(s string) (PostType, error) {
	switch PostType(s) {
	case PostTypeSell:
		return PostTypeSell, nil
	case PostTypeBuy:
		return PostTypeBuy, nil
	default:
		return "", errs.ErrInvalidPostType
	}
}

type ModerationService struct {
	queries              *sqlc.Queries
	notificationSender   NotificationSender
	auctionDurationHours int
}

func NewModerationService(queries *sqlc.Queries, notificationSender NotificationSender, auctionDurationHours int) *ModerationService {
	return &ModerationService{
		queries:              queries,
		notificationSender:   notificationSender,
		auctionDurationHours: auctionDurationHours,
	}
}

// PendingPost is the flattened shape the admin queue renders, so one endpoint
// can return both post types in a single list.
type PendingPost struct {
	PublicID     string  `json:"public_id"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Description  *string `json:"description,omitempty"`
	ImageURL     string  `json:"image_url"`
	OwnerName    string  `json:"owner_name"`
	RegionName   string  `json:"region_name"`
	InterestName string  `json:"interest_name"`
	Unit         string  `json:"unit"`
	Quantity     string  `json:"quantity"`
	UnitPrice    string  `json:"unit_price,omitempty"`
	Status       string  `json:"status"`
	Reason       *string `json:"moderation_reason,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

func (s *ModerationService) ListPending(ctx context.Context, page, pageSize int32) ([]PendingPost, int64, error) {
	limit, offset := paginate(page, pageSize)

	auctions, err := s.queries.ListPendingApprovalSellAuctions(ctx, sqlc.ListPendingApprovalSellAuctionsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list pending sell auctions: %w", err)
	}
	requests, err := s.queries.ListPendingApprovalBuyRequests(ctx, sqlc.ListPendingApprovalBuyRequestsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list pending buy requests: %w", err)
	}

	sellTotal, err := s.queries.CountPendingApprovalSellAuctions(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count pending sell auctions: %w", err)
	}
	buyTotal, err := s.queries.CountPendingApprovalBuyRequests(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count pending buy requests: %w", err)
	}

	posts := make([]PendingPost, 0, len(auctions)+len(requests))
	for _, a := range auctions {
		posts = append(posts, sellToPending(a))
	}
	for _, r := range requests {
		posts = append(posts, buyToPending(r))
	}
	return posts, sellTotal + buyTotal, nil
}

// ListPublished backs the admin's "published" tab, where a live post can be
// suspended and a suspended one restored.
func (s *ModerationService) ListPublished(ctx context.Context, page, pageSize int32) ([]PendingPost, int64, error) {
	limit, offset := paginate(page, pageSize)

	auctions, err := s.queries.ListModeratableSellAuctions(ctx, sqlc.ListModeratableSellAuctionsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list published sell auctions: %w", err)
	}
	requests, err := s.queries.ListModeratableBuyRequests(ctx, sqlc.ListModeratableBuyRequestsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list published buy requests: %w", err)
	}

	sellTotal, err := s.queries.CountModeratableSellAuctions(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count published sell auctions: %w", err)
	}
	buyTotal, err := s.queries.CountModeratableBuyRequests(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count published buy requests: %w", err)
	}

	posts := make([]PendingPost, 0, len(auctions)+len(requests))
	for _, a := range auctions {
		posts = append(posts, sellToPending(a))
	}
	for _, r := range requests {
		posts = append(posts, buyToPending(r))
	}
	return posts, sellTotal + buyTotal, nil
}

// Approve publishes a pending post, or restores a suspended one. Either way the
// deal starts a fresh full-length clock from this moment — a post that waited in
// the queue would otherwise go live already half expired.
func (s *ModerationService) Approve(ctx context.Context, adminID int32, postType PostType, publicID uuid.UUID) error {
	pgID := pgtype.UUID{Bytes: publicID, Valid: true}

	if postType == PostTypeSell {
		a, err := s.queries.ApproveSellAuction(ctx, sqlc.ApproveSellAuctionParams{
			PublicID: pgID, AdminID: adminID, DurationHours: int32(s.auctionDurationHours),
		})
		if err != nil {
			return wrapModerationErr(err, "approve sell auction")
		}
		s.notify(ctx, a.OwnerID, notifModel.EventPostApproved, "", map[string]string{
			"type": "new_sell_auction", "auction_id": a.PublicID.String(),
		})
		return nil
	}

	r, err := s.queries.ApproveBuyRequest(ctx, sqlc.ApproveBuyRequestParams{
		PublicID: pgID, AdminID: adminID, DurationHours: int32(s.auctionDurationHours),
	})
	if err != nil {
		return wrapModerationErr(err, "approve buy request")
	}
	s.notify(ctx, r.OwnerID, notifModel.EventPostApproved, "", map[string]string{
		"type": "new_buy_request", "request_id": r.PublicID.String(),
	})
	return nil
}

func (s *ModerationService) Reject(ctx context.Context, adminID int32, postType PostType, publicID uuid.UUID, reason string) error {
	if reason == "" {
		return errs.ErrModerationReasonRequired
	}
	pgID := pgtype.UUID{Bytes: publicID, Valid: true}

	if postType == PostTypeSell {
		a, err := s.queries.RejectSellAuction(ctx, sqlc.RejectSellAuctionParams{PublicID: pgID, AdminID: adminID, Reason: reason})
		if err != nil {
			return wrapModerationErr(err, "reject sell auction")
		}
		// Carry the post id: the owner taps the notification to go and fix the
		// post, and without an id there is nowhere for it to lead. Their own
		// non-active posts are visible to them on the detail page.
		s.notify(ctx, a.OwnerID, notifModel.EventPostRejected, reason, map[string]string{
			"type": "post_rejected", "auction_id": a.PublicID.String(),
		})
		return nil
	}

	r, err := s.queries.RejectBuyRequest(ctx, sqlc.RejectBuyRequestParams{PublicID: pgID, AdminID: adminID, Reason: reason})
	if err != nil {
		return wrapModerationErr(err, "reject buy request")
	}
	s.notify(ctx, r.OwnerID, notifModel.EventPostRejected, reason, map[string]string{
		"type": "post_rejected", "request_id": r.PublicID.String(),
	})
	return nil
}

// Suspend takes a live post down. Bidders get their active-bid slot back for
// free: the slot count only joins posts whose status is 'active'.
func (s *ModerationService) Suspend(ctx context.Context, adminID int32, postType PostType, publicID uuid.UUID, reason string) error {
	if reason == "" {
		return errs.ErrModerationReasonRequired
	}
	pgID := pgtype.UUID{Bytes: publicID, Valid: true}

	if postType == PostTypeSell {
		a, err := s.queries.SuspendSellAuction(ctx, sqlc.SuspendSellAuctionParams{PublicID: pgID, AdminID: adminID, Reason: reason})
		if err != nil {
			return wrapModerationErr(err, "suspend sell auction")
		}
		s.notify(ctx, a.OwnerID, notifModel.EventPostSuspended, reason, map[string]string{
			"type": "post_suspended", "auction_id": a.PublicID.String(),
		})
		return nil
	}

	r, err := s.queries.SuspendBuyRequest(ctx, sqlc.SuspendBuyRequestParams{PublicID: pgID, AdminID: adminID, Reason: reason})
	if err != nil {
		return wrapModerationErr(err, "suspend buy request")
	}
	s.notify(ctx, r.OwnerID, notifModel.EventPostSuspended, reason, map[string]string{
		"type": "post_suspended", "request_id": r.PublicID.String(),
	})
	return nil
}

// notify sends the event's standard copy, appending the admin's reason when
// there is one so the owner knows what to fix.
func (s *ModerationService) notify(ctx context.Context, ownerID int32, event notifModel.EventType, reason string, data map[string]string) {
	if reason == "" {
		s.notificationSender.Send(ctx, ownerID, event, data)
		return
	}
	msg := notifModel.NotificationMessages[event]
	s.notificationSender.SendToUser(ctx, ownerID, msg.Title, msg.Body+": "+reason, data)
}

// wrapModerationErr turns "no row updated" into a clear conflict: the post was
// either not found or is not in a state this action applies to.
func wrapModerationErr(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrPostNotModeratable
	}
	return fmt.Errorf("%s: %w", op, err)
}

func paginate(page, pageSize int32) (limit, offset int32) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return pageSize, (page - 1) * pageSize
}

func sellToPending(a sqlc.SellAuction) PendingPost {
	p := PendingPost{
		PublicID: a.PublicID.String(), Type: string(PostTypeSell), Title: a.Title,
		ImageURL: a.ImageUrl, OwnerName: a.OwnerName, RegionName: a.RegionName,
		InterestName: a.InterestName, Unit: a.Unit,
		Quantity:  numericToString(a.Quantity),
		UnitPrice: numericToString(a.UnitPrice),
		Status:    a.Status,
		CreatedAt: a.CreatedAt.Time.Format(time.RFC3339),
	}
	if a.Description.Valid {
		p.Description = &a.Description.String
	}
	if a.ModerationReason.Valid {
		p.Reason = &a.ModerationReason.String
	}
	return p
}

func buyToPending(r sqlc.BuyRequest) PendingPost {
	p := PendingPost{
		PublicID: r.PublicID.String(), Type: string(PostTypeBuy), Title: r.Title,
		ImageURL: r.ImageUrl, OwnerName: r.OwnerName, RegionName: r.RegionName,
		InterestName: r.InterestName, Unit: r.Unit,
		Quantity:  numericToString(r.Quantity),
		Status:    r.Status,
		CreatedAt: r.CreatedAt.Time.Format(time.RFC3339),
	}
	if r.Description.Valid {
		p.Description = &r.Description.String
	}
	if r.ModerationReason.Valid {
		p.Reason = &r.ModerationReason.String
	}
	return p
}

var _ = model.SellAuctionResponse{}
