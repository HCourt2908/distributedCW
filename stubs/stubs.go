package stubs

import "uk.ac.bris.cs/gameoflife/util"

// ProcessTurns executes all the specified turns of GOL
var GolHandler = "GolOperations.ProcessTurns"

// ReturnAliveCells returns how many turns have passed so far and how many cells are alive
var AliveCellHandler = "GolOperations.ReturnAliveCells"

// SaveCurrentState occurs when the client presses the s key. It will force output of a PGM file of the current state
var SaveCurrentState = "GolOperations.SaveCurrentState"

// CloseClientConnection occurs when the client presses the q key. It severs the connection between the client and the server,
// without causing an error on the server's side.
var CloseClientConnection = "GolOperations.CloseClientConnection"

// CloseAllComponents occurs when the client presses the k key. It outputs a PGM file of the current state, and then close
// both the server and the client. CurrentStateResponse can be reused for the response here.
var CloseAllComponents = "GolOperations.CloseAllComponents"

// PauseProcessingToggle occurs when the client presses the p key. It alternates between two behaviours:
// Pause processing on the server and have the client print the current turn being processed
// Resume processing on the server and have the client print Continuing
var PauseProcessingToggle = "GolOperations.PauseProcessingToggle"

// When we call ProcessTurns, we want to provide the ImageWidth, ImageHeight, the number of turns to execute and the initial world
type Request struct {
	ImageWidth  int
	ImageHeight int
	Turns       int
	World       [][]byte
}

type Response struct {
	CompletedTurns int
	World          [][]byte
	AliveCells     []util.Cell
	NumAliveCells  int
	TerminateTurns int
	OutString      string
}
