package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	auctionRepo "rabhana/auction/repository"
	"rabhana/auction/service"
	"rabhana/db/sqlc"
	"rabhana/order/model"
	"rabhana/order/repository"
	notifModel "rabhana/notification/model"
	"rabhana/pkg/errs"
)

type OrderService struct {
	orderRepo          repository.OrderRepository
	sellAuctionRepo    auctionRepo.SellAuctionRepository
	buyRequestRepo     auctionRepo.BuyRequestRepository
	notificationSender service.NotificationSender
}

func NewOrderService(
	orderRepo repository.OrderRepository,
	sellAuctionRepo auctionRepo.SellAuctionRepository,
	buyRequestRepo auctionRepo.BuyRequestRepository,
	notificationSender service.NotificationSender,
) *OrderService {
	return &OrderService{
		orderRepo:          orderRepo,
		sellAuctionRepo:    sellAuctionRepo,
		buyRequestRepo:     buyRequestRepo,
		notificationSender: notificationSender,
	}
}

func (s *OrderService) ConfirmOrder(ctx context.Context, userID int32, orderPublicID uuid.UUID) error {
	order, err := s.orderRepo.GetByPublicID(ctx, orderPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrOrderNotFound
		}
		return fmt.Errorf("failed to get order: %w", err)
	}

	if order.SellerID != userID && order.BuyerID != userID {
		return errs.ErrNotOrderParticipant
	}

	// Check if order is already cancelled
	if order.Status == "cancelled" {
		return errs.ErrOrderAlreadyExpired
	}

	// Check if deadline has expired
	if order.ConfirmationDeadline.Valid && order.ConfirmationDeadline.Time.Before(time.Now()) {
		return errs.ErrOrderConfirmationExpired
	}

	// Check if already completed
	if order.Status == "completed" {
		return errs.ErrAlreadyConfirmed
	}

	isSeller := order.SellerID == userID
	isBuyer := order.BuyerID == userID

	// Simplified confirmation logic - only one party needs to confirm
	if isBuyer && order.Status == "seller_confirmed" {
		// Buyer confirming when seller already confirmed - complete the order
		if err := s.orderRepo.ConfirmAsBuyer(ctx, order.ID); err != nil {
			return fmt.Errorf("failed to confirm as buyer: %w", err)
		}
	} else if isSeller && order.Status == "buyer_confirmed" {
		// Seller confirming when buyer already confirmed - complete the order
		if err := s.orderRepo.ConfirmAsSeller(ctx, order.ID); err != nil {
			return fmt.Errorf("failed to confirm as seller: %w", err)
		}
	} else {
		// This shouldn't happen with the new flow (orders start pre-confirmed)
		// but handle legacy cases or edge cases
		return errs.ErrAlreadyConfirmed
	}

	// Notify the other party that order is now completed
	otherPartyID := order.SellerID
	if isSeller {
		otherPartyID = order.BuyerID
	}

	s.notificationSender.Send(ctx, otherPartyID, notifModel.EventOrderCompleted, map[string]string{
		"order_id": order.PublicID.String(),
	})

	return nil
}

func (s *OrderService) CancelExpiredOrders(ctx context.Context) error {
	expiredOrders, err := s.orderRepo.GetOrdersPendingConfirmation(ctx)
	if err != nil {
		return fmt.Errorf("failed to get expired orders: %w", err)
	}

	for _, order := range expiredOrders {
		// Cancel the order
		if err := s.orderRepo.CancelOrder(ctx, order.ID); err != nil {
			// Log error but continue processing other orders
			fmt.Printf("Failed to cancel expired order %d: %v\n", order.ID, err)
			continue
		}

		// Revert auction/request to pending_selection
		if order.SellAuctionID.Valid {
			if err := s.sellAuctionRepo.RevertToPendingSelection(ctx, order.SellAuctionID.Int32); err != nil {
				fmt.Printf("Failed to revert sell auction %d: %v\n", order.SellAuctionID.Int32, err)
			}
		}

		if order.BuyRequestID.Valid {
			if err := s.buyRequestRepo.RevertToPendingSelection(ctx, order.BuyRequestID.Int32); err != nil {
				fmt.Printf("Failed to revert buy request %d: %v\n", order.BuyRequestID.Int32, err)
			}
		}

		// Notify both parties about order expiration
		s.notificationSender.Send(ctx, order.SellerID, notifModel.EventOrderExpired, map[string]string{
			"order_id": order.PublicID.String(),
		})

		s.notificationSender.Send(ctx, order.BuyerID, notifModel.EventOrderExpired, map[string]string{
			"order_id": order.PublicID.String(),
		})

		// Send special notification to the party that DID confirm (they need to know it expired)
		var confirmedPartyID int32
		var confirmedMessage string

		if order.Status == "seller_confirmed" {
			confirmedPartyID = order.SellerID
			confirmedMessage = "تم إلغاء الطلب لعدم تأكيد المشتري خلال 30 دقيقة"
		} else if order.Status == "buyer_confirmed" {
			confirmedPartyID = order.BuyerID
			confirmedMessage = "تم إلغاء الطلب لعدم تأكيد البائع خلال 30 دقيقة"
		}

		if confirmedPartyID > 0 {
			s.notificationSender.SendToUser(ctx, confirmedPartyID, "إلغاء الطلب", confirmedMessage, map[string]string{
				"order_id": order.PublicID.String(),
				"type":     "order_cancelled_timeout",
			})
		}
	}

	return nil
}

func (s *OrderService) GetOrderDetail(ctx context.Context, userID int32, orderPublicID uuid.UUID) (*model.OrderResponse, error) {
	order, err := s.orderRepo.GetByPublicID(ctx, orderPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	if order.SellerID != userID && order.BuyerID != userID {
		return nil, errs.ErrNotOrderParticipant
	}

	return s.toResponse(order, userID), nil
}

func (s *OrderService) ListMyOrders(ctx context.Context, userID int32, page, pageSize int32) ([]model.OrderResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	orders, err := s.orderRepo.ListByUser(ctx, sqlc.ListOrdersByUserParams{
		SellerID: userID,
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list orders: %w", err)
	}

	total, err := s.orderRepo.CountByUser(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	var responses []model.OrderResponse
	for _, order := range orders {
		responses = append(responses, *s.toResponse(order, userID))
	}

	return responses, total, nil
}

func (s *OrderService) toResponse(order sqlc.Order, requestingUserID int32) *model.OrderResponse {
	iAmSeller := order.SellerID == requestingUserID
	iAmBuyer := order.BuyerID == requestingUserID

	sourceType := "sell_auction"
	if order.BuyRequestID.Valid {
		sourceType = "buy_request"
	}

	sourceID := ""
	if order.SourcePublicID.Valid {
		sourceID = order.SourcePublicID.String()
	}

	finalPrice := numericToString(order.FinalPrice)
	quantity := numericToString(order.Quantity)

	unitPrice := "0"
	if quantity != "0" {
		decFinalPrice, _ := decimal.NewFromString(finalPrice)
		decQuantity, _ := decimal.NewFromString(quantity)
		if decQuantity.IsPositive() {
			unitPriceDec := decFinalPrice.Div(decQuantity)
			unitPrice = unitPriceDec.String()
		}
	}

	var confirmationDeadline *string
	if order.ConfirmationDeadline.Valid {
		deadline := order.ConfirmationDeadline.Time.Format(time.RFC3339)
		confirmationDeadline = &deadline
	}

	sellerName := order.SellerName
	sellerPhone := order.SellerPhone
	sellerRegion := order.SellerRegion
	buyerName := order.BuyerName
	buyerPhone := order.BuyerPhone
	buyerRegion := order.BuyerRegion

	var maskedMessage *string

	if order.Status == "seller_confirmed" || order.Status == "buyer_confirmed" {
		if !iAmSeller {
			sellerName = ""
			sellerPhone = ""
			sellerRegion = ""
			msg := "سيتم عرض معلومات البائع بعد تأكيد الطلب"
			maskedMessage = &msg
		}
		if !iAmBuyer {
			buyerName = ""
			buyerPhone = ""
			buyerRegion = ""
			msg := "سيتم عرض معلومات المشتري بعد تأكيد الطلب"
			maskedMessage = &msg
		}
	}

	return &model.OrderResponse{
		PublicID:             order.PublicID.String(),
		SourceType:           sourceType,
		SourceID:             sourceID,
		SellerName:           sellerName,
		SellerPhone:          sellerPhone,
		SellerRegion:         sellerRegion,
		BuyerName:            buyerName,
		BuyerPhone:           buyerPhone,
		BuyerRegion:          buyerRegion,
		FinalPrice:           finalPrice,
		UnitPrice:            unitPrice,
		Quantity:             quantity,
		Unit:                 order.Unit,
		Status:               order.Status,
		ConfirmationDeadline: confirmationDeadline,
		MaskedMessage:        maskedMessage,
		IsSellerConfirmed:    order.SellerConfirmedAt.Valid,
		IsBuyerConfirmed:     order.BuyerConfirmedAt.Valid,
		IAmSeller:            iAmSeller,
		IAmBuyer:             iAmBuyer,
		CreatedAt:            order.CreatedAt.Time.Format(time.RFC3339),
	}
}

func numericToString(n pgtype.Numeric) string {
	if !n.Valid || n.Int == nil {
		return "0"
	}
	dec := decimal.NewFromBigInt(n.Int, n.Exp)
	return dec.String()
}
