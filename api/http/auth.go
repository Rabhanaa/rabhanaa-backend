package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"rabhana/auth/model"
	authservice "rabhana/auth/service"
	uploadservice "rabhana/upload/service"
	"rabhana/pkg/errs"
)

type AuthHandler struct {
	authService   *authservice.AuthService
	uploadService *uploadservice.UploadService
}

func NewAuthHandler(authService *authservice.AuthService, uploadService *uploadservice.UploadService) *AuthHandler {
	return &AuthHandler{authService: authService, uploadService: uploadService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.RegisterUser(c.Request.Context(), req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deviceInfo := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	resp, err := h.authService.Login(c.Request.Context(), req, deviceInfo, ipAddress)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) GetOTP(c *gin.Context) {
	var req model.GetOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.authService.GetOTP(c.Request.Context(), req.Phone)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP sent"})
}

func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req model.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deviceInfo := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	resp, err := h.authService.VerifyOTP(c.Request.Context(), req, deviceInfo, ipAddress)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.authService.GetCurrentUser(c.Request.Context(), int32(userID))
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), int32(userID), req); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.authService.Logout(c.Request.Context(), int32(userID)); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

func (h *AuthHandler) SubmitDocuments(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	docTypes := []string{"business_license", "national_id", "tax_card"}
	var docs []authservice.DocumentUpload

	for _, docType := range docTypes {
		fh, err := c.FormFile(docType)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("missing file: %s", docType)})
			return
		}

		f, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
			return
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
			return
		}

		objectKey, err := h.uploadService.UploadPrivateFile(c.Request.Context(), data, fh.Filename)
		if err != nil {
			handleError(c, err)
			return
		}

		docs = append(docs, authservice.DocumentUpload{
			DocumentType: docType,
			ObjectKey:    objectKey,
		})
	}

	if err := h.authService.SubmitDocuments(c.Request.Context(), int32(userID), docs); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "documents submitted successfully"})
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req model.ProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.UpdateProfile(c.Request.Context(), int32(userID), req); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "profile updated successfully"})
}

func (h *AuthHandler) UpdateInterests(c *gin.Context) {
	fmt.Printf("[INTERESTS-HANDLER] Entered handler\n")
	userID := c.GetInt("userID")
	fmt.Printf("[INTERESTS-HANDLER] UserID from context: %d\n", userID)
	if userID == 0 {
		fmt.Printf("[INTERESTS-HANDLER] ❌ REJECT: UserID is 0\n")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req model.InterestsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("[INTERESTS-HANDLER] ❌ REJECT: JSON bind error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Printf("[INTERESTS-HANDLER] Request bound, interest_ids count: %d\n", len(req.InterestIDs))

	fmt.Printf("[INTERESTS-HANDLER] Calling AuthService.UpdateInterests(userID=%d, interests=%v)\n",
		userID, req.InterestIDs)
	if err := h.authService.UpdateInterests(c.Request.Context(), int32(userID), req); err != nil {
		fmt.Printf("[INTERESTS-HANDLER] ❌ Service error: %v\n", err)
		handleError(c, err)
		return
	}
	fmt.Printf("[INTERESTS-HANDLER] ✅ Interests updated successfully\n")

	c.JSON(http.StatusOK, gin.H{"message": "interests updated successfully"})
}

func (h *AuthHandler) GetDocuments(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	resp, err := h.authService.GetUserDocuments(c.Request.Context(), int32(userID))
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) UpdateLocation(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	fmt.Printf("[UPDATE-LOCATION] Request from userID: %d\n", userID)

	var req authservice.UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("[UPDATE-LOCATION] ❌ JSON binding error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("[UPDATE-LOCATION] Request bound successfully, lat=%f, lng=%f\n", req.Latitude, req.Longitude)

	if err := h.authService.UpdateLocation(c.Request.Context(), int32(userID), req); err != nil {
		fmt.Printf("[UPDATE-LOCATION] ❌ Service error: %v\n", err)
		handleError(c, err)
		return
	}

	fmt.Printf("[UPDATE-LOCATION] ✅ Location updated successfully\n")
	c.JSON(http.StatusOK, gin.H{"message": "location updated successfully"})
}

func (h *AuthHandler) GetStatus(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	resp, err := h.authService.GetUserStatus(c.Request.Context(), int32(userID))
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func handleError(c *gin.Context, err error) {
	code := http.StatusBadRequest
	switch {
	case errors.Is(err, errs.ErrCannotActOnAdmin),
		errors.Is(err, errs.ErrCannotActOnSelf),
		errors.Is(err, errs.ErrUserSuspended),
		errors.Is(err, errs.ErrUserBanned):
		code = http.StatusForbidden
	case errors.Is(err, errs.ErrInvalidStatusTransition):
		code = http.StatusConflict
	}
	arabicMessage := errs.GetArabicMessage(err)
	c.JSON(code, gin.H{"error": err.Error(), "message": arabicMessage})
}
