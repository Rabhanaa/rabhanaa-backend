package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appctx "rabhana/api/context"
	"rabhana/db/sqlc"
)

type ConfigHandler struct {
	queries *sqlc.Queries
	config  *appctx.AppConfig
}

func NewConfigHandler(queries *sqlc.Queries, config *appctx.AppConfig) *ConfigHandler {
	return &ConfigHandler{queries: queries, config: config}
}

func (h *ConfigHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"max_bids_per_auction":          h.config.MaxBidsPerAuction,
		"max_active_bids_per_user":      h.config.MaxActiveBidsPerUser,
		"max_cancellations_per_month":   h.config.MaxCancellationsPerMonth,
		"max_open_issues":               h.config.MaxOpenIssuesPerUser,
		"auction_duration_hours":        h.config.AuctionDurationHours,
		"selection_window_hours":        h.config.SelectionWindowHours,
		"max_notifications_per_user":    h.config.MaxNotificationsPerUser,
		"min_interests_at_registration": h.config.MinInterests,
		"bid_floor_percentage":          h.config.BidFloorPercentage,
		"support_phone":                 h.config.SupportPhone,
		"require_documents":             h.config.RequireDocuments,
		"units":                         []string{"kg", "ton", "piece", "box"},
	})
}

func (h *ConfigHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
