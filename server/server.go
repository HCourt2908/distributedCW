package main

import (
	"flag"
	"net"
	"net/rpc"
	"strconv"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolOperations struct{}

var completedTurns int
var numAliveCells int
var currentWorld [][]byte

var terminate bool = false
var paused bool = false
var terminateTurns int
var mutex sync.Mutex
var endCurrentStateChan = make(chan bool)

var currAliveCellCount int // current number of alive cells to be sent down channel

// this goroutine holds the current number of completed turns and the number of alive cells.
// when a new turn finishes executing, the completedTurns and numAliveCells global variables will be updated at the exact same time.
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
	terminate = false
	paused = false

	//three channels to send the turns processed, the alive cells and the current world
	updateState := make(chan bool)
	updateStateTurns := make(chan int)
	updateNumAliveCells := make(chan int)
	updateStateWorld := make(chan [][]byte)

	go holdCurrentState(updateState, updateStateTurns, updateNumAliveCells, updateStateWorld)

	//copies the data contained in the request
	world := req.World
	ImageWidth := req.ImageWidth
	ImageHeight := req.ImageHeight
	Turns := req.Turns

	//send the turn's state down the respective channels
	currAliveCellCount = len(calculateAliveCells(ImageHeight, ImageWidth, world))

	updateState <- true
	updateNumAliveCells <- currAliveCellCount
	updateStateTurns <- 0
	updateStateWorld <- world

	//repeat this for the number of turns specified in input params
	for count := 0; count < Turns; count++ {
		//calculates the next state of the world
		world = calculateNextState(ImageHeight, ImageWidth, world)

		currAliveCellCount = len(calculateAliveCells(ImageHeight, ImageWidth, world))

		updateState <- true
		updateNumAliveCells <- currAliveCellCount
		updateStateTurns <- count + 1
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
	res.NumAliveCells = numAliveCells
	res.CompletedTurns = completedTurns
	mutex.Unlock()

	return
}

func (g *GolOperations) SaveCurrentState(req stubs.Request, res *stubs.Response) (err error) {

	mutex.Lock()
	res.World = currentWorld
	res.CompletedTurns = completedTurns
	mutex.Unlock()

	return
}

func (g *GolOperations) PauseProcessingToggle(req stubs.Request, res *stubs.Response) (err error) {

	mutex.Lock()
	if paused {
		res.OutString = "Continuing"
		res.CompletedTurns = completedTurns
	} else {
		res.CompletedTurns = completedTurns + 1
		res.OutString = strconv.Itoa(completedTurns + 1)
	}
	paused = !paused
	mutex.Unlock()

	return
}

func (g *GolOperations) CloseClientConnection(req stubs.Request, res *stubs.Response) (err error) {

	//the client wants to disconnect, so we will set clientConnected to false
	mutex.Lock()
	terminate = true
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
	rpc.Accept(listener)
}
