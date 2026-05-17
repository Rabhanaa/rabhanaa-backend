package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
)

// DocumentUpload carries a document type and the MinIO object key for a private file.
type DocumentUpload struct {
	DocumentType string
	ObjectKey    string
}

func (s *AuthService) SubmitDocuments(ctx context.Context, userID int32, docs []DocumentUpload) error {
	required := map[string]bool{
		"business_license": false,
		"national_id":      false,
		"tax_card":         false,
	}

	for _, d := range docs {
		if _, ok := required[d.DocumentType]; !ok {
			return errors.New("invalid document type")
		}
		required[d.DocumentType] = true
	}

	for docType, found := range required {
		if !found {
			return fmt.Errorf("missing required document type: %s", docType)
		}
	}

	for _, d := range docs {
		_, err := s.repo.CreateUserDocument(ctx, sqlc.CreateUserDocumentParams{
			UserID:       userID,
			DocumentType: d.DocumentType,
			FilePath:     d.ObjectKey,
		})
		if err != nil {
			return fmt.Errorf("failed to create document %s: %w", d.DocumentType, err)
		}
	}

	return s.repo.UpdateUserStatus(ctx, sqlc.UpdateUserStatusParams{
		ID:     userID,
		Status: "pending_review",
	})
}

type UserDocument struct {
	ID           int32  `json:"id"`
	DocumentType string `json:"document_type"`
	ObjectKey    string `json:"-"` // internal use only; never serialised to JSON
	UploadedAt   string `json:"uploaded_at"`
}

type GetUserDocumentsResponse struct {
	Documents []UserDocument `json:"documents"`
	Missing   []string       `json:"missing"`
}

func (s *AuthService) GetUserDocuments(ctx context.Context, userID int32) (*GetUserDocumentsResponse, error) {
	docs, err := s.repo.GetUserDocuments(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user documents: %w", err)
	}

	documents := make([]UserDocument, len(docs))
	for i, doc := range docs {
		documents[i] = UserDocument{
			ID:           doc.ID,
			DocumentType: doc.DocumentType,
			ObjectKey:    doc.FilePath,
			UploadedAt:   doc.UploadedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}

	missingTypes, err := s.repo.GetMissingDocumentTypes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get missing document types: %w", err)
	}

	return &GetUserDocumentsResponse{
		Documents: documents,
		Missing:   missingTypes,
	}, nil
}

type UpdateLocationRequest struct {
	Latitude  float64 `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" binding:"required,min=-180,max=180"`
}

func (s *AuthService) UpdateLocation(ctx context.Context, userID int32, req UpdateLocationRequest) error {
	latNumeric := pgtype.Numeric{}
	if err := latNumeric.Scan(fmt.Sprintf("%.7f", req.Latitude)); err != nil {
		return fmt.Errorf("invalid latitude: %w", err)
	}

	lngNumeric := pgtype.Numeric{}
	if err := lngNumeric.Scan(fmt.Sprintf("%.7f", req.Longitude)); err != nil {
		return fmt.Errorf("invalid longitude: %w", err)
	}

	if err := s.repo.UpdateUserLocation(ctx, sqlc.UpdateUserLocationParams{
		ID:        userID,
		Latitude:  latNumeric,
		Longitude: lngNumeric,
	}); err != nil {
		return fmt.Errorf("failed to update user location: %w", err)
	}

	return nil
}
