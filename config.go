package grainfs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// Configuration constants
	ConfigVersion     = "1.0.0"
	DefaultIterations = 100000
	SaltSize          = 32
	KeySize           = 32
	FilenameKeySize   = 32

	// Directory and file names
	GrainFSDir  = ".grainfs"
	ConfigFile  = "config.json"
	FilemapFile = "filemap.json"
)

// Config represents the GrainFS configuration stored in .grainfs/config.json
type Config struct {
	Salt       []byte `json:"salt"`
	Iterations int    `json:"iterations"`
	Version    string `json:"version"`
}

// initializeConfig creates a new configuration with random salt
func (fs *GrainFS) initializeConfig() error {
	// Generate random salt
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	config := &Config{
		Salt:       salt,
		Iterations: DefaultIterations,
		Version:    ConfigVersion,
	}

	return fs.saveConfig(config)
}

// loadConfig loads the configuration from .grainfs/config.json
func (fs *GrainFS) loadConfig() (*Config, error) {
	configPath := filepath.Join(GrainFSDir, ConfigFile)

	file, err := fs.underlying.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config doesn't exist, initialize it
			if err := fs.initializeConfig(); err != nil {
				return nil, fmt.Errorf("failed to initialize config: %w", err)
			}
			return fs.loadConfig()
		}
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Validate config
	if len(config.Salt) != SaltSize {
		return nil, fmt.Errorf("invalid salt size: expected %d, got %d", SaltSize, len(config.Salt))
	}
	if config.Iterations <= 0 {
		return nil, fmt.Errorf("invalid iterations: %d", config.Iterations)
	}

	return &config, nil
}

// saveConfig saves the configuration to .grainfs/config.json
func (fs *GrainFS) saveConfig(config *Config) error {
	// Ensure .grainfs directory exists
	if err := fs.underlying.MkdirAll(GrainFSDir, 0755); err != nil {
		return fmt.Errorf("failed to create .grainfs directory: %w", err)
	}

	configPath := filepath.Join(GrainFSDir, ConfigFile)

	file, err := fs.underlying.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// deriveKeys derives the master key and filename key from password and salt
func deriveKeys(password string, salt []byte, iterations int) (masterKey, filenameKey []byte) {
	// Derive master key for file content encryption
	masterKey = pbkdf2.Key([]byte(password), salt, iterations, KeySize, sha256.New)

	// Derive filename key using master key as input with different salt
	filenameSalt := append(salt, []byte("filename")...)
	filenameKey = pbkdf2.Key(masterKey, filenameSalt, iterations, FilenameKeySize, sha256.New)

	return masterKey, filenameKey
}

// ensureGrainFSDir ensures the .grainfs directory exists in the given directory
func (fs *GrainFS) ensureGrainFSDir(dir string) error {
	// Get the obfuscated directory path
	obfuscatedDir, err := fs.getObfuscatedPath(dir)
	if err != nil {
		return fmt.Errorf("failed to get obfuscated directory path: %w", err)
	}

	grainfsPath := filepath.Join(obfuscatedDir, GrainFSDir)
	return fs.underlying.MkdirAll(grainfsPath, 0755)
}
