package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
	"rabhana/pkg/errs"
)

// quoteTarget is the job a quote points at, resolved once so the rest of the
// service does not branch on three kinds at every step.
//
// notifyIDs is who hears about the quote: a post has one owner, an order has two
// parties and either of them may be the one arranging transport.
type quoteTarget struct {
	kind          string
	publicID      uuid.UUID
	orderID       pgtype.Int4
	sellAuctionID pgtype.Int4
	buyRequestID  pgtype.Int4
	notifyIDs     []int32
}

// notificationData gives the client enough to route a tap to the job. The keys
// match what buildLink and the frontend resolver already read for other events.
func (t quoteTarget) notificationData() map[string]string {
	data := map[string]string{}
	switch t.kind {
	case KindOrder:
		data["order_id"] = t.publicID.String()
	case KindSellAuction:
		data["auction_id"] = t.publicID.String()
	case KindBuyRequest:
		data["request_id"] = t.publicID.String()
	}
	return data
}

// resolveTarget looks a job up for a carrier about to quote it. It enforces the
// admin's stage setting as well as the job's own state: the UI hides what cannot
// be quoted, but the endpoint has to refuse it too.
func (s *Service) resolveTarget(ctx context.Context, kind string, publicID uuid.UUID) (*quoteTarget, error) {
	pgID := pgtype.UUID{Bytes: publicID, Valid: true}

	switch kind {
	case KindOrder:
		if !s.settings.QuotesOnOrders() {
			return nil, errs.ErrQuoteStageDisabled
		}
		order, err := s.queries.GetOrderByPublicID(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errs.ErrJobNotQuotable
			}
			return nil, fmt.Errorf("failed to get order: %w", err)
		}
		// A cancelled deal is not moving, and one that already has a carrier is
		// settled. Quoting either would waste the carrier's time.
		if order.Status == "cancelled" || order.CarrierID.Valid {
			return nil, errs.ErrJobNotQuotable
		}
		return &quoteTarget{
			kind:      KindOrder,
			publicID:  publicID,
			orderID:   pgtype.Int4{Int32: order.ID, Valid: true},
			notifyIDs: []int32{order.SellerID, order.BuyerID},
		}, nil

	case KindSellAuction:
		if !s.settings.QuotesOnPosts() {
			return nil, errs.ErrQuoteStageDisabled
		}
		auction, err := s.queries.GetSellAuctionByPublicID(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errs.ErrJobNotQuotable
			}
			return nil, fmt.Errorf("failed to get sell auction: %w", err)
		}
		if auction.Status != "active" {
			return nil, errs.ErrJobNotQuotable
		}
		return &quoteTarget{
			kind:          KindSellAuction,
			publicID:      publicID,
			sellAuctionID: pgtype.Int4{Int32: auction.ID, Valid: true},
			notifyIDs:     []int32{auction.OwnerID},
		}, nil

	case KindBuyRequest:
		if !s.settings.QuotesOnPosts() {
			return nil, errs.ErrQuoteStageDisabled
		}
		request, err := s.queries.GetBuyRequestByPublicID(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errs.ErrJobNotQuotable
			}
			return nil, fmt.Errorf("failed to get buy request: %w", err)
		}
		if request.Status != "active" {
			return nil, errs.ErrJobNotQuotable
		}
		return &quoteTarget{
			kind:         KindBuyRequest,
			publicID:     publicID,
			buyRequestID: pgtype.Int4{Int32: request.ID, Valid: true},
			notifyIDs:    []int32{request.OwnerID},
		}, nil
	}

	return nil, errs.ErrInvalidPostType
}

// resolveTargetForOwner is the same lookup with an ownership check, for the
// merchant endpoints. It deliberately does not check the stage: a merchant must
// still be able to see and answer quotes that arrived before the admin changed
// the setting.
func (s *Service) resolveTargetForOwner(ctx context.Context, kind string, publicID uuid.UUID, userID int32) (*quoteTarget, error) {
	pgID := pgtype.UUID{Bytes: publicID, Valid: true}

	switch kind {
	case KindOrder:
		order, err := s.queries.GetOrderByPublicID(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errs.ErrQuoteNotFound
			}
			return nil, fmt.Errorf("failed to get order: %w", err)
		}
		// Either party to the deal: both see the order screen, and either could be
		// the one who arranges the transport.
		if order.SellerID != userID && order.BuyerID != userID {
			return nil, errs.ErrQuoteNotFound
		}
		return &quoteTarget{
			kind:      KindOrder,
			publicID:  publicID,
			orderID:   pgtype.Int4{Int32: order.ID, Valid: true},
			notifyIDs: []int32{order.SellerID, order.BuyerID},
		}, nil

	case KindSellAuction:
		auction, err := s.queries.GetSellAuctionByPublicID(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errs.ErrQuoteNotFound
			}
			return nil, fmt.Errorf("failed to get sell auction: %w", err)
		}
		if auction.OwnerID != userID {
			return nil, errs.ErrQuoteNotFound
		}
		return &quoteTarget{
			kind:          KindSellAuction,
			publicID:      publicID,
			sellAuctionID: pgtype.Int4{Int32: auction.ID, Valid: true},
			notifyIDs:     []int32{auction.OwnerID},
		}, nil

	case KindBuyRequest:
		request, err := s.queries.GetBuyRequestByPublicID(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errs.ErrQuoteNotFound
			}
			return nil, fmt.Errorf("failed to get buy request: %w", err)
		}
		if request.OwnerID != userID {
			return nil, errs.ErrQuoteNotFound
		}
		return &quoteTarget{
			kind:         KindBuyRequest,
			publicID:     publicID,
			buyRequestID: pgtype.Int4{Int32: request.ID, Valid: true},
			notifyIDs:    []int32{request.OwnerID},
		}, nil
	}

	return nil, errs.ErrInvalidPostType
}

// targetOfQuote works backwards from a quote row to the job it points at, and
// checks the caller owns that job. Used by accept and reject, where the client
// sends only the quote id.
func (s *Service) targetOfQuote(ctx context.Context, quote sqlc.ShippingQuote, userID int32) (*quoteTarget, error) {
	switch {
	case quote.OrderID.Valid:
		order, err := s.queries.GetOrderByID(ctx, quote.OrderID.Int32)
		if err != nil {
			return nil, fmt.Errorf("failed to get order: %w", err)
		}
		if order.SellerID != userID && order.BuyerID != userID {
			return nil, errs.ErrQuoteNotFound
		}
		return &quoteTarget{
			kind:      KindOrder,
			publicID:  uuid.UUID(order.PublicID.Bytes),
			orderID:   quote.OrderID,
			notifyIDs: []int32{order.SellerID, order.BuyerID},
		}, nil

	case quote.SellAuctionID.Valid:
		auction, err := s.queries.GetSellAuctionByID(ctx, quote.SellAuctionID.Int32)
		if err != nil {
			return nil, fmt.Errorf("failed to get sell auction: %w", err)
		}
		if auction.OwnerID != userID {
			return nil, errs.ErrQuoteNotFound
		}
		return &quoteTarget{
			kind:          KindSellAuction,
			publicID:      uuid.UUID(auction.PublicID.Bytes),
			sellAuctionID: quote.SellAuctionID,
			notifyIDs:     []int32{auction.OwnerID},
		}, nil

	case quote.BuyRequestID.Valid:
		request, err := s.queries.GetBuyRequestByID(ctx, quote.BuyRequestID.Int32)
		if err != nil {
			return nil, fmt.Errorf("failed to get buy request: %w", err)
		}
		if request.OwnerID != userID {
			return nil, errs.ErrQuoteNotFound
		}
		return &quoteTarget{
			kind:         KindBuyRequest,
			publicID:     uuid.UUID(request.PublicID.Bytes),
			buyRequestID: quote.BuyRequestID,
			notifyIDs:    []int32{request.OwnerID},
		}, nil
	}

	// The CHECK constraint on shipping_quotes makes this unreachable.
	return nil, errs.ErrQuoteNotFound
}

// quoteRow flattens the three near-identical merchant list queries into one
// shape, so ListQuotesForJob has a single loop rather than three.
type quoteRow struct {
	publicID     pgtype.UUID
	carrierName  string
	carrierPhone string
	carrierLogo  *string
	carrierNotes *string
	price        string
	notes        *string
	status       string
	createdAt    string
}

func (s *Service) listQuoteRows(ctx context.Context, target *quoteTarget) ([]quoteRow, error) {
	var out []quoteRow

	switch target.kind {
	case KindOrder:
		rows, err := s.queries.ListShippingQuotesForOrder(ctx, target.orderID)
		if err != nil {
			return nil, fmt.Errorf("failed to list quotes for order: %w", err)
		}
		for _, r := range rows {
			out = append(out, quoteRow{
				publicID: r.PublicID, carrierName: r.CarrierName, carrierPhone: r.CarrierPhone.String,
				carrierLogo: textPtr(r.CarrierLogo), carrierNotes: textPtr(r.CarrierNotes),
				price: numericString(r.Price), notes: textPtr(r.Notes), status: r.Status,
				createdAt: timeString(r.CreatedAt),
			})
		}
	case KindSellAuction:
		rows, err := s.queries.ListShippingQuotesForSellAuction(ctx, target.sellAuctionID)
		if err != nil {
			return nil, fmt.Errorf("failed to list quotes for sell auction: %w", err)
		}
		for _, r := range rows {
			out = append(out, quoteRow{
				publicID: r.PublicID, carrierName: r.CarrierName, carrierPhone: r.CarrierPhone.String,
				carrierLogo: textPtr(r.CarrierLogo), carrierNotes: textPtr(r.CarrierNotes),
				price: numericString(r.Price), notes: textPtr(r.Notes), status: r.Status,
				createdAt: timeString(r.CreatedAt),
			})
		}
	case KindBuyRequest:
		rows, err := s.queries.ListShippingQuotesForBuyRequest(ctx, target.buyRequestID)
		if err != nil {
			return nil, fmt.Errorf("failed to list quotes for buy request: %w", err)
		}
		for _, r := range rows {
			out = append(out, quoteRow{
				publicID: r.PublicID, carrierName: r.CarrierName, carrierPhone: r.CarrierPhone.String,
				carrierLogo: textPtr(r.CarrierLogo), carrierNotes: textPtr(r.CarrierNotes),
				price: numericString(r.Price), notes: textPtr(r.Notes), status: r.Status,
				createdAt: timeString(r.CreatedAt),
			})
		}
	}
	return out, nil
}
