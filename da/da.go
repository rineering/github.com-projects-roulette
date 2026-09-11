package da

import (
	"crypto/rand"
	"math/big"
	"fmt"
	//"strings"
	"roulette/state"
)

// Глобальная (или внутри структуры) функция выбора награды
func SelectRandomPrize(prizes []state.Prize, categories []state.PrizeCategory) (state.Prize, error) {
	if len(prizes) == 0 {
		return state.Prize{}, fmt.Errorf("список наград пуст")
	}

	categoryWeights := make(map[int64]int, len(categories))
	for _, cat := range categories {
		categoryWeights[cat.Id] = cat.Weight
	}

	// СЦЕНАРИЙ 2: Общий режим (донаты или ручной запуск "Все категории")
	// Шаг 2.1: Считаем суммарный вес всех уникальных редкостей
	// Так как в вашей таблице вес (процент) прописан у каждой шмотки (благодаря нашему парсеру), 
	// нам нужно собрать уникальные пары "Редкость -> Процент"
	rarityWeights := make(map[int64]int)
	for _, p := range prizes {
		// Проверяем, существует ли категория в карте (заодно избегаем паники)
		// if weight, exists := categoryWeights[p.CatId]; exists {
		// 	fmt.Printf("Приз: %s, ID Категории: %d, Вес: %d\n", p.Name, p.CatId, weight)
		// } else {
		// 	fmt.Printf("Приз: %s имеет неизвестную категорию %d\n", p.Name, p.CatId)
		// }
		rarityWeights[p.CatId] = categoryWeights[p.CatId]
	}

	totalWeight := 0
	for _, w := range rarityWeights {
		totalWeight += w
	}

	if totalWeight == 0 {
		return state.Prize{}, fmt.Errorf("суммарный вес наград равен 0")
	}

	// Роллим случайное число в диапазоне от 0 до totalWeight-1
	roll, err := randInt(totalWeight)
	if err != nil {
		return state.Prize{}, err
	}

	// Определяем, какая редкость выпала
	var selectedRarity int64
	currentSum := 0
	for rarity, w := range rarityWeights {
		currentSum += w
		if roll < currentSum {
			selectedRarity = rarity
			break
		}
	}

	// Шаг 2.2: Выбираем случайный предмет внутри выпавшей редкости
	var pool []state.Prize
	for _, p := range prizes {
		if p.CatId == selectedRarity {
			pool = append(pool, p)
		}
	}

	if len(pool) == 0 {
		// На случай, если редкость в таблице есть, а предметов в ней почему-то нет
		idx, _ := randInt(len(prizes))
		return prizes[idx], nil
	}

	idx, _ := randInt(len(pool))
	return pool[idx], nil
}

// Криптографически безопасный генератор случайных чисел [0, max-1]
func randInt(max int) (int, error) {
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(nBig.Int64()), nil
}

// func (c *DonationAlertsClient) handleMessage(msg []byte) {
// 	// НОВОЕ: Проверяем динамическое состояние через потокобезопасный метод
// 	if state.IsManualMode() {
// 		log.Println("[DA] Получен донат, но проигнорирован: активен Ручной режим!")
// 		return
// 	}
    
//     // ... дальнейший парсинг доната ...
// }