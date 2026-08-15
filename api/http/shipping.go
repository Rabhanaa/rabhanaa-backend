package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
	"rabhana/pkg/errs"
)

// ShippingHandler serves the carrier directory. Merchants get a read-only view
// scoped to a governorate; admins get full CRUD.
//
// Follows the ReferenceHandler shape in reference.go, which is how the other
// lookup endpoints are written.
type ShippingHandler struct {
	queries *sqlc.Queries
}

func NewShippingHandler(queries *sqlc.Queries) *ShippingHandler {
	return &ShippingHandler{queries: queries}
}

type shippingCompanyResponse struct {
	PublicID  string   `json:"public_id"`
	Name      string   `json:"name"`
	Phone     string   `json:"phone"`
	LogoURL   *string  `json:"logo_url,omitempty"`
	Notes     *string  `json:"notes,omitempty"`
	IsActive  bool     `json:"is_active"`
	RegionIDs []int32  `json:"region_ids,omitempty"`
	Regions   []string `json:"regions,omitempty"`
}

type shippingCompanyRequest struct {
	Name      string  `json:"name" binding:"required,min=2,max=150"`
	Phone     string  `json:"phone" binding:"required,min=6,max=20"`
	LogoURL   *string `json:"logo_url"`
	Notes     *string `json:"notes"`
	IsActive  *bool   `json:"is_active"`
	RegionIDs []int32 `json:"region_ids" binding:"required,min=1"`
}

// ListForRegion backs the create-post form. Authenticated rather than public
// like /regions and /interests, because it returns partner phone numbers.
func (h *ShippingHandler) ListForRegion(c *gin.Context) {
	regionID, err := strconv.Atoi(c.Query("region_id"))
	if err != nil || regionID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_REGION_ID", "message": "المنطقة غير صحيحة"})
		return
	}

	companies, err := h.queries.ListShippingCompaniesByRegion(c.Request.Context(), int32(regionID))
	if err != nil {
		handleError(c, err)
		return
	}

	out := make([]shippingCompanyResponse, 0, len(companies))
	for _, sc := range companies {
		out = append(out, toShippingResponse(sc, nil, nil))
	}
	c.JSON(http.StatusOK, gin.H{"companies": out})
}

// AdminList returns every carrier, deactivated ones included, each with the
// governorates it covers so the edit form can pre-tick them.
func (h *ShippingHandler) AdminList(c *gin.Context) {
	companies, err := h.queries.ListAllShippingCompanies(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	out := make([]shippingCompanyResponse, 0, len(companies))
	for _, sc := range companies {
		ids, names := h.regionsFor(c, sc.ID)
		out = append(out, toShippingResponse(sc, ids, names))
	}
	c.JSON(http.StatusOK, gin.H{"companies": out})
}

func (h *ShippingHandler) AdminCreate(c *gin.Context) {
	var req shippingCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_BODY", "message": err.Error()})
		return
	}

	company, err := h.queries.CreateShippingCompany(c.Request.Context(), sqlc.CreateShippingCompanyParams{
		Name:    req.Name,
		Phone:   req.Phone,
		LogoUrl: textOrNull(req.LogoURL),
		Notes:   textOrNull(req.Notes),
	})
	if err != nil {
		handleError(c, err)
		return
	}

	if err := h.replaceRegions(c, company.ID, req.RegionIDs); err != nil {
		handleError(c, err)
		return
	}

	ids, names := h.regionsFor(c, company.ID)
	c.JSON(http.StatusCreated, toShippingResponse(company, ids, names))
}

func (h *ShippingHandler) AdminUpdate(c *gin.Context) {
	id, ok := shippingPublicID(c)
	if !ok {
		return
	}

	var req shippingCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_BODY", "message": err.Error()})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	company, err := h.queries.UpdateShippingCompany(c.Request.Context(), sqlc.UpdateShippingCompanyParams{
		PublicID: id,
		Name:     req.Name,
		Phone:    req.Phone,
		LogoUrl:  textOrNull(req.LogoURL),
		Notes:    textOrNull(req.Notes),
		IsActive: isActive,
	})
	if err != nil {
		// Otherwise the driver's "no rows in result set" reaches the client.
		if errors.Is(err, pgx.ErrNoRows) {
			handleError(c, errs.ErrShippingCompanyNotFound)
			return
		}
		handleError(c, err)
		return
	}

	// Replaced wholesale rather than diffed — coverage is a checkbox list.
	if err := h.replaceRegions(c, company.ID, req.RegionIDs); err != nil {
		handleError(c, err)
		return
	}

	ids, names := h.regionsFor(c, company.ID)
	c.JSON(http.StatusOK, toShippingResponse(company, ids, names))
}

// AdminDeactivate is a soft delete — see the query comment.
func (h *ShippingHandler) AdminDeactivate(c *gin.Context) {
	id, ok := shippingPublicID(c)
	if !ok {
		return
	}
	if err := h.queries.DeactivateShippingCompany(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deactivated"})
}

func (h *ShippingHandler) replaceRegions(c *gin.Context, companyID int32, regionIDs []int32) error {
	ctx := c.Request.Context()
	if err := h.queries.ClearShippingCompanyRegions(ctx, companyID); err != nil {
		return err
	}
	return h.queries.SetShippingCompanyRegions(ctx, sqlc.SetShippingCompanyRegionsParams{
		ShippingCompanyID: companyID,
		RegionIds:         regionIDs,
	})
}

func (h *ShippingHandler) regionsFor(c *gin.Context, companyID int32) ([]int32, []string) {
	rows, err := h.queries.ListShippingCompanyRegions(c.Request.Context(), companyID)
	if err != nil {
		return nil, nil
	}
	ids := make([]int32, 0, len(rows))
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
		names = append(names, r.NameAr)
	}
	return ids, names
}

func shippingPublicID(c *gin.Context) (pgtype.UUID, bool) {
	parsed, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_ID", "message": "المعرف غير صحيح"})
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, true
}

func toShippingResponse(sc sqlc.ShippingCompany, regionIDs []int32, regionNames []string) shippingCompanyResponse {
	out := shippingCompanyResponse{
		PublicID:  sc.PublicID.String(),
		Name:      sc.Name,
		Phone:     sc.Phone,
		IsActive:  sc.IsActive,
		RegionIDs: regionIDs,
		Regions:   regionNames,
	}
	if sc.LogoUrl.Valid && sc.LogoUrl.String != "" {
		out.LogoURL = &sc.LogoUrl.String
	}
	if sc.Notes.Valid && sc.Notes.String != "" {
		out.Notes = &sc.Notes.String
	}
	return out
}

func textOrNull(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}
