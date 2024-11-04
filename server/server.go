package main

import (
	"flag"
	"net"
	"net/rpc"
	"strconv"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolOperations struct{}

// shared global state
var completedTurns int
var numAliveCells int
var currentWorld [][]byte
var terminate bool = false
var paused bool = false
var terminateTurns int
var killServer bool = false

var mutex sync.Mutex
var endCurrentStateChan = make(chan bool) // used to stop holdCurrentState goroutine when game ends

var currAliveCellCount int // current number of alive cells to be sent down channel

// goroutine holds the current state of the game (no. of alive cells, no. of completed turns, current world)
func holdCurrentState(updateState chan bool, updateStateTurns chan int, updateNumAliveCells chan int, updateStateWorld chan [][]byte) {
	for {
		select {
		case <-endCurrentStateChan:
			return
		case <-updateState:
			mutex.Lock()
			numAliveCells = <-updateNumAliveCells
			completedTurns = <-updateStateTurns
			currentWorld = <-updateStateWorld
			mutex.Unlock()

		}
	}
}

// this is the function that will process the turns of GOL
func (g *GolOperations) ProcessTurns(req stubs.Request, res *stubs.Response) (err error) {
	// sets the game to be un-paused and not terminating on initialisation
	// to allow new clients to join server and one leaves
	terminate = false
	paused = false
	killServer = false

	//three channels to send the no. of turns completed, the no. of alive cells and the current world
	updateState := make(chan bool)
	updateStateTurns := make(chan int)
	updateNumAliveCells := make(chan int)
	updateStateWorld := make(chan [][]byte)

	go holdCurrentState(updateState, updateStateTurns, updateNumAliveCells, updateStateWorld)

	// copies the data contained in the request
	world := req.World
	ImageWidth := req.ImageWidth
	ImageHeight := req.ImageHeight
	Turns := req.Turns

	currAliveCellCount = len(calculateAliveCells(ImageHeight, ImageWidth, world))

	// send the initial state down the respective channels
	updateState <- true
	updateNumAliveCells <- currAliveCellCount
	updateStateTurns <- 0
	updateStateWorld <- world

	// repeat this for the number of turns specified in input params
	for count := 0; count < Turns; count++ {
		// calculates the next state of the world
		world = calculateNextState(ImageHeight, ImageWidth, world)

		currAliveCellCount = len(calculateAliveCells(ImageHeight, ImageWidth, world))

		// send the current state down the respective channels
		updateState <- true
		updateNumAliveCells <- currAliveCellCount
		updateStateTurns <- count + 1 // count is incremented manually because it isn't auto-incremented until start of next turn
		updateStateWorld <- world

		for {
			mutex.Lock()
			if !paused {
				mutex.Unlock()
				break
			}
			mutex.Unlock()

			// q and s still work even while the game is paused
			mutex.Lock()
			if terminate {
				terminateTurns = count + 1
				mutex.Unlock()
				break
			}
			mutex.Unlock()
		}

		mutex.Lock()
		if terminate {
			terminateTurns = count + 1
			mutex.Unlock()
			break
		}
		mutex.Unlock()
	}

	// this will cause the holdCurrentState goroutine to stop
	endCurrentStateChan <- true

	mutex.Lock()
	if !terminate {
		res.TerminateTurns = Turns
	} else {
		res.TerminateTurns = terminateTurns
	}

	res.World = world
	res.AliveCells = calculateAliveCells(ImageHeight, ImageWidth, world)
	mutex.Unlock()

	return
}

// returns the number of turns that have passed so far, as well as the number of alive cells
func (g *GolOperations) ReturnAliveCells(req stubs.Request, res *stubs.Response) (err error) {

	mutex.Lock()
	res.CompletedTurns = completedTurns
	res.NumAliveCells = numAliveCells
	mutex.Unlock()

	return
}

func (g *GolOperations) SaveCurrentState(req stubs.Request, res *stubs.Response) (err error) {

	mutex.Lock()
	res.CompletedTurns = completedTurns
	res.World = currentWorld
	mutex.Unlock()

	return
}

func (g *GolOperations) PauseProcessingToggle(req stubs.Request, res *stubs.Response) (err error) {

	mutex.Lock()
	if paused { // un-pausing
		res.CompletedTurns = completedTurns
		res.OutString = "Continuing"
	} else { // pausing
		res.CompletedTurns = completedTurns + 1
		res.OutString = strconv.Itoa(completedTurns + 1)
	}
	paused = !paused
	mutex.Unlock()

	return
}

func (g *GolOperations) CloseClientConnection(req stubs.Request, res *stubs.Response) (err error) {
	// the client wants to disconnect, so we will set terminate to true
	mutex.Lock()
	terminate = true
	mutex.Unlock()

	return

}

func (g *GolOperations) CloseAllComponents(req stubs.Request, res *stubs.Response) (err error) {
	//disconnect the client and close the server
	mutex.Lock()
	terminate = true
	killServer = true
	mutex.Unlock()

	return
}

// code used directly from CSA Lab 1
func calculateNextState(ImageHeight, ImageWidth int, world [][]byte) [][]byte {

	newWorld := make([][]byte, ImageHeight)
	for i := range world {
		newWorld[i] = make([]byte, ImageWidth)
		copy(newWorld[i], world[i])
	}

	for i := 0; i < ImageHeight; i++ {
		for j := 0; j < ImageWidth; j++ {
			iBehind := (i - 1 + ImageHeight) % ImageHeight
			iAhead := (i + 1 + ImageHeight) % ImageHeight
			jBehind := (j - 1 + ImageWidth) % ImageWidth
			jAhead := (j + 1 + ImageWidth) % ImageWidth

			neighbourSum := int(world[iBehind][jBehind]) + int(world[iBehind][j]) + int(world[iBehind][jAhead]) + int(world[i][jBehind]) + int(world[i][jAhead]) + int(world[iAhead][jBehind]) + int(world[iAhead][j]) + int(world[iAhead][jAhead])
			liveNeighbours := neighbourSum / 255

			if world[i][j] == 255 {
				if (liveNeighbours < 2) || (liveNeighbours > 3) {
					newWorld[i][j] = 0
				}
			} else if world[i][j] == 0 {
				if liveNeighbours == 3 {
					newWorld[i][j] = 255
				}
			}
		}
	}

	return newWorld
}

func calculateAliveCells(ImageHeight, ImageWidth int, world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}

	for i := 0; i < ImageHeight; i++ {
		for j := 0; j < ImageWidth; j++ {
			if world[i][j] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: j, Y: i})
			}
		}
	}

	return aliveCells
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	//registers the golOperations with rpc, to allow the client to call these functions
	rpc.Register(&GolOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	//checks if the server is supposed to be killed
	go func() {
		for {
			if killServer {
				time.Sleep(1 * time.Second)
				listener.Close()
			}
		}
	}()
	rpc.Accept(listener)

}
