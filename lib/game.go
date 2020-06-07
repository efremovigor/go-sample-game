package lib

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const RequestTypeNewCommand = "newCommand"
const RequestTypeNewPlayer = "newPlayer"

const commandFire = "FIRE"
const commandUp = "UP"
const commandDown = "DOWN"
const commandLeft = "LEFT"
const commandRight = "RIGHT"

const bulletDirUp = "up"
const bulletDirDown = "down"

const SignalNewGameState = "SIGNAL_NEW_GAME_STATE"
const SignalStartTheGame = "SIGNAL_START_THE_GAME"
const SignalFinishGame = "SIGNAL_FINISH_GAME"
const SignalToWaitOpponent = "SIGNAL_TO_WAIT_OPPONENT"

const maxPlayerHp = 10

const maxXPosition = 19
const maxYPosition = 33

const minYPositionPlayer1 = 1
const maxYPositionPlayer1 = 6
const minYPositionPlayer2 = 27
const minXPositionPlayers = 1
const minYPositionPlayers = minYPositionPlayer1

const patternEndGame = "Игра окончена. Вы %s (%s(%d)/%s(%d))"

var PlayersWait []*PlayerConnection
var Connections = make(map[string]*PlayerConnection)

type PlayerConnection struct {
	Connection *ConnectionReceiver
	Name       string
	Command    chan string
	InGame     bool
}

func (p PlayerConnection) sendState(me PlayerGame, opponent PlayerGame, bullets []GameBullet) {

	response := StartGameStateResponse{
		Type: SignalNewGameState,
		Payload: StartGameStatePayload{
			Me:       me,
			Opponent: opponent,
			Bullets:  bullets,
		},
	}
	p.Connection.pushData(response)
}

type Game struct {
	Player1 *PlayerConnection
	Player2 *PlayerConnection
	state   GameState
}

func (game Game) sendStateGame() {
	game.Player1.sendState(game.state.Player1, game.state.Player2, game.state.Bullets)

	opponent := PlayerGame{
		Xpos: maxXPosition - game.state.Player1.Xpos,
		Ypos: maxYPosition - game.state.Player1.Ypos,
		Hp:   game.state.Player1.Hp,
	}

	me := PlayerGame{
		Xpos: maxXPosition - game.state.Player2.Xpos,
		Ypos: maxYPosition - game.state.Player2.Ypos,
		Hp:   game.state.Player2.Hp,
	}

	var bullets []GameBullet

	for _, bullet := range game.state.Bullets {
		if bullet.Dir == bulletDirUp {
			bullet.Dir = bulletDirDown
		}
		bullet.X = maxXPosition - bullet.X
		bullet.Y = maxYPosition - bullet.Y
		bullets = append(bullets, bullet)
	}

	game.Player2.sendState(me, opponent, bullets)

}

func (game Game) endGame(isWonPlayer1 bool) {
	response := LoginJsonRequest{
		Type:    SignalFinishGame,
		Payload: Payload{},
	}

	if isWonPlayer1 {
		response.Payload.Message = createEndMessage(game.state.Player1, game.state.Player2, "победил")
		game.Player1.Connection.pushData(response)
		response.Payload.Message = createEndMessage(game.state.Player1, game.state.Player2, "проиграл")
		game.Player2.Connection.pushData(response)
	} else {
		response.Payload.Message = createEndMessage(game.state.Player1, game.state.Player2, "победил")
		game.Player2.Connection.pushData(response)
		response.Payload.Message = createEndMessage(game.state.Player1, game.state.Player2, "проиграл")
		game.Player1.Connection.pushData(response)
	}
}

func createEndMessage(player1 PlayerGame, player2 PlayerGame, state string) string {
	return fmt.Sprintf(patternEndGame, state+"и", player1.Name, player1.Hp, player2.Name, player2.Hp)
}

func (p *PlayerGame) move(dir string, player1 bool) {
	switch dir {
	case commandUp:
		if player1 {
			p.Ypos++
			if p.Ypos > maxYPositionPlayer1 {
				p.Ypos = maxYPositionPlayer1
			}
		} else {
			p.Ypos--
			if p.Ypos < minYPositionPlayer2 {
				p.Ypos = minYPositionPlayer2
			}
		}
	case commandDown:
		if player1 {
			p.Ypos--
			if p.Ypos < minYPositionPlayer1 {
				p.Ypos = minYPositionPlayer1
			}
		} else {
			p.Ypos++
			if p.Ypos > maxYPosition-1 {
				p.Ypos = maxYPosition - 1
			}
		}
	case commandLeft:
		if player1 {
			p.Xpos--
			if p.Xpos < minXPositionPlayers {
				p.Xpos = minXPositionPlayers
			}
		} else {
			p.Xpos++
			if p.Xpos > maxXPosition-1 {
				p.Xpos = maxXPosition - 1
			}
		}
	case commandRight:
		if player1 {
			p.Xpos++
			if p.Xpos > maxXPosition-1 {
				p.Xpos = maxXPosition - 1
			}
		} else {
			p.Xpos--
			if p.Xpos < minXPositionPlayers {
				p.Xpos = minXPositionPlayers
			}
		}
	}
}

func (game Game) StartGame() {

	game.state = GameState{
		Bullets: []GameBullet{},
		Player1: PlayerGame{
			Xpos: minXPositionPlayers,
			Ypos: minYPositionPlayers,
			Hp:   maxPlayerHp,
			Name: game.Player1.Name,
		},
		Player2: PlayerGame{
			Xpos: maxXPosition - 1,
			Ypos: maxYPosition - 1,
			Hp:   maxPlayerHp,
			Name: game.Player2.Name,
		},
	}

	go func(game *Game) {
		for {
			select {
			case command := <-game.Player1.Command:
				if command == commandFire {
					game.state.Bullets = append(game.state.Bullets, GameBullet{
						X:   game.state.Player1.Xpos,
						Y:   game.state.Player1.Ypos + 1,
						Dir: bulletDirUp,
					})
					continue
				}
				game.state.Player1.move(command, true)
			case command := <-game.Player2.Command:
				if command == commandFire {
					game.state.Bullets = append(game.state.Bullets, GameBullet{
						X:   game.state.Player2.Xpos,
						Y:   game.state.Player2.Ypos - 1,
						Dir: bulletDirDown,
					})
					continue
				}
				game.state.Player2.move(command, false)
			}
		}
	}(&game)

	for {
		time.Sleep(200 * time.Millisecond)

		if len(game.state.Bullets) > 0 {
			var bullets []GameBullet
			gameBullets := &game.state.Bullets
			for _, bullet := range *gameBullets {
				switch bullet.Dir {
				case bulletDirDown:
					bullet.Y--
					if math.Abs(float64(game.state.Player1.Xpos-bullet.X)) <= 1 {
						if math.Abs(float64(game.state.Player1.Ypos-bullet.Y)) <= 1 {
							game.state.Player1.Hp--
							continue
						}
					}
				case bulletDirUp:
					bullet.Y++
					if math.Abs(float64(game.state.Player2.Xpos-bullet.X)) <= 1 {
						if math.Abs(float64(game.state.Player2.Ypos-bullet.Y)) <= 1 {
							game.state.Player2.Hp--
							continue
						}
					}
				}
				if bullet.Y > maxYPosition || bullet.Y < 0 {
					continue
				}
				bullets = append(bullets, bullet)
			}
			game.state.Bullets = bullets
		}

		if game.state.Player1.Hp <= 0 {
			game.endGame(true)
			return
		}

		if game.state.Player2.Hp <= 0 {
			game.endGame(false)
			return
		}

		game.sendStateGame()
	}
}

func PlayerSelection() {
	var response = StartGameResponse{Type: SignalStartTheGame}
	var responseJson []byte
	for len(PlayersWait) > 0 && len(PlayersWait)%2 == 0 {

		player1 := PlayersWait[len(PlayersWait)-1]
		PlayersWait = PlayersWait[:len(PlayersWait)-1]

		player2 := PlayersWait[len(PlayersWait)-1]
		PlayersWait = PlayersWait[:len(PlayersWait)-1]

		game := Game{Player1: player1, Player2: player2}

		response.Payload.Me = player1.Name
		response.Payload.Opponent = player2.Name
		responseJson, _ = json.Marshal(response)
		game.Player1.InGame = true
		game.Player1.Connection.WriteChannel <- responseJson

		response.Payload.Me = player2.Name
		response.Payload.Opponent = player1.Name
		responseJson, _ = json.Marshal(response)
		game.Player2.InGame = true
		game.Player2.Connection.WriteChannel <- responseJson
		go game.StartGame()
	}
}

type GameState struct {
	Bullets []GameBullet `json:"bullets"`
	Player1 PlayerGame   `json:"Player1"`
	Player2 PlayerGame   `json:"Player2"`
}

type GameBullet struct {
	X   int    `json:"x"`
	Y   int    `json:"y"`
	Dir string `json:"dir"`
}

type PlayerGame struct {
	Xpos int    `json:"xpos"`
	Ypos int    `json:"ypos"`
	Hp   int    `json:"hp"`
	Name string `json:"Name"`
}

type StartGameResponse struct {
	Type    string           `json:"type"`
	Payload StartGamePayload `json:"payload"`
}

type StartGamePayload struct {
	Me       string `json:"me"`
	Opponent string `json:"opponent"`
}

type StartGameStateResponse struct {
	Type    string                `json:"type"`
	Payload StartGameStatePayload `json:"payload"`
}

type StartGameStatePayload struct {
	Me       PlayerGame   `json:"me"`
	Opponent PlayerGame   `json:"opponent"`
	Bullets  []GameBullet `json:"bullets"`
}
