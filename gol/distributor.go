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
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

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

	//execute all turns of the Game of Life

	//requests a tcp connection with the server running on AWS node.
	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	fmt.Println("conn made")
	defer client.Close()

	//creates a request to be sent to the server to process GOL
	fullInfoReq := stubs.FullInfoRequest{
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
		Turns:       p.Turns,
		World:       world,
	}

	//creates an empty response pointer for the server to return the final output of GOL
	fullInfoRes := new(stubs.FullInfoResponse)
	tickerRes := new(stubs.TickerResponse)
	paused := false

	runGol := client.Go(stubs.GolHandler, fullInfoReq, fullInfoRes, nil)

	//creates a new ticker to sound every two seconds
	ticker := time.NewTicker(2 * time.Second)
	time.Sleep(10 * time.Millisecond)

	execute := true
	for execute {

		select {
		case <-runGol.Done:
			execute = false
		case <-ticker.C:
			//creates a new tickerResponse pointer for the server to return the output of GOL every two seconds
			//the client calls the server requesting the current number of alive cells and how many turns have passed so far
			client.Call(stubs.AliveCellHandler, fullInfoReq, tickerRes)

			//creates a new event based on the server's response
			c.events <- AliveCellsCount{
				CompletedTurns: tickerRes.CompletedTurns,
				CellsCount:     tickerRes.NumAliveCells,
			}

			fmt.Println(tickerRes.CompletedTurns, tickerRes.NumAliveCells)

		case keyPressed := <-keyPresses:

			if keyPressed == 's' {
				client.Call(stubs.CurrentStateSave, fullInfoReq, fullInfoRes)

				currentStateFileName := filename + "x" + strconv.Itoa(fullInfoRes.CompletedTurns)

				makeOutputPGM(p, c, fullInfoRes.World, currentStateFileName, fullInfoRes.CompletedTurns)

			} else if keyPressed == 'q' {
				//close client without affecting the server
				client.Call(stubs.CloseClientConnection, fullInfoReq, fullInfoRes)

			} else if keyPressed == 'k' {
				//close all components and generate pgm file of final state
			} else if keyPressed == 'p' {

				client.Call(stubs.PauseProcessingToggle, fullInfoReq, fullInfoRes)
				fmt.Println("Current turn: " + fullInfoRes.OutString)

				//if the new state of the execution is paused
				if paused {
					c.events <- StateChange{
						CompletedTurns: fullInfoRes.CompletedTurns,
						NewState:       Executing,
					}
				} else {
					c.events <- StateChange{
						CompletedTurns: fullInfoRes.CompletedTurns,
						NewState:       Paused,
					}
				}
				paused = !paused
			}
		}
	}

	ticker.Stop()

	// reports the final state using FinalTurnCompleteEvent

	c.events <- FinalTurnComplete{CompletedTurns: fullInfoRes.TerminateTurns, Alive: fullInfoRes.AliveCells}

	//updates filename for the final output PGM
	finalOutFileName := filename + "x" + strconv.Itoa(fullInfoRes.TerminateTurns)

	makeOutputPGM(p, c, fullInfoRes.World, finalOutFileName, fullInfoRes.TerminateTurns)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{fullInfoRes.TerminateTurns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func makeOutputPGM(p Params, c distributorChannels, world [][]byte, filename string, completedTurns int) {
	//signals that we are ready to create an output file
	c.ioCommand <- ioOutput
	//sends the output filename down the filename channel
	c.ioFilename <- filename

	//sends the output of the final world byte by byte down the output channel
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- ImageOutputComplete{CompletedTurns: completedTurns, Filename: filename}
}
