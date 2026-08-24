package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/pkg/errs"
	"rabhana/shipping/service"
)

// ShippingQuoteHandler is the merchant's side of #14: the prices carriers have
// offered on a deal, and accepting or refusing one.
type ShippingQuoteHandler struct {
	shippingService *service.Service
}

func NewShippingQuoteHandler(shippingService *service.Service) *ShippingQuoteHandler {
	return &ShippingQuoteHandler{shippingService: shippingService}
}

// ListForOrder, ListForSellAuction and ListForBuyRequest differ only in which
// kind of job they name, so they share one implementation.
func (h *ShippingQuoteHandler) ListForOrder(c *gin.Context) {
	h.list(c, service.KindOrder)
}

func (h *ShippingQuoteHandler) ListForSellAuction(c *gin.Context) {
	h.list(c, service.KindSellAuction)
}

func (h *ShippingQuoteHandler) ListForBuyRequest(c *gin.Context) {
	h.list(c, service.KindBuyRequest)
}

func (h *ShippingQuoteHandler) list(c *gin.Context, kind string) {
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   errs.ErrQuoteNotFound.Error(),
			"message": errs.GetArabicMessage(errs.ErrQuoteNotFound),
		})
		return
	}
	quotes, err := h.shippingService.ListQuotesForJob(c.Request.Context(), int32(c.GetInt("userID")), kind, jobID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, quotes)
}

// Accept picks a carrier and answers every other quote on the same job.
func (h *ShippingQuoteHandler) Accept(c *gin.Context) {
	quoteID, ok := parseQuoteID(c)
	if !ok {
		return
	}
	if err := h.shippingService.AcceptQuote(c.Request.Context(), int32(c.GetInt("userID")), quoteID); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "تم قبول عرض الشحن"})
}

// Reject turns one carrier down without choosing anyone else.
func (h *ShippingQuoteHandler) Reject(c *gin.Context) {
	quoteID, ok := parseQuoteID(c)
	if !ok {
		return
	}
	if err := h.shippingService.RejectQuote(c.Request.Context(), int32(c.GetInt("userID")), quoteID); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "تم رفض عرض الشحن"})
}

func parseQuoteID(c *gin.Context) (uuid.UUID, bool) {
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   errs.ErrQuoteNotFound.Error(),
			"message": errs.GetArabicMessage(errs.ErrQuoteNotFound),
		})
		return uuid.UUID{}, false
	}
	return quoteID, true
}
