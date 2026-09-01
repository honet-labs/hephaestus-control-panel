package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
)

type AuthService struct {
	userRepo   *repository.UserRepository
	configRepo *repository.ConfigRepository
}

func NewAuthService(userRepo *repository.UserRepository, configRepo *repository.ConfigRepository) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		configRepo: configRepo,
	}
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*domain.User, string, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		_ = s.userRepo.LogActivity(ctx, "Auth", "Login Failed", fmt.Sprintf("Username '%s' not found", username), "FAILED", nil)
		return nil, "", errors.New("invalid username or password")
	}

	if !config.CheckPasswordHash(password, user.PasswordHash) {
		_ = s.userRepo.LogActivity(ctx, "Auth", "Login Failed", fmt.Sprintf("Incorrect password for '%s'", username), "FAILED", &user.ID)
		return nil, "", errors.New("invalid username or password")
	}

	// Generate secure 32-byte session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate session token: %w", err)
	}
	rawToken := hex.EncodeToString(tokenBytes)
	tokenHash := config.HashToken(rawToken)

	// Session valid for 24 hours (sliding window max 7 days)
	_, err = s.userRepo.CreateSession(ctx, user.ID, tokenHash, 24*time.Hour)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	_ = s.userRepo.LogActivity(ctx, "Auth", "Login Success", fmt.Sprintf("User '%s' logged in", username), "SUCCESS", &user.ID)
	return user, rawToken, nil
}

func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	tokenHash := config.HashToken(rawToken)
	return s.userRepo.DeleteSession(ctx, tokenHash)
}

func (s *AuthService) ValidateSession(ctx context.Context, rawToken string) (*domain.User, error) {
	tokenHash := config.HashToken(rawToken)
	_, user, err := s.userRepo.GetSessionByToken(ctx, tokenHash)
	if err != nil {
		return nil, errors.New("invalid or expired session")
	}
	return user, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID int, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if !config.CheckPasswordHash(oldPassword, user.PasswordHash) {
		return errors.New("current password is incorrect")
	}

	if len(newPassword) < 6 {
		return errors.New("new password must be at least 6 characters")
	}

	newHash, err := config.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	err = s.userRepo.UpdatePassword(ctx, userID, newHash, false)
	if err == nil {
		_ = s.userRepo.LogActivity(ctx, "User", "Change Password", "User changed password successfully", "SUCCESS", &userID)
	}
	return err
}

func (s *AuthService) IsSetupCompleted(ctx context.Context) bool {
	val, err := s.configRepo.GetAppConfig(ctx, "setup_completed")
	if err != nil {
		return false
	}
	return val == "true"
}

func (s *AuthService) CompleteSetup(ctx context.Context, adminUsername, adminPassword string) (*domain.User, string, error) {
	if s.IsSetupCompleted(ctx) {
		return nil, "", errors.New("setup has already been completed")
	}

	if adminUsername == "" || adminPassword == "" {
		return nil, "", errors.New("admin username and password are required")
	}

	hash, err := config.HashPassword(adminPassword)
	if err != nil {
		return nil, "", err
	}

	user, err := s.userRepo.Create(ctx, adminUsername, hash, "ADMIN", false)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create admin user: %w", err)
	}

	_ = s.configRepo.SetAppConfig(ctx, "setup_completed", "true")
	_ = s.userRepo.LogActivity(ctx, "Setup", "Initial Setup Completed", fmt.Sprintf("Admin user '%s' created", adminUsername), "SUCCESS", &user.ID)

	// Auto-login newly created admin
	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)
	rawToken := hex.EncodeToString(tokenBytes)
	tokenHash := config.HashToken(rawToken)
	_, _ = s.userRepo.CreateSession(ctx, user.ID, tokenHash, 24*time.Hour)

	return user, rawToken, nil
}
