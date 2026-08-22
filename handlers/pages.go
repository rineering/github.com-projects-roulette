package handlers

import (
	"fmt"
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
	Config 		config.Config // config.json
	State 		state.State // state.json
	Profile 	state.RouletteProfile // профиль рулетки
	AlertStatus string
	AppDataPath string

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

	// catsMap := make(map[string]bool)
	// for _, prize := range allPrizes {
	// 	if prize.Category != "" { catsMap[prize.Category] = true }
	// }
	// var categories []string
	// for c := range catsMap { categories = append(categories, c) }

	// Собираем объединенные данные для страницы в структуру
	data := AdminPageData{
		Config: 		config.GetConfig(),
		State: 			state.GetState(),
		Profile:		state.GetActiveProfile(),
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