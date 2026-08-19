package handlers

import (
	"log"
	"os"
	"path/filepath"
)

var logPath string

// InitLogger настраивает запись всех логов в файл внутри папки приложения
func InitLogger() error {
	// Узнаем системный путь к папке RouletteService
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	appDir := filepath.Join(baseDir, "RouletteService")

	// Убедимся, что папка существует
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return err
	}

	// Открываем файл app.log
	logPath := filepath.Join(appDir, "app.log")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// ⚠️ САМОЕ ВАЖНОЕ: Перенаправляем глобальный логгер Go в этот файл
	log.SetOutput(logFile)
	
	return nil
}