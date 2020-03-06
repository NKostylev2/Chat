package main

import (
	"./websocket" 
	"fmt"
	"net/http"
	"sync"
	"os"
)

type ChatRoom struct {
	clients map[string]Client
	clientsMtx sync.Mutex
	queue   chan string
}

func (cr *ChatRoom) Init() {
	cr.queue = make(chan string, 5)
	cr.clients = make(map[string]Client)

	go func() {
		for {
			cr.BroadCast()
		}
	}()
}

func (cr *ChatRoom) Join(name string, conn *websocket.Conn) *Client {
	defer cr.clientsMtx.Unlock();

	cr.clientsMtx.Lock();
	if _, exists := cr.clients[name]; exists {
		return nil
	}
	client := Client{
		name:      name,
		conn:      conn,
		belongsTo: cr,
	}
	cr.clients[name] = client
	 
	cr.AddMsg("<B>" + name + "</B> присоеденился.")
	return &client
}

func (cr *ChatRoom) Leave(name string) {
	cr.clientsMtx.Lock(); 
	delete(cr.clients, name)
	cr.clientsMtx.Unlock(); 
	cr.AddMsg("<B>" + name + "</B> вышел из чата.")
}

func (cr *ChatRoom) AddMsg(msg string) {
	cr.queue <- msg
}

func (cr *ChatRoom) BroadCast() {
	msgBlock := ""
infLoop:
	for {
		select {
		case m := <-cr.queue:
			msgBlock += m + "<BR>"
		default:
			break infLoop
		}
	}
	if len(msgBlock) > 0 {
		for _, client := range cr.clients {
			client.Send(msgBlock)
		}
	}
}


type Client struct {
	name      string
	conn      *websocket.Conn
	belongsTo *ChatRoom
}

func (cl *Client) NewMsg(msg string) {
	cl.belongsTo.AddMsg("<B>" + cl.name + ":</B> " + msg)
}

func (cl *Client) Exit() {
	cl.belongsTo.Leave(cl.name)
}

func (cl *Client) Send(msgs string) {
	cl.conn.WriteMessage(websocket.TextMessage, []byte(msgs))
}

var chat ChatRoom

func staticFiles(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./net"+r.URL.Path)
}
	
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		fmt.Println("Не удалось обновить:", err)
		return
	}
	go func() {
		_, msg, err := conn.ReadMessage()
		client := chat.Join(string(msg), conn)
		if client == nil || err != nil {
			conn.Close() 
			return
		}

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				client.Exit()
				return
			}
			client.NewMsg(string(msg))
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
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/", staticFiles)
	chat.Init()
	http.ListenAndServe(":8000", nil)
}