package handlers

import (
	"fmt"
	"strconv"
	"log"
	"net/http"
	"strings"
	"io"
	"os"
	"time"
	"sync/atomic"
	"encoding/csv"
	"path/filepath"
	"archive/zip"

	"roulette/state"
)

//
// !----- импорт наград ------! 
//
func HandleImportPrizes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	profileSlug := r.FormValue("current_profile_slug")
	url := r.FormValue("google_sheets_url")
	if url == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	pr, err := state.ReadProfileBySlug(profileSlug)
	if err != nil {
		log.Printf("[Import] Ошибка чтения профиля: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка чтения профиля: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[Import] Запуск скачивания наград по ссылке: %s", url)
	
	cats, prizes, err := downloadAndParsePrizes(url)
	if err != nil {
		log.Printf("[Import] Ошибка импорта: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка импорта: %v", err), http.StatusInternalServerError)
		return
	}

	if len(prizes) == 0 {
		log.Println("[Warning] Внимание: призов 0! Возможно, таблица приватна или пуста.")
	} else {
		pr.Categories = cats
		pr.Prizes = prizes
		
		if err := state.SaveProfile(*pr); err != nil {
			log.Printf("[Error] Не удалось сохранить рулетку: %v", err)
			http.Error(w, "Ошибка сохранения файла рулетки", http.StatusInternalServerError)
			return
		}
	
		log.Println("[Roulette] Рулетка "+pr.Name+" успешно сохранена.")
	}

	log.Printf("[Import] Успешно импортировано наград: %d", len(prizes))

	redirectUrl := fmt.Sprintf("/admin?profile=%s&status=success#templates", profileSlug)
	http.Redirect(w, r, redirectUrl, http.StatusSeeOther)
}

var idCounter int64 = time.Now().UnixMilli() 
// Безопасно увеличивает значение на 1 и возвращает новый результат
func nextID() int64 {
	return atomic.AddInt64(&idCounter, 1)
}

// Вспомогательная функция парсинга «умного» разворота объединенных ячеек
func downloadAndParsePrizes(url string) ([]state.PrizeCategory, []state.Prize, error) {
	if strings.Contains(url, "/edit") {
		// Отрезаем всё, что идет после /edit (включая сам /edit)
		parts := strings.Split(url, "/edit")
		// И собираем прямую ссылку на экспорт первого листа в формате CSV
		url = parts[0] + "/export?format=csv"
		
		gid := strings.Split(parts[1], "#")
		if len(gid) > 1 {
			url += "&" + gid[1]
		}
	}
	
	log.Printf("[Parser] Итоговый URL для запроса к Google: %s", url)
		
	
	log.Printf("[Parser] Запрос к эндпоинту Google: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось выполнить запрос: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("google вернул статус: %s", resp.Status)
	}

	reader := csv.NewReader(resp.Body)
	reader.Comma = ',' // Так как мы принудительно засилили format=csv
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true 
	
	var newPrizes []state.Prize
	var newCats []state.PrizeCategory
	var prevId int64

	prevRarity := ""
	prevWeight := 1
	currentRarity := ""
	currentWeight := 1
	isFirstRow := true

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		if isFirstRow {
			isFirstRow = false
			continue
		}

		if len(record) < 1 || strings.TrimSpace(record[0]) == "" {
			continue
		}

		prizeName := strings.TrimSpace(record[0])

		if len(record) > 1 && strings.TrimSpace(record[1]) != "" {
			currentRarity = strings.TrimSpace(record[1])
		}

		if len(record) > 2 && strings.TrimSpace(record[2]) != "" {
			weightStr := strings.ReplaceAll(record[2], "%", "")
			weightStr = strings.TrimSpace(weightStr)
			if parsedWeight, err := strconv.Atoi(weightStr); err == nil {
				currentWeight = parsedWeight
			}
		}

		ms := nextID()
		if (prevRarity != currentRarity && prevWeight != currentWeight) {
			prevId = ms
			prevRarity = currentRarity
			prevWeight = currentWeight
			newCats = append(newCats, state.PrizeCategory{
				Id: 		prevId,
				Name:     	prevRarity,
				Weight:   	prevWeight,
			})
		} else {
			ms = prevId
		}

		newPrizes = append(newPrizes, state.Prize{
			Id: 		nextID(),
			Name:     	prizeName,
			CatId:   	ms,
		})
	}
	return newCats, newPrizes, nil
}

//
// !----- загрузка файлов шаблонов ------! 
//
func HandleUploadTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		log.Printf("[Upload Error] Критическая ошибка фронтенда! Запрос пришел в неверном формате: '%s'. Ожидался multipart/form-data", contentType)
		http.Error(w, "Ошибка отправки данных: фронтенд не передал файлы (тип запроса должен быть multipart)", http.StatusBadRequest)
		return
	}

	// Ограничиваем максимальный размер файла до 25 МБ (с запасом для картинок/звуков в ZIP)
	r.Body = http.MaxBytesReader(w, r.Body, 25*1024*1024)
	if err := r.ParseMultipartForm(25 * 1024 * 1024); err != nil {
		http.Error(w, "Файл слишком большой (макс. 25МБ)", http.StatusBadRequest)
		log.Printf("[Template] Файл слишком большой: %v", err)
		return
	}

	file, header, err := r.FormFile("template_file")
	if err != nil {
		http.Error(w, "Ошибка получения файла из формы", http.StatusBadRequest)
		log.Printf("[Template] Ошибка получения файла из формы: %v", err)
		return
	}
	defer file.Close()

	profileSlug := r.FormValue("current_profile_slug")
	pr, err := state.ReadProfileBySlug(profileSlug)
	if err != nil {
		log.Printf("[Import] Ошибка чтения профиля: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка чтения профиля: %v", err), http.StatusInternalServerError)
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".zip" && ext != ".html" && ext != ".htm" {
		http.Error(w, "Разрешены только файлы .html или .zip архивы", http.StatusBadRequest)
		log.Printf("[Template] Неправильное расширения файлов: %v", err)
		return
	}

	// Имя шаблона — это имя файла без расширения
	templateName := strings.TrimSuffix(header.Filename, ext)
	templateName = strings.ReplaceAll(templateName, " ", "_") // убираем опасные пробелы

	log.Printf("[Template] Название шаблона: %s", templateName)

	templatesBaseDir, err := state.ResolveTemplatesDirPath()
	if err != nil {
		http.Error(w, "Ошибка операционной системы", http.StatusInternalServerError)
		log.Printf("[Template] Ошибка операционной системы: %v", err)
		return
	}

	targetDir := filepath.Join(templatesBaseDir, templateName)

	// СЦЕНАРИЙ 1: Пользователь загрузил одиночный HTML-файл
	if ext == ".html" || ext == ".htm" {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			http.Error(w, "Не удалось создать папку шаблона", http.StatusInternalServerError)
			log.Printf("[Template] Не удалось создать папку шаблона: %v", err)
			return
		}
		
		// Сохраняем всегда как index.html, чтобы хендлер /overlay всегда его находил
		finalPath := filepath.Join(targetDir, "index.html")
		outFile, err := os.Create(finalPath)
		if err != nil {
			http.Error(w, "Ошибка создания файла на сервере", http.StatusInternalServerError)
			log.Printf("[Template] Ошибка создания файла на сервере: %v", err)
			return
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, file); err != nil {
			http.Error(w, "Ошибка записи файла", http.StatusInternalServerError)
			log.Printf("[Template] Ошибка записи файла: %v", err)
			return
		}
		log.Printf("[Template] Успешно загружен одиночный HTML-шаблон: %s", templateName)
	}

	// СЦЕНАРИЙ 2: Пользователь загрузил ZIP-архив со звуками и графикой
	if ext == ".zip" {
		tempFile, err := os.CreateTemp("", "upload-*.zip")
		if err != nil {
			http.Error(w, "Ошибка сервера при обработке архива", http.StatusInternalServerError)
			return
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		if _, err := io.Copy(tempFile, file); err != nil {
			http.Error(w, "Ошибка сохранения архива", http.StatusInternalServerError)
			return
		}

		// Вызываем нашу функцию unzipTarget (код из прошлых шагов)
		if err := unzipTarget(tempFile.Name(), targetDir); err != nil {
			log.Printf("[Template] Ошибка распаковки %s: %v", header.Filename, err)
			http.Error(w, "Ошибка распаковки ZIP-архива", http.StatusInternalServerError)
			return
		}
		log.Printf("[Template] Успешно распакован ZIP-шаблон: %s", templateName)
	}

	if pr.Template == "" {
		pr.Template = templateName

		if err := state.SaveProfile(*pr); err != nil {
			log.Printf("[Error] Не удалось сохранить рулетку: %v", err)
			http.Error(w, "Ошибка сохранения файла рулетки", http.StatusInternalServerError)
			return
		}

		log.Println("[Roulette] Рулетка "+pr.Name+" успешно сохранена.")
	}

	// Возвращаемся на вкладку оформления
	//http.Redirect(w, r, "/#templates", http.StatusSeeOther)
	// redirectUrl := fmt.Sprintf("/?profile=%s&status=success#templates", profileSlug)
	// http.Redirect(w, r, redirectUrl, http.StatusSeeOther)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // Отдаем статус 200 вместо 303

	// Возвращаем JSON с урлом для редиректа
	redirectUrl := fmt.Sprintf("/admin?profile=%s&status=success#templates", profileSlug)
	fmt.Fprintf(w, `{"redirect": "%s"}`, redirectUrl)
}

// Вспомогательная функция безопасной распаковки ZIP
func unzipTarget(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Защита от атаки Zip Slip (выход за пределы целевой папки через "../")
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("обнаружен небезопасный путь в архиве: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}