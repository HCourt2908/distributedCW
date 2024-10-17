package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var GolHandler = "GolOperations.ProcessTurns"

type Response struct {
	World      [][]byte
	AliveCells []util.Cell
}

type Request struct {
	ImageWidth  int
	ImageHeight int
	Turns       int
	World       [][]byte
}
