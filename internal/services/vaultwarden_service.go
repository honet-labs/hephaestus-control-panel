package services

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
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

// -------------------------------------------------------------
// Bitwarden E2EE Decryption Engine
// -------------------------------------------------------------

func (s *VaultwardenService) fetchAndDecryptVault(ctx context.Context, serverURL, email, password string) ([]domain.VaultCredentialItem, error) {
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

	// Step 6: Sync Vault Data
	syncData, err := s.doSync(ctx, serverURL, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("vault sync failed: %w", err)
	}

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
