package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"./websocket"
)

var chat chatroom

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type client struct {
	name    string
	conn    *websocket.Conn
	belongs *chatroom
}

type chatroom struct {
	clients    map[string]client
	clientsMtx sync.Mutex // Инициализация мьютекса
	queue      chan string //Инициализация канала
}

func (cr *chatroom) init() {
	cr.queue = make(chan string) 
	cr.clients = make(map[string]client)

	go func() {
		for {
			cr.BroadCast()
		}
	}()
}

func (cr *chatroom) join(name string, conn *websocket.Conn) *client {
	// Оператор defer откладывает выполнение функции до того момента, как произойдет возврат из окружающей функции.
	defer cr.clientsMtx.Unlock()
    // предотвращение одновременного доступа к карте клиентов
	cr.clientsMtx.Lock()
	if _, exists := cr.clients[name]; exists {
		return nil
	}
	client := client{
		name:    name,
		conn:    conn,
		belongs: cr,
	}
	cr.clients[name] = client

	cr.AddMsg("<B>" + name + "</B> присоеденился.")
	return &client
}

func (cr *chatroom) leave(name string) {
	cr.clientsMtx.Lock()
	delete(cr.clients, name)
	cr.clientsMtx.Unlock()
	cr.AddMsg("<B>" + name + "</B> вышел из чата.")
}
// Отправление сообщений в канал
func (cr *chatroom) AddMsg(msg string) {
	cr.queue <- msg
}
// Трансляция сообщений
func (cr *chatroom) BroadCast() {
	msgBlock := ""
inf:
	for {
		select {
		case m := <-cr.queue:
			msgBlock += m + "<BR>"
		default:
			break inf
		}
	}
	if len(msgBlock) > 0 {
		for _, client := range cr.clients {
			client.send(msgBlock)
		}
	}
}

func (cl *client) newmsg(msg string) {
	cl.belongs.AddMsg("<B>" + cl.name + ":</B> " + msg)
}

func (cl *client) exit() {
	cl.belongs.leave(cl.name)
}

func (cl *client) send(msgs string) {
	cl.conn.WriteMessage(websocket.TextMessage, []byte(msgs))
}

func fileServer(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./net"+r.URL.Path)
}

func handleMessages(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		fmt.Println("Не удалось обновить:", err)
		return
	}
	go func() {
		_, msg, err := conn.ReadMessage()
		client := chat.join(string(msg), conn)
		if client == nil || err != nil {
			conn.Close()
			return
		}

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				client.exit()
				return
			}
			client.newmsg(string(msg))
		}

	}()
}

func printHostname() {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println("http://" + hostname + ":8000/\n")

}

func main() {
	printHostname()
	http.HandleFunc("/ws", handleMessages)
	http.HandleFunc("/", fileServer)
	chat.init()
	http.ListenAndServe(":8000", nil)
}
