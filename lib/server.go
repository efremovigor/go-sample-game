package lib

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"html/template"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{}
var store = sessions.NewFilesystemStore("./session", []byte("MTU4MjQ0NTc0NnxEdi1CQkFFQ180SU"))
var RequestChan chan UserRequest

type UserRequest struct {
	SessionId string
	Request   LoginJsonRequest
	Receiver  *ConnectionReceiver
}

type LoginJsonRequest struct {
	Type    string  `json:"type"`
	Payload Payload `json:"payload"`
}

type Payload struct {
	Username string `json:"username"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type ConnectionReceiver struct {
	conn         *websocket.Conn
	readChannel  chan string
	WriteChannel chan []byte
	closeConnect chan int
}

func (receiver ConnectionReceiver) pushData(jsonObject interface{}) {
	responseJson, _ := json.Marshal(jsonObject)
	receiver.WriteChannel <- responseJson
}

func getSession(r *http.Request) (session *sessions.Session) {
	session, _ = store.Get(r, "session-Name")
	return
}

func webHandler(w http.ResponseWriter, _ *http.Request) {
	tmpl, _ := template.ParseFiles("static/index.html")
	if err := tmpl.Execute(w, ""); err != nil {
		log.Fatalf("404: %v", err)
	}
}

func webSocketHandler(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session.IsNew {
		if err := session.Save(r, w); err != nil {
			panic(err)
		}
	}
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	receiver := &ConnectionReceiver{conn: conn, readChannel: make(chan string), WriteChannel: make(chan []byte), closeConnect: make(chan int)}

	go func() {

		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		go func(receiver *ConnectionReceiver) {
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					receiver.closeConnect <- 0
					break
				}
				receiver.handleRequest(message, session)
			}
		}(receiver)

		go func(receiver *ConnectionReceiver) {
			for {
				err := conn.WriteMessage(1, <-receiver.WriteChannel)
				if err != nil {
					receiver.closeConnect <- 0
					break
				}
			}
		}(receiver)

		for {
			select {
			case <-receiver.closeConnect:
				if game, ok := Games[session.ID]; ok {
					game.stopGame()
				}
				_ = conn.Close()
				break
			}
		}
	}()
}

func (receiver *ConnectionReceiver) handleRequest(message []byte, session *sessions.Session) {
	var request LoginJsonRequest
	err := json.Unmarshal(message, &request)
	if err != nil {
		switch string(message) {
		case "update":
			return
		}
	}
	RequestChan <- UserRequest{Request: request, SessionId: session.ID, Receiver: receiver}
}

func RunServer(socket string, requests chan UserRequest) {
	RequestChan = requests

	router := mux.NewRouter()
	router.HandleFunc("/", webHandler)
	router.HandleFunc("/finish", webHandler)
	router.HandleFunc("/ws", webSocketHandler)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	server := http.Server{
		Addr:    socket,
		Handler: router,
	}
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe(): %v", err)
	}
}
