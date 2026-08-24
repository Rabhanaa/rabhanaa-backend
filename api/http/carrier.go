package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/pkg/errs"
	"rabhana/shipping/model"
	"rabhana/shipping/service"
)

// CarrierHandler serves the shipping-company side of #14: the jobs a carrier can
// move, the prices it has offered, and its own coverage.
//
// Every route behind this handler is gated by middleware.Carrier(), so the
// handlers do not re-check the role.
type CarrierHandler struct {
	shippingService *service.Service
}

func NewCarrierHandler(shippingService *service.Service) *CarrierHandler {
	return &CarrierHandler{shippingService: shippingService}
}

// ListJobs returns the jobs in this carrier's governorates. Which kinds appear
// depends on the carrier_quote_stage setting, which the response echoes so the
// client can label an indicative price as such.
func (h *CarrierHandler) ListJobs(c *gin.Context) {
	page, pageSize := paginationParams(c)
	offset := (page - 1) * pageSize

	jobs, err := h.shippingService.ListJobs(c.Request.Context(), int32(c.GetInt("userID")), pageSize, offset)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobs)
}

// CreateQuote records a price for one job. The job's kind is a path segment
// rather than guesswork, because the same public id space is not shared between
// orders and posts.
func (h *CarrierHandler) CreateQuote(c *gin.Context) {
	kind := c.Param("kind")
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   errs.ErrJobNotQuotable.Error(),
			"message": errs.GetArabicMessage(errs.ErrJobNotQuotable),
		})
		return
	}

	var req model.CreateQuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	quote, err := h.shippingService.CreateQuote(c.Request.Context(), int32(c.GetInt("userID")), kind, jobID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, quote)
}

// ListMyQuotes is the carrier's own record, prices included — they are its own.
func (h *CarrierHandler) ListMyQuotes(c *gin.Context) {
	page, pageSize := paginationParams(c)
	offset := (page - 1) * pageSize

	quotes, err := h.shippingService.ListMyQuotes(c.Request.Context(), int32(c.GetInt("userID")), pageSize, offset)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, quotes)
}

func (h *CarrierHandler) WithdrawQuote(c *gin.Context) {
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   errs.ErrQuoteNotFound.Error(),
			"message": errs.GetArabicMessage(errs.ErrQuoteNotFound),
		})
		return
	}
	if err := h.shippingService.WithdrawQuote(c.Request.Context(), int32(c.GetInt("userID")), quoteID); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "تم سحب عرض الشحن"})
}

func (h *CarrierHandler) GetProfile(c *gin.Context) {
	profile, err := h.shippingService.GetProfile(c.Request.Context(), int32(c.GetInt("userID")))
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

// UpdateProfile replaces coverage. A carrier that clears its own governorates
// ends up with an empty job list, which is why the request requires at least one.
func (h *CarrierHandler) UpdateProfile(c *gin.Context) {
	var req model.UpdateCarrierProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.shippingService.UpdateProfile(c.Request.Context(), int32(c.GetInt("userID")), req); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "تم تحديث بيانات شركة الشحن"})
}
