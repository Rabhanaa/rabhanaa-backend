package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"firebase.google.com/go/v4/messaging"
	"rabhana/db/sqlc"
	"rabhana/lib/firebase"
	"rabhana/notification/model"
)

type NotificationService struct {
	queries    *sqlc.Queries
	firebase   *firebase.Client
	maxPerUser int
}

func NewNotificationService(queries *sqlc.Queries, firebaseClient *firebase.Client, maxPerUser int) *NotificationService {
	if maxPerUser == 0 {
		maxPerUser = 10
	}
	return &NotificationService{
		queries:    queries,
		firebase:   firebaseClient,
		maxPerUser: maxPerUser,
	}
}

func (s *NotificationService) SendToUser(ctx context.Context, userID int32, title, body string, data map[string]string) {
	switch data["type"] {
	case "account_suspended", "account_banned", "account_unsuspended", "account_unbanned":
		// System status notifications bypass the suspension/ban gate
	default:
		user, err := s.queries.GetUserStatusByID(ctx, userID)
		if err != nil {
			slog.Error("failed to get user status for notification gate", "error", err, "user_id", userID)
			return
		}
		if user.Status == "suspended" || user.Status == "banned" {
			return
		}
	}

	eventType := "general"
	if t, ok := data["type"]; ok && t != "" {
		eventType = t
	}

	dataBytes, _ := json.Marshal(data)

	// Create in-app notification
	_, err := s.queries.CreateNotification(ctx, sqlc.CreateNotificationParams{
		UserID:    userID,
		Title:     title,
		Body:      body,
		EventType: eventType,
		Data:      dataBytes,
	})
	if err != nil {
		slog.Error("failed to create notification", "error", err)
		return
	}

	// Cleanup old notifications
	s.queries.DeleteOldNotifications(ctx, sqlc.DeleteOldNotificationsParams{
		UserID:    userID,
		KeepCount: int32(s.maxPerUser),
	})

	// Send push notification asynchronously — detach from request context so
	// the goroutine isn't cancelled when the HTTP handler returns.
	go s.sendPushNotification(context.WithoutCancel(ctx), userID, title, body, data)
}

func (s *NotificationService) sendPushNotification(ctx context.Context, userID int32, title, body string, data map[string]string) {
	if s.firebase == nil {
		return
	}

	tokens, err := s.queries.GetActiveDeviceTokensByUser(ctx, userID)
	if err != nil {
		slog.Error("failed to get device tokens", "error", err, "user_id", userID)
		return
	}

	if len(tokens) == 0 {
		return
	}

	tokenStrings := make([]string, len(tokens))
	for i, t := range tokens {
		tokenStrings[i] = t.Token
	}

	slog.Info("sendPushNotification: dispatching", "user_id", userID, "token_count", len(tokenStrings))

	response, err := s.firebase.SendMulticast(ctx, tokenStrings, title, body, data)
	if err != nil {
		slog.Error("failed to send multicast notification", "error", err, "user_id", userID)
		return
	}

	if response != nil {
		for i, resp := range response.Responses {
			if !resp.Success && resp.Error != nil {
				errMsg := resp.Error.Error()
				slog.Info("sendPushNotification: token response", "user_id", userID, "success", false, "error", errMsg)
				if messaging.IsUnregistered(resp.Error) || messaging.IsInvalidArgument(resp.Error) {
					if err := s.queries.DeactivateDeviceToken(ctx, tokenStrings[i]); err != nil {
						slog.Error("failed to deactivate invalid token", "error", err, "token", tokenStrings[i])
					} else {
						slog.Info("sendPushNotification: deactivated token", "user_id", userID, "reason", errMsg)
					}
				}
			}
		}
	}
}

func (s *NotificationService) Send(ctx context.Context, userID int32, event model.EventType, data map[string]string) {
	msg, ok := model.NotificationMessages[event]
	if !ok {
		slog.Error("unknown notification event type", "event", event)
		return
	}
	if data == nil {
		data = map[string]string{}
	}
	data["type"] = string(event)
	s.SendToUser(ctx, userID, msg.Title, msg.Body, data)
}

func (s *NotificationService) ListNotifications(ctx context.Context, userID int32, limit int32) ([]model.NotificationResponse, error) {
	rows, err := s.queries.ListNotificationsByUser(ctx, sqlc.ListNotificationsByUserParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}

	result := make([]model.NotificationResponse, len(rows))
	for i, row := range rows {
		data := json.RawMessage("{}")
		if len(row.Data) > 0 {
			data = json.RawMessage(row.Data)
		}
		result[i] = model.NotificationResponse{
			ID:        row.ID,
			UserID:    row.UserID,
			Title:     row.Title,
			Body:      row.Body,
			EventType: row.EventType,
			Data:      data,
			IsRead:    row.IsRead,
			CreatedAt: row.CreatedAt.Time,
		}
	}
	return result, nil
}

func (s *NotificationService) MarkAsRead(ctx context.Context, userID int32, notificationID int32) error {
	return s.queries.MarkNotificationRead(ctx, sqlc.MarkNotificationReadParams{
		ID:     notificationID,
		UserID: userID,
	})
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID int32) error {
	return s.queries.MarkAllNotificationsRead(ctx, userID)
}

func (s *NotificationService) RegisterDeviceToken(ctx context.Context, userID int32, token, platform string) error {
	return s.queries.UpsertDeviceToken(ctx, sqlc.UpsertDeviceTokenParams{
		UserID:   userID,
		Token:    token,
		Platform: platform,
	})
}

func (s *NotificationService) DeregisterDeviceToken(ctx context.Context, token string) error {
	return s.queries.DeactivateDeviceToken(ctx, token)
}
