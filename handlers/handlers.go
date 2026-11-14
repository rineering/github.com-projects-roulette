package handlers

import (
	"fmt"
	"strconv"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
	"io"
	//"strings"
	"encoding/json"
	"crypto/sha256"
	"encoding/hex"

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

func HashConfigLocation(name string) string {
	str := sha256.Sum256([]byte(name))
	return hex.EncodeToString(str[:]) // Строка из 64 символов [0-9a-f]
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

	name := values.Get("name")
	tmpl := values.Get("template")
	slug := HashConfigLocation(name)

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
	} else {

		if tmpl != "" {
			profile.Template = tmpl
		}

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
	}
	profile.Name = name
	profile.MinAmountSpin = amount
	profile.StartDelay = delay

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

	redirectUrl := fmt.Sprintf("/admin?profile=%s&status=success#templates", slug)
	http.Redirect(w, r, redirectUrl, http.StatusSeeOther)
}