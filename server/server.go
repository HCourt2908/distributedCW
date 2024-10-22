package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolOperations struct{}

var completedTurns int
var numAliveCells int

func holdCurrentState(updateStateTurns chan int, updateStateAlive chan int) {
	for {
		select {
		case completedTurns = <-updateStateTurns:
			numAliveCells = <-updateStateAlive
		}

	}
}

func (g *GolOperations) ProcessTurns(req stubs.Request, res *stubs.Response) (err error) {

	updateStateTurns := make(chan int)
	updateStateAlive := make(chan int)
	go holdCurrentState(updateStateTurns, updateStateAlive)

	world := req.World
	ImageWidth := req.ImageWidth
	ImageHeight := req.ImageHeight
	Turns := req.Turns

	numAliveCells := len(calculateAliveCells(ImageHeight, ImageWidth, world))
	//completedTurnsChannel <- 1

	for i := 0; i < Turns; i++ {
		//completedTurnsChannel <- i
		//aliveCellCountChannel <- numAliveCells
		updateStateTurns <- i
		updateStateAlive <- numAliveCells
		world = calculateNextState(ImageHeight, ImageWidth, world)
		numAliveCells = len(calculateAliveCells(ImageHeight, ImageWidth, world))
	}

	res.World = world
	res.AliveCells = calculateAliveCells(ImageHeight, ImageWidth, world)

	return
}

func (g *GolOperations) ReturnAliveCells(req stubs.Request, tickerRes *stubs.TickerResponse) (err error) {

	tickerRes.NumAliveCells = numAliveCells
	tickerRes.CompletedTurns = completedTurns

	return
}

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
	rpc.Register(&GolOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	fmt.Println("hi")
	defer listener.Close()
	rpc.Accept(listener)
	fmt.Println("hello")
}
