package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"strconv"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolOperations struct{}

var completedTurns int
var aliveCells []util.Cell
var numAliveCells int
var currentWorld [][]byte
var updateMutex sync.Mutex
var terminate bool = false
var paused bool = false
var terminateTurns int
var endCurrentStateChan = make(chan bool)

var currAliveCells []util.Cell
var currAliveCellCount int // current number of alive cells to be sent down channel

//var terminateMutex sync.Mutex

// this goroutine holds the current number of completed turns and the number of alive cells.
// when a new turn finishes executing, the completedTurns and numAliveCells global variables will be updated at the exact same time.
func holdCurrentState(updateState chan bool, updateAliveCells chan []util.Cell, updateStateTurns chan int, updateStateAlive chan int, updateStateWorld chan [][]byte) {
	for {
		select {
		case <-endCurrentStateChan:
			return
		case <-updateState:
			updateMutex.Lock()
			aliveCells = <-updateAliveCells
			numAliveCells = <-updateStateAlive
			completedTurns = <-updateStateTurns
			currentWorld = <-updateStateWorld
			updateMutex.Unlock()

		}
	}
}

// this is the function that will process the turns of GOL
func (g *GolOperations) ProcessTurns(req stubs.Request, currStateRes *stubs.CurrentStateResponse) (err error) {
	// for new clients that take over
	terminate = false
	paused = false

	//three channels to send the turns processed, the alive cells and the current world
	updateState := make(chan bool)
	updateStateTurns := make(chan int)
	updateStateAlive := make(chan int)
	updateStateWorld := make(chan [][]byte)
	updateAliveCells := make(chan []util.Cell)

	go holdCurrentState(updateState, updateAliveCells, updateStateTurns, updateStateAlive, updateStateWorld)

	//copies the data contained in the request
	world := req.World
	ImageWidth := req.ImageWidth
	ImageHeight := req.ImageHeight
	Turns := req.Turns

	//send the turn's state down the respective channels
	currAliveCells = calculateAliveCells(ImageHeight, ImageWidth, world)
	currAliveCellCount = len(currAliveCells)

	updateMutex.Lock()
	aliveCells = currAliveCells
	numAliveCells = currAliveCellCount
	completedTurns = 0
	currentWorld = world
	updateMutex.Unlock()

	/*
		updateState <- true
		updateAliveCells <- currAliveCells
		updateStateAlive <- currAliveCellCount
		updateStateTurns <- 0
		updateStateWorld <- world
	*/

	//repeat this for the number of turns specified in input params
	for i := 0; i < Turns; i++ {
		//calculates the next state of the world
		world = calculateNextState(ImageHeight, ImageWidth, world)
		//send the turn's state down the respective channels
		currAliveCells = calculateAliveCells(ImageHeight, ImageWidth, world)
		currAliveCellCount = len(currAliveCells)

		/* <- true
		updateAliveCells <- currAliveCells
		updateStateAlive <- currAliveCellCount
		updateStateTurns <- i + 1
		updateStateWorld <- world
		*/
		updateMutex.Lock()
		aliveCells = currAliveCells
		numAliveCells = currAliveCellCount
		completedTurns = i + 1
		currentWorld = world
		updateMutex.Unlock()

		for {

			updateMutex.Lock()
			if !paused {
				updateMutex.Unlock()
				break
			}
			updateMutex.Unlock()

			// q and s still work even while the game is paused
			updateMutex.Lock()
			if terminate {
				terminateTurns = i + 1
				updateMutex.Unlock()
				break
			}
			updateMutex.Unlock()
		}

		// if q is pressed, terminate early

		updateMutex.Lock()
		if terminate {
			terminateTurns = i + 1
			updateMutex.Unlock()
			break
		}
		updateMutex.Unlock()
	}

	updateMutex.Lock()
	if !terminate {
		currStateRes.TerminateTurns = Turns
	} else {
		currStateRes.TerminateTurns = terminateTurns
	}

	currStateRes.World = world
	currStateRes.AliveCells = aliveCells
	updateMutex.Unlock()

	fmt.Println("TESTTTTTT")
	return
}

// returns the number of turns that have passed so far, as well as the number of alive cells
func (g *GolOperations) ReturnAliveCells(req stubs.Request, tickerRes *stubs.TickerResponse) (err error) {

	updateMutex.Lock()
	tickerRes.NumAliveCells = numAliveCells
	tickerRes.CompletedTurns = completedTurns
	updateMutex.Unlock()

	return
}

func (g *GolOperations) SaveCurrentState(req stubs.Request, currentStateRes *stubs.CurrentStateResponse) (err error) {

	updateMutex.Lock()
	currentStateRes.World = currentWorld
	currentStateRes.CompletedTurns = completedTurns
	updateMutex.Unlock()

	return
}

func (g *GolOperations) PauseProcessingToggle(req stubs.Request, pauseProcessingRes *stubs.PauseProcessingResponse) (err error) {

	updateMutex.Lock()
	pauseProcessingRes.CompletedTurns = completedTurns + 1
	pauseProcessingRes.OutString = strconv.Itoa(completedTurns + 1)
	paused = true
	updateMutex.Unlock()

	return
}

func (g *GolOperations) UnpauseProcessingToggle(req stubs.Request, pauseProcessingRes *stubs.PauseProcessingResponse) (err error) {

	updateMutex.Lock()
	pauseProcessingRes.OutString = "Continuing"
	pauseProcessingRes.CompletedTurns = completedTurns
	paused = false
	updateMutex.Unlock()

	return
}

func (g *GolOperations) CloseClientConnection(req stubs.Request, currStateRes *stubs.CurrentStateResponse) (err error) {

	//the client wants to disconnect, so we will set clientConnected to false
	updateMutex.Lock()
	terminate = true
	endCurrentStateChan <- true
	updateMutex.Unlock()

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
