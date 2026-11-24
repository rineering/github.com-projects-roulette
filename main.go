package main

import (
	"embed"
    "log"
    "net/http"

    "github.com/projects/roulette/config"
    "github.com/projects/roulette/handlers"
    "github.com/projects/roulette/state"
    "github.com/projects/roulette/hub"
)

//go:embed static/*
var StaticFS embed.FS

func main() {
	// 1. Инициализация WebSocket Хаба
	hub.InitHub()

	// Передаем вшитую статику в пакет handlers при старте
	handlers.InitStatic(&StaticFS)

	// 1. Сразу включаем запись логов в файл через handlers
	if err := handlers.InitLogger(); err != nil {
		// Если логи не завелись, пишем ошибку в консоль и выключаем программу
		log.Fatalf("Критическая ошибка инициализации логов: %v", err)
	}
	log.Println("Инициализация сервера...")

    // 1. Загружаем конфигурацию и состояние при старте
    if _, err := config.Load(); err != nil {
        log.Fatalf("Ошибка config: %v", err)
    }
    if _, err := state.Load(); err != nil {
        log.Fatalf("Ошибка state: %v", err)
    }

    // 2. Регистрируем обработчики из нашего нового пакета handlers
	http.HandleFunc("/", handlers.HandleHomePage) // страница показа рулетки - для OBS
    http.HandleFunc("/admin", handlers.HandleAdminPage) // страница админки
    http.HandleFunc("/admin-form/save-config", handlers.HandleSaveConfig) // сохранение конфигов
    http.HandleFunc("/shutdown", handlers.HandleShutdown) // выключение
	http.HandleFunc("/admin-form/profile", handlers.LoadProfileHandler) // загрузка профиля рулетки
	http.HandleFunc("/admin-form/save-profile", handlers.HandleSaveProfile) // сохранение профиля
	http.HandleFunc("/admin-form/import-prizes", handlers.HandleImportPrizes) // импорт призов из гугл-таблиц
	http.HandleFunc("/admin-form/upload-template", handlers.HandleUploadTemplate) // загрузка файлов для рулетки
	http.HandleFunc("/save-manual", handlers.HandleTriggerManual) // ручной запуск рулетки

	// Роут для подключения OBS по WebSockets
	//http.HandleFunc("/", hub.HandleWs)
	// func(w http.ResponseWriter, r *http.Request) {
	// 	if r.URL.Path != "/" {
	// 		http.NotFound(w, r)
	// 		return
	// 	}
	// 	// В будущем здесь будет отдаваться виджет рулетки для OBS
	// 	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 	fmt.Fprint(w, "<h1>Тут будет крутиться рулетка в OBS</h1>")
	// }

	http.Handle("/static/", http.FileServer(http.FS(StaticFS)))

	cfg := config.GetConfig()

    // Запускаем автооткрытие браузера в отдельном потоке (горутине),
    // чтобы код не ждал открытия браузера, а сразу пошел запускать сервер
	url := "http://localhost:"+cfg.ServerPort+"/admin"
    go handlers.OpenBrowser(url)
    
    if err := http.ListenAndServe(":"+cfg.ServerPort, nil); err != nil {
        log.Fatalf("Ошибка сервера: %v", err)
    }
}

