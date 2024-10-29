package main

import (
	"flag"
	"net"
	"net/rpc"
	"strconv"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolOperations struct{}

var completedTurns int
var numAliveCells int
var currentWorld [][]byte
var paused = false
var clientConnected = true

//this goroutine holds the current number of completed turns and the number of alive cells.
//when a new turn finishes executing, the completedTurns and numAliveCells global variables will be updated at the exact same time.
func holdCurrentState(updateStateTurns chan int, updateStateAlive chan int, updateStateWorld chan [][]byte) {
	for {
		select {
		case numAliveCells = <-updateStateAlive:
			completedTurns = <-updateStateTurns
			currentWorld = <-updateStateWorld
		}
	}
}

//this is the function that will process the turns of GOL
func (g *GolOperations) ProcessTurns(req stubs.Request, res *stubs.FinalStateResponse) (err error) {
	//ProcessTurns is the first function that a client will run, so we automatically know that the client will be connected
	clientConnected = true
	//resets the pause value so when another client joins after a pause and quit, it won't just stay paused forever
	paused = false

	//three channels to send the turns processed, the alive cells and the current world
	updateStateTurns := make(chan int)
	updateStateAlive := make(chan int)
	updateStateWorld := make(chan [][]byte)

	go holdCurrentState(updateStateTurns, updateStateAlive, updateStateWorld)

	//copies the data contained in the request
	world := req.World
	ImageWidth := req.ImageWidth
	ImageHeight := req.ImageHeight
	Turns := req.Turns

	//send the turn's state down the respective channels
	updateStateAlive <- len(calculateAliveCells(ImageHeight, ImageWidth, world))
	updateStateTurns <- 0
	updateStateWorld <- world

	//repeat this for the number of turns specified in input params
	for i := 0; i < Turns; i++ {
		//calculates the next state of the world
		world = calculateNextState(ImageHeight, ImageWidth, world)
		//send the turn's state down the respective channels
		updateStateAlive <- len(calculateAliveCells(ImageHeight, ImageWidth, world))
		updateStateTurns <- i + 1
		updateStateWorld <- world

		//either bypasses this for loop, or waits until the second pause request has been made by the client
		for paused {
			//if the client presses q while paused, we stop all execution
			if !clientConnected {
				break
			}
		}
		//if the client presses q, we exit the state calculation loop early
		if !clientConnected {
			break
		}
	}

	//once all the turns have been calculated, copy the world into the response and calculate the alive cells for this world
	res.World = world
	res.AliveCells = calculateAliveCells(ImageHeight, ImageWidth, world)

	return
}

// returns the number of turns that have passed so far, as well as the number of alive cells
func (g *GolOperations) ReturnAliveCells(req stubs.Request, tickerRes *stubs.TickerResponse) (err error) {

	tickerRes.NumAliveCells = numAliveCells
	tickerRes.CompletedTurns = completedTurns

	return
}

func (g *GolOperations) SaveCurrentState(req stubs.Request, currentStateRes *stubs.CurrentStateResponse) (err error) {

	currentStateRes.World = currentWorld
	currentStateRes.CompletedTurns = completedTurns

	return
}

func (g *GolOperations) PauseProcessingToggle(req stubs.Request, pauseProcessingRes *stubs.PauseProcessingResponse) (err error) {

	if paused {
		pauseProcessingRes.OutString = "Continuing"
	} else {
		pauseProcessingRes.OutString = strconv.Itoa(completedTurns)
	}
	paused = !paused
	pauseProcessingRes.PausedProcessing = paused
	pauseProcessingRes.CompletedTurns = completedTurns

	return
}

func (g *GolOperations) CloseClientConnection(req stubs.Request, CloseClientRes *stubs.CloseClientResponse) (err error) {

	//the client wants to disconnect, so we will set clientConnected to false
	clientConnected = false

	CloseClientRes.AliveCells = calculateAliveCells(req.ImageHeight, req.ImageWidth, currentWorld)
	CloseClientRes.CompletedTurns = completedTurns

	return

}

//code used directly from CSA Lab 1
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
