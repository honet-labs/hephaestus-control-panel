package services

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
)

type VaultwardenService struct {
	repo       *repository.VaultwardenRepository
	httpClient *http.Client
}

func NewVaultwardenService(repo *repository.VaultwardenRepository) *VaultwardenService {
	return &VaultwardenService{
		repo: repo,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// StartBackgroundSync starts periodic background synchronization every 5 minutes
func (s *VaultwardenService) StartBackgroundSync(stopChan <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				cfg, err := s.repo.GetConfig(context.Background())
				if err == nil && cfg != nil && cfg.ServerURL != "" && cfg.MasterPassword != "" && cfg.IsActive {
					_, syncErr := s.SyncVault(context.Background())
					if syncErr != nil {
						logger.Warn("Vaultwarden", fmt.Sprintf("Background auto-sync failed: %v", syncErr))
					} else {
						logger.Info("Vaultwarden", "Background auto-sync completed successfully.")
					}
				}
			}
		}
	}()
}

// -------------------------------------------------------------
// Bitwarden API Data Structures
// -------------------------------------------------------------

type bwPreloginResponse struct {
	Kdf            int  `json:"kdf"`
	KdfIterations  int  `json:"kdfIterations"`
	KdfMemory      *int `json:"kdfMemory"`
	KdfParallelism *int `json:"kdfParallelism"`
}

type bwTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Key         string `json:"Key"`
	PrivateKey  string `json:"PrivateKey"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

type bwSyncResponse struct {
	Profile struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"profile"`
	Folders []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"folders"`
	Ciphers []bwRawCipher `json:"ciphers"`
}

type bwRawCipher struct {
	ID           string           `json:"id"`
	FolderID     *string          `json:"folderId"`
	Type         int              `json:"type"`
	Name         string           `json:"name"`
	Notes        *string          `json:"notes"`
	RevisionDate string           `json:"revisionDate"`
	Login        *bwRawCipherLogin `json:"login"`
}

type bwRawCipherLogin struct {
	Username *string         `json:"username"`
	Password *string         `json:"password"`
	TOTP     *string         `json:"totp"`
	URIs     []bwRawCipherURI `json:"uris"`
}

type bwRawCipherURI struct {
	URI string `json:"uri"`
}

// -------------------------------------------------------------
// Service Methods
// -------------------------------------------------------------

// TestConnection verifies credentials against the Vaultwarden instance and tests cipher decryption
func (s *VaultwardenService) TestConnection(ctx context.Context, serverURL, email, masterPassword string) (bool, string, int) {
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	email = strings.ToLower(strings.TrimSpace(email))

	if serverURL == "" || email == "" || masterPassword == "" {
		return false, "Server URL, email, and master password are required", 0
	}

	items, err := s.fetchAndDecryptVault(ctx, serverURL, email, masterPassword)
	if err != nil {
		return false, err.Error(), 0
	}

	return true, fmt.Sprintf("Successfully connected to Vaultwarden! Found %d credentials.", len(items)), len(items)
}

// SyncVault triggers an immediate synchronization, decrypts all credentials, and stores them in local cache
func (s *VaultwardenService) SyncVault(ctx context.Context) (*domain.VaultSyncResponse, error) {
	cfg, err := s.repo.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve vaultwarden configuration: %w", err)
	}
	if cfg == nil || cfg.ServerURL == "" || cfg.Email == "" || cfg.MasterPassword == "" {
		return nil, errors.New("vaultwarden is not configured yet. Please configure server URL, email, and master password")
	}

	items, err := s.fetchAndDecryptVault(ctx, cfg.ServerURL, cfg.Email, cfg.MasterPassword)
	if err != nil {
		logger.Error("Vaultwarden", "Sync failed", err)
		return nil, fmt.Errorf("sync failed: %w", err)
	}

	now := time.Now()
	if err := s.repo.UpdateCachedCiphers(ctx, cfg.ID, items, now); err != nil {
		logger.Warn("Vaultwarden", fmt.Sprintf("Failed to update cached ciphers in database: %v", err))
	}

	loginsCount := 0
	notesCount := 0
	for _, it := range items {
		if it.Type == 1 {
			loginsCount++
		} else if it.Type == 2 {
			notesCount++
		}
	}

	return &domain.VaultSyncResponse{
		Success:      true,
		Message:      fmt.Sprintf("Synchronized %d credentials successfully", len(items)),
		TotalItems:   len(items),
		LoginsCount:  loginsCount,
		NotesCount:   notesCount,
		LastSyncedAt: now,
		Items:        items,
	}, nil
}

// GetCiphers returns credentials with optional keyword and folder filtering
func (s *VaultwardenService) GetCiphers(ctx context.Context, keyword, folder string) ([]domain.VaultCredentialItem, error) {
	cfg, err := s.repo.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return []domain.VaultCredentialItem{}, nil
	}

	items := cfg.CachedCiphers
	// If cache is empty and config exists, perform initial sync
	if len(items) == 0 && cfg.MasterPassword != "" {
		syncRes, err := s.SyncVault(ctx)
		if err == nil && syncRes != nil {
			items = syncRes.Items
		}
	}

	filtered := make([]domain.VaultCredentialItem, 0, len(items))
	kwLower := strings.ToLower(strings.TrimSpace(keyword))
	folderLower := strings.ToLower(strings.TrimSpace(folder))

	for _, item := range items {
		// Folder filter
		if folderLower != "" && folderLower != "all" {
			if strings.ToLower(item.FolderName) != folderLower && strings.ToLower(item.TypeLabel) != folderLower {
				continue
			}
		}

		// Keyword search across Name, Username, URIs, and Notes
		if kwLower != "" {
			match := strings.Contains(strings.ToLower(item.Name), kwLower) ||
				strings.Contains(strings.ToLower(item.Username), kwLower) ||
				strings.Contains(strings.ToLower(item.Notes), kwLower)

			if !match {
				for _, u := range item.URIs {
					if strings.Contains(strings.ToLower(u), kwLower) {
						match = true
						break
					}
				}
			}

			if !match {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	// Sort alphabetically by Name
	sort.Slice(filtered, func(i, j int) bool {
		return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
	})

	return filtered, nil
}

type vwAuthContext struct {
	ServerURL   string
	AccessToken string
	UserEncKey  []byte
	UserMacKey  []byte
}

func (s *VaultwardenService) authenticateAndGetKeys(ctx context.Context, serverURL, email, password string) (*vwAuthContext, error) {
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	email = strings.ToLower(strings.TrimSpace(email))

	// Step 1: Prelogin (get KDF algorithm & iterations)
	prelogin, err := s.doPrelogin(ctx, serverURL, email)
	if err != nil {
		return nil, fmt.Errorf("prelogin failed: %w", err)
	}

	// Step 2: Derive Master Key
	masterKey, err := s.deriveMasterKey(email, password, prelogin)
	if err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}

	// Step 3: Compute Master Password Hash for Authentication
	authPasswordHash := s.computePasswordHash(masterKey, password)

	// Step 4: OAuth2 Login Token & User Encrypted Key
	tokenResp, err := s.doLogin(ctx, serverURL, email, authPasswordHash)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Step 5: Decrypt User Symmetric Key (64 bytes: 32 enc + 32 mac)
	userEncKey, userMacKey, err := s.decryptUserKey(masterKey, tokenResp.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt user symmetric key: %w", err)
	}

	return &vwAuthContext{
		ServerURL:   serverURL,
		AccessToken: tokenResp.AccessToken,
		UserEncKey:  userEncKey,
		UserMacKey:  userMacKey,
	}, nil
}

func (s *VaultwardenService) fetchAndDecryptVault(ctx context.Context, serverURL, email, password string) ([]domain.VaultCredentialItem, error) {
	authCtx, err := s.authenticateAndGetKeys(ctx, serverURL, email, password)
	if err != nil {
		return nil, err
	}

	// Step 6: Sync Vault Data
	syncData, err := s.doSync(ctx, authCtx.ServerURL, authCtx.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("vault sync failed: %w", err)
	}

	userEncKey := authCtx.UserEncKey
	userMacKey := authCtx.UserMacKey

	// Step 7: Decrypt Folder Names
	folderMap := make(map[string]string)
	for _, f := range syncData.Folders {
		decName, err := s.decryptCipherString(f.Name, userEncKey, userMacKey)
		if err == nil {
			folderMap[f.ID] = decName
		} else {
			folderMap[f.ID] = f.Name
		}
	}

	// Step 8: Decrypt Ciphers
	items := make([]domain.VaultCredentialItem, 0, len(syncData.Ciphers))
	for _, c := range syncData.Ciphers {
		item := domain.VaultCredentialItem{
			ID:       c.ID,
			FolderID: c.FolderID,
			Type:     c.Type,
		}

		if c.FolderID != nil && *c.FolderID != "" {
			if name, ok := folderMap[*c.FolderID]; ok {
				item.FolderName = name
			}
		}

		switch c.Type {
		case 1:
			item.TypeLabel = "Login"
		case 2:
			item.TypeLabel = "Secure Note"
		case 3:
			item.TypeLabel = "Card"
		case 4:
			item.TypeLabel = "Identity"
		default:
			item.TypeLabel = "Item"
		}

		// Decrypt Name
		if decName, err := s.decryptCipherString(c.Name, userEncKey, userMacKey); err == nil {
			item.Name = decName
		} else {
			item.Name = "[Encrypted Name]"
		}

		// Decrypt Notes
		if c.Notes != nil && *c.Notes != "" {
			if decNotes, err := s.decryptCipherString(*c.Notes, userEncKey, userMacKey); err == nil {
				item.Notes = decNotes
			}
		}

		// Decrypt Login credentials (username, password, totp, URIs)
		if c.Login != nil {
			if c.Login.Username != nil && *c.Login.Username != "" {
				if decUser, err := s.decryptCipherString(*c.Login.Username, userEncKey, userMacKey); err == nil {
					item.Username = decUser
				}
			}

			if c.Login.Password != nil && *c.Login.Password != "" {
				if decPass, err := s.decryptCipherString(*c.Login.Password, userEncKey, userMacKey); err == nil {
					item.Password = decPass
				}
			}

			if c.Login.TOTP != nil && *c.Login.TOTP != "" {
				if decTotp, err := s.decryptCipherString(*c.Login.TOTP, userEncKey, userMacKey); err == nil {
					item.TOTP = decTotp
				}
			}

			if len(c.Login.URIs) > 0 {
				item.URIs = make([]string, 0, len(c.Login.URIs))
				for _, rawURI := range c.Login.URIs {
					if rawURI.URI != "" {
						if decURI, err := s.decryptCipherString(rawURI.URI, userEncKey, userMacKey); err == nil && decURI != "" {
							item.URIs = append(item.URIs, decURI)
						}
					}
				}
			}
		}

		if revDate, err := time.Parse(time.RFC3339, c.RevisionDate); err == nil {
			item.RevisionDate = revDate
		} else {
			item.RevisionDate = time.Now()
		}

		items = append(items, item)
	}

	return items, nil
}

func (s *VaultwardenService) doPrelogin(ctx context.Context, serverURL, email string) (*bwPreloginResponse, error) {
	endpoint := fmt.Sprintf("%s/api/accounts/prelogin", serverURL)
	reqBody, _ := json.Marshal(map[string]string{"email": email})

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rawErr, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prelogin HTTP %d: %s", resp.StatusCode, string(rawErr))
	}

	var prelogin bwPreloginResponse
	if err := json.NewDecoder(resp.Body).Decode(&prelogin); err != nil {
		return nil, fmt.Errorf("invalid prelogin JSON response: %w", err)
	}

	if prelogin.KdfIterations <= 0 {
		prelogin.KdfIterations = 600000 // Bitwarden standard default
	}

	return &prelogin, nil
}

func (s *VaultwardenService) deriveMasterKey(email, password string, prelogin *bwPreloginResponse) ([]byte, error) {
	if prelogin.Kdf == 1 {
		// Argon2id
		salt := sha256.Sum256([]byte(email))
		memory := 64
		parallelism := 4
		if prelogin.KdfMemory != nil && *prelogin.KdfMemory > 0 {
			memory = *prelogin.KdfMemory
		}
		if prelogin.KdfParallelism != nil && *prelogin.KdfParallelism > 0 {
			parallelism = *prelogin.KdfParallelism
		}
		key := argon2.IDKey([]byte(password), salt[:], uint32(prelogin.KdfIterations), uint32(memory*1024), uint8(parallelism), 32)
		return key, nil
	}

	// Default: PBKDF2 with SHA-256
	salt := []byte(email)
	key := pbkdf2.Key([]byte(password), salt, prelogin.KdfIterations, 32, sha256.New)
	return key, nil
}

func (s *VaultwardenService) computePasswordHash(masterKey []byte, password string) string {
	// Bitwarden spec: pbkdf2(masterKey, password, 1, 32, SHA256) -> base64
	hashBytes := pbkdf2.Key(masterKey, []byte(password), 1, 32, sha256.New)
	return base64.StdEncoding.EncodeToString(hashBytes)
}

func (s *VaultwardenService) doLogin(ctx context.Context, serverURL, email, passwordHash string) (*bwTokenResponse, error) {
	endpoint := fmt.Sprintf("%s/identity/connect/token", serverURL)

	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", email)
	data.Set("password", passwordHash)
	data.Set("scope", "api offline_access")
	data.Set("client_id", "web")
	data.Set("deviceType", "14")
	data.Set("deviceName", "Hephaestus Control Panel")
	data.Set("deviceIdentifier", "hcp-engine-client")

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach identity endpoint: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp bwTokenResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return nil, fmt.Errorf("login response parse failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := tokenResp.ErrorDesc
		if msg == "" {
			msg = tokenResp.Error
		}
		if msg == "" {
			msg = string(bodyBytes)
		}
		return nil, fmt.Errorf("authentication error (%d): %s", resp.StatusCode, msg)
	}

	if tokenResp.AccessToken == "" || tokenResp.Key == "" {
		return nil, errors.New("login succeeded but server returned empty access token or symmetric key")
	}

	return &tokenResp, nil
}

func (s *VaultwardenService) decryptUserKey(masterKey []byte, encryptedKeyCipherStr string) ([]byte, []byte, error) {
	// HKDF-Expand masterKey to obtain masterEncKey and masterMacKey
	encReader := hkdf.Expand(sha256.New, masterKey, []byte("enc"))
	masterEncKey := make([]byte, 32)
	if _, err := io.ReadFull(encReader, masterEncKey); err != nil {
		return nil, nil, err
	}

	macReader := hkdf.Expand(sha256.New, masterKey, []byte("mac"))
	masterMacKey := make([]byte, 32)
	if _, err := io.ReadFull(macReader, masterMacKey); err != nil {
		return nil, nil, err
	}

	decryptedUserKeyBytes, err := s.decryptCipherBytes(encryptedKeyCipherStr, masterEncKey, masterMacKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt user key: %w", err)
	}

	if len(decryptedUserKeyBytes) < 64 {
		return nil, nil, fmt.Errorf("decrypted user key length is %d, expected 64 bytes", len(decryptedUserKeyBytes))
	}

	userEncKey := decryptedUserKeyBytes[0:32]
	userMacKey := decryptedUserKeyBytes[32:64]
	return userEncKey, userMacKey, nil
}

func (s *VaultwardenService) doSync(ctx context.Context, serverURL, accessToken string) (*bwSyncResponse, error) {
	endpoint := fmt.Sprintf("%s/api/sync", serverURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sync request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sync returned HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var syncData bwSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncData); err != nil {
		return nil, fmt.Errorf("failed to decode sync response: %w", err)
	}

	return &syncData, nil
}

// decryptCipherString parses CipherString format (e.g. "2.iv|ciphertext|mac") and decrypts into plaintext UTF-8
func (s *VaultwardenService) decryptCipherString(cipherStr string, encKey, macKey []byte) (string, error) {
	plainBytes, err := s.decryptCipherBytes(cipherStr, encKey, macKey)
	if err != nil {
		return "", err
	}
	return string(plainBytes), nil
}

func (s *VaultwardenService) decryptCipherBytes(cipherStr string, encKey, macKey []byte) ([]byte, error) {
	cipherStr = strings.TrimSpace(cipherStr)
	if cipherStr == "" {
		return []byte{}, nil
	}

	parts := strings.SplitN(cipherStr, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed cipher string: missing type dot separator")
	}

	encType, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid encryption type: %w", err)
	}

	subParts := strings.Split(parts[1], "|")

	switch encType {
	case 2:
		// AesCbc256_HmacSha256_B64: "2.iv|ciphertext|mac"
		if len(subParts) != 3 {
			return nil, errors.New("type 2 cipher string requires iv|ciphertext|mac")
		}

		iv, err := base64.StdEncoding.DecodeString(subParts[0])
		if err != nil {
			return nil, fmt.Errorf("failed to decode IV: %w", err)
		}
		ciphertext, err := base64.StdEncoding.DecodeString(subParts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
		}
		expectedMac, err := base64.StdEncoding.DecodeString(subParts[2])
		if err != nil {
			return nil, fmt.Errorf("failed to decode MAC: %w", err)
		}

		// Verify HMAC-SHA256: hmac(macKey, iv + ciphertext)
		h := hmac.New(sha256.New, macKey)
		h.Write(iv)
		h.Write(ciphertext)
		computedMac := h.Sum(nil)

		if subtle.ConstantTimeCompare(computedMac, expectedMac) != 1 {
			return nil, errors.New("HMAC verification failed: ciphertext was tampered or key is invalid")
		}

		// Decrypt AES-256-CBC
		return s.aesCbcDecrypt(ciphertext, encKey, iv)

	case 0:
		// AesCbc256_B64 (Legacy without MAC): "0.iv|ciphertext"
		if len(subParts) != 2 {
			return nil, errors.New("type 0 cipher string requires iv|ciphertext")
		}
		iv, err := base64.StdEncoding.DecodeString(subParts[0])
		if err != nil {
			return nil, fmt.Errorf("failed to decode IV: %w", err)
		}
		ciphertext, err := base64.StdEncoding.DecodeString(subParts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
		}

		return s.aesCbcDecrypt(ciphertext, encKey, iv)

	default:
		return nil, fmt.Errorf("unsupported cipher encryption type: %d", encType)
	}
}

func (s *VaultwardenService) aesCbcDecrypt(ciphertext, key, iv []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return []byte{}, nil
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// PKCS7 Unpadding
	unpadded, err := pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("padding error: %w", err)
	}

	return unpadded, nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("empty data")
	}
	if length%blockSize != 0 {
		return nil, errors.New("data is not a multiple of blockSize")
	}
	padLen := int(data[length-1])
	if padLen == 0 || padLen > blockSize || padLen > length {
		return nil, errors.New("invalid PKCS7 padding length")
	}
	for i := length - padLen; i < length; i++ {
		if int(data[i]) != padLen {
			return nil, errors.New("invalid PKCS7 padding bytes")
		}
	}
	return data[:length-padLen], nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

// encryptCipherString encrypts plaintext UTF-8 into Bitwarden CipherString Type 2: "2.iv|ciphertext|mac"
func (s *VaultwardenService) encryptCipherString(plaintext string, encKey, macKey []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// 1. Generate 16 bytes IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	// 2. PKCS7 Padding
	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)

	// 3. AES-256-CBC Encrypt
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return "", err
	}
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// 4. Compute HMAC-SHA256 over (iv + ciphertext)
	h := hmac.New(sha256.New, macKey)
	h.Write(iv)
	h.Write(ciphertext)
	mac := h.Sum(nil)

	// 5. Format as Type 2: 2.iv|ciphertext|mac
	ivB64 := base64.StdEncoding.EncodeToString(iv)
	cipherB64 := base64.StdEncoding.EncodeToString(ciphertext)
	macB64 := base64.StdEncoding.EncodeToString(mac)

	return fmt.Sprintf("2.%s|%s|%s", ivB64, cipherB64, macB64), nil
}

// CreateCipher encrypts and posts a new credential directly to the remote Vaultwarden instance
func (s *VaultwardenService) CreateCipher(ctx context.Context, req domain.CreateVaultCipherRequest) (*domain.VaultCredentialItem, error) {
	cfg, err := s.repo.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve vaultwarden configuration: %w", err)
	}
	if cfg == nil || cfg.ServerURL == "" || cfg.Email == "" || cfg.MasterPassword == "" {
		return nil, errors.New("vaultwarden is not configured yet. Please configure Vaultwarden connection first")
	}

	authCtx, err := s.authenticateAndGetKeys(ctx, cfg.ServerURL, cfg.Email, cfg.MasterPassword)
	if err != nil {
		return nil, fmt.Errorf("vaultwarden authentication failed: %w", err)
	}

	encName, err := s.encryptCipherString(req.Name, authCtx.UserEncKey, authCtx.UserMacKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt name: %w", err)
	}

	var encNotes *string
	if req.Notes != "" {
		en, err := s.encryptCipherString(req.Notes, authCtx.UserEncKey, authCtx.UserMacKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt notes: %w", err)
		}
		encNotes = &en
	}

	cipherType := req.Type
	if cipherType <= 0 {
		cipherType = 1 // Default to Login
	}

	bodyMap := map[string]interface{}{
		"type":           cipherType,
		"folderId":       req.FolderID,
		"organizationId": nil,
		"name":           encName,
		"notes":          encNotes,
		"favorite":       false,
	}

	if cipherType == 1 {
		loginObj := make(map[string]interface{})
		if req.Username != "" {
			eu, err := s.encryptCipherString(req.Username, authCtx.UserEncKey, authCtx.UserMacKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt username: %w", err)
			}
			loginObj["username"] = eu
		}
		if req.Password != "" {
			ep, err := s.encryptCipherString(req.Password, authCtx.UserEncKey, authCtx.UserMacKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt password: %w", err)
			}
			loginObj["password"] = ep
		}
		if req.URI != "" {
			eu, err := s.encryptCipherString(req.URI, authCtx.UserEncKey, authCtx.UserMacKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt URI: %w", err)
			}
			loginObj["uris"] = []map[string]interface{}{
				{"uri": eu, "match": nil},
			}
		}
		bodyMap["login"] = loginObj
	} else if cipherType == 2 {
		bodyMap["secureNote"] = map[string]interface{}{"type": 0}
	}

	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to encode cipher request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/ciphers", authCtx.ServerURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+authCtx.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to post cipher to Vaultwarden: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vaultwarden rejected cipher creation (HTTP %d): %s", resp.StatusCode, string(raw))
	}

	var createdRaw struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&createdRaw)

	// Sync local vault cache in background
	go func() {
		_, _ = s.SyncVault(context.Background())
	}()

	uris := []string{}
	if req.URI != "" {
		uris = append(uris, req.URI)
	}

	return &domain.VaultCredentialItem{
		ID:           createdRaw.ID,
		Name:         req.Name,
		Type:         cipherType,
		TypeLabel:    map[int]string{1: "Login", 2: "Secure Note"}[cipherType],
		Username:     req.Username,
		Password:     req.Password,
		Notes:        req.Notes,
		URIs:         uris,
		RevisionDate: time.Now(),
	}, nil
}

// DeleteCipher removes a credential item from the remote Vaultwarden instance
func (s *VaultwardenService) DeleteCipher(ctx context.Context, cipherID string) error {
	cipherID = strings.TrimSpace(cipherID)
	if cipherID == "" {
		return errors.New("cipher ID is required")
	}

	cfg, err := s.repo.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve vaultwarden configuration: %w", err)
	}
	if cfg == nil || cfg.ServerURL == "" || cfg.Email == "" || cfg.MasterPassword == "" {
		return errors.New("vaultwarden is not configured")
	}

	authCtx, err := s.authenticateAndGetKeys(ctx, cfg.ServerURL, cfg.Email, cfg.MasterPassword)
	if err != nil {
		return fmt.Errorf("vaultwarden authentication failed: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/ciphers/%s", authCtx.ServerURL, url.PathEscape(cipherID))
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+authCtx.AccessToken)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to delete cipher from Vaultwarden: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vaultwarden rejected cipher deletion (HTTP %d): %s", resp.StatusCode, string(raw))
	}

	// Trigger vault sync to update local cache
	_, err = s.SyncVault(ctx)
	return err
}

// UpdateCipher encrypts and updates an existing credential directly in the remote Vaultwarden instance
func (s *VaultwardenService) UpdateCipher(ctx context.Context, cipherID string, req domain.CreateVaultCipherRequest) (*domain.VaultCredentialItem, error) {
	cipherID = strings.TrimSpace(cipherID)
	if cipherID == "" {
		return nil, errors.New("cipher ID is required")
	}

	cfg, err := s.repo.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve vaultwarden configuration: %w", err)
	}
	if cfg == nil || cfg.ServerURL == "" || cfg.Email == "" || cfg.MasterPassword == "" {
		return nil, errors.New("vaultwarden is not configured yet. Please configure Vaultwarden connection first")
	}

	authCtx, err := s.authenticateAndGetKeys(ctx, cfg.ServerURL, cfg.Email, cfg.MasterPassword)
	if err != nil {
		return nil, fmt.Errorf("vaultwarden authentication failed: %w", err)
	}

	encName, err := s.encryptCipherString(req.Name, authCtx.UserEncKey, authCtx.UserMacKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt name: %w", err)
	}

	var encNotes *string
	if req.Notes != "" {
		en, err := s.encryptCipherString(req.Notes, authCtx.UserEncKey, authCtx.UserMacKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt notes: %w", err)
		}
		encNotes = &en
	}

	cipherType := req.Type
	if cipherType <= 0 {
		cipherType = 1 // Default to Login
	}

	bodyMap := map[string]interface{}{
		"type":           cipherType,
		"folderId":       req.FolderID,
		"organizationId": nil,
		"name":           encName,
		"notes":          encNotes,
		"favorite":       false,
	}

	if cipherType == 1 {
		loginObj := make(map[string]interface{})
		if req.Username != "" {
			eu, err := s.encryptCipherString(req.Username, authCtx.UserEncKey, authCtx.UserMacKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt username: %w", err)
			}
			loginObj["username"] = eu
		}
		if req.Password != "" {
			ep, err := s.encryptCipherString(req.Password, authCtx.UserEncKey, authCtx.UserMacKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt password: %w", err)
			}
			loginObj["password"] = ep
		}
		if req.URI != "" {
			eu, err := s.encryptCipherString(req.URI, authCtx.UserEncKey, authCtx.UserMacKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt URI: %w", err)
			}
			loginObj["uris"] = []map[string]interface{}{
				{"uri": eu, "match": nil},
			}
		}
		bodyMap["login"] = loginObj
	} else if cipherType == 2 {
		bodyMap["secureNote"] = map[string]interface{}{"type": 0}
	}

	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to encode cipher request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/ciphers/%s", authCtx.ServerURL, url.PathEscape(cipherID))
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+authCtx.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to put cipher to Vaultwarden: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vaultwarden rejected cipher update (HTTP %d): %s", resp.StatusCode, string(raw))
	}

	// Sync local vault cache in background
	go func() {
		_, _ = s.SyncVault(context.Background())
	}()

	uris := []string{}
	if req.URI != "" {
		uris = append(uris, req.URI)
	}

	return &domain.VaultCredentialItem{
		ID:           cipherID,
		Name:         req.Name,
		Type:         cipherType,
		TypeLabel:    map[int]string{1: "Login", 2: "Secure Note"}[cipherType],
		Username:     req.Username,
		Password:     req.Password,
		Notes:        req.Notes,
		URIs:         uris,
		RevisionDate: time.Now(),
	}, nil
}

