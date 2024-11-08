package main

import (
	"flag"
	"net"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type BrokerOperations struct{}

// shared global state
var completedTurns int
var numAliveCells int
var aliveCells []util.Cell
var currentWorld [][]byte
var terminate bool = false
var paused bool = false
var terminateTurns int
var killBroker bool = false

// tells the program how big to make the slices containing the information related to each server
var numberOfServers = 2
var servers = make([]*rpc.Client, numberOfServers)

var ips = []string{
	"127.0.0.1:8031",
	"127.0.0.1:8032",
	"127.0.0.1:8033",
	"127.0.0.1:8034",
}

var mutex sync.Mutex
var endCurrentStateChan = make(chan bool)

var currentAliveCellCount int
var currentAliveCells []util.Cell

// goroutine holds the current state of the game (no. of alive cells, no. of completed turns, current world etc.)
func holdCurrentState(updateState chan bool, updateStateTurns chan int, updateNumAliveCells chan int, updateAliveCells chan []util.Cell, updateStateWorld chan [][]byte) {
	for {
		select {
		case <-endCurrentStateChan:
			return
		case <-updateState:
			mutex.Lock()
			numAliveCells = <-updateNumAliveCells
			aliveCells = <-updateAliveCells
			completedTurns = <-updateStateTurns
			currentWorld = <-updateStateWorld
			mutex.Unlock()

		}
	}
}

func (b *BrokerOperations) Broker(req stubs.Request, res *stubs.Response) (err error) {
	//sets the game to be un-paused and not terminating on initialisation
	//to allow new clients to join server and one leaves
	terminate = false
	paused = false
	killBroker = false

	//slices to hold the server requests, server responses and the server RPC connections
	serverRequests := make([]*stubs.ServerRequest, numberOfServers)
	serverResponses := make([]*stubs.ServerResponse, numberOfServers)
	servers = make([]*rpc.Client, numberOfServers)

	//channels to update the global state store
	updateState := make(chan bool)
	updateStateTurns := make(chan int)
	updateAliveCells := make(chan []util.Cell)
	updateNumAliveCells := make(chan int)
	updateStateWorld := make(chan [][]byte)

	//starts the global state store goroutine
	go holdCurrentState(updateState, updateStateTurns, updateNumAliveCells, updateAliveCells, updateStateWorld)

	ImageHeight := req.ImageHeight
	ImageWidth := req.ImageWidth

	//copies the inital world
	world := make([][]byte, ImageHeight)
	for i := range world {
		world[i] = make([]byte, ImageWidth)
		copy(world[i], req.World[i])
	}
	Turns := req.Turns

	//repeats the number of times as there are servers. creates a new connection, a unique server request and an empty server response
	for i, _ := range servers {
		servers[i], _ = rpc.Dial("tcp", ips[i])
		serverReq := new(stubs.ServerRequest)
		*serverReq = stubs.ServerRequest{
			World:        append(make([][]byte, 0), world...),
			ImageWidth:   ImageWidth,
			ImageHeight:  ImageHeight,
			NoOfServers:  numberOfServers,
			ServerNumber: i,
		}
		serverRequests[i] = serverReq
		serverResponses[i] = new(stubs.ServerResponse)
	}

	//calculates the initial number of alive cells
	currentAliveCells = calculateAliveCells(ImageHeight, ImageWidth, world)
	currentAliveCellCount = len(currentAliveCells)

	//send the initial state down the respective channels
	updateState <- true
	updateNumAliveCells <- currentAliveCellCount
	updateAliveCells <- currentAliveCells
	updateStateTurns <- 0
	updateStateWorld <- world

	//repeat this for the number of turns specified in input params
	for count := 0; count < Turns; count++ {
		//make a slice to hold all the rpc call pointers. this is done for syncing reasons, e.g., we want to put together the sections of world in order, and only when they're done should we do this
		doneProcessing := make([]*rpc.Call, numberOfServers)
		for i, server := range servers {
			//make a non-blocking rpc call to each server to process their section of GOL
			doneProcessing[i] = server.Go(stubs.CalculateNextState, serverRequests[i], serverResponses[i], nil)
		}

		//create an empty 2d slice to eventually hold the new full world (advanced by one turn)
		connWorld := make([][]byte, ImageHeight)
		for i, response := range serverResponses {
			//we need to add the slices back in order, so we wait until the first one is done, then the second one, etc...
			synced := false
			for {
				//if there's a value in the Call's Done channel, then the rpc call has finished, and we can safely add the slices to connWorld
				select {
				case <-doneProcessing[i].Done:
					synced = true
					break
				}
				if synced {
					break
				}
			}

			//adds the results slice by slice to connWorld
			//for each server, it will start putting in slices at the 'startIndex' and end when there's nothing left to put in
			for j, row := range response.World {
				connWorld[i*(ImageHeight/numberOfServers)+j] = make([]byte, ImageWidth)
				copy(connWorld[i*(ImageHeight/numberOfServers)+j], row)
			}
		}

		//since we use the same serverRequest structs for each turn, we need to update their version of the current world
		for _, request := range serverRequests {
			copy(request.World, connWorld)
		}

		currentAliveCells = calculateAliveCells(ImageHeight, ImageWidth, connWorld)
		currentAliveCellCount = len(currentAliveCells)

		// send the current state down the respective channels
		updateState <- true
		updateNumAliveCells <- currentAliveCellCount
		updateAliveCells <- currentAliveCells
		updateStateTurns <- count + 1
		updateStateWorld <- connWorld

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

	//this will cause the holdCurrentState goroutine to stop
	endCurrentStateChan <- true

	mutex.Lock()
	if !terminate {
		res.TerminateTurns = Turns
	} else {
		res.TerminateTurns = terminateTurns
	}
	mutex.Unlock()

	mutex.Lock()
	res.World = currentWorld
	res.AliveCells = aliveCells

	mutex.Unlock()

	return
}

func calculateAliveCells(ImageHeight, ImageWidth int, world [][]byte) []util.Cell {

	liveCells := []util.Cell{}

	for i := 0; i < ImageHeight; i++ {
		for j := 0; j < ImageWidth; j++ {
			if world[i][j] == 255 {
				liveCells = append(liveCells, util.Cell{X: j, Y: i})
			}
		}
	}

	return liveCells
}

func (b *BrokerOperations) ReturnAliveCells(req stubs.Request, res *stubs.Response) (err error) {

	mutex.Lock()
	res.CompletedTurns = completedTurns
	res.NumAliveCells = numAliveCells
	mutex.Unlock()

	return
}

func (b *BrokerOperations) SaveCurrentState(req stubs.Request, res *stubs.Response) (err error) {

	mutex.Lock()
	res.CompletedTurns = completedTurns
	res.World = currentWorld
	mutex.Unlock()

	return
}

func (b *BrokerOperations) PauseProcessingToggle(req stubs.Request, res *stubs.Response) (err error) {

	mutex.Lock()
	if paused { //un-pausing
		res.CompletedTurns = completedTurns
	} else { // pausing
		res.CompletedTurns = completedTurns + 1
	}
	paused = !paused
	mutex.Unlock()

	return
}

func (b *BrokerOperations) CloseClientConnection(req stubs.Request, res *stubs.Response) (err error) {
	//the client wants to disconnect, so we will set terminate to true
	mutex.Lock()
	terminate = true
	mutex.Unlock()

	return
}

func (b *BrokerOperations) CloseAllComponents(req stubs.Request, res *stubs.Response) (err error) {
	mutex.Lock()
	for _, server := range servers {
		//iterates over all servers and sends a kill request to all of them
		server.Call(stubs.KillServer, req, res)
	}
	time.Sleep(25 * time.Millisecond)
	terminate = true
	killBroker = true
	mutex.Unlock()

	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	//registers the brokerOperations with rpc, to allow the client to call these functions
	rpc.Register(&BrokerOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	//checks if the broker is supposed to be killed
	go func() {
		for {
			if killBroker {
				time.Sleep(1 * time.Second)
				listener.Close()
			}
		}
	}()
	rpc.Accept(listener)
}
