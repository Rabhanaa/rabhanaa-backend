package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	analyticsModel "rabhana/analytics/model"
	analyticsSvc "rabhana/analytics/service"
)

type AdminAnalyticsHandler struct {
	svc *analyticsSvc.AnalyticsService
}

func NewAdminAnalyticsHandler(svc *analyticsSvc.AnalyticsService) *AdminAnalyticsHandler {
	return &AdminAnalyticsHandler{svc: svc}
}

func parseRangeParams(c *gin.Context) (from, to time.Time, ok bool) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		// default to last 30 days
		to = time.Now().UTC()
		from = to.AddDate(0, 0, -30)
		return from, to, true
	}
	var err error
	from, err = time.Parse(time.RFC3339, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_from", "message": "from must be RFC3339"})
		return time.Time{}, time.Time{}, false
	}
	to, err = time.Parse(time.RFC3339, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_to", "message": "to must be RFC3339"})
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func (h *AdminAnalyticsHandler) Overview(c *gin.Context) {
	from, to, ok := parseRangeParams(c)
	if !ok {
		return
	}
	if err := h.svc.ValidateRange(from, to); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_range", "message": err.Error()})
		return
	}
	summary, err := h.svc.Overview(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *AdminAnalyticsHandler) TimeSeries(c *gin.Context) {
	metricStr := c.Query("metric")
	metric := analyticsModel.MetricKey(metricStr)
	switch metric {
	case analyticsModel.MetricNewUsers, analyticsModel.MetricFailedLogins, analyticsModel.MetricBids,
		analyticsModel.MetricClosedAuctions, analyticsModel.MetricOrders, analyticsModel.MetricBuyRequests,
		analyticsModel.MetricIssues:
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_metric",
			"message": "metric must be one of: new_users, failed_logins, bids, closed_auctions, orders, buy_requests, issues",
		})
		return
	}

	granularity := analyticsModel.Granularity(c.Query("granularity"))
	if granularity == "" {
		granularity = analyticsModel.GranularityDay
	}
	if !granularity.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_granularity",
			"message": "granularity must be one of: day, week, month",
		})
		return
	}

	from, to, ok := parseRangeParams(c)
	if !ok {
		return
	}
	if err := h.svc.ValidateRange(from, to); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_range", "message": err.Error()})
		return
	}

	points, err := h.svc.TimeSeries(c.Request.Context(), metric, from, to, granularity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	if points == nil {
		points = []analyticsModel.TimeSeriesPoint{}
	}
	c.JSON(http.StatusOK, points)
}

func (h *AdminAnalyticsHandler) UsersStatus(c *gin.Context) {
	dist, err := h.svc.StatusDistribution(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	if dist == nil {
		dist = []analyticsModel.StatusCount{}
	}
	c.JSON(http.StatusOK, dist)
}

func (h *AdminAnalyticsHandler) IssuesBreakdown(c *gin.Context) {
	from, to, ok := parseRangeParams(c)
	if !ok {
		return
	}
	if err := h.svc.ValidateRange(from, to); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_range", "message": err.Error()})
		return
	}
	summary, err := h.svc.IssueBreakdown(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *AdminAnalyticsHandler) UsersSourceDistribution(c *gin.Context) {
	from, to, ok := parseRangeParams(c)
	if !ok {
		return
	}
	if err := h.svc.ValidateRange(from, to); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_range", "message": err.Error()})
		return
	}
	dist, err := h.svc.UsersBySource(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	if dist == nil {
		dist = []analyticsModel.SourceCount{}
	}
	c.JSON(http.StatusOK, dist)
}

func (h *AdminAnalyticsHandler) UsersSourceByDay(c *gin.Context) {
	from, to, ok := parseRangeParams(c)
	if !ok {
		return
	}
	if err := h.svc.ValidateRange(from, to); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_range", "message": err.Error()})
		return
	}
	points, err := h.svc.UsersBySourceByDay(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	if points == nil {
		points = []analyticsModel.SourceDayPoint{}
	}
	c.JSON(http.StatusOK, points)
}

func (h *AdminAnalyticsHandler) UsersByInterestCounts(c *gin.Context) {
	counts, err := h.svc.UsersCountByInterest(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	if counts == nil {
		counts = []analyticsModel.InterestCount{}
	}
	c.JSON(http.StatusOK, counts)
}

func (h *AdminAnalyticsHandler) UsersByInterest(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil || id64 <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_interest_id", "message": "interest id must be a positive integer"})
		return
	}
	users, err := h.svc.UsersByInterest(c.Request.Context(), int32(id64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	if users == nil {
		users = []analyticsModel.InterestUser{}
	}
	c.JSON(http.StatusOK, users)
}

func (h *AdminAnalyticsHandler) SubscriptionStats(c *gin.Context) {
	stats, err := h.svc.SubscriptionStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}
