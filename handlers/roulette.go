package handlers


import (
	"strconv"
	"log"
	"net/http"
	"time"
	"strings"

	"roulette/state"
	"roulette/da"
	"roulette/hub"
)

// Обработчик кнопки запуска ручного режима
func HandleTriggerManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	category := r.FormValue("category")
	state.SetManualMode(true, category)

	// Асинхронно запускаем прокрутку, чтобы не вешать веб-интерфейс админки
	go func() {
		prizes := state.GetPrizes()
		prize, err := da.SelectRandomPrize(prizes, category)
		if err != nil {
			log.Printf("[Manual] Ошибка выбора награды: %v", err)
			state.SetManualMode(false, "")
			return
		}

		log.Printf("[Roulette] РЕЗУЛЬТАТ ВРАЩЕНИЯ: [%s] %s. Отправка в OBS...", prize.Category, prize.Name)
		
		// ВЫЗОВ ХАБА: Отправляем команду на запуск колеса в OBS
		activeTemplate := state.GetActiveTemplate()
		hub.BroadcastSpin(prize.Name, prize.Category, activeTemplate)
		
		// Имитируем время прокрутки рулетки стримера (анимация в CSS длится, например, 10 секунд)
		// На это время бэкенд блокирует прием сетевых донат-сообщений
		time.Sleep(10 * time.Second)

		state.SetManualMode(false, "")
		log.Println("[Manual] Ручной режим завершен, оверлей свободен.")
	}()

	http.Redirect(w, r, "/admin#manual", http.StatusSeeOther)
}

func HandleSavePrizesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим мульти-инпуты формы (массивы prize_name[], prize_category[], prize_weight[])
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ошибка парсинга формы", http.StatusBadRequest)
		return
	}

	names := r.Form["prize_name[]"]
	categories := r.Form["prize_category[]"]
	weights := r.Form["prize_weight[]"]

	var updatedPrizes []state.Prize

	for i := 0; i < len(names); i++ {
		// Проверяем границы срезов, на всякий случай
		if i >= len(categories) || i >= len(weights) {
			break
		}

		name := strings.TrimSpace(names[i])
		category := strings.TrimSpace(categories[i])
		
		weight, err := strconv.Atoi(weights[i])
		if err != nil || weight < 1 {
			weight = 1
		}

		if name != "" {
			updatedPrizes = append(updatedPrizes, state.Prize{
				Name:     name,
				Category: category,
				Weight:   weight,
			})
		}
	}

	// Сохраняем получившийся список в стейт и на диск в state.json
	if err := state.UpdatePrizes(updatedPrizes); err != nil {
		log.Printf("[Admin] Ошибка сохранения отредактированных наград: %v", err)
		http.Error(w, "Ошибка сохранения списка", http.StatusInternalServerError)
		return
	}

	log.Printf("[Admin] Список наград успешно отредактирован вручную. Всего: %d", len(updatedPrizes))
	
	// Перенаправляем обратно на главную
	http.Redirect(w, r, "/admin#manual", http.StatusSeeOther)
}