package service

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"rabhana/db/sqlc"
)

const (
	seedUserEmail         = "seed@rabhana.local"
	seedUserName          = "بائع رابحنة"
	seedRegionID          = int32(1)
	seedJobID             = int32(1)
	seedLatitude          = "30.0444"
	seedLongitude         = "31.2357"
	seedPhone             = "+201000000000"
	seedUnit              = "kg"
	seedMeatInterestID    = int32(4)
	seedPoultryInterestID = int32(5)
	seedPoultryTitle      = "فراخ برازيلي"
)

var seedProductTitles = []string{
	"كتف برازيلي",
	"كبدة ناشونال",
	"ظهر هندي",
	"موزة فخدة برازيلي",
	"سن هندي",
	"كبدة بتلو",
	"روزبيف هندي",
	"رقبة برازيلي",
	"كلاوي أمريكي",
	"ضلع هندي",
	"عكاوي برازيلي",
	"سهانة هندي",
	"سويفت G 969",
	"كتف هندي",
	"فلتو هندي",
	"كبدة استرالي",
	"موزة أمامي برازيلي",
	"رقبة هندي",
	"كتف بتلو بالعظم",
	"كبدة منيرا",
	"وش هندي",
	"سن برازيلي",
	"انتركوت هندي",
	"فراخ برازيلي",
	"موزة خلفي برازيلي",
	"سويفت 969",
	"أربع هندي",
	"كولاطة هندي",
	"ضلع برازيلي",
	"كبدة ثري دي",
	"عرق هندي",
	"فخدة هندي",
	"كبدة كوينشين",
}

type SeedService struct {
	queries              *sqlc.Queries
	auctionDurationHours int
	defaultImageURL      string
	lastSeeded           time.Time
	mu                   sync.Mutex
}

func NewSeedService(queries *sqlc.Queries, auctionDurationHours int, defaultImageURL string) *SeedService {
	return &SeedService{
		queries:              queries,
		auctionDurationHours: auctionDurationHours,
		defaultImageURL:      defaultImageURL,
	}
}

func (s *SeedService) ensureSeedUser(ctx context.Context) (sqlc.User, error) {
	lat, _ := decimal.NewFromString(seedLatitude)
	lng, _ := decimal.NewFromString(seedLongitude)
	return s.queries.EnsureSeedUser(ctx, sqlc.EnsureSeedUserParams{
		Email:     seedUserEmail,
		Name:      seedUserName,
		Phone:     pgtype.Text{String: seedPhone, Valid: true},
		RegionID:  pgtype.Int4{Int32: seedRegionID, Valid: true},
		JobID:     pgtype.Int4{Int32: seedJobID, Valid: true},
		Latitude:  pgtype.Numeric{Int: lat.Coefficient(), Exp: lat.Exponent(), Valid: true},
		Longitude: pgtype.Numeric{Int: lng.Coefficient(), Exp: lng.Exponent(), Valid: true},
	})
}

func (s *SeedService) SeedAuctions(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Only seed once per hour
	if time.Since(s.lastSeeded) < time.Hour {
		return nil
	}

	interests, err := s.queries.ListInterests(ctx)
	if err != nil {
		slog.Error("failed to list interests for seeding", "error", err)
		return nil
	}
	if len(interests) == 0 {
		slog.Warn("no active interests found, skipping seed")
		return nil
	}

	var meatInterest, poultryInterest *sqlc.Interest
	for i := range interests {
		switch interests[i].ID {
		case seedMeatInterestID:
			meatInterest = &interests[i]
		case seedPoultryInterestID:
			poultryInterest = &interests[i]
		}
	}
	if meatInterest == nil {
		slog.Warn("meat interest not found, skipping seed", "interest_id", seedMeatInterestID)
		return nil
	}
	if poultryInterest == nil {
		poultryInterest = meatInterest
	}

	regions, err := s.queries.ListRegions(ctx)
	if err != nil {
		slog.Error("failed to list regions for seeding", "error", err)
		return nil
	}
	if len(regions) == 0 {
		slog.Warn("no active regions found, skipping seed")
		return nil
	}

	seedUser, err := s.ensureSeedUser(ctx)
	if err != nil {
		slog.Error("failed to ensure seed user", "error", err)
		return nil
	}

	now := time.Now()
	endTime := now.Add(time.Duration(s.auctionDurationHours) * time.Hour)
	endTimePg := pgtype.Timestamptz{Time: endTime, Valid: true}

	// Create 30 sell auctions
	for i := 0; i < 30; i++ {
		region := regions[rand.Intn(len(regions))]
		title := seedProductTitles[rand.Intn(len(seedProductTitles))]
		interest := meatInterest
		if title == seedPoultryTitle {
			interest = poultryInterest
		}
		unitPrice := decimal.NewFromFloat(float64(rand.Intn(48500)+1500) / 100.0)
		quantity := decimal.NewFromFloat(float64(rand.Intn(900) + 100))

		params := sqlc.CreateSellAuctionParams{
			OwnerID:       seedUser.ID,
			RegionID:      region.ID,
			InterestID:    interest.ID,
			Title:         title,
			Description:   pgtype.Text{Valid: false},
			ImageUrl:      s.defaultImageURL,
			Unit:          seedUnit,
			Quantity:      pgtype.Numeric{Int: quantity.BigInt(), Valid: true},
			UnitPrice:     pgtype.Numeric{Int: unitPrice.BigInt(), Valid: true},
			BuyAllFromOne: false,
			EndTime:       endTimePg,
			OwnerName:     seedUser.Name,
			RegionName:    region.NameAr,
			InterestName:  interest.NameAr,
		}

		if _, err := s.queries.CreateSellAuction(ctx, params); err != nil {
			slog.Error("failed to create seed sell auction", "error", err)
			continue
		}
	}

	// Create 30 buy requests
	for i := 0; i < 30; i++ {
		region := regions[rand.Intn(len(regions))]
		title := seedProductTitles[rand.Intn(len(seedProductTitles))]
		interest := meatInterest
		if title == seedPoultryTitle {
			interest = poultryInterest
		}
		quantity := decimal.NewFromFloat(float64(rand.Intn(900) + 100))

		params := sqlc.CreateBuyRequestParams{
			OwnerID:       seedUser.ID,
			RegionID:      region.ID,
			InterestID:    interest.ID,
			Title:         title,
			Description:   pgtype.Text{Valid: false},
			ImageUrl:      s.defaultImageURL,
			Unit:          seedUnit,
			Quantity:      pgtype.Numeric{Int: quantity.BigInt(), Valid: true},
			BuyAllFromOne: false,
			EndTime:       endTimePg,
			OwnerName:     seedUser.Name,
			RegionName:    region.NameAr,
			InterestName:  interest.NameAr,
		}

		if _, err := s.queries.CreateBuyRequest(ctx, params); err != nil {
			slog.Error("failed to create seed buy request", "error", err)
			continue
		}
	}

	s.lastSeeded = time.Now()
	slog.Info("seed completed", "sell_auctions", 30, "buy_requests", 30, "interests", len(interests), "regions", len(regions))
	return nil
}
