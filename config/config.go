package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	ServerPort            string `json:"server_port"`
	MinAmountOneSpin      int    `json:"min_amount_one_spin"`
	ActiveTemplate        string `json:"active_template"`
	StartDelaySeconds     int    `json:"start_delay_seconds"`
	SocketConnectionToken string `json:"socket_connection_token"`
	DaUserID              string `json:"da_user_id"`
}

var (
	mu         sync.RWMutex
	cfg        Config
	configPath string // Здесь будет храниться полный абсолютный путь к файлу
)

// Helper-функция для получения пути к файлу в системной папке
func resolvePath() (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	// Находит AppData/Roaming на Windows или ~/.config на Linux
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	// Создаем персональную папку для нашего приложения
	appDir := filepath.Join(baseDir, "RouletteService")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}

	configPath = filepath.Join(appDir, "config.json")
	return configPath, nil
}

func DefaultConfig() Config {
	return Config{
		ServerPort:            "8085",
		MinAmountOneSpin:      100,
		ActiveTemplate:        "default",
		StartDelaySeconds:     3,
		SocketConnectionToken: "",
		DaUserID:              "",
	}
}

func Load() (*Config, error) {
	mu.Lock()
	defer mu.Unlock()

	path, err := resolvePath()
	if err != nil {
		return nil, err
	}

	// Если файла нет по системному пути — генерируем дефолтный
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		cfg = DefaultConfig()
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, err
		}
		return &cfg, nil
	}

	// Если файл есть — читаем
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func Save(newCfg *Config) error {
	mu.Lock()
	defer mu.Unlock()

	path, err := resolvePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(newCfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	cfg = *newCfg
	return nil
}

func Get() Config {
	mu.RLock()
	defer mu.RUnlock()
	return cfg
}
