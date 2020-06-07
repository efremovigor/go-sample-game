package main

import (
	"encoding/json"
	"sample-game/lib"
)

func handleRequest(request lib.UserRequest) {
	if request.Request.Type == lib.RequestTypeNewCommand {
		if connection, ok := lib.Connections[request.SessionId]; ok && connection.InGame {
			connection.Command <- request.Request.Payload.Code
			return
		}
	}
	playerConnection := getPlayConnection(request)

	if request.Request.Type == lib.RequestTypeNewPlayer {
		if len(lib.PlayersWait)%2 == 1 {
			request.Request.Type = lib.SignalToWaitOpponent
			response, _ := json.Marshal(request)
			playerConnection.Connection.WriteChannel <- response
		} else {
			lib.PlayerSelection()
		}
	}
}

func getPlayConnection(request lib.UserRequest) *lib.PlayerConnection {
	var playerConnection, ok = lib.Connections[request.SessionId]
	if !ok {
		playerConnection = &lib.PlayerConnection{Name: request.Request.Payload.Username, Connection: request.Receiver, Command: make(chan string)}
		lib.Connections[request.SessionId] = playerConnection
		lib.PlayersWait = append(lib.PlayersWait, playerConnection)
	}
	return playerConnection
}

func main() {
	lib.RequestChan = make(chan lib.UserRequest)

	go lib.RunServer("127.0.0.1:3000", lib.RequestChan)
	for {
		select {
		case request := <-lib.RequestChan:
			handleRequest(request)
		}
	}
}
