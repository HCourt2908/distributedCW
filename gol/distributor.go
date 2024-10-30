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

	//var clientClosedMutex sync.Mutex
	//var turnsMutex sync.Mutex

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
	fullInfoReq := stubs.Request{
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
		Turns:       p.Turns,
		World:       world,
	}

	//creates an empty response pointer for the server to return the final output of GOL
	currStateRes := new(stubs.CurrentStateResponse)
	stateSaveRes := new(stubs.StateSaveResponse)
	tickerRes := new(stubs.TickerResponse)
	pauseProcessingRes := new(stubs.PauseProcessingResponse)
	pause := false

	go func() {
		//this code executes whenever there's a new value in the ticker channel or a keystroke is detected
		time.Sleep(50 * time.Millisecond)

		for {

			select {

			case <-ticker.C:
				//creates a new tickerResponse pointer for the server to return the output of GOL every two seconds
				//the client calls the server requesting the current number of alive cells and how many turns have passed so far
				client.Call(stubs.AliveCellHandler, fullInfoReq, &tickerRes)

				//creates a new event based on the server's response
				c.events <- AliveCellsCount{
					CompletedTurns: tickerRes.CompletedTurns,
					CellsCount:     tickerRes.NumAliveCells,
				}

			case keyPressed := <-keyPresses:

				if keyPressed == 's' {
					//generate pgm file of current state
					client.Call(stubs.CurrentStateSave, fullInfoReq, &stateSaveRes)

					//currentStateFileName
					currentStateFileName := filename + "x" + strconv.Itoa(stateSaveRes.CompletedTurns)

					makeOutputPGM(p, c, stateSaveRes.World, currentStateFileName, stateSaveRes.CompletedTurns)

				} else if keyPressed == 'q' {
					//close client without affecting the server
					client.Call(stubs.CloseClientConnection, fullInfoReq, &currStateRes)

				} else if keyPressed == 'k' {
					//close all components and generate pgm file of final state
				} else if keyPressed == 'p' {
					//pause processing on AWS node, client print current turn being processed
					//resume processing on AWS node, client print Continuing

					//if the new state of the execution is paused
					if pause {
						client.Call(stubs.UnpauseProcessingToggle, fullInfoReq, &pauseProcessingRes)
						fmt.Println(pauseProcessingRes.OutString)
						c.events <- StateChange{
							CompletedTurns: pauseProcessingRes.CompletedTurns,
							NewState:       Executing,
						}
						pause = false
					} else {
						client.Call(stubs.PauseProcessingToggle, fullInfoReq, &pauseProcessingRes)
						fmt.Println(pauseProcessingRes.OutString)
						c.events <- StateChange{
							CompletedTurns: pauseProcessingRes.CompletedTurns,
							NewState:       Paused,
						}
						pause = true
					}

					//client calls server requesting to toggle the pause (either resume processing or pause processing)

				}

			}

		}

	}()

	//client calls the server to process the turns of GOL
	client.Call(stubs.GolHandler, fullInfoReq, &currStateRes)

	ticker.Stop()

	// reports the final state using FinalTurnCompleteEvent

	c.events <- FinalTurnComplete{CompletedTurns: currStateRes.TerminateTurns, Alive: currStateRes.AliveCells}

	//updates filename for the final output PGM
	finalOutFileName := filename + "x" + strconv.Itoa(currStateRes.TerminateTurns)

	makeOutputPGM(p, c, currStateRes.World, finalOutFileName, currStateRes.TerminateTurns)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{currStateRes.TerminateTurns, Quitting}

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
