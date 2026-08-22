package handlers

import (
	"fmt"
	"strconv"
	"log"
	"net/http"
	"os"
	"time"
	"strings"

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

	slug := strings.TrimSpace(r.FormValue("profile_slug"))
	name := strings.TrimSpace(r.FormValue("profile_name"))

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

	if err := state.SaveProfile(profile); err != nil {
		log.Printf("[Error] Не удалось сохранить рулетку: %v", err)
		http.Error(w, "Ошибка сохранения файла рулетки", http.StatusInternalServerError)
		return
	}
	
	log.Println("[Roulette] Рулетка "+profile.Name+" успешно сохранена.")

	if (len(state.GetState().ActiveProfile) == 0) {
		if err := state.SetActiveProfile(profile.Slug); err != nil {
			log.Printf("[Error] Первая созданная рулетка не активировалась: %v", err)
		} else {
			log.Println("[Roulette] Автоматически установили "+profile.Name+" активной рулеткой.")
		}
	}

	allProfiles := state.GetAllProfiles() // Напишите этот геттер в модуле state, если его нет

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