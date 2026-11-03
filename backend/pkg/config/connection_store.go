package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sqlmanager/pkg/secret"
	"sync"
	"time"
)

// dataDir is the runtime data directory, computed lazily via getDataDir().
// Layout: binary lives at <root>/app/bin/<exe>, data directory is <root>/data
// (matching the package layout produced by the backend Makefile).
var (
	dataDirOnce sync.Once
	dataDirPath string
)

func getDataDir() string {
	dataDirOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			// fallback: use the current working directory
			cwd, _ := os.Getwd()
			dataDirPath = filepath.Join(cwd, "data")
		} else {
			// <root>/app/bin/<exe> -> up 2 levels -> <root>/data
			binDir := filepath.Dir(exe)
			dataDirPath = filepath.Clean(filepath.Join(binDir, "../../data"))
		}
		if err := os.MkdirAll(dataDirPath, 0755); err != nil {
			log.Printf("[ConnectionStore] WARNING: cannot create data directory %s: %v", dataDirPath, err)
		}
		log.Printf("[ConnectionStore] data directory: %s", dataDirPath)
	})
	return dataDirPath
}

// DataDir returns the runtime data directory used by DBManager.
func DataDir() string {
	return getDataDir()
}

func encryptionFilePath() string {
	return filepath.Join(getDataDir(), ".dbmanager_connections.enc")
}
func encryptionFileBakPath() string {
	return filepath.Join(getDataDir(), ".dbmanager_connections.enc.bak")
}
func keyFilePath() string {
	return filepath.Join(getDataDir(), ".dbmanager_secret")
}

// SavedConnection represents a saved database connection configuration
type SavedConnection struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	DBType    string    `json:"dbType"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Username  string    `json:"username"`
	Password  string    `json:"password"` // Will be encrypted
	Database  string    `json:"database"`
	FilePath  string    `json:"filePath"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ConnectionStore manages encrypted connection configurations
type ConnectionStore struct {
	mu            sync.RWMutex
	encryptionKey []byte
}

var (
	globalStore *ConnectionStore
	storeOnce   sync.Once
)

// GetConnectionStore returns the global connection store instance
func GetConnectionStore() *ConnectionStore {
	storeOnce.Do(func() {
		key, err := loadOrCreateEncryptionKey()
		if err != nil {
			// This should not happen in practice (random generation does not fail);
			// panic to surface the problem loudly.
			panic(fmt.Sprintf("[ConnectionStore] failed to load or create encryption key: %v", err))
		}
		globalStore = &ConnectionStore{
			encryptionKey: key,
		}
	})
	return globalStore
}

// loadOrCreateEncryptionKey obtains the encryption key by priority:
//  1. DB_ENCRYPTION_KEY environment variable, or the file named by
//     DB_ENCRYPTION_KEY_FILE (64-char hex; keeps the key off the data
//     volume, e.g. a Docker/K8s secret, or shared by several instances)
//  2. Local key file .dbmanager_secret (randomly generated and persisted on first start)
func loadOrCreateEncryptionKey() ([]byte, error) {
	envKey, err := secret.FromEnv("DB_ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}

	// Priority 1: environment variable or key file outside the data dir
	if envKey != "" {
		key, err := hex.DecodeString(envKey)
		if err == nil && len(key) == 32 {
			log.Println("[ConnectionStore] using DB_ENCRYPTION_KEY / DB_ENCRYPTION_KEY_FILE")
			return key, nil
		}
		log.Println("[ConnectionStore] WARNING: DB_ENCRYPTION_KEY(_FILE) is invalid (expect 64-char hex), ignoring")
	}

	// Priority 2: read the local key file
	kf := keyFilePath()
	if data, err := os.ReadFile(kf); err == nil {
		key, err := hex.DecodeString(string(data))
		if err == nil && len(key) == 32 {
			return key, nil
		}
		// File contents are corrupted, regenerate
		log.Printf("[ConnectionStore] WARNING: key file %s is invalid, regenerating", kf)
	}

	// First start: generate a random 32-byte key and persist it
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	keyHex := hex.EncodeToString(key)
	if err := os.WriteFile(kf, []byte(keyHex), 0600); err != nil {
		return nil, fmt.Errorf("failed to save key file: %w", err)
	}
	log.Printf("[ConnectionStore] first start, generated random encryption key at %s", kf)
	return key, nil
}

// encrypt encrypts data using AES-256-GCM
func (cs *ConnectionStore) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(cs.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// decrypt decrypts data using AES-256-GCM
func (cs *ConnectionStore) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(cs.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// loadConnections loads and decrypts connections from file.
// If the file is corrupted or was encrypted with a different key, it backs up
// the old file and returns an empty map so new connections can be saved normally.
func (cs *ConnectionStore) loadConnections() (map[string]*SavedConnection, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	enc := encryptionFilePath()
	bak := encryptionFileBakPath()

	// Check if file exists
	if _, err := os.Stat(enc); os.IsNotExist(err) {
		return make(map[string]*SavedConnection), nil
	}

	// Read encrypted file
	encryptedData, err := os.ReadFile(enc)
	if err != nil {
		return nil, fmt.Errorf("failed to read connections file: %w", err)
	}

	if len(encryptedData) == 0 {
		return make(map[string]*SavedConnection), nil
	}

	// Decrypt data
	decryptedData, err := cs.decrypt(encryptedData)
	if err != nil {
		log.Printf("[ConnectionStore] WARNING: cannot decrypt connections file (%v). "+
			"Backing up to %s and starting fresh.\n", err, bak)
		_ = os.Rename(enc, bak)
		return make(map[string]*SavedConnection), nil
	}

	// Parse JSON
	var connections map[string]*SavedConnection
	if err := json.Unmarshal(decryptedData, &connections); err != nil {
		log.Printf("[ConnectionStore] WARNING: cannot parse connections file (%v). "+
			"Backing up to %s and starting fresh.\n", err, bak)
		_ = os.Rename(enc, bak)
		return make(map[string]*SavedConnection), nil
	}

	return connections, nil
}

// saveConnections encrypts and saves connections to file
func (cs *ConnectionStore) saveConnections(connections map[string]*SavedConnection) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Serialize to JSON
	data, err := json.Marshal(connections)
	if err != nil {
		return fmt.Errorf("failed to serialize connections: %w", err)
	}

	// Encrypt data
	encryptedData, err := cs.encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt connections: %w", err)
	}

	// Write to file with restrictive permissions
	if err := os.WriteFile(encryptionFilePath(), encryptedData, 0600); err != nil {
		return fmt.Errorf("failed to write connections file: %w", err)
	}

	return nil
}

// SaveConnection saves or updates a connection configuration
func (cs *ConnectionStore) SaveConnection(conn *SavedConnection) error {
	connections, err := cs.loadConnections()
	if err != nil {
		return err
	}

	// Generate ID if new
	if conn.ID == "" {
		conn.ID = generateID()
		conn.CreatedAt = time.Now()
	}
	conn.UpdatedAt = time.Now()

	connections[conn.ID] = conn

	return cs.saveConnections(connections)
}

// ListConnections returns all saved connections (without passwords)
func (cs *ConnectionStore) ListConnections() ([]*SavedConnection, error) {
	connections, err := cs.loadConnections()
	if err != nil {
		return nil, err
	}

	result := make([]*SavedConnection, 0, len(connections))
	for _, conn := range connections {
		// Create copy without password
		safeCopy := *conn
		safeCopy.Password = "" // Don't expose password in list
		result = append(result, &safeCopy)
	}

	return result, nil
}

// GetConnection retrieves a single connection by ID (with password)
func (cs *ConnectionStore) GetConnection(id string) (*SavedConnection, error) {
	connections, err := cs.loadConnections()
	if err != nil {
		return nil, err
	}

	conn, exists := connections[id]
	if !exists {
		return nil, fmt.Errorf("connection not found: %s", id)
	}

	return conn, nil
}

// DeleteConnection removes a connection by ID
func (cs *ConnectionStore) DeleteConnection(id string) error {
	connections, err := cs.loadConnections()
	if err != nil {
		return err
	}

	if _, exists := connections[id]; !exists {
		return fmt.Errorf("connection not found: %s", id)
	}

	delete(connections, id)

	return cs.saveConnections(connections)
}

// generateID generates a unique ID for a connection
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
