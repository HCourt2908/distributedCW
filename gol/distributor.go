package gol

import (
	"fmt"
	"net/rpc"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	//Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	//creates the filename based on the width and height parameters
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)

	//tells the command channel that we are ready to accept input
	c.ioCommand <- ioInput
	//provides the readPGM function the filename
	c.ioFilename <- filename

	//copies the starting world byte by byte from the input PGM image
	for i := range world {
		for j := range world[i] {
			world[i][j] = <-c.ioInput
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	//creates a new ticker to sound every two seconds
	ticker := time.NewTicker(2 * time.Second)

	//execute all turns of the Game of Life

	//requests a tcp connection with the server running on AWS node.
	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	fmt.Println("conn made")
	defer client.Close()

	//creates a request to be sent to the server to process GOL
	req := stubs.Request{
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
		Turns:       p.Turns,
		World:       world,
	}

	//creates an empty response for the server to return the final output of GOL
	res := new(stubs.Response)

	//creates a new tickerResponse for the server to return the output of GOL every two seconds
	tickerRes := new(stubs.TickerResponse)

	go func() {
		//this code executes whenever there's a new value in the ticker channel, aka every two seconds
		for range ticker.C {
			//the client calls the server requesting the current number of alive cells and how many turns have passed so far
			client.Call(stubs.AliveCellHandler, req, tickerRes)

			//creates a new event based on the server's response
			c.events <- AliveCellsCount{
				CompletedTurns: tickerRes.CompletedTurns,
				CellsCount:     tickerRes.NumAliveCells,
			}
		}

	}()

	//client calls the server to process the turns of GOL
	client.Call(stubs.GolHandler, req, res)

	ticker.Stop()

	// reports the final state using FinalTurnCompleteEvent

	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: res.AliveCells}

	//signals that we are ready to create an output file
	c.ioCommand <- ioOutput
	//creates the output filename
	filename = filename + "x" + strconv.Itoa(p.Turns)
	//sends the output filename down the filename channel
	c.ioFilename <- filename

	//sends the output of the final world byte by byte down the output channel
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- res.World[i][j]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
