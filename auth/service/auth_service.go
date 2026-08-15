package service

import (
	"context"

	authctx "rabhana/auth/context"
	"rabhana/lib/email"
	notifModel "rabhana/notification/model"
)

type NotificationSender interface {
	SendToUser(ctx context.Context, userID int32, title, body string, data map[string]string)
	Send(ctx context.Context, userID int32, event notifModel.EventType, data map[string]string)
}

type AuthService struct {
	repo               AuthRepository
	config             *authctx.AuthConfig
	notificationSender NotificationSender
	emailClient        *email.Client
}

func NewAuthService(repo AuthRepository, config *authctx.AuthConfig, sender NotificationSender, emailClient *email.Client) *AuthService {
	return &AuthService{
		repo:               repo,
		config:             config,
		notificationSender: sender,
		emailClient:        emailClient,
	}
}
