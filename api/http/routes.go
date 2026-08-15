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
	configHandler := NewConfigHandler(ctx.Queries, ctx.Config)
	router.GET("/health", configHandler.Health)

	apiV1 := router.Group("/api/v1")

	// Public routes
	authHandler := NewAuthHandler(ctx.AuthService, ctx.UploadService)
	apiV1.POST("/auth/register", authHandler.Register)
	apiV1.POST("/auth/login", authHandler.Login)
	apiV1.POST("/auth/forgot-password", authHandler.ForgotPassword)
	apiV1.POST("/auth/verify-reset-code", authHandler.VerifyResetCode)
	apiV1.POST("/auth/reset-password", authHandler.ResetPassword)
	apiV1.POST("/auth/get-otp", authHandler.GetOTP)
	apiV1.POST("/auth/verify-otp", authHandler.VerifyOTP)

	// Reference data (public)
	refHandler := NewReferenceHandler(ctx.Queries)
	apiV1.GET("/regions", refHandler.ListRegions)
	apiV1.GET("/interests", refHandler.ListInterests)
	apiV1.GET("/jobs", refHandler.ListJobs)

	// Auth-only routes (JWT required, no account-status enforcement — accessible to banned/suspended users)
	authOnly := apiV1.Group("")
	authOnly.Use(middleware.Auth(ctx.AuthService))
	authOnly.POST("/auth/logout", authHandler.Logout)

	// Protected routes (require auth + account status check with subscription)
	protected := apiV1.Group("")
	protected.Use(middleware.Auth(ctx.AuthService))
	protected.Use(middleware.AccountStatus(ctx.Queries))

	// Upload routes
	uploadHandler := NewUploadHandler(ctx.UploadService)
	protected.POST("/upload", uploadHandler.UploadFile)

	// Config endpoint (authenticated users only)
	protected.GET("/config", configHandler.GetConfig)

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
	protected.POST("/sell-auctions", sellHandler.Create)
	protected.GET("/sell-auctions", sellHandler.List)
	protected.GET("/sell-auctions/search", sellHandler.Search)
	protected.GET("/sell-auctions/mine", sellHandler.ListMine)
	protected.GET("/sell-auctions/:id", sellHandler.GetDetail)
	protected.POST("/sell-auctions/:id/cancel", sellHandler.Cancel)

	// Buy request routes
	buyHandler := NewBuyRequestHandler(ctx.BuyRequestService)
	protected.POST("/buy-requests", buyHandler.Create)
	protected.GET("/buy-requests", buyHandler.List)
	protected.GET("/buy-requests/search", buyHandler.Search)
	protected.GET("/buy-requests/mine", buyHandler.ListMine)
	protected.GET("/buy-requests/:id", buyHandler.GetDetail)
	protected.POST("/buy-requests/:id/cancel", buyHandler.Cancel)

	// Bid routes
	sellBidHandler := NewSellBidHandler(ctx.SellBiddingService)
	protected.GET("/sell-auctions/:id/bids", sellBidHandler.ListByAuction)
	protected.POST("/sell-auctions/:id/bids", sellBidHandler.PlaceBid)
	protected.GET("/my-bids/sell", sellBidHandler.ListMyBids)

	// User bids summary route
	userBidsHandler := NewUserBidsHandler(ctx.SellBidRepo, ctx.SupplyOfferRepo)
	protected.GET("/user-bids/active-count", userBidsHandler.GetActiveBidCount)

	// Supply offer routes
	supplyOfferHandler := NewSupplyOfferHandler(ctx.SupplyOfferingService)
	protected.GET("/buy-requests/:id/offers", supplyOfferHandler.ListByRequest)
	protected.POST("/buy-requests/:id/offers", supplyOfferHandler.PlaceOffer)
	protected.GET("/my-bids/supply", supplyOfferHandler.ListMyOffers)

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
