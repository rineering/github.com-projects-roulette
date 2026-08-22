package handlers


// import (
// 	"fmt"
// 	"strconv"
// 	"log"
// 	"net/http"
// 	"strings"
// 	"io"
// 	"encoding/csv"

// 	"roulette/state"
// )

// // Обработчик импорта Google Таблицы с учетом объединенных ячеек
// func HandleImportPrizes(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	url := r.FormValue("google_sheets_url")
// 	if url == "" {
// 		http.Error(w, "URL is required", http.StatusBadRequest)
// 		return
// 	}

// 	log.Printf("[Import] Запуск скачивания наград по ссылке: %s", url)
	
// 	prizes, err := downloadAndParsePrizes(url)
// 	if err != nil {
// 		log.Printf("[Import] Ошибка импорта: %v", err)
// 		http.Error(w, fmt.Sprintf("Ошибка импорта: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// Сохраняем обновленный конфиг с новыми наградами в AppData
// 	if err := state.UpdatePrizes(prizes); err != nil {
// 		log.Printf("[Import] Ошибка сохранения config.json: %v", err)
// 		http.Error(w, "Ошибка сохранения локального конфига", http.StatusInternalServerError)
// 		return
// 	}

// 	log.Printf("[Import] Успешно импортировано наград: %d", len(prizes))
// 	http.Redirect(w, r, "/admin#manual", http.StatusSeeOther)
// }

// func downloadAndParsePrizes(url string) ([]state.Prize, error) {
// 	if strings.Contains(url, "/edit") {
// 		// Отрезаем всё, что идет после /edit (включая сам /edit)
// 		parts := strings.Split(url, "/edit")
// 		// И собираем прямую ссылку на экспорт первого листа в формате CSV
// 		url = parts[0] + "/export?format=csv"

// 		gid := strings.Split(parts[1], "#")
// 		if len(gid) > 1 {
// 			url += "&" + gid[1]
// 		}
// 	}
	
// 	log.Printf("[Parser] Итоговый URL для запроса к Google: %s", url)
		
	
// 	log.Printf("[Parser] Запрос к эндпоинту Google: %s", url)

// 	resp, err := http.Get(url)
// 	if err != nil {
// 		return nil, fmt.Errorf("не удалось выполнить запрос: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		return nil, fmt.Errorf("google вернул статус: %s", resp.Status)
// 	}

// 	reader := csv.NewReader(resp.Body)
// 	reader.Comma = ',' // Так как мы принудительно засилили format=csv
// 	reader.FieldsPerRecord = -1
// 	reader.LazyQuotes = true 
	
// 	var newPrizes []state.Prize
// 	currentRarity := ""
// 	currentWeight := 1
// 	isFirstRow := true

// 	for {
// 		record, err := reader.Read()
// 		if err == io.EOF {
// 			break
// 		}
// 		if err != nil {
// 			return nil, err
// 		}

// 		if isFirstRow {
// 			isFirstRow = false
// 			continue
// 		}

// 		if len(record) < 1 || strings.TrimSpace(record[0]) == "" {
// 			continue
// 		}

// 		prizeName := strings.TrimSpace(record[0])

// 		if len(record) > 1 && strings.TrimSpace(record[1]) != "" {
// 			currentRarity = strings.TrimSpace(record[1])
// 		}

// 		if len(record) > 2 && strings.TrimSpace(record[2]) != "" {
// 			weightStr := strings.ReplaceAll(record[2], "%", "")
// 			weightStr = strings.TrimSpace(weightStr)
// 			if parsedWeight, err := strconv.Atoi(weightStr); err == nil {
// 				currentWeight = parsedWeight
// 			}
// 		}

// 		newPrizes = append(newPrizes, state.Prize{
// 			Category: currentRarity,
// 			Name:     prizeName,
// 			Weight:   currentWeight,
// 		})
// 	}
// 	return newPrizes, nil
// }