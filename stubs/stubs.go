package stubs

import "uk.ac.bris.cs/gameoflife/util"

//ProcessTurns executes all the specified turns of GOL
var GolHandler = "GolOperations.ProcessTurns"

//When we call ProcessTurns, we want to return the final state world and a slice of all the alive cells
type FinalStateResponse struct {
	World      [][]byte
	AliveCells []util.Cell
}

//ReturnAliveCells returns how many turns have passed so far and how many cells are alive
var AliveCellHandler = "GolOperations.ReturnAliveCells"

//When we call ReturnAliveCells, we want to return the number of alive cells and how many turns have been completed
type TickerResponse struct {
	NumAliveCells  int
	CompletedTurns int
}

//SaveCurrentState occurs when the client presses the s key. It will force output of a PGM file of the current state
//TODO put this in the response and then use the output event to make output file
var CurrentStateSave = "GolOperations.SaveCurrentState"

//When we call CurrentStateSave, we want to return the current state of the GOL world,
//as well as the number of turns completed for unique filenaming
type CurrentStateResponse struct {
	World          [][]byte
	CompletedTurns int
}

//CloseClientConnection occurs when the client presses the q key. It severs the connection between the client and the server,
//without causing an error on the server's side.
var CloseClientConnection = "GolOperations.CloseClientConnection"

type CloseClientResponse struct {
	AliveCells     []util.Cell
	CompletedTurns int
}

//CloseAllComponents occurs when the client presses the k key. It outputs a PGM file of the current state, and then close
//both the server and the client. CurrentStateResponse can be reused for the response here.
var CloseAllComponents = "GolOperations.CloseAllComponents"

//PauseProcessingToggle occurs when the client presses the p key. It alternates between two behaviours:
//Pause processing on the server and have the client print the current turn being processed
//Resume processing on the server and have the client print Continuing
var PauseProcessingToggle = "GolOperations.PauseProcessingToggle"

//When we call PauseProcessingToggle, a string will be printed on the clients side, the contents of OutString.
type PauseProcessingResponse struct {
	OutString        string
	PausedProcessing bool
	CompletedTurns   int
}

//When we call ProcessTurns, we want to provide the ImageWidth, ImageHeight, the number of turns to execute and the initial world
type Request struct {
	ImageWidth  int
	ImageHeight int
	Turns       int
	World       [][]byte
}
