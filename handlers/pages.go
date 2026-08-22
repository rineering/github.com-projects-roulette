package handlers

import (
	"fmt"
	//"strconv"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"roulette/config"
	"roulette/state"
)

// AdminPageData объединяет всё, что нужно отобразить на странице админки
type AdminPageData struct {
	Config 			config.Config // config.json
	State 			state.State // state.json
	ActiveProfile 	state.RouletteProfile // профиль активной рулетки
	AlertStatus 	string
	AppDataPath 	string

}

func HandleAdminPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin" {
		http.NotFound(w, r)
		return
	}

	// Используем template.ParseFS вместо ParseFiles, чтобы читать файл прямо из памяти .exe
	tmpl, err := template.ParseFS(
		staticFS, 
		"static/admin/admin.html",
		"static/admin/tab_conf.html",
		"static/admin/tab_template.html",
		"static/admin/tab_handle.html",
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка компиляции шаблонов: %v", err), http.StatusInternalServerError)
		return
	}

	status := r.URL.Query().Get("status")
	// Получаем скрытый системный путь (AppData/Roaming или Application Support) для вывода в инструкцию
	baseDir, _ := os.UserConfigDir()
	appDir := filepath.Join(baseDir, "RouletteService")

	// Собираем объединенные данные для страницы в структуру
	data := AdminPageData{
		Config: 		config.GetConfig(),
		State: 			state.GetState(),
		ActiveProfile:	state.GetActiveProfile(),
		AlertStatus: 	status, // Передаем статус (success/error) в HTML
		AppDataPath: 	appDir, // Путь к скрытым файлам
	}

	// заголовки ответа и рендерим страницу в браузер
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = tmpl.ExecuteTemplate(w, "admin.html", data)
	if err != nil {
		log.Printf("[Error] Ошибка заполнения шаблона: %v", err)
	}
}

// 1. Метод вызывается, когда выбирают профиль в селекте (Запрос от hx-get="/admin/profile/load")
func loadProfileHandler(w http.ResponseWriter, r *http.Request) {
	//idStr := r.URL.Query().Get("profile_id")
	//sysName, _ := strconv.Atoi(idStr)

	activeProfile := state.GetActiveProfile()

	// Парсим тот же файл таба
	tmpl, _ := template.ParseFiles("static/admin/tab_template.html")
	
	// ВАЖНО: Рендерим ТОЛЬКО блок "profile_form_block" и передаем туда структуру профиля
	tmpl.ExecuteTemplate(w, "profile_form_block", activeProfile)
}

// 2. Метод вызывается, когда нажали "+ Новая рулетка" (Запрос от hx-get="/admin/profile/new-form")
func newProfileFormHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.ParseFiles("static/admin/tab_handle.html")
	
	// Передаем ПУСТУЮ структуру. В инпутах везде будет пусто, а ID будет равен 0
	emptyProfile := state.RouletteProfile{}
	tmpl.ExecuteTemplate(w, "profile_form_block", emptyProfile)
}

//// Конвертируем StartDelaySeconds из строки в int
	// delay, err := strconv.Atoi(r.FormValue("start_delay_seconds"))
	// if err != nil {
	// 	hasError = true
	// 	log.Printf("[Warning] Не удалось распарсить start_delay_seconds, оставляем старое значение. Ошибка: %v", err)
	// } else {
	// 	activeProfile.StartDelay = delay
	// }