package service

import (
	"context"
	"log/slog"
	"time"

	auctionRepo "rabhana/auction/repository"
	authRepo "rabhana/auth/repository"
	"rabhana/db/sqlc"
	notifModel "rabhana/notification/model"
)

// OrderService interface for cron service dependencies
type OrderService interface {
	CancelExpiredOrders(ctx context.Context) error
}

type CronService struct {
	sellAuctionRepo    auctionRepo.SellAuctionRepository
	buyRequestRepo     auctionRepo.BuyRequestRepository
	sellBidRepo        auctionRepo.SellBidRepository
	supplyOfferRepo    auctionRepo.SupplyOfferRepository
	authRepo           authRepo.Repository
	orderService       OrderService
	seedService        *SeedService
	notificationSender NotificationSender
	queries            *sqlc.Queries
	selectionWindow    int32
	regionFilter       bool
	ticker             *time.Ticker
	done               chan bool
}

func NewCronService(
	sellRepo auctionRepo.SellAuctionRepository,
	buyRepo auctionRepo.BuyRequestRepository,
	sellBidRepo auctionRepo.SellBidRepository,
	supplyOfferRepo auctionRepo.SupplyOfferRepository,
	authRepo authRepo.Repository,
	orderService OrderService,
	notificationSender NotificationSender,
	seedService *SeedService,
	queries *sqlc.Queries,
	selectionWindowHours int,
	regionFilter bool,
) *CronService {
	return &CronService{
		sellAuctionRepo:    sellRepo,
		buyRequestRepo:     buyRepo,
		sellBidRepo:        sellBidRepo,
		supplyOfferRepo:    supplyOfferRepo,
		authRepo:           authRepo,
		orderService:       orderService,
		notificationSender: notificationSender,
		seedService:        seedService,
		queries:            queries,
		selectionWindow:    int32(selectionWindowHours),
		regionFilter:       regionFilter,
	}
}

func (c *CronService) Start() {
	c.ticker = time.NewTicker(1 * time.Minute)
	c.done = make(chan bool)

	go func() {
		c.run()
		for {
			select {
			case <-c.done:
				return
			case <-c.ticker.C:
				c.run()
			}
		}
	}()

	slog.Info("cron service started", "interval", "1 minute")
}

func (c *CronService) Stop() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
	if c.done != nil {
		c.done <- true
	}
	slog.Info("cron service stopped")
}

func (c *CronService) run() {
	ctx := context.Background()
	c.seedAuctions(ctx)
	c.processExpiredAuctions(ctx)
	c.processExpiredSelections(ctx)
	c.processNewListings(ctx)
	c.processSelectionWarnings(ctx)
	c.processExpiredOrders(ctx)
	c.processExpiredSessions(ctx)
	c.processMotivationalMessages(ctx)
}

func (c *CronService) seedAuctions(ctx context.Context) {
	if c.seedService == nil {
		return
	}
	if err := c.seedService.SeedAuctions(ctx); err != nil {
		slog.Error("failed to seed auctions", "error", err)
	}
}

func (c *CronService) processExpiredAuctions(ctx context.Context) {
	expiredSell, err := c.sellAuctionRepo.GetExpiredActive(ctx)
	if err != nil {
		slog.Error("failed to get expired sell auctions", "error", err)
		return
	}

	for _, auction := range expiredSell {
		if auction.BidCount > 0 {
			c.sellAuctionRepo.UpdateStatus(ctx, sqlc.UpdateSellAuctionStatusParams{
				ID:     auction.ID,
				Status: "pending_selection",
			})
			c.notificationSender.Send(ctx, auction.OwnerID, notifModel.EventAuctionEnded, map[string]string{
				"auction_id": auction.PublicID.String(),
			})
		} else {
			c.sellAuctionRepo.UpdateStatus(ctx, sqlc.UpdateSellAuctionStatusParams{
				ID:     auction.ID,
				Status: "expired",
			})
			c.notificationSender.Send(ctx, auction.OwnerID, notifModel.EventAuctionEndedNoBids, map[string]string{
				"auction_id": auction.PublicID.String(),
			})
		}
	}

	expiredBuy, err := c.buyRequestRepo.GetExpiredActive(ctx)
	if err != nil {
		slog.Error("failed to get expired buy requests", "error", err)
		return
	}

	for _, request := range expiredBuy {
		if request.OfferCount > 0 {
			c.buyRequestRepo.UpdateStatus(ctx, sqlc.UpdateBuyRequestStatusParams{
				ID:     request.ID,
				Status: "pending_selection",
			})
			c.notificationSender.Send(ctx, request.OwnerID, notifModel.EventRequestEnded, map[string]string{
				"request_id": request.PublicID.String(),
			})
		} else {
			c.buyRequestRepo.UpdateStatus(ctx, sqlc.UpdateBuyRequestStatusParams{
				ID:     request.ID,
				Status: "expired",
			})
			c.notificationSender.Send(ctx, request.OwnerID, notifModel.EventRequestEndedNoOffers, map[string]string{
				"request_id": request.PublicID.String(),
			})
		}
	}
}

func (c *CronService) processExpiredSelections(ctx context.Context) {
	expiredSell, _ := c.sellAuctionRepo.GetExpiredPendingSelection(ctx, c.selectionWindow)
	for _, auction := range expiredSell {
		c.sellAuctionRepo.UpdateStatus(ctx, sqlc.UpdateSellAuctionStatusParams{
			ID:     auction.ID,
			Status: "expired",
		})
		c.sellBidRepo.MarkNotChosen(ctx, auction.ID)
		c.notificationSender.SendToUser(ctx, auction.OwnerID, "انتهت فترة الاختيار", "انتهت مهلة اختيار الفائز وتم إلغاء الصفقة", map[string]string{
			"type":       "selection_expired",
			"auction_id": auction.PublicID.String(),
		})
		slog.Info("sell auction selection expired", "auction_id", auction.PublicID)
	}

	expiredBuy, _ := c.buyRequestRepo.GetExpiredPendingSelection(ctx, c.selectionWindow)
	for _, request := range expiredBuy {
		c.buyRequestRepo.UpdateStatus(ctx, sqlc.UpdateBuyRequestStatusParams{
			ID:     request.ID,
			Status: "expired",
		})
		c.supplyOfferRepo.MarkNotChosen(ctx, request.ID)
		c.notificationSender.SendToUser(ctx, request.OwnerID, "انتهت فترة الاختيار", "انتهت مهلة اختيار المورد وتم إلغاء الطلب", map[string]string{
			"type":       "selection_expired",
			"request_id": request.PublicID.String(),
		})
		slog.Info("buy request selection expired", "request_id", request.PublicID)
	}
}

func (c *CronService) processExpiredOrders(ctx context.Context) {
	if err := c.orderService.CancelExpiredOrders(ctx); err != nil {
		slog.Error("failed to cancel expired orders", "error", err)
	}
}

func (c *CronService) processSelectionWarnings(ctx context.Context) {
	warningSell, _ := c.sellAuctionRepo.GetSoonExpiringSelection(ctx, c.selectionWindow)
	for _, auction := range warningSell {
		c.notificationSender.SendToUser(ctx, auction.OwnerID, "تنبيه", "باقي ساعة لاختيار الفائز", map[string]string{
			"auction_id": auction.PublicID.String(),
			"type":       "selection_expiring",
		})
		if err := c.sellAuctionRepo.MarkSelectionWarned(ctx, auction.ID); err != nil {
			slog.Error("failed to mark sell auction selection warned", "error", err, "auction_id", auction.ID)
		}
	}

	warningBuy, _ := c.buyRequestRepo.GetSoonExpiringSelection(ctx, c.selectionWindow)
	for _, request := range warningBuy {
		c.notificationSender.SendToUser(ctx, request.OwnerID, "تنبيه", "باقي ساعة لاختيار المورد", map[string]string{
			"request_id": request.PublicID.String(),
			"type":       "selection_expiring",
		})
		if err := c.buyRequestRepo.MarkSelectionWarned(ctx, request.ID); err != nil {
			slog.Error("failed to mark buy request selection warned", "error", err, "request_id", request.ID)
		}
	}
}

func (c *CronService) processMotivationalMessages(ctx context.Context) {
	sellAuctions, err := c.queries.GetMotivatableActiveSellAuctions(ctx)
	if err != nil {
		slog.Error("failed to get motivatable sell auctions", "error", err)
	} else {
		for _, auction := range sellAuctions {
			title, body := sellMotivation(auction.InterestName, auction.EndTime.Time)
			users, err := c.queries.GetActiveUsersByInterest(ctx, sqlc.GetActiveUsersByInterestParams{
				InterestID:     auction.InterestID,
				ExcludeUserID:  auction.OwnerID,
				FilterRegionID: c.notifyRegion(auction.RegionID),
			})
			if err != nil {
				slog.Error("failed to get interested users for motivation", "error", err, "auction_id", auction.ID)
				continue
			}
			for _, uid := range users {
				c.notificationSender.SendToUser(ctx, uid, title, body, map[string]string{
					"type":       "auction_motivation",
					"auction_id": auction.PublicID.String(),
				})
			}
			if err := c.queries.MarkSellAuctionMotivated(ctx, auction.ID); err != nil {
				slog.Error("failed to mark sell auction motivated", "error", err, "auction_id", auction.ID)
			}
		}
	}

	buyRequests, err := c.queries.GetMotivatableActiveBuyRequests(ctx)
	if err != nil {
		slog.Error("failed to get motivatable buy requests", "error", err)
		return
	}
	for _, request := range buyRequests {
		title, body := buyMotivation(request.InterestName, request.EndTime.Time)
		users, err := c.queries.GetActiveUsersByInterest(ctx, sqlc.GetActiveUsersByInterestParams{
			InterestID:     request.InterestID,
			ExcludeUserID:  request.OwnerID,
			FilterRegionID: c.notifyRegion(request.RegionID),
		})
		if err != nil {
			slog.Error("failed to get interested users for motivation", "error", err, "request_id", request.ID)
			continue
		}
		for _, uid := range users {
			c.notificationSender.SendToUser(ctx, uid, title, body, map[string]string{
				"type":       "request_motivation",
				"request_id": request.PublicID.String(),
			})
		}
		if err := c.queries.MarkBuyRequestMotivated(ctx, request.ID); err != nil {
			slog.Error("failed to mark buy request motivated", "error", err, "request_id", request.ID)
		}
	}
}

func sellMotivation(interestName string, endTime time.Time) (string, string) {
	minsLeft := int(time.Until(endTime).Minutes())
	switch {
	case minsLeft >= 20:
		return "🔥 الأسعار ترتفع", interestName + " — قدم عرضك الآن"
	case minsLeft >= 10:
		return "⏰ الوقت ينفد", "باقي وقت قليل على " + interestName + "، انضم الآن"
	default:
		return "🚨 آخر فرصة!", "الصفقة على " + interestName + " تنتهي قريباً"
	}
}

func buyMotivation(interestName string, endTime time.Time) (string, string) {
	minsLeft := int(time.Until(endTime).Minutes())
	switch {
	case minsLeft >= 20:
		return "🔥 طلب شراء نشط", interestName + " — قدم عرضك الآن"
	case minsLeft >= 10:
		return "⏰ الوقت ينفد", "باقي وقت قليل على طلب " + interestName
	default:
		return "🚨 آخر فرصة!", "طلب الشراء على " + interestName + " ينتهي قريباً"
	}
}

func (c *CronService) processExpiredSessions(ctx context.Context) {
	err := c.authRepo.DeleteExpiredSessions(ctx)
	if err != nil {
		slog.Error("failed to delete expired sessions", "error", err)
	}
}

func (c *CronService) processNewListings(ctx context.Context) {
	auctions, err := c.queries.GetUnnotifiedActiveSellAuctions(ctx)
	if err != nil {
		slog.Error("failed to get unnotified active sell auctions", "error", err)
		return
	}

	for _, auction := range auctions {
		matchingUsers, err := c.queries.GetActiveUsersByInterest(ctx, sqlc.GetActiveUsersByInterestParams{
			InterestID:     auction.InterestID,
			ExcludeUserID:  auction.OwnerID,
			FilterRegionID: c.notifyRegion(auction.RegionID),
		})
		if err != nil {
			slog.Error("failed to get active users by interest for sell auction", "error", err, "auction_id", auction.ID)
			continue
		}

		for _, userID := range matchingUsers {
			c.notificationSender.SendToUser(ctx, userID, auction.InterestName+" - صفقة جديدة", auction.Title, map[string]string{
				"type":       "new_sell_auction",
				"auction_id": auction.PublicID.String(),
			})
		}

		if err := c.queries.MarkSellAuctionNotified(ctx, auction.ID); err != nil {
			slog.Error("failed to mark sell auction as notified", "error", err, "auction_id", auction.ID)
		}
	}

	requests, err := c.queries.GetUnnotifiedActiveBuyRequests(ctx)
	if err != nil {
		slog.Error("failed to get unnotified active buy requests", "error", err)
		return
	}

	for _, request := range requests {
		matchingUsers, err := c.queries.GetActiveUsersByInterest(ctx, sqlc.GetActiveUsersByInterestParams{
			InterestID:     request.InterestID,
			ExcludeUserID:  request.OwnerID,
			FilterRegionID: c.notifyRegion(request.RegionID),
		})
		if err != nil {
			slog.Error("failed to get active users by interest for buy request", "error", err, "request_id", request.ID)
			continue
		}

		for _, userID := range matchingUsers {
			c.notificationSender.SendToUser(ctx, userID, request.InterestName+" - طلب شراء جديد", request.Title, map[string]string{
				"type":       "new_buy_request",
				"request_id": request.PublicID.String(),
			})
		}

		if err := c.queries.MarkBuyRequestNotified(ctx, request.ID); err != nil {
			slog.Error("failed to mark buy request as notified", "error", err, "request_id", request.ID)
		}
	}
}

// See SellAuctionService.notifyRegion.
func (c *CronService) notifyRegion(postRegionID int32) int32 {
	if c.regionFilter {
		return postRegionID
	}
	return 0
}
