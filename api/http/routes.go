package http

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	appctx "rabhana/api/context"
	"rabhana/api/http/middleware"
)

func RegisterRoutes(router *gin.Engine, ctx *appctx.AppContext) {
	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.Logging())

	// Health check
	configHandler := NewConfigHandler(ctx.Queries, ctx.Config, ctx.SettingsService)
	router.GET("/health", configHandler.Health)

	apiV1 := router.Group("/api/v1")

	// Public routes
	authHandler := NewAuthHandler(ctx.AuthService, ctx.UploadService)
	apiV1.POST("/auth/register", authHandler.Register)
	apiV1.POST("/auth/login", authHandler.Login)
	apiV1.POST("/auth/forgot-password", authHandler.ForgotPassword)
	apiV1.POST("/auth/verify-reset-code", authHandler.VerifyResetCode)
	apiV1.POST("/auth/reset-password", authHandler.ResetPassword)

	// Reference data (public)
	refHandler := NewReferenceHandler(ctx.Queries)
	apiV1.GET("/regions", refHandler.ListRegions)
	apiV1.GET("/interests", refHandler.ListInterests)
	apiV1.GET("/jobs", refHandler.ListJobs)

	// Auth-only routes (JWT required, no account-status enforcement — accessible to banned/suspended users)
	authOnly := apiV1.Group("")
	authOnly.Use(middleware.Auth(ctx.AuthService))
	authOnly.POST("/auth/logout", authHandler.Logout)

	// Public browsing (#4): readable without an account, but OptionalAuth still
	// identifies a signed-in visitor so they keep their region filter, retailer
	// filter, own-post exclusion and is_owner flag. Rate limited because these
	// are the only unauthenticated endpoints that touch business data.
	publicRead := apiV1.Group("")
	// Order matters: OptionalAuth first so the limiter can tell a member from a
	// visitor and only throttle the latter.
	publicRead.Use(middleware.OptionalAuth(ctx.AuthService))
	publicRead.Use(middleware.PublicRateLimit(ctx.Config.PublicRateLimitPerMinute))

	// Protected routes (require auth + account status check with subscription)
	protected := apiV1.Group("")
	protected.Use(middleware.Auth(ctx.AuthService))
	protected.Use(middleware.AccountStatus(ctx.Queries))

	// Upload routes
	uploadHandler := NewUploadHandler(ctx.UploadService)
	protected.POST("/upload", uploadHandler.UploadFile)

	// Config endpoint (authenticated users only)
	// Public: the register prompt and WhatsApp button need support_phone before
	// anyone has logged in. The remaining values are UI hints.
	publicRead.GET("/config", configHandler.GetConfig)

	protected.GET("/auth/me", authHandler.GetMe)
	protected.GET("/auth/status", authHandler.GetStatus)
	protected.POST("/auth/documents", authHandler.SubmitDocuments)
	protected.GET("/auth/documents", authHandler.GetDocuments)
	protected.POST("/auth/profile", authHandler.UpdateProfile)
	protected.POST("/auth/interests", authHandler.UpdateInterests)
	protected.POST("/auth/change-password", authHandler.ChangePassword)
	protected.POST("/auth/location", authHandler.UpdateLocation)

	// Auction routes
	sellHandler := NewSellAuctionHandler(ctx.SellAuctionService)
	protected.POST("/sell-auctions", middleware.NotCarrier(), sellHandler.Create)
	// NotCarrier on public routes: a carrier reads the price-free job feed, and
	// letting the same account fetch the merchant feed would make that pointless.
	// It only bites a carrier presenting a token — anonymous callers are what #4
	// opened these up for, and a signed-out carrier is indistinguishable from one.
	publicRead.GET("/sell-auctions", middleware.NotCarrier(), sellHandler.List)
	publicRead.GET("/sell-auctions/search", middleware.NotCarrier(), sellHandler.Search)
	protected.GET("/sell-auctions/mine", sellHandler.ListMine)
	publicRead.GET("/sell-auctions/:id", middleware.NotCarrier(), sellHandler.GetDetail)
	protected.POST("/sell-auctions/:id/cancel", sellHandler.Cancel)

	// Buy request routes
	buyHandler := NewBuyRequestHandler(ctx.BuyRequestService)
	protected.POST("/buy-requests", middleware.NotCarrier(), buyHandler.Create)
	publicRead.GET("/buy-requests", middleware.NotCarrier(), buyHandler.List)
	publicRead.GET("/buy-requests/search", middleware.NotCarrier(), buyHandler.Search)
	protected.GET("/buy-requests/mine", buyHandler.ListMine)
	publicRead.GET("/buy-requests/:id", middleware.NotCarrier(), buyHandler.GetDetail)
	protected.POST("/buy-requests/:id/cancel", buyHandler.Cancel)

	// Bid routes
	sellBidHandler := NewSellBidHandler(ctx.SellBiddingService)
	// Bid and offer lists are merchant market data — a carrier prices transport,
	// not goods. Bids already refuse non-owners; offers do not, so this is the only
	// thing standing between a carrier and every supplier's price.
	protected.GET("/sell-auctions/:id/bids", middleware.NotCarrier(), sellBidHandler.ListByAuction)
	protected.POST("/sell-auctions/:id/bids", middleware.NotCarrier(), sellBidHandler.PlaceBid)
	protected.GET("/my-bids/sell", sellBidHandler.ListMyBids)

	// User bids summary route
	userBidsHandler := NewUserBidsHandler(ctx.SellBidRepo, ctx.SupplyOfferRepo)
	protected.GET("/user-bids/active-count", userBidsHandler.GetActiveBidCount)

	// Supply offer routes
	supplyOfferHandler := NewSupplyOfferHandler(ctx.SupplyOfferingService)
	protected.GET("/buy-requests/:id/offers", middleware.NotCarrier(), supplyOfferHandler.ListByRequest)
	protected.POST("/buy-requests/:id/offers", middleware.NotCarrier(), supplyOfferHandler.PlaceOffer)
	protected.GET("/my-bids/supply", supplyOfferHandler.ListMyOffers)

	// Shipping quotes (#14) — the merchant's side. Carriers offer a price on a
	// deal; whoever owns that deal accepts one. Ownership is checked in the
	// service, so these are ordinary protected routes.
	shippingQuoteHandler := NewShippingQuoteHandler(ctx.ShippingService)
	protected.GET("/orders/:id/shipping-quotes", shippingQuoteHandler.ListForOrder)
	protected.GET("/sell-auctions/:id/shipping-quotes", shippingQuoteHandler.ListForSellAuction)
	protected.GET("/buy-requests/:id/shipping-quotes", shippingQuoteHandler.ListForBuyRequest)
	protected.POST("/shipping-quotes/:id/accept", shippingQuoteHandler.Accept)
	protected.POST("/shipping-quotes/:id/reject", shippingQuoteHandler.Reject)

	// Carrier-facing routes. Carrier() rejects every other role, so a merchant
	// cannot browse the price-free job feed and a carrier cannot reach the
	// trading routes above.
	carrierGroup := protected.Group("/carrier")
	carrierGroup.Use(middleware.Carrier())
	carrierHandler := NewCarrierHandler(ctx.ShippingService)
	carrierGroup.GET("/jobs", carrierHandler.ListJobs)
	carrierGroup.POST("/jobs/:kind/:id/quotes", carrierHandler.CreateQuote)
	carrierGroup.GET("/quotes", carrierHandler.ListMyQuotes)
	carrierGroup.DELETE("/quotes/:id", carrierHandler.WithdrawQuote)
	carrierGroup.GET("/profile", carrierHandler.GetProfile)
	carrierGroup.PATCH("/profile", carrierHandler.UpdateProfile)

	// Selection routes
	sellSelectionHandler := NewSellSelectionHandler(ctx.SellSelectionService)
	protected.POST("/sell-auctions/:id/select", sellSelectionHandler.SelectWinner)

	buySelectionHandler := NewBuySelectionHandler(ctx.BuySelectionService)
	protected.POST("/buy-requests/:id/accept", buySelectionHandler.AcceptOffer)

	// Order routes
	orderHandler := NewOrderHandler(ctx.OrderService)
	protected.GET("/orders", orderHandler.List)
	protected.GET("/orders/:id", orderHandler.GetDetail)
	protected.POST("/orders/:id/confirm", orderHandler.Confirm)

	// Notification routes
	notifHandler := NewNotificationHandler(ctx.NotificationService)
	protected.GET("/notifications", notifHandler.List)
	protected.POST("/notifications/:id/read", notifHandler.MarkAsRead)
	protected.POST("/notifications/read-all", notifHandler.MarkAllAsRead)
	protected.POST("/notifications/device-token", notifHandler.RegisterDeviceToken)
	protected.DELETE("/notifications/device-token", notifHandler.DeregisterDeviceToken)

	// Issue routes
	issueHandler := NewIssueHandler(ctx.Queries)
	protected.POST("/issues", issueHandler.Create)
	protected.GET("/issues", issueHandler.List)
	protected.GET("/issues/:id", issueHandler.GetDetail)

	// Subscription routes
	subHandler := NewSubscriptionHandler(ctx.Queries)
	protected.GET("/subscription", subHandler.GetMySubscription)
	protected.GET("/subscription/status", subHandler.GetSubscriptionStatus)
	protected.GET("/subscription/tiers", subHandler.GetSubscriptionTiers)
	protected.GET("/subscription/my-referral", subHandler.GetMyReferralCode)
	protected.POST("/subscription/apply-referral", subHandler.ApplyReferralCode)

	// Admin routes
	adminGroup := protected.Group("/admin")
	adminGroup.Use(middleware.Admin())

	adminHandler := NewAdminHandler(ctx.AuthService, ctx.UploadService)
	adminGroup.GET("/users/pending", adminHandler.ListPendingUsers)
	adminGroup.GET("/users", adminHandler.ListAllUsers)
	adminGroup.GET("/users/:id", adminHandler.GetUser)
	adminGroup.POST("/users/:id/approve", adminHandler.ApproveUser)
	adminGroup.POST("/users/:id/reject", adminHandler.RejectUser)
	adminGroup.POST("/users/:id/suspend", adminHandler.SuspendUser)
	adminGroup.POST("/users/:id/unsuspend", adminHandler.UnsuspendUser)
	adminGroup.POST("/users/:id/ban", adminHandler.BanUser)
	adminGroup.POST("/users/:id/unban", adminHandler.UnbanUser)
	adminGroup.GET("/users/:id/documents", adminHandler.GetUserDocuments)

	adminSubHandler := NewAdminSubscriptionHandler(ctx.SubscriptionAdminService)
	adminGroup.GET("/users/:id/subscriptions", adminSubHandler.ListUserSubscriptions)
	adminGroup.POST("/users/:id/subscription", adminSubHandler.GrantSubscription)
	adminGroup.PATCH("/users/:id/subscription/:sub_id", adminSubHandler.UpdateSubscription)

	adminGroup.GET("/issues", issueHandler.AdminListAll)
	adminGroup.GET("/issues/:id", issueHandler.AdminGetDetail)
	adminGroup.PATCH("/issues/:id/close", issueHandler.AdminCloseIssue)

	moderationHandler := NewAdminModerationHandler(ctx.ModerationService)
	adminGroup.GET("/posts/pending", moderationHandler.ListPending)
	adminGroup.GET("/posts/published", moderationHandler.ListPublished)
	adminGroup.POST("/posts/:id/approve", moderationHandler.Approve)
	adminGroup.POST("/posts/:id/reject", moderationHandler.Reject)
	adminGroup.POST("/posts/:id/suspend", moderationHandler.Suspend)
	adminGroup.POST("/posts/:id/unsuspend", moderationHandler.Unsuspend)

	settingsHandler := NewAdminSettingsHandler(ctx.SettingsService)
	adminGroup.GET("/settings", settingsHandler.List)
	adminGroup.PATCH("/settings", settingsHandler.Update)

	analyticsHandler := NewAdminAnalyticsHandler(ctx.AnalyticsService)
	adminGroup.GET("/analytics/overview", analyticsHandler.Overview)
	adminGroup.GET("/analytics/timeseries", analyticsHandler.TimeSeries)
	adminGroup.GET("/analytics/users/status-distribution", analyticsHandler.UsersStatus)
	adminGroup.GET("/analytics/users/source-distribution", analyticsHandler.UsersSourceDistribution)
	adminGroup.GET("/analytics/users/source-by-day", analyticsHandler.UsersSourceByDay)
	adminGroup.GET("/analytics/issues/breakdown", analyticsHandler.IssuesBreakdown)
	adminGroup.GET("/analytics/subscriptions/stats", analyticsHandler.SubscriptionStats)
	adminGroup.GET("/analytics/users/by-interest", analyticsHandler.UsersByInterestCounts)
	adminGroup.GET("/analytics/users/by-interest/:id", analyticsHandler.UsersByInterest)
}
