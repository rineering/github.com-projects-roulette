package hub

// import (
// 	"log"
// 	"sync"

// 	"github.com/gorilla/websocket"
// 	"net/http"
// )

// var upgrader = websocket.Upgrader{
// 	CheckOrigin: func(r *http.Request) bool {
// 		return true // Разрешаем OBS подключаться локально
// 	},
// }

// type WsHub struct {
// 	clients    map[*websocket.Conn]bool
// 	clientsMu  sync.Mutex
// 	Register   chan *websocket.Conn
// 	Unregister chan *websocket.Conn
// }

// var GlobalHub *WsHub

// func InitHub() {
// 	GlobalHub = &WsHub{
// 		clients:    make(map[*websocket.Conn]bool),
// 		Register:   make(chan *websocket.Conn),
// 		Unregister: make(chan *websocket.Conn),
// 	}
// 	go GlobalHub.run()
// }

// func (h *WsHub) run() {
// 	for {
// 		select {
// 		case conn := <-h.Register:
// 			h.clientsMu.Lock()
// 			h.clients[conn] = true
// 			h.clientsMu.Unlock()
// 			log.Println("[WS Hub] OBS-оверлей успешно подключен")

// 		case conn := <-h.Unregister:
// 			h.clientsMu.Lock()
// 			if _, ok := h.clients[conn]; ok {
// 				delete(h.clients, conn)
// 				conn.Close()
// 				log.Println("[WS Hub] OBS-оверлей отключен")
// 			}
// 			h.clientsMu.Unlock()
// 		}
// 	}
// }

// // Обработчик WebSocket роута
// func HandleWs(w http.ResponseWriter, r *http.Request) {
// 	conn, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		log.Printf("[WS] Ошибка апгрейда соединения: %v", err)
// 		return
// 	}
// 	GlobalHub.Register <- conn

// 	// Держим соединение открытым, читаем пинги/сообщения от клиента
// 	go func() {
// 		defer func() {
// 			GlobalHub.Unregister <- conn
// 		}()
// 		for {
// 			_, _, err := conn.ReadMessage()
// 			if err != nil {
// 				break
// 			}
// 		}
// 	}()
// }

// // Функция отправки команды запуска анимации во все открытые оверлеи OBS
// func BroadcastSpin(prizeName string, rarity string, templateName string) {
// 	h := GlobalHub
// 	h.clientsMu.Lock()
// 	defer h.clientsMu.Unlock()

// 	// Структура JSON-пакета для фронтенда
// 	payload := map[string]string{
// 		"action":   "spin",
// 		"prize":    prizeName,
// 		"rarity":   rarity,
// 		"template": templateName,
// 	}

// 	for client := range h.clients {
// 		err := client.WriteJSON(payload)
// 		if err != nil {
// 			log.Printf("[WS] Ошибка отправки данных клиенту: %v", err)
// 			client.Close()
// 			delete(h.clients, client)
// 		}
// 	}
// }
