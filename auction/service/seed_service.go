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

// The seeder posts one demo listing at a time, a random three to nine minutes
// apart — about ten an hour — alternating sell and buy. It used to post sixty
// at once every hour (and again on every restart), which reached members'
// phones as a burst of notifications.
const (
	seedMinGap = 3 * time.Minute
	seedMaxGap = 9 * time.Minute
)

type SeedService struct {
	queries              *sqlc.Queries
	auctionDurationHours int
	defaultImageURL      string
	// When the next demo listing is due. Zero after a restart, so one post
	// goes out straight away — one, not a batch.
	nextSeedAt time.Time
	nextIsBuy  bool
	mu         sync.Mutex
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

// SeedAuctions posts the next demo listing when one is due. Called every
// minute by the cron.
func (s *SeedService) SeedAuctions(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if now.Before(s.nextSeedAt) {
		return nil
	}

	interests, err := s.queries.ListInterests(ctx)
	if err != nil {
		slog.Error("failed to list interests for seeding", "error", err)
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

	// Scheduled before posting, so a failing insert waits for the next slot
	// rather than retrying every minute.
	s.nextSeedAt = now.Add(seedMinGap + time.Duration(rand.Int63n(int64(seedMaxGap-seedMinGap))))
	isBuy := s.nextIsBuy
	s.nextIsBuy = !s.nextIsBuy

	region := regions[rand.Intn(len(regions))]
	title := seedProductTitles[rand.Intn(len(seedProductTitles))]
	interest := meatInterest
	if title == seedPoultryTitle {
		interest = poultryInterest
	}
	quantity := decimal.NewFromFloat(float64(rand.Intn(900) + 100))
	endTimePg := pgtype.Timestamptz{Time: now.Add(time.Duration(s.auctionDurationHours) * time.Hour), Valid: true}

	if isBuy {
		_, err = s.queries.CreateBuyRequest(ctx, sqlc.CreateBuyRequestParams{
			OwnerID:       seedUser.ID,
			RegionID:      region.ID,
			InterestID:    interest.ID,
			Title:         title,
			Description:   pgtype.Text{Valid: false},
			ImageUrl:      s.defaultImageURL,
			Unit:          seedUnit,
			Quantity:      decimalToNumeric(quantity),
			BuyAllFromOne: false,
			EndTime:       endTimePg,
			OwnerName:     seedUser.Name,
			RegionName:    region.NameAr,
			InterestName:  interest.NameAr,
			// Demo listings bypass moderation.
			Status: "active",
		})
	} else {
		unitPrice := decimal.NewFromFloat(float64(rand.Intn(48500)+1500) / 100.0)
		_, err = s.queries.CreateSellAuction(ctx, sqlc.CreateSellAuctionParams{
			OwnerID:       seedUser.ID,
			RegionID:      region.ID,
			InterestID:    interest.ID,
			Title:         title,
			Description:   pgtype.Text{Valid: false},
			ImageUrl:      s.defaultImageURL,
			Unit:          seedUnit,
			Quantity:      decimalToNumeric(quantity),
			UnitPrice:     decimalToNumeric(unitPrice),
			BuyAllFromOne: false,
			EndTime:       endTimePg,
			OwnerName:     seedUser.Name,
			RegionName:    region.NameAr,
			InterestName:  interest.NameAr,
			// Demo listings bypass moderation.
			Status: "active",
		})
	}
	if err != nil {
		slog.Error("failed to create seed listing", "buy", isBuy, "error", err)
	}
	return nil
}

// SeedUserEmail identifies the account the seeder posts as. Exported so other
// packages can exclude synthetic activity — commission must never bill it (#13)
// — without redeclaring the address and letting the two drift apart.
const SeedUserEmail = seedUserEmail
