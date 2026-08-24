// Package service implements carrier accounts and shipping quotes (#14).
//
// The shape mirrors the bidding services in auction/service: a carrier submits a
// price on a job, the merchant who owns that job accepts one, and accepting
// answers the rest.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"rabhana/db/sqlc"
	notifModel "rabhana/notification/model"
	"rabhana/pkg/errs"
	settingsSvc "rabhana/settings/service"
	"rabhana/shipping/model"
)

// Job kinds, matching the three targets a quote can point at.
const (
	KindOrder       = "order"
	KindSellAuction = "sell_auction"
	KindBuyRequest  = "buy_request"
)

type NotificationSender interface {
	Send(ctx context.Context, userID int32, event notifModel.EventType, data map[string]string)
}

type Service struct {
	queries  *sqlc.Queries
	settings *settingsSvc.Service
	notifier NotificationSender
}

func NewService(queries *sqlc.Queries, settings *settingsSvc.Service, notifier NotificationSender) *Service {
	return &Service{queries: queries, settings: settings, notifier: notifier}
}

// ---------------------------------------------------------------- carrier side

// ListJobs returns what this carrier could quote on, which depends on both the
// governorates it covers and the stage the admin has chosen.
func (s *Service) ListJobs(ctx context.Context, carrierID int32, limit, offset int32) (*model.CarrierJobsResponse, error) {
	regionIDs, err := s.queries.ListCarrierRegionIDs(ctx, carrierID)
	if err != nil {
		return nil, fmt.Errorf("failed to list carrier regions: %w", err)
	}
	stage := s.settings.CarrierQuoteStage()
	out := &model.CarrierJobsResponse{Jobs: []model.CarrierJob{}, Stage: stage}

	// No coverage means no jobs. Registration requires at least one governorate,
	// so this is the case where a carrier has cleared its own coverage.
	if len(regionIDs) == 0 {
		return out, nil
	}

	// The page is filled from up to three queries, so the caller's page size is
	// split between them. Without this, page_size=20 in "both" mode returns 60
	// rows and every client's paging arithmetic is wrong.
	perKind := limit
	if kinds := s.activeKindCount(); kinds > 1 {
		perKind = limit / int32(kinds)
		if perKind < 1 {
			perKind = 1
		}
	}

	if s.settings.QuotesOnOrders() {
		rows, err := s.queries.ListQuotableOrdersForCarrier(ctx, sqlc.ListQuotableOrdersForCarrierParams{
			Limit: perKind, Offset: offset, CarrierID: carrierID, RegionIds: regionIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list quotable orders: %w", err)
		}
		total, err := s.queries.CountQuotableOrdersForCarrier(ctx, regionIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to count quotable orders: %w", err)
		}
		out.Total += total
		for _, r := range rows {
			toRegion := r.BuyerRegion
			out.Jobs = append(out.Jobs, model.CarrierJob{
				Kind:          KindOrder,
				PublicID:      uuidString(r.PublicID),
				Title:         firstText(r.SellTitle, r.BuyTitle),
				InterestName:  firstText(r.SellInterest, r.BuyInterest),
				Quantity:      numericString(r.Quantity),
				Unit:          r.Unit,
				FromRegion:    r.SellerRegion,
				ToRegion:      &toRegion,
				CreatedAt:     timeString(r.CreatedAt),
				AlreadyQuoted: r.AlreadyQuoted,
			})
		}
	}

	if s.settings.QuotesOnPosts() {
		sells, err := s.queries.ListQuotableSellAuctionsForCarrier(ctx, sqlc.ListQuotableSellAuctionsForCarrierParams{
			Limit: perKind, Offset: offset, CarrierID: carrierID, RegionIds: regionIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list quotable sell auctions: %w", err)
		}
		sellTotal, err := s.queries.CountQuotableSellAuctionsForCarrier(ctx, regionIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to count quotable sell auctions: %w", err)
		}
		out.Total += sellTotal
		for _, r := range sells {
			deadline := timeString(r.EndTime)
			out.Jobs = append(out.Jobs, model.CarrierJob{
				Kind:         KindSellAuction,
				PublicID:     uuidString(r.PublicID),
				Title:        r.Title,
				InterestName: r.InterestName,
				Quantity:     numericString(r.Quantity),
				Unit:         r.Unit,
				FromRegion:   r.RegionName,
				// No destination: the winning bidder does not exist yet, which is
				// why a post-stage price is indicative.
				Deadline:      &deadline,
				CreatedAt:     timeString(r.CreatedAt),
				AlreadyQuoted: r.AlreadyQuoted,
			})
		}

		buys, err := s.queries.ListQuotableBuyRequestsForCarrier(ctx, sqlc.ListQuotableBuyRequestsForCarrierParams{
			Limit: perKind, Offset: offset, CarrierID: carrierID, RegionIds: regionIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list quotable buy requests: %w", err)
		}
		buyTotal, err := s.queries.CountQuotableBuyRequestsForCarrier(ctx, regionIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to count quotable buy requests: %w", err)
		}
		out.Total += buyTotal
		for _, r := range buys {
			deadline := timeString(r.EndTime)
			out.Jobs = append(out.Jobs, model.CarrierJob{
				Kind:          KindBuyRequest,
				PublicID:      uuidString(r.PublicID),
				Title:         r.Title,
				InterestName:  r.InterestName,
				Quantity:      numericString(r.Quantity),
				Unit:          r.Unit,
				FromRegion:    r.RegionName,
				Deadline:      &deadline,
				CreatedAt:     timeString(r.CreatedAt),
				AlreadyQuoted: r.AlreadyQuoted,
			})
		}
	}

	return out, nil
}

// activeKindCount is how many job kinds the current stage puts in one page:
// orders, or both post kinds, or all three.
func (s *Service) activeKindCount() int {
	n := 0
	if s.settings.QuotesOnOrders() {
		n++
	}
	if s.settings.QuotesOnPosts() {
		n += 2 // sell posts and buy requests are separate lists
	}
	return n
}

// CreateQuote records a carrier's price for one job and tells the merchant.
//
// The job is resolved from its public id and validated against the current
// stage: a carrier must not be able to quote an order while the platform is in
// post mode by calling the endpoint directly.
func (s *Service) CreateQuote(ctx context.Context, carrierID int32, kind string, jobPublicID uuid.UUID, req model.CreateQuoteRequest) (*model.Quote, error) {
	// The client asked for carriers to be vetted, and registration parks them in
	// pending_review — but middleware.AccountStatus only turns away banned and
	// suspended accounts, so without this check an unapproved carrier could quote
	// on real deals the moment it signed up. Browsing stays open: seeing the work
	// is what makes waiting for approval worth it.
	carrier, err := s.queries.GetUserByID(ctx, carrierID)
	if err != nil {
		return nil, fmt.Errorf("failed to get carrier: %w", err)
	}
	if carrier.Status != "active" {
		return nil, errs.ErrPendingReview
	}

	target, err := s.resolveTarget(ctx, kind, jobPublicID)
	if err != nil {
		return nil, err
	}

	price := decimal.NewFromFloat(req.Price)
	var priceNumeric pgtype.Numeric
	if err := priceNumeric.Scan(price.StringFixed(2)); err != nil {
		return nil, fmt.Errorf("failed to convert price: %w", err)
	}

	notes := pgtype.Text{}
	if req.Notes != nil && *req.Notes != "" {
		notes = pgtype.Text{String: *req.Notes, Valid: true}
	}

	quote, err := s.queries.CreateShippingQuote(ctx, sqlc.CreateShippingQuoteParams{
		CarrierID:     carrierID,
		SellAuctionID: target.sellAuctionID,
		BuyRequestID:  target.buyRequestID,
		OrderID:       target.orderID,
		Price:         priceNumeric,
		Notes:         notes,
	})
	if err != nil {
		// The partial unique indexes are the real guard against double quoting,
		// so a duplicate arrives as a constraint violation rather than a race we
		// lost.
		if isUniqueViolation(err) {
			return nil, errs.ErrAlreadyQuoted
		}
		return nil, fmt.Errorf("failed to create shipping quote: %w", err)
	}

	data := target.notificationData()
	for _, id := range target.notifyIDs {
		s.notify(ctx, id, notifModel.EventShippingQuoteReceived, data)
	}

	return &model.Quote{
		PublicID:  uuidString(quote.PublicID),
		JobKind:   kind,
		JobID:     jobPublicID.String(),
		Price:     numericString(quote.Price),
		Notes:     textPtr(quote.Notes),
		Status:    quote.Status,
		CreatedAt: timeString(quote.CreatedAt),
	}, nil
}

// ListMyQuotes is the carrier's own record of what it has offered.
func (s *Service) ListMyQuotes(ctx context.Context, carrierID int32, limit, offset int32) (*model.QuotesResponse, error) {
	rows, err := s.queries.ListShippingQuotesByCarrier(ctx, sqlc.ListShippingQuotesByCarrierParams{
		CarrierID: carrierID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list carrier quotes: %w", err)
	}
	total, err := s.queries.CountShippingQuotesByCarrier(ctx, carrierID)
	if err != nil {
		return nil, fmt.Errorf("failed to count carrier quotes: %w", err)
	}

	out := &model.QuotesResponse{Quotes: []model.Quote{}, Total: total}
	for _, r := range rows {
		kind, jobID := KindOrder, uuidString(r.OrderPublicID)
		switch {
		case r.SellAuctionID.Valid:
			kind, jobID = KindSellAuction, uuidString(r.SellAuctionPublicID)
		case r.BuyRequestID.Valid:
			kind, jobID = KindBuyRequest, uuidString(r.BuyRequestPublicID)
		}
		out.Quotes = append(out.Quotes, model.Quote{
			PublicID:  uuidString(r.PublicID),
			JobKind:   kind,
			JobID:     jobID,
			JobTitle:  r.JobTitle,
			JobRegion: r.JobRegion,
			Price:     numericString(r.Price),
			Notes:     textPtr(r.Notes),
			Status:    r.Status,
			CreatedAt: timeString(r.CreatedAt),
		})
	}
	return out, nil
}

// WithdrawQuote lets a carrier take back a price nobody has answered yet.
func (s *Service) WithdrawQuote(ctx context.Context, carrierID int32, quotePublicID uuid.UUID) error {
	quote, err := s.queries.GetShippingQuoteByPublicID(ctx, pgtype.UUID{Bytes: quotePublicID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrQuoteNotFound
		}
		return fmt.Errorf("failed to get quote: %w", err)
	}
	// Not found rather than forbidden: one carrier has no business learning that
	// another carrier's quote exists.
	if quote.CarrierID != carrierID {
		return errs.ErrQuoteNotFound
	}
	if _, err := s.queries.WithdrawShippingQuote(ctx, quote.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrQuoteNotPending
		}
		return fmt.Errorf("failed to withdraw quote: %w", err)
	}
	return nil
}

// GetProfile returns a carrier's own details and coverage.
func (s *Service) GetProfile(ctx context.Context, carrierID int32) (*model.CarrierProfile, error) {
	user, err := s.queries.GetUserByID(ctx, carrierID)
	if err != nil {
		return nil, fmt.Errorf("failed to get carrier: %w", err)
	}
	regions, err := s.queries.ListCarrierRegions(ctx, carrierID)
	if err != nil {
		return nil, fmt.Errorf("failed to list carrier regions: %w", err)
	}

	out := &model.CarrierProfile{
		Name:      user.Name,
		Phone:     user.Phone.String,
		RegionIDs: []int32{},
		Regions:   []string{},
	}
	for _, r := range regions {
		out.RegionIDs = append(out.RegionIDs, r.RegionID)
		out.Regions = append(out.Regions, r.NameAr)
	}

	profile, err := s.queries.GetCarrierProfile(ctx, carrierID)
	if err == nil {
		out.LogoURL = textPtr(profile.LogoUrl)
		out.Notes = textPtr(profile.Notes)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to get carrier profile: %w", err)
	}
	return out, nil
}

// UpdateProfile replaces coverage wholesale — the form always sends the complete
// set, and diffing would leave stale governorates behind.
func (s *Service) UpdateProfile(ctx context.Context, carrierID int32, req model.UpdateCarrierProfileRequest) error {
	for _, regionID := range req.RegionIDs {
		if _, err := s.queries.GetRegionByID(ctx, regionID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.ErrInvalidRegionID
			}
			return fmt.Errorf("failed to validate region: %w", err)
		}
	}
	if err := s.queries.ReplaceCarrierRegions(ctx, carrierID); err != nil {
		return fmt.Errorf("failed to clear carrier regions: %w", err)
	}
	for _, regionID := range req.RegionIDs {
		if err := s.queries.AddCarrierRegion(ctx, sqlc.AddCarrierRegionParams{
			UserID: carrierID, RegionID: regionID,
		}); err != nil {
			return fmt.Errorf("failed to add carrier region: %w", err)
		}
	}

	logo, notes := pgtype.Text{}, pgtype.Text{}
	if req.LogoURL != nil && *req.LogoURL != "" {
		logo = pgtype.Text{String: *req.LogoURL, Valid: true}
	}
	if req.Notes != nil && *req.Notes != "" {
		notes = pgtype.Text{String: *req.Notes, Valid: true}
	}
	if _, err := s.queries.UpsertCarrierProfile(ctx, sqlc.UpsertCarrierProfileParams{
		UserID: carrierID, LogoUrl: logo, Notes: notes,
	}); err != nil {
		return fmt.Errorf("failed to save carrier profile: %w", err)
	}
	return nil
}

// --------------------------------------------------------------- merchant side

// ListQuotesForJob returns the quotes on a job to the merchant who owns it.
func (s *Service) ListQuotesForJob(ctx context.Context, userID int32, kind string, jobPublicID uuid.UUID) (*model.MerchantQuotesResponse, error) {
	target, err := s.resolveTargetForOwner(ctx, kind, jobPublicID, userID)
	if err != nil {
		return nil, err
	}

	out := &model.MerchantQuotesResponse{Quotes: []model.MerchantQuote{}, Stage: s.settings.CarrierQuoteStage()}
	rows, err := s.listQuoteRows(ctx, target)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		q := model.MerchantQuote{
			PublicID:     uuidString(r.publicID),
			CarrierName:  r.carrierName,
			CarrierLogo:  r.carrierLogo,
			CarrierNotes: r.carrierNotes,
			Price:        r.price,
			Notes:        r.notes,
			Status:       r.status,
			CreatedAt:    r.createdAt,
		}
		// Contact details are the reward for accepting, not for looking.
		if r.status == "accepted" {
			phone := r.carrierPhone
			q.CarrierPhone = &phone
		}
		out.Quotes = append(out.Quotes, q)
	}
	return out, nil
}

// AcceptQuote picks a carrier. Accepting answers every other quote on the job,
// and each losing carrier is told — the difference between knowing where you
// stand and being ignored.
func (s *Service) AcceptQuote(ctx context.Context, userID int32, quotePublicID uuid.UUID) error {
	quote, err := s.queries.GetShippingQuoteByPublicID(ctx, pgtype.UUID{Bytes: quotePublicID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrQuoteNotFound
		}
		return fmt.Errorf("failed to get quote: %w", err)
	}

	target, err := s.targetOfQuote(ctx, quote, userID)
	if err != nil {
		return err
	}

	accepted, err := s.queries.AcceptShippingQuote(ctx, quote.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrQuoteNotPending
		}
		return fmt.Errorf("failed to accept quote: %w", err)
	}

	// Record the carrier on the deal when there is a deal to record it on. In
	// post stage there is no order yet, so the accepted quote is the record until
	// one exists.
	if target.orderID.Valid {
		if _, err := s.queries.AttachCarrierToOrder(ctx, sqlc.AttachCarrierToOrderParams{
			ID:            target.orderID.Int32,
			CarrierID:     pgtype.Int4{Int32: quote.CarrierID, Valid: true},
			ShippingPrice: accepted.Price,
		}); err != nil {
			return fmt.Errorf("failed to attach carrier to order: %w", err)
		}
	}

	losers, err := s.queries.RejectSiblingShippingQuotes(ctx, sqlc.RejectSiblingShippingQuotesParams{
		AcceptedID:    quote.ID,
		OrderID:       zeroIfInvalid(target.orderID),
		SellAuctionID: zeroIfInvalid(target.sellAuctionID),
		BuyRequestID:  zeroIfInvalid(target.buyRequestID),
	})
	if err != nil {
		// The winner is already recorded; failing the whole call here would tell
		// the merchant nothing happened when it did.
		slog.Error("failed to reject sibling shipping quotes", "error", err, "quote_id", quote.ID)
	}

	data := target.notificationData()
	s.notify(ctx, quote.CarrierID, notifModel.EventShippingQuoteAccepted, data)
	for _, l := range losers {
		s.notify(ctx, l.CarrierID, notifModel.EventShippingQuoteRejected, data)
	}
	return nil
}

// RejectQuote turns one carrier down without choosing anyone.
func (s *Service) RejectQuote(ctx context.Context, userID int32, quotePublicID uuid.UUID) error {
	quote, err := s.queries.GetShippingQuoteByPublicID(ctx, pgtype.UUID{Bytes: quotePublicID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrQuoteNotFound
		}
		return fmt.Errorf("failed to get quote: %w", err)
	}
	target, err := s.targetOfQuote(ctx, quote, userID)
	if err != nil {
		return err
	}
	if _, err := s.queries.RejectShippingQuote(ctx, quote.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrQuoteNotPending
		}
		return fmt.Errorf("failed to reject quote: %w", err)
	}
	s.notify(ctx, quote.CarrierID, notifModel.EventShippingQuoteRejected, target.notificationData())
	return nil
}

func (s *Service) notify(ctx context.Context, userID int32, event notifModel.EventType, data map[string]string) {
	if s.notifier == nil {
		return
	}
	s.notifier.Send(ctx, userID, event, data)
}

// ------------------------------------------------------------------- internals

func timeString(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func numericString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0"
	}
	v, err := n.Value()
	if err != nil {
		return "0"
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return s
	}
	return d.StringFixed(2)
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid || t.String == "" {
		return nil
	}
	v := t.String
	return &v
}

func firstText(values ...pgtype.Text) string {
	for _, v := range values {
		if v.Valid && v.String != "" {
			return v.String
		}
	}
	return ""
}

func zeroIfInvalid(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

// isUniqueViolation spots the partial unique indexes on shipping_quotes firing,
// which is how a second quote from the same carrier is refused.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
