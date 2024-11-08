package main

import (
	"flag"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

var killServer = false
var numberOfThreads int

type GolOperations struct{}

// code used directly from CSA Lab 1
func (g *GolOperations) CalculateNextState(req *stubs.ServerRequest, res *stubs.ServerResponse) (err error) {

	ImageWidth := req.ImageWidth
	ImageHeight := req.ImageHeight
	world := req.World

	serverNumber := req.ServerNumber
	numberOfServers := req.NoOfServers
	//what rows of cells should this specific server update? startIndex is inclusive but endIndex is exclusive
	startIndex := (serverNumber) * (ImageHeight / numberOfServers)
	endIndex := (serverNumber + 1) * (ImageHeight / numberOfServers)

	//if the number of servers doesn't evenly divide, give the last server the rest
	//the max number of AWS nodes will be 4
	//3 is the only number that doesn't evenly divide into 16, 64, and 512
	//in this case, the last server will only be given an additional 1, 1 and 2 rows respectively
	if serverNumber == numberOfServers-1 {
		endIndex = ImageHeight
	}

	totalRows := endIndex - startIndex
	rowsPerThread := totalRows / numberOfThreads

	threadSlice := make([]chan [][]byte, numberOfThreads)

	for i, _ := range threadSlice {
		threadSlice[i] = make(chan [][]byte)
	}

	for i, _ := range threadSlice {
		if i == len(threadSlice)-1 {
			go UpdateCells(world, startIndex+(i*rowsPerThread), endIndex, ImageHeight, ImageWidth, threadSlice[i])
		} else {
			go UpdateCells(world, startIndex+(i*rowsPerThread), startIndex+((i+1)*rowsPerThread), ImageHeight, ImageWidth, threadSlice[i])
		}
	}

	newWorld := make([][]byte, 0)

	for i, _ := range threadSlice {
		newWorld = append(newWorld, <-threadSlice[i]...)
	}

	res.World = newWorld

	return
}

func UpdateCells(world [][]byte, startIndex, endIndex, ImageHeight, ImageWidth int, rowsChan chan [][]byte) {
	workerWorld := make([][]byte, endIndex-startIndex)
	for i := range workerWorld {
		workerWorld[i] = make([]byte, ImageWidth)
		copy(workerWorld[i], world[i+startIndex])
	}

	for i := startIndex; i < endIndex; i++ {
		for j := 0; j < ImageWidth; j++ {
			iBehind := (i - 1 + ImageHeight) % ImageHeight
			iAhead := (i + 1 + ImageHeight) % ImageHeight
			jBehind := (j - 1 + ImageWidth) % ImageWidth
			jAhead := (j + 1 + ImageWidth) % ImageWidth

			neighbourSum := int(world[iBehind][jBehind]) + int(world[iBehind][j]) + int(world[iBehind][jAhead]) + int(world[i][jBehind]) + int(world[i][jAhead]) + int(world[iAhead][jBehind]) + int(world[iAhead][j]) + int(world[iAhead][jAhead])
			liveNeighbours := neighbourSum / 255

			if world[i][j] == 255 {
				if (liveNeighbours < 2) || (liveNeighbours > 3) {
					//i-startIndex because the startIndex may be something like 8, but newWorld starts at index 0
					workerWorld[i-startIndex][j] = 0
				}
			} else if world[i][j] == 0 {
				if liveNeighbours == 3 {
					workerWorld[i-startIndex][j] = 255
				}
			}
		}
	}
	rowsChan <- workerWorld
	return
}

// kills the server
func (g *GolOperations) KillServer(req stubs.Request, res *stubs.Response) (err error) {
	killServer = true

	return
}

func main() {
	pAddr := flag.String("port", "8031", "Port to listen on")
	pThreads := flag.Int("threads", 1, "Number of threads to use")
	flag.Parse()
	numberOfThreads = *pThreads
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
