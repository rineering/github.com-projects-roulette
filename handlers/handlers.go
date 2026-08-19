package handlers

import (
	"embed"
	"fmt"
	"html/template"
	"strconv"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"strings"

	"os/exec"
	"runtime"

	"roulette/config" // Замените на имя вашего модуля из go.mod
	"roulette/state"
)

var staticFS *embed.FS

// InitStatic сохраняет вшитую файловую систему внутри пакета handlers
func InitStatic(fs *embed.FS) {
	staticFS = fs
}

// AdminPageData объединяет всё, что нужно отобразить на странице админки
type AdminPageData struct {
	Config       		config.Config // Все настройки из config.json
	TailBalance  		float64       // Баланс в рублях (переведенный из копеек)
	AlertStatus  		string
	AppDataPath  		string
	AvailableTemplates  []string
}

func HandleHomePage(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    http.ServeFile(w, r, "static/index.html")
}

// HandleAdminPage отвечает за отображение главной страницы админки
func HandleAdminPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin" {
		http.NotFound(w, r)
		return
	}

	// === ЧАСТЬ 1: Находим и сканируем папку пользовательских шаблонов рулетки ===
	
	// Узнаем, где лежит сам исполняемый файл нашей программы (.exe)
	exePath, err := os.Executable()
	if err != nil {
		http.Error(w, "Ошибка определения пути программы", http.StatusInternalServerError)
		return
	}
	exeDir := filepath.Dir(exePath)

	// Строим путь к папке "templates" прямо рядом с программой
	templatesDir := filepath.Join(exeDir, "templates")

	// Автоматически создаем её при первом запуске, если её ещё нет
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		log.Printf("[Error] Не удалось создать папку templates: %v", err)
	}

	// Сканируем папку на наличие вложенных папок-шаблонов
	files, err := os.ReadDir(templatesDir)
	var available []string
	if err == nil {
		for _, f := range files {
			// Проверяем, что это папка и пропускаем скрытые системные файлы (вроде .DS_Store на Маке)
			if f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
				available = append(available, f.Name())
			}
		}
	}

	// === ЧАСТЬ 2: Загружаем сам файл админки admin.html ===

	// Используем template.ParseFS вместо ParseFiles, чтобы читать файл прямо из памяти .exe
	tmpl, err := template.ParseFS(staticFS, "static/admin.html")
	if err != nil {
		log.Printf("[Error] Не удалось прочитать вшитый шаблон: %v", err)
		http.Error(w, "Ошибка шаблона", http.StatusInternalServerError)
		return
	}

	// === ЧАСТЬ 3: Собираем данные и отправляем стримеру ===

	// Читаем статус из URL (например, /admin?status=success)
	status := r.URL.Query().Get("status")
	// Получаем скрытый системный путь (AppData/Roaming или Application Support) для вывода в инструкцию
	baseDir, _ := os.UserConfigDir()
	appDir := filepath.Join(baseDir, "RouletteService")

	// Собираем объединенные данные для страницы в структуру
	data := AdminPageData{
		Config:             config.Get(),                       // Берем актуальный конфиг
		TailBalance:        float64(state.GetBalance()) / 100.0, // Переводим копейки в рубли для стримера
		AlertStatus:        status,                             // Передаем статус (success/error) в HTML
		AppDataPath:        appDir,                             // Путь к скрытым файлам
		AvailableTemplates: available,                          // Список папок для таба "Внешний вид"
	}

	// Устанавливаем заголовки ответа и рендерим страницу в браузер
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Printf("[Error] Ошибка заполнения шаблона: %v", err)
	}
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

	updatedCfg := config.Get()
	updatedCfg.ServerPort = r.FormValue("server_port")
	updatedCfg.DaUserID = r.FormValue("da_user_id")
	updatedCfg.SocketConnectionToken = r.FormValue("da_token")

	// Конвертируем StartDelaySeconds из строки в int
	delay, err := strconv.Atoi(r.FormValue("start_delay_seconds"))
	if err != nil {
		hasError = true
		log.Printf("[Warning] Не удалось распарсить start_delay_seconds, оставляем старое значение. Ошибка: %v", err)
	} else {
		updatedCfg.StartDelaySeconds = delay
	}

	// Конвертируем MinAmountOneSpin из строки в int
	minAmount, err := strconv.Atoi(r.FormValue("min_amount_one_spin"))
	if err != nil {
		hasError = true
		log.Printf("[Warning] Не удалось распарсить min_amount_one_spin, оставляем старое значение. Ошибка: %v", err)
	} else {
		updatedCfg.MinAmountOneSpin = minAmount
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
