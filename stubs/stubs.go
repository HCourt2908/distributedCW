package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

//ProcessTurns executes all the specified turns of GOL
var GolHandler = "GolOperations.ProcessTurns"

//ReturnAliveCells returns how many turns have passed so far and how many cells are alive
var AliveCellHandler = "GolOperations.ReturnAliveCells"

//When we call ReturnAliveCells, we want to return the number of alive cells and how many turns have been completed
type TickerResponse struct {
	NumAliveCells  int
	CompletedTurns int
}

//When we call ProcessTurns, we want to return the final state world and a slice of all the alive cells
type Response struct {
	World      [][]byte
	AliveCells []util.Cell
}

//When we call ProcessTurns, we want to provide the ImageWidth, ImageHeight, the number of turns to execute and the initial world
type Request struct {
	ImageWidth  int
	ImageHeight int
	Turns       int
	World       [][]byte
}
