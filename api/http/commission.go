package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	commissionModel "rabhana/commission/model"
	"rabhana/commission/service"
)

// CommissionHandler serves both sides of the platform commission (#13): what a
// seller owes, and the admin's collection worklist.
type CommissionHandler struct {
	commissionService *service.Service
}

func NewCommissionHandler(commissionService *service.Service) *CommissionHandler {
	return &CommissionHandler{commissionService: commissionService}
}

// ------------------------------------------------------------- seller side

// Summary is scoped to the caller. There is no route for reading someone else's
// balance: the seller side always reads its own id from the token.
func (h *CommissionHandler) Summary(c *gin.Context) {
	page, pageSize := paginationParams(c)
	offset := (page - 1) * pageSize

	summary, err := h.commissionService.GetSellerSummary(c.Request.Context(), int32(c.GetInt("userID")), pageSize, offset)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

// ListMyCharges is the itemisation behind the total, so a seller can check the
// number against their own sales rather than take it on trust.
func (h *CommissionHandler) ListMyCharges(c *gin.Context) {
	page, pageSize := paginationParams(c)
	offset := (page - 1) * pageSize

	charges, err := h.commissionService.ListSellerCharges(c.Request.Context(), int32(c.GetInt("userID")), pageSize, offset)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"charges": charges})
}

// ------------------------------------------------------------- admin side

// AdminList is the collection worklist: sellers who owe, overdue first.
func (h *CommissionHandler) AdminList(c *gin.Context) {
	page, pageSize := paginationParams(c)
	offset := (page - 1) * pageSize
	overdueOnly := c.Query("filter") == "overdue"

	balances, err := h.commissionService.ListBalances(c.Request.Context(), overdueOnly, pageSize, offset)
	if err != nil {
		handleError(c, err)
		return
	}
	balances.Page = page
	c.JSON(http.StatusOK, balances)
}

// AdminMarkPaid records a payment collected off-platform. It does not reactivate
// a suspended seller — unblocking stays a separate, deliberate admin action.
func (h *CommissionHandler) AdminMarkPaid(c *gin.Context) {
	invoiceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invoice id"})
		return
	}

	var req commissionModel.PayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.commissionService.MarkPaid(c.Request.Context(), invoiceID, int32(c.GetInt("userID")), req); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "تم تسجيل الدفعة"})
}

// AdminWaive writes an invoice off. The reason is required: this is the only way
// to cancel a debt, and an unexplained write-off is indistinguishable from a
// mistake when someone reviews the ledger later.
func (h *CommissionHandler) AdminWaive(c *gin.Context) {
	invoiceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invoice id"})
		return
	}

	var req commissionModel.WaiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.commissionService.Waive(c.Request.Context(), invoiceID, int32(c.GetInt("userID")), req.Reason); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "تم إلغاء الفاتورة"})
}

// AdminSellerDetail backs the payment dialog: one seller's unpaid invoices and
// the sales behind them.
func (h *CommissionHandler) AdminSellerDetail(c *gin.Context) {
	sellerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	detail, err := h.commissionService.GetSellerDetail(c.Request.Context(), sellerID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}
