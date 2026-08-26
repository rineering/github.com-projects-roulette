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
	Profiles		[]state.RouletteProfile
	AlertStatus 	string
	AppDataPath 	string

}

func formatMoney(amount int) string {
	return fmt.Sprintf("%.2f", float64(amount)/100.0)
}

var funcMap = template.FuncMap{
		"money": formatMoney,
	}

func HandleAdminPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin" {
		http.NotFound(w, r)
		return
	}

	tmpl := template.New("admin.html").Funcs(funcMap)
	
	// Используем template.ParseFS вместо ParseFiles, чтобы читать файл прямо из памяти .exe
	tmpl, err := tmpl.ParseFS(
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
		Profiles:		state.GetAllProfiles(),
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

// 1. Метод вызывается, когда выбирают профиль в селекте (Запрос от hx-get="/admin-form/profile")
func LoadProfileHandler(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("profile_slug")
	action := r.URL.Query().Get("action")

	log.Printf("[Debug] Мы пришли за формой профиля, slug: %s , action: %s", slug, action)

	// Структура, которую мы передадим конкретно во фрагмент формы
	type FormData struct {
		Profile       *state.RouletteProfile
		State         state.State
		ActiveProfile state.RouletteProfile
	}
	var data FormData

	data.State = state.GetState()
	data.ActiveProfile = state.GetActiveProfile()

	// Логика определения того, какой профиль показать в форме:
	if action == "new" {
		// Нажали кнопку "Создать" -> отдаем пустой профиль
		data.Profile = &state.RouletteProfile{}
	} else if slug != "" {
		// Пользователь сам выбрал профиль в селекте
		profileL, err := state.ReadProfileBySlug(slug)
		if err != nil {
			log.Printf("[Error] Ошибка чтения профиля по slug %s: %v", slug, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data.Profile = profileL
	} else if data.ActiveProfile.Slug != "" {
		// Сработал триггер `load` при старте страницы: slug пустой, 
		// но у нас есть активный профиль в базе. Загружаем его по умолчанию!
		data.Profile = &data.ActiveProfile
	} else {
		// Вообще ничего нет (база пуста) -> отдаем пустую структуру
		data.Profile = &state.RouletteProfile{}
	}

	tmpl := template.New("static/admin/tab_template.html").Funcs(funcMap)
	// Парсим файл таба (Используй ParseFiles, как у тебя, если это отдельный вызов)
	tmpl, err := tmpl.ParseFiles("static/admin/tab_template.html")
	if err != nil {
		log.Printf("[Error] Ошибка парсинга файла шаблона: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Важно: имя "tab_template_add_profile" должно быть внутри файла через {{define}}
	err = tmpl.ExecuteTemplate(w, "tab_template_add_profile", data)
	if err != nil {
		log.Printf("[Error] Ошибка выполнения шаблона: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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