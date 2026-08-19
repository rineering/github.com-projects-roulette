package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// State хранит текущий неделимый остаток баланса
type State struct {
	SavedDonationsTailBalance int `json:"saved_donations_tail_balance"`
}

var (
	mu 			sync.RWMutex
	st 			State
	statePath 	string
)

func resolvePath() (string, error) {
	if statePath != "" {
		return statePath, nil
	}

	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(baseDir, "RouletteService")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}

	statePath = filepath.Join(appDir, "state.json")
	return statePath, nil
}

// Load загружает состояние из файла или создает его с нулевым балансом
func Load() (*State, error) {
	mu.Lock()
	defer mu.Unlock()

	path, err := resolvePath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		st = State{SavedDonationsTailBalance: 0}
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, err
		}
		return &st, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}

	return &st, nil
}

// GetBalance возвращает текущий сохраненный баланс в рублях
func GetBalance() int {
	mu.RLock()
	defer mu.RUnlock()
	return st.SavedDonationsTailBalance
}

// SetBalance принудительно обновляет баланс и записывает в файл
func SetBalance(amount int) error {
	mu.Lock()
	defer mu.Unlock()

	path, err := resolvePath()
	if err != nil {
		return err
	}

	st.SavedDonationsTailBalance = amount
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}
