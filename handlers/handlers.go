package handlers

import (
	"fmt"
	"strconv"
	"log"
	"net/http"
	"os"
	"time"

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

// HandleSaveConfig принимает данные формы из админки и сохраняет в config.json
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

	activeProfile := state.GetActiveProfile()

	// Конвертируем StartDelaySeconds из строки в int
	delay, err := strconv.Atoi(r.FormValue("start_delay_seconds"))
	if err != nil {
		hasError = true
		log.Printf("[Warning] Не удалось распарсить start_delay_seconds, оставляем старое значение. Ошибка: %v", err)
	} else {
		activeProfile.StartDelay = delay
	}

	// Конвертируем MinAmountOneSpin из строки в int
	minAmount, err := strconv.Atoi(r.FormValue("min_amount_one_spin"))
	if err != nil {
		hasError = true
		log.Printf("[Warning] Не удалось распарсить min_amount_one_spin, оставляем старое значение. Ошибка: %v", err)
	} else {
		activeProfile.MinAmountSpin = minAmount
	}

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

	// ВАЖНО: перенаправляем обратно на /admin
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
	// При успешном исходе добавляем параметр ?status=success
	http.Redirect(w, r, "/admin?status=success", http.StatusSeeOther)
}

// HandleShutdown останавливает сервер рулетки прямо из браузера
func HandleShutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<h3>Сервер рулетки успешно остановлен. Теперь вы можете закрыть эту вкладку.</h3>")
	
	log.Println("[System] Получен сигнал на выключение из админки. Завершение работы...")

	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

// openBrowser автоматически открывает указанную ссылку в стандартном браузере ОС
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