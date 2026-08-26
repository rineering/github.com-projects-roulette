package handlers

import (
	"fmt"
	"strconv"
	"log"
	"net/http"
	"net/url"
	"os"
	"encoding/csv"
	"time"
	"io"
	"strings"
	"encoding/json"
	"sync/atomic"

	"os/exec"
	"runtime"

	"roulette/config" // Замените на имя вашего модуля из go.mod
	"roulette/state"
)


func HandleHomePage(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    http.ServeFile(w, r, "static/index.html")
}

// данные формы #configForm - tab_conf.html
func HandleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		// В случае ошибки парсинга формы уходим на статус error
		http.Redirect(w, r, "/admin?status=error", http.StatusSeeOther)
		return
	}

	// Переменная для отслеживания ошибок валидации чисел
	hasError := false

	updatedCfg := config.GetConfig()
	updatedCfg.ServerPort = r.FormValue("server_port")
	updatedCfg.DaUserID = r.FormValue("da_user_id")
	updatedCfg.DaToken = r.FormValue("da_token")

	if err := config.Save(&updatedCfg); err != nil {
		hasError = true
		log.Printf("[Error] Не удалось сохранить конфиг: %v", err)
		http.Error(w, "Ошибка сохранения файла", http.StatusInternalServerError)
		return
	}

	tailBalance, err := strconv.ParseFloat(r.FormValue("saved_donations_tail_balance"), 64)
	if err != nil {
		hasError = true
		log.Printf("[Warning] Не удалось распарсить saved_donations_tail_balance, оставляем старое значение. Ошибка: %v", err)
	} else {
		state.SetBalance(int(tailBalance * 100))
	}

	// Если на этапе конвертации чисел была ошибка, не сохраняем файл, а сразу сигнализируем пользователю
	if hasError {
		http.Redirect(w, r, "/admin?status=error", http.StatusSeeOther)
		return
	}

	log.Println("[Config] Настройки успешно обновлены!")

	http.Redirect(w, r, "/admin?status=success", http.StatusSeeOther)
}

// останавливает сервер рулетки прямо из браузера
func HandleShutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<h3>Сервер рулетки успешно остановлен. Теперь вы можете закрыть эту вкладку.</h3>")
	
	log.Println("[System] Получен сигнал на выключение из админки. Завершение работы...")

	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

// автоматически открывает указанную ссылку в браузере
func OpenBrowser(url string) {
    // Даем серверу полсекунды, чтобы он точно успел подняться в памяти
    time.Sleep(500 * time.Millisecond)

    var cmd *exec.Cmd

    switch runtime.GOOS {
    case "windows":
        cmd = exec.Command("cmd", "/c", "start", url)
    case "darwin": // macOS
        cmd = exec.Command("open", url)
    case "linux":
        cmd = exec.Command("xdg-open", url)
    default:
        log.Printf("Неподдерживаемая ОС для автооткрытия браузера. Ссылка: %s", url)
        return
    }

    if err := cmd.Start(); err != nil {
        log.Printf("Не удалось открыть браузер автоматически: %v", err)
    }
}

// 1. создание профиля рулетки с #rouletteProfileForm - tab_template.html
func HandleSaveProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		// В случае ошибки парсинга формы уходим на статус error
		http.Redirect(w, r, "/admin?status=error#templates", http.StatusSeeOther)
		return
	}

	// Читаем сырые байты из тела запроса
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Ошибка чтения body:", err)
	}

	queryString := string(bodyBytes)
	values, err := url.ParseQuery(queryString)
	if err != nil {
		log.Println("Ошибка парсинга формы:", err)
	}

	slug := values.Get("slug")
	name := values.Get("name")

	amountFloat, err := strconv.ParseFloat(values.Get("amount"), 64)
	if err != nil {
		log.Printf("[Warning] Значение цены прокрутки неверно: %v", err)
	}
	amount := int(amountFloat * 100)

	delay, err := strconv.Atoi(values.Get("delay"))
	if err != nil {
		log.Printf("[Warning] Значение таймаута неверно: %v", err)
	}
	
	categoriesSlice := values["categories"] 
	prizesSlice := values["prizes"]
	//log.Println("Строка категорий:", categoriesRaws)

	if slug == "" || name == "" {
		http.Error(w, "Slug и Name не могут быть пустыми", http.StatusBadRequest)
		return
	}

	rouletteProfile, err := state.ReadProfileBySlug(slug)

	var profile state.RouletteProfile

	if err != nil {
		// Если профиля еще нет (создается новый), инициализируем дефолтными значениями
		profile = state.RouletteProfile{
			Slug: 			"",
			Name: 			"",
			Template:    	"",
			MinAmountSpin: 	10000, // 100 рублей по дефолту
			StartDelay: 	0,
			Categories: 	[]state.PrizeCategory{},
		}
	} else {
		profile = *rouletteProfile
	}

	if (len(profile.Slug) == 0) {
		profile.Slug = slug
	}
	profile.Name = name
	profile.MinAmountSpin = amount
	profile.StartDelay = delay

	// Создаем срез для хранения готовых структур
	var cats []state.PrizeCategory
	// Перебираем каждую JSON-строку в цикле
	for _, jsonStr := range categoriesSlice {
		log.Println("Строка категории:", jsonStr)
		var cat state.PrizeCategory
		if err := json.Unmarshal([]byte(jsonStr), &cat); err == nil {
			cats = append(cats, cat)
		}
	}
	profile.Categories = cats

	var prizes []state.Prize
	for _, itemJson := range prizesSlice {
		log.Println("Строка награды:", itemJson)
		var item state.Prize
		if err := json.Unmarshal([]byte(itemJson), &item); err == nil {
			prizes = append(prizes, item)
		}
	}
	profile.Prizes = prizes

	if err := state.SaveProfile(profile); err != nil {
		log.Printf("[Error] Не удалось сохранить рулетку: %v", err)
		http.Error(w, "Ошибка сохранения файла рулетки", http.StatusInternalServerError)
		return
	}
	
	log.Println("[Roulette] Рулетка "+profile.Name+" успешно сохранена.")

	if (len(state.GetState().ActiveProfileSlug) == 0) {
		if err := state.SetActiveProfile(profile.Slug); err != nil {
			log.Printf("[Error] Первая созданная рулетка не активировалась: %v", err)
		} else {
			log.Println("[Roulette] Автоматически установили "+profile.Name+" активной рулеткой.")
		}
	}

	allProfiles := state.GetAllProfiles()

	// Ищем, есть ли уже профиль с таким slug в общем списке, чтобы обновить его
	found := false
	for i, p := range allProfiles {
		if p.Slug == profile.Slug {
			allProfiles[i] = profile
			found = true
			break
		}
	}
	// Если профиль абсолютно новый — добавляем его в общий список
	if !found {
		allProfiles = append(allProfiles, profile)
		log.Println("[Roulette] Добавили "+profile.Name+" в список рулеток.")
	}

	// Сохраняем обновленный КУМУЛЯТИВНЫЙ список профилей
	if err := state.UpdateProfiles(allProfiles); err != nil {
		log.Printf("[Error] Не удалось обновить общий список рулеток: %v", err)
	}

	http.Redirect(w, r, "/admin?status=success#templates", http.StatusSeeOther)
}

//
// !----- импорт наград ------! 
//
func HandleImportPrizes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	profileSlug := r.FormValue("current_profile_slug")
	url := r.FormValue("google_sheets_url")
	if url == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	pr, err := state.ReadProfileBySlug(profileSlug)
	if err != nil {
		log.Printf("[Import] Ошибка чтения профиля: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка чтения профиля: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[Import] Запуск скачивания наград по ссылке: %s", url)
	
	cats, prizes, err := downloadAndParsePrizes(url)
	if err != nil {
		log.Printf("[Import] Ошибка импорта: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка импорта: %v", err), http.StatusInternalServerError)
		return
	}

	if len(prizes) == 0 {
		log.Println("[Warning] Внимание: призов 0! Возможно, таблица приватна или пуста.")
	} else {
		pr.Categories = cats
		pr.Prizes = prizes
		
		if err := state.SaveProfile(*pr); err != nil {
			log.Printf("[Error] Не удалось сохранить рулетку: %v", err)
			http.Error(w, "Ошибка сохранения файла рулетки", http.StatusInternalServerError)
			return
		}
	
		log.Println("[Roulette] Рулетка "+pr.Name+" успешно сохранена.")
	}

	log.Printf("[Import] Успешно импортировано наград: %d", len(prizes))
	http.Redirect(w, r, "/admin?status=success", http.StatusSeeOther)
}

var idCounter int64 = time.Now().UnixMilli() 

func nextID() int64 {
	// Безопасно увеличивает значение на 1 и возвращает новый результат
	return atomic.AddInt64(&idCounter, 1)
}

// Вспомогательная функция парсинга «умного» разворота объединенных ячеек
func downloadAndParsePrizes(url string) ([]state.PrizeCategory, []state.Prize, error) {
	if strings.Contains(url, "/edit") {
		// Отрезаем всё, что идет после /edit (включая сам /edit)
		parts := strings.Split(url, "/edit")
		// И собираем прямую ссылку на экспорт первого листа в формате CSV
		url = parts[0] + "/export?format=csv"
		
		gid := strings.Split(parts[1], "#")
		if len(gid) > 1 {
			url += "&" + gid[1]
		}
	}
	
	log.Printf("[Parser] Итоговый URL для запроса к Google: %s", url)
		
	
	log.Printf("[Parser] Запрос к эндпоинту Google: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось выполнить запрос: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("google вернул статус: %s", resp.Status)
	}

	reader := csv.NewReader(resp.Body)
	reader.Comma = ',' // Так как мы принудительно засилили format=csv
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true 
	
	var newPrizes []state.Prize
	var newCats []state.PrizeCategory
	var prevId int64

	prevRarity := ""
	prevWeight := 1
	currentRarity := ""
	currentWeight := 1
	isFirstRow := true

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		if isFirstRow {
			isFirstRow = false
			continue
		}

		if len(record) < 1 || strings.TrimSpace(record[0]) == "" {
			continue
		}

		prizeName := strings.TrimSpace(record[0])

		if len(record) > 1 && strings.TrimSpace(record[1]) != "" {
			currentRarity = strings.TrimSpace(record[1])
		}

		if len(record) > 2 && strings.TrimSpace(record[2]) != "" {
			weightStr := strings.ReplaceAll(record[2], "%", "")
			weightStr = strings.TrimSpace(weightStr)
			if parsedWeight, err := strconv.Atoi(weightStr); err == nil {
				currentWeight = parsedWeight
			}
		}

		ms := nextID()
		if (prevRarity != currentRarity && prevWeight != currentWeight) {
			prevId = ms
			prevRarity = currentRarity
			prevWeight = currentWeight
			newCats = append(newCats, state.PrizeCategory{
				Id: 		prevId,
				Name:     	prevRarity,
				Weight:   	prevWeight,
			})
		} else {
			ms = prevId
		}

		newPrizes = append(newPrizes, state.Prize{
			Id: 		nextID(),
			Name:     	prizeName,
			CatId:   	ms,
		})
	}
	return newCats, newPrizes, nil
}