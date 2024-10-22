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

	// TODO: Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	for i := range world {
		for j := range world[i] {
			world[i][j] = <-c.ioInput
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	ticker := time.NewTicker(2 * time.Second)

	// TODO: Execute all turns of the Game of Life.

	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	fmt.Println("conn made")
	defer client.Close()

	req := stubs.Request{
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
		Turns:       p.Turns,
		World:       world,
	}

	res := new(stubs.Response)

	tickerRes := new(stubs.TickerResponse)

	go func() {
		for range ticker.C {
			client.Call(stubs.AliveCellHandler, req, tickerRes)

			c.events <- AliveCellsCount{
				CompletedTurns: tickerRes.CompletedTurns,
				CellsCount:     tickerRes.NumAliveCells,
			}
		}

	}()

	client.Call(stubs.GolHandler, req, res)

	ticker.Stop()

	// TODO: Report the final state using FinalTurnCompleteEvent.

	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: res.AliveCells}

	c.ioCommand <- ioOutput
	filename = filename + "x" + strconv.Itoa(p.Turns)
	c.ioFilename <- filename
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
