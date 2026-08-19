package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Prize struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Weight   int    `json:"weight"` // Здесь будет храниться процент очищенный от % (например 50, 32)
}

type Mode struct {
	ManualMode     bool   `json:"manual_mode"`     // активен ли ручной режим прямо сейчас
	ManualCategory string `json:"manual_category"` // какая категория крутится в ручном режиме
}

// State хранит текущий неделимый остаток баланса
type State struct {
	SavedDonationsTailBalance int `json:"saved_donations_tail_balance"`
	Prizes []Prize 				  `json:"prizes"`
}

var (
	mu 			sync.RWMutex
	st 			State
	statePath 	string
	rouletteMode Mode
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

// Сохранение состояния (включая награды и копейки) в state.json
func SaveState() error {
	mu.Lock()
	defer mu.Unlock()

	userConfig, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(userConfig, "RouletteService")
	
	// Сохраняем баланс и награды. Флаги ручного режима зануляем перед сохранением,
	// чтобы при перезапуске сервиса ручной режим всегда сбрасывался.
	toSave := st
	rouletteMode = Mode{false, ""}

	fileBytes, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "state.json"), fileBytes, 0644)
}

// Перезаписать список наград в памяти и сохранить на диск
func UpdatePrizes(newPrizes []Prize) error {
	mu.Lock()
	st.Prizes = newPrizes
	mu.Unlock()

	return SaveState()
}

// Безопасное получение копии наград для рандомизатора
func GetPrizes() []Prize {
	mu.RLock()
	defer mu.RUnlock()
	
	// Возвращаем копию слайса во избежание data race
	cp := make([]Prize, len(st.Prizes))
	copy(cp, st.Prizes)
	return cp
}

// Получить список уникальных категорий для селектора админки
func GetUniqueCategories() []string {
	mu.RLock()
	defer mu.RUnlock()

	catsMap := make(map[string]bool)
	for _, p := range st.Prizes {
		if p.Category != "" {
			catsMap[p.Category] = true
		}
	}

	var categories []string
	for c := range catsMap {
		categories = append(categories, c)
	}
	return categories
}

// Потокобезопасное включение ручного режима
func SetManualMode(active bool, category string) {
	mu.Lock()
	defer mu.Unlock()
	
	rouletteMode.ManualMode = active
	rouletteMode.ManualCategory = category
	
	// Флаги ручного режима можно даже НЕ сохранять в state.json,
	// чтобы при перезапуске микросервиса ручной режим всегда сбрасывался в false!
}

// Потокобезопасное чтение флага ручного режима
func IsManualMode() bool {
	mu.RLock()
	defer mu.RUnlock()
	return rouletteMode.ManualMode
}

// Получить текущую ручную категорию
func GetManualCategory() string {
	mu.RLock()
	defer mu.RUnlock()
	return rouletteMode.ManualCategory
}