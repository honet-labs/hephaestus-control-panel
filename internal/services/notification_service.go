package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"
)

type NotificationService struct {
	configRepo *repository.ConfigRepository
	httpClient *http.Client
}

func NewNotificationService(configRepo *repository.ConfigRepository) *NotificationService {
	return &NotificationService{
		configRepo: configRepo,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (s *NotificationService) SendAlert(ctx context.Context, payload domain.NotificationPayload) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 1. Check Telegram webhook config
		tgEnabled, _ := s.configRepo.GetAppConfig(bgCtx, "alert_telegram_enabled")
		if tgEnabled == "true" {
			botToken, _ := s.configRepo.GetAppConfig(bgCtx, "alert_telegram_bot_token")
			chatID, _ := s.configRepo.GetAppConfig(bgCtx, "alert_telegram_chat_id")
			if botToken != "" && chatID != "" {
				s.sendTelegram(bgCtx, botToken, chatID, payload)
			}
		}

		// 2. Check Discord webhook config
		discordEnabled, _ := s.configRepo.GetAppConfig(bgCtx, "alert_discord_enabled")
		if discordEnabled == "true" {
			webhookURL, _ := s.configRepo.GetAppConfig(bgCtx, "alert_discord_webhook_url")
			if webhookURL != "" {
				s.sendDiscord(bgCtx, webhookURL, payload)
			}
		}
	}()
}

func (s *NotificationService) sendTelegram(ctx context.Context, botToken, chatID string, p domain.NotificationPayload) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	icon := "ℹ️"
	if p.Level == "error" {
		icon = "🚨"
	} else if p.Level == "warning" {
		icon = "⚠️"
	} else if p.Level == "success" {
		icon = "✅"
	}

	text := fmt.Sprintf("%s *[%s] %s*\n\n%s\n\n_Time: %s_",
		icon, p.Module, p.Title, p.Message, p.Timestamp.Format("2006-01-02 15:04:05"))

	body, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		_, _ = s.httpClient.Do(req)
		logger.Info("Alert", fmt.Sprintf("Telegram notification dispatched for %s", p.Title))
	}
}

func (s *NotificationService) sendDiscord(ctx context.Context, webhookURL string, p domain.NotificationPayload) {
	color := 3447003 // Blue
	if p.Level == "error" {
		color = 15158332 // Red
	} else if p.Level == "warning" {
		color = 15105570 // Orange
	} else if p.Level == "success" {
		color = 3066993 // Green
	}

	embed := map[string]interface{}{
		"title":       fmt.Sprintf("[%s] %s", p.Module, p.Title),
		"description": p.Message,
		"color":       color,
		"timestamp":   p.Timestamp.Format(time.RFC3339),
	}

	body, _ := json.Marshal(map[string]interface{}{
		"username": "Hephaestus Alert Manager",
		"embeds":   []interface{}{embed},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		_, _ = s.httpClient.Do(req)
		logger.Info("Alert", fmt.Sprintf("Discord notification dispatched for %s", p.Title))
	}
}
