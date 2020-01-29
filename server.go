package main

import (
	"bufio"
	"flag"
	"fmt"
	"golang.org/x/net/websocket"
	"log"
	"net/http"
	"os"

	"github.com/fatih/color"
)

type bike struct {
	Player []sprite
	Direction string
}

type sprite struct {
	Row  int
	Col  int
	Here bool
}

type Message struct {
	PlayerA bike `json:"playerA"`
	PlayerB bike `json:"playerB"`
}

type inComing struct {
	Player	string `json:"bike"`
	Command string	`json:"command"`
}

type hub struct {
	clients          map[string]*websocket.Conn
	addClientChan    chan *websocket.Conn
	removeClientChan chan *websocket.Conn
	broadcastChan    chan Message
}

var (
	maze      []string
	PlayerA   bike
	PlayerB   bike
	maxLength = 150

	port = flag.String("port", "9000", "port used for ws connection")
)

func main() {
	loadMaze("maze.txt")

	flag.Parse()
	log.Fatal(server(*port))
}

func updateLogic(PlayerA, PlayerB bike, inp string) (crash bool, Player1, Player2 bike){
	// Update the direction
	Player1, Player2 = playerDirection(PlayerA, PlayerB, inp)

	// Calculate Movement
	if PlayerA, crash = playerMovement(PlayerA); crash {
		color.Blue("WASD Wins")
		return
	}
	if PlayerB, crash = playerMovement(PlayerB); crash {
		color.Red("Arrows Wins")
		return
	}

	// Calculate winners
	if collisionDetection(PlayerA, PlayerB.Player[0]) {
		color.Red("Arrows Wins")
		return
	}
	if collisionDetection(PlayerB, PlayerA.Player[0]) {
		color.Blue("WASD Wins")
		return
	}

	return
}

// server creates a websocket server at port <port> and registers the sole handler
func server(port string) error {
	h := newHub()
	mux := http.NewServeMux()
	mux.Handle("/", websocket.Handler(func(ws *websocket.Conn) {
		handler(ws, h)
	}))

	s := http.Server{Addr: ":" + port, Handler: mux}
	return s.ListenAndServe()
}

// handler registers a new chat client conn;
// It runs the hub, adds the client to the connection pool
// and broadcasts received message
func handler(ws *websocket.Conn, h *hub) {
	go h.run()

	h.addClientChan <- ws

	for {
		var m inComing
		var crash bool
		err := websocket.JSON.Receive(ws, &m)
		if err != nil {
			//h.broadcastChan <- Message{"ERROR",err.Error()}
			h.removeClient(ws)
			return
		}
		//TODO: Currently it doesn't matter who send the command bceause you still use wasd v arrows
		crash, PlayerA, PlayerB = updateLogic(PlayerA, PlayerB, m.Command)
		if !crash {
			log.Println("updated A", PlayerA, "   Updated B", PlayerB)
			h.broadcastChan <- Message{PlayerA:PlayerA, PlayerB:PlayerB}
		} else {
			log.Println("CRASH")
		}

	}
}

// newHub returns a new hub object
func newHub() *hub {
	return &hub{
		clients:          make(map[string]*websocket.Conn),
		addClientChan:    make(chan *websocket.Conn),
		removeClientChan: make(chan *websocket.Conn),
		broadcastChan:    make(chan Message),
	}
}

// run receives from the hub channels and calls the appropriate hub method
func (h *hub) run() {
	for {
		select {
		case conn := <-h.addClientChan:
			h.addClient(conn)
		case conn := <-h.removeClientChan:
			h.removeClient(conn)
		case m := <-h.broadcastChan:
			h.broadcastMessage(m)
		}
	}
}

// removeClient removes a conn from the pool
func (h *hub) removeClient(conn *websocket.Conn) {
	delete(h.clients, conn.LocalAddr().String())
}

// addClient adds a conn to the pool
func (h *hub) addClient(conn *websocket.Conn) {
	h.clients[conn.RemoteAddr().String()] = conn
}

// broadcastMessage sends a message to all client conns in the pool
func (h *hub) broadcastMessage(m Message) {
	for _, conn := range h.clients {
		err := websocket.JSON.Send(conn, m)
		if err != nil {
			fmt.Println("Error broadcasting message: ", err)
			return
		}
	}
}

func loadMaze(file string) error {
	f, err := os.Open(file)
	if err != nil{
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		maze = append(maze, scanner.Text())
	}

	for row, line := range maze {
		for col, chr := range line {
			switch chr {
			case 'a':
				PlayerA.Player = append(PlayerA.Player, sprite{row, col, true})
			case 'b':
				PlayerB.Player = append(PlayerB.Player, sprite{row, col, true})
			}
		}
	}

	return nil
}

func collisionDetection(user bike, opp sprite) bool {
	for ind, seg := range user.Player {
		if ind != 0 {
			if seg == opp {
				return true
			}
		}
	}

	return false
}

func playerDirection(user1, user2 bike, input string) (bike, bike) {
	switch input {
	case "UP":
		user1.Direction = "UP"
	case "DOWN":
		user1.Direction = "DOWN"
	case "RIGHT":
		user1.Direction = "LEFT"
	case "LEFT":
		user1.Direction = "RIGHT"
	case "w":
		user2.Direction = "UP"
	case "s":
		user2.Direction = "DOWN"
	case "d":
		user2.Direction = "LEFT"
	case "a":
		user2.Direction = "RIGHT"
	}

	return user1, user2
}

func playerMovement(Player bike) (bike, bool) {
	if Player.Direction != "" {
		var newRow sprite

		switch Player.Direction {
		case "UP": newRow = sprite{Player.Player[0].Row - 1, Player.Player[0].Col, true}
		case "DOWN": newRow = sprite{Player.Player[0].Row + 1, Player.Player[0].Col, true}
		case "LEFT": newRow = sprite{Player.Player[0].Row, Player.Player[0].Col + 1, true}
		case "RIGHT": newRow = sprite{Player.Player[0].Row, Player.Player[0].Col - 1, true}
		}
		if newRow.Here != false {
			Player.Player = append([]sprite{newRow}, Player.Player...)
		}

		if len(Player.Player) > maxLength {
			Player.Player = Player.Player[:len(Player.Player)-1]
		}

		if Player.Player[0].Row >= len(maze)-1 {
			Player.Player[0].Row = 1
		} else if Player.Player[0].Row <= 0 {
			Player.Player[0].Row = len(maze)-2
		}
		if Player.Player[0].Col > len(maze[0])-1 {
			Player.Player[0].Col = 0
		} else if Player.Player[0].Col < 0 {
			Player.Player[0].Col = len(maze[0])
		}

		for ind, seg := range Player.Player {
			if ind != 0 {
				if Player.Player[0] == seg {
					return Player, true
				}
			}
		}
	}
	return Player, false
}