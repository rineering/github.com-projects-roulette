package main

import (
	"embed"
    "log"
    "net/http"

    "roulette/config"
    "roulette/handlers"
    "roulette/state"
)

//go:embed static/*
var StaticFS embed.FS

func main() {
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
	http.HandleFunc("/", handlers.HandleHomePage)
    http.HandleFunc("/admin", handlers.HandleAdminPage)
    http.HandleFunc("/api/save-config", handlers.HandleSaveConfig)
    http.HandleFunc("/api/shutdown", handlers.HandleShutdown)
	http.HandleFunc("/import-prizes", handlers.HandleImportPrizes)
	http.HandleFunc("/save-manual", handlers.HandleTriggerManual)

	// 2. Маршрут для OBS (корень)
	// http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	// 	if r.URL.Path != "/" {
	// 		http.NotFound(w, r)
	// 		return
	// 	}
	// 	// В будущем здесь будет отдаваться виджет рулетки для OBS
	// 	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 	fmt.Fprint(w, "<h1>Тут будет крутиться рулетка в OBS</h1>")
	// })

	// 3. Раздача статических файлов (Bootstrap, CSS, JS)
	//fs := http.FileServer(http.Dir("./static"))
	//http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.Handle("/static/", http.FileServer(http.FS(StaticFS)))

	cfg := config.Get()
	//log.Printf("Панель управления доступна на http://localhost:%s/admin", cfg.ServerPort)
	//log.Printf("Ссылка для OBS виджета: http://localhost:%s/", cfg.ServerPort)


    // Запускаем автооткрытие браузера в отдельном потоке (горутине),
    // чтобы код не ждал открытия браузера, а сразу пошел запускать сервер
	url := "http://localhost:"+cfg.ServerPort+"/admin"
    go handlers.OpenBrowser(url)
    
    if err := http.ListenAndServe(":"+cfg.ServerPort, nil); err != nil {
        log.Fatalf("Ошибка сервера: %v", err)
    }
}

