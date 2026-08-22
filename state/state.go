package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"log"
)

type Prize struct {
	Id 		int 	`json:"id"`
	Name 	string 	`json:"name"`
}

type PrizeCategory struct {
	Id 		int 	`json:"id"`
	Name 	string 	`json:"name"`
	Weight 	int 	`json:"weight"`
	Prizes 	[]Prize `json:"prizes"`
}

// Структура отдельной рулетки
type RouletteProfile struct {
	Slug			string  		`json:"slug"`
	Name           	string  		`json:"name"`            // название
	Template 		string  		`json:"template"` // шаблон оверлея
	MinAmountSpin  	int     		`json:"min_amount_spin"` // минимальный донат для прокрутки
	StartDelay 		int    			`json:"start_delay_seconds"`
	Categories      []PrizeCategory `json:"prize_categories"`          // список наград
}

type Mode struct {
	ManualMode     	bool   `json:"manual_mode"`     // активен ли ручной режим прямо сейчас
	ManualTemplate 	string `json:"manual_template"` // шаблон используемый для ручного запуска
	ManualCategory 	string `json:"manual_category"` // какая категория крутится в ручном режиме
}

// State хранит текущий неделимый остаток баланса
type State struct {
	DonateTail 		int 	`json:"saved_donations_tail_balance"`
	ActiveProfile 	string 	`json:"active_profile"` // Какая рулетка активна
	Profiles 		[]RouletteProfile
}

var (
	mu 				sync.RWMutex
	st 				State
	statePath 		string
	rouletteMode 	Mode
	activeProfile 	RouletteProfile
	profileDir 		string
)

// создание файла state.json
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
		st = State{DonateTail: 0}
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

func GetState() State {
	mu.RLock()
	defer mu.RUnlock()
	return st
}

// Сохранение состояния (включая награды и копейки) в state.json
func SaveState() error {
	userConfig, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(userConfig, "RouletteService")

	fileBytes, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "state.json"), fileBytes, 0644)
}

// GetBalance возвращает текущий сохраненный баланс в рублях
func GetBalance() int {
	mu.RLock()
	defer mu.RUnlock()
	return st.DonateTail
}

// обновляет баланс
func SetBalance(amount int) error {
	mu.Lock()
	defer mu.Unlock()

	st.DonateTail = amount
	return SaveState()
}

// Потокобезопасное включение ручного режима
func SetManualMode(active bool, template string, category string) {
	mu.Lock()
	defer mu.Unlock()
	
	rouletteMode.ManualMode = active
	rouletteMode.ManualTemplate = template
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

func GetManualCategory() string {
	mu.RLock()
	defer mu.RUnlock()
	return rouletteMode.ManualCategory
}

func GetManualTemplatet() string {
	mu.RLock()
	defer mu.RUnlock()
	return rouletteMode.ManualTemplate
}

//
// !----- папка profiles ------! 
//
func resolveProfilesDirPath() (string, error) {
	if profileDir != "" {
		return profileDir, nil
	}

	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(baseDir, "RouletteService")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}

	profileDir = filepath.Join(appDir, "profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		log.Fatalf("Не удалось создать папку для рулеток: %v", err)
		return "", err
	}
	return profileDir, nil
}

func ReadProfileBySlug(profileSlug string) (*RouletteProfile, error) {
	dir, err := resolveProfilesDirPath()
	profilePath := filepath.Join(dir, profileSlug+".json")

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, err
	}

	var profile RouletteProfile

	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

func GetActiveProfile() RouletteProfile {
	mu.RLock()
	slug := st.ActiveProfile
	mu.RUnlock() // Сразу же отпускаем мьютекс!
	
	roulette, err := ReadProfileBySlug(slug)
	if err != nil {
		return RouletteProfile{}
	}

	return *roulette
}

func SetActiveProfile(activeProfileSlug string) error {
	mu.Lock()
	defer mu.Unlock()

	st.ActiveProfile = activeProfileSlug
	return SaveState()
}

func SaveProfile(updProfile RouletteProfile) error {
	mu.Lock()
	defer mu.Unlock()
	
	dir, err := resolveProfilesDirPath()
	profilePath := filepath.Join(dir, updProfile.Slug+".json")

	data, err := json.MarshalIndent(updProfile, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(profilePath, data, 0644); err != nil {
		return err
	}

	return nil
}

func UpdateProfiles(profiles []RouletteProfile) error {
	mu.Lock()
	defer mu.Unlock()

	st.Profiles = profiles
	return SaveState()
}

func GetAllProfiles() []RouletteProfile {
	mu.RLock()
	defer mu.RUnlock()
	
	// Допустим, внутри AppState или глобально у вас хранится срез зарегистрированных рулеток
	cp := make([]RouletteProfile, len(st.Profiles))
	copy(cp, st.Profiles)
	return cp
}