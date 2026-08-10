package context

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	analyticsRepoPkg "rabhana/analytics/repository"
	analyticsSvcPkg "rabhana/analytics/service"
	auctionRepoPkg "rabhana/auction/repository"
	auctionSvcPkg "rabhana/auction/service"
	authCtxPkg "rabhana/auth/context"
	authRepoPkg "rabhana/auth/repository"
	authSvcPkg "rabhana/auth/service"
	"rabhana/db/sqlc"
	"rabhana/lib/firebase"
	minioPkg "rabhana/lib/minio"
	"rabhana/lib/postgres"
	notificationSvcPkg "rabhana/notification/service"
	orderRepoPkg "rabhana/order/repository"
	orderSvcPkg "rabhana/order/service"
	subscriptionSvcPkg "rabhana/subscription/service"
	uploadSvcPkg "rabhana/upload/service"
)

type AppContext struct {
	DB      *postgres.Client
	Queries *sqlc.Queries
	Config  *AppConfig

	MinioClient              *minioPkg.Client
	AuthService              *authSvcPkg.AuthService
	SellAuctionService       *auctionSvcPkg.SellAuctionService
	BuyRequestService        *auctionSvcPkg.BuyRequestService
	SellBiddingService       *auctionSvcPkg.SellBiddingService
	SupplyOfferingService    *auctionSvcPkg.SupplyOfferingService
	SellSelectionService     *auctionSvcPkg.SellSelectionService
	BuySelectionService      *auctionSvcPkg.BuySelectionService
	CronService              *auctionSvcPkg.CronService
	SeedService              *auctionSvcPkg.SeedService
	OrderService             *orderSvcPkg.OrderService
	NotificationService      *notificationSvcPkg.NotificationService
	SubscriptionAdminService *subscriptionSvcPkg.AdminService
	UploadService            *uploadSvcPkg.UploadService
	AnalyticsService         *analyticsSvcPkg.AnalyticsService

	// Repositories (for handlers that need direct access)
	SellBidRepo     auctionRepoPkg.SellBidRepository
	SupplyOfferRepo auctionRepoPkg.SupplyOfferRepository
}

func NewAppContext(ctx context.Context, cfg *AppConfig) (*AppContext, error) {
	// Database connection
	dbClient, err := postgres.NewClient(ctx, &postgres.Config{
		URL:     cfg.DatabaseURL,
		Timeout: cfg.DatabaseTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	queries := sqlc.New(dbClient.Pool)

	// Initialize auth layer (repository only; service wired after notification service)
	authConfig := authCtxPkg.LoadAuthConfig()
	authRepository := authRepoPkg.NewRepository(queries)

	// Initialize MinIO client (non-fatal: server starts without storage, uploads return errors)
	minioClient, err := minioPkg.NewClient(ctx, minioPkg.Config{
		Endpoint:      cfg.MinioEndpoint,
		AccessKey:     cfg.MinioAccessKey,
		SecretKey:     cfg.MinioSecretKey,
		UseSSL:        cfg.MinioUseSSL,
		PublicBucket:  cfg.MinioPublicBucket,
		PrivateBucket: cfg.MinioPrivateBucket,
		PublicBaseURL: cfg.MinioPublicBaseURL,
	})
	if err != nil {
		fmt.Printf("WARNING: MinIO unavailable (%v) — file uploads will be disabled until configured\n", err)
		minioClient = nil
	}

	// Initialize upload service
	uploadService := uploadSvcPkg.NewUploadService(minioClient, 10)

	// Initialize Firebase client
	firebaseClient, err := firebase.NewClient(ctx, cfg.FirebaseCredentialsPath, cfg.FirebaseCredentialsJSON, cfg.AppBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase: %w", err)
	}

	// Initialize notification service
	notificationService := notificationSvcPkg.NewNotificationService(queries, firebaseClient, cfg.MaxNotificationsPerUser)

	// Auth service wired after notification service so it can dispatch alerts
	authService := authSvcPkg.NewAuthService(authRepository, authConfig, notificationService)

	// Subscription admin service
	subscriptionAdminSvc := subscriptionSvcPkg.NewAdminService(queries, authRepository)

	// Initialize auction repositories
	sellAuctionRepo := auctionRepoPkg.NewSellAuctionRepository(queries)
	buyRequestRepo := auctionRepoPkg.NewBuyRequestRepository(queries)
	sellBidRepo := auctionRepoPkg.NewSellBidRepository(queries)
	supplyOfferRepo := auctionRepoPkg.NewSupplyOfferRepository(queries)
	orderRepository := orderRepoPkg.NewOrderRepository(queries)

	// Initialize auction services
	sellAuctionService := auctionSvcPkg.NewSellAuctionService(
		sellAuctionRepo,
		queries,
		notificationService,
		cfg.AuctionDurationHours,
		uploadService,
		cfg.DefaultImageURL,
	)

	buyRequestService := auctionSvcPkg.NewBuyRequestService(
		buyRequestRepo,
		queries,
		notificationService,
		cfg.AuctionDurationHours,
		uploadService,
		cfg.DefaultImageURL,
	)

	sellBiddingService := auctionSvcPkg.NewSellBiddingService(
		sellBidRepo,
		supplyOfferRepo,
		sellAuctionRepo,
		queries,
		notificationService,
		cfg.MaxBidsPerAuction,
		cfg.MaxActiveBidsPerUser,
	)

	supplyOfferingService := auctionSvcPkg.NewSupplyOfferingService(
		supplyOfferRepo,
		sellBidRepo,
		buyRequestRepo,
		queries,
		notificationService,
		cfg.MaxBidsPerAuction,
		cfg.MaxActiveBidsPerUser,
	)

	sellSelectionService := auctionSvcPkg.NewSellSelectionService(
		sellAuctionRepo,
		sellBidRepo,
		orderRepository,
		*authRepository,
		notificationService,
		cfg.SelectionWindowHours,
	)

	buySelectionService := auctionSvcPkg.NewBuySelectionService(
		buyRequestRepo,
		supplyOfferRepo,
		orderRepository,
		*authRepository,
		notificationService,
		cfg.SelectionWindowHours,
	)

	orderService := orderSvcPkg.NewOrderService(
		orderRepository,
		sellAuctionRepo,
		buyRequestRepo,
		notificationService,
	)

	// Left nil when disabled — CronService already skips seeding on a nil service.
	var seedService *auctionSvcPkg.SeedService
	if cfg.SeedEnabled {
		seedService = auctionSvcPkg.NewSeedService(queries, cfg.AuctionDurationHours, cfg.DefaultImageURL)
	} else {
		slog.Info("seeding disabled", "reason", "SEED_ENABLED=false")
	}

	analyticsRepo := analyticsRepoPkg.NewRepository(queries)
	analyticsService := analyticsSvcPkg.NewAnalyticsService(analyticsRepo)

	cronService := auctionSvcPkg.NewCronService(
		sellAuctionRepo,
		buyRequestRepo,
		sellBidRepo,
		supplyOfferRepo,
		*authRepository,
		orderService,
		notificationService,
		seedService,
		queries,
		cfg.SelectionWindowHours,
	)

	return &AppContext{
		DB:                       dbClient,
		Queries:                  queries,
		Config:                   cfg,
		MinioClient:              minioClient,
		AuthService:              authService,
		SellAuctionService:       sellAuctionService,
		BuyRequestService:        buyRequestService,
		SellBiddingService:       sellBiddingService,
		SupplyOfferingService:    supplyOfferingService,
		SellSelectionService:     sellSelectionService,
		BuySelectionService:      buySelectionService,
		CronService:              cronService,
		SeedService:              seedService,
		OrderService:             orderService,
		NotificationService:      notificationService,
		SubscriptionAdminService: subscriptionAdminSvc,
		UploadService:            uploadService,
		AnalyticsService:         analyticsService,
		SellBidRepo:              sellBidRepo,
		SupplyOfferRepo:          supplyOfferRepo,
	}, nil
}

type AppConfig struct {
	ServerPort               string
	DatabaseURL              string
	DatabaseTimeout          int
	AuctionDurationHours     int
	MaxBidsPerAuction        int
	MaxActiveBidsPerUser     int
	SelectionWindowHours     int
	MaxCancellationsPerMonth int
	MaxOpenIssuesPerUser     int
	MaxNotificationsPerUser  int
	MinInterests             int
	BidFloorPercentage       int
	SupportPhone             string
	DefaultImageURL          string
	SeedEnabled              bool
	FirebaseCredentialsPath  string
	FirebaseCredentialsJSON  string
	AppBaseURL               string

	MinioEndpoint      string
	MinioAccessKey     string
	MinioSecretKey     string
	MinioUseSSL        bool
	MinioPublicBucket  string
	MinioPrivateBucket string
	MinioPublicBaseURL string
}

func LoadAppConfig() *AppConfig {
	return &AppConfig{
		ServerPort:               getEnv("SERVER_PORT", "8080"),
		DatabaseURL:              getEnv("DATABASE_URL", "postgres://localhost:5432/rab?sslmode=disable"),
		DatabaseTimeout:          getEnvAsInt("DATABASE_TIMEOUT_SECONDS", 10),
		AuctionDurationHours:     getEnvAsInt("AUCTION_DURATION_HOURS", 24),
		MaxBidsPerAuction:        getEnvAsInt("MAX_BIDS_PER_AUCTION", 10),
		MaxActiveBidsPerUser:     getEnvAsInt("MAX_ACTIVE_BIDS_PER_USER", 3),
		SelectionWindowHours:     getEnvAsInt("SELECTION_WINDOW_HOURS", 24),
		MaxCancellationsPerMonth: getEnvAsInt("MAX_CANCELLATIONS_PER_MONTH", 3),
		MaxOpenIssuesPerUser:     getEnvAsInt("MAX_OPEN_ISSUES_PER_USER", 3),
		MaxNotificationsPerUser:  getEnvAsInt("MAX_NOTIFICATIONS_PER_USER", 10),
		MinInterests:             getEnvAsInt("MIN_INTERESTS_AT_REGISTRATION", 1),
		BidFloorPercentage:       getEnvAsInt("BID_FLOOR_PERCENTAGE", 85),
		SupportPhone:             getEnv("SUPPORT_PHONE", "01107286690"),
		DefaultImageURL:          getEnv("DEFAULT_IMAGE_URL", ""),
		SeedEnabled:              getEnvAsBool("SEED_ENABLED", true),
		FirebaseCredentialsPath:  getEnv("FIREBASE_CREDENTIALS_PATH", ""),
		FirebaseCredentialsJSON:  getEnv("FIREBASE_CREDENTIALS_JSON", ""),
		AppBaseURL:               getEnv("APP_BASE_URL", "http://localhost:8080"),

		MinioEndpoint:      getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:     getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:     getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioUseSSL:        getEnv("MINIO_USE_SSL", "") == "true",
		MinioPublicBucket:  getEnv("MINIO_PUBLIC_BUCKET", "auction-images"),
		MinioPrivateBucket: getEnv("MINIO_PRIVATE_BUCKET", "documents"),
		MinioPublicBaseURL: getEnv("MINIO_PUBLIC_BASE_URL", "http://localhost:9000"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
