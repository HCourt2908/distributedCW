package stubs

import "uk.ac.bris.cs/gameoflife/util"

// Broker executes all the specified turns of GOL
var Broker = "BrokerOperations.Broker"

// BrokerAliveCellHandler returns how many turns have passed so far, and how many cells are alive at this turn
var BrokerAliveCellHandler = "BrokerOperations.ReturnAliveCells"

// BrokerSaveCurrentState occurs when the client presses the s key. It will force output of a PGM file of the current state
var BrokerSaveCurrentState = "BrokerOperations.SaveCurrentState"

// BrokerCloseClientConnection occurs when the client presses the q key. It severs the connection between the client and the broker.
// It does not cause an error in the broker or servers, and the client is provided the most recent state to output a PGM image of.
var BrokerCloseClientConnection = "BrokerOperations.CloseClientConnection"

// BrokerCloseAllComponents occurs when the client presses the k key. It outputs a PGM file of the current state, then closes
// the client, the servers and the broker
var BrokerCloseAllComponents = "BrokerOperations.CloseAllComponents"

// BrokerPauseProcessingToggle occurs when the client presses the p key. It alternates between two behaviours:
// Pause processing on the broker and have the client print the current turn being processed
// Resume processing on the broker and have the client print Continuing
var BrokerPauseProcessingToggle = "BrokerOperations.PauseProcessingToggle"

// CalculateNextState is called by the broker on all the servers when it wants one turn of GOL processed.
var CalculateNextState = "GolOperations.CalculateNextState"

// KillServer is called by the broker on each of the servers when it wants to terminate them
var KillServer = "GolOperations.KillServer"

// Request We want to provide the broker with the ImageWidth, ImageHeight, the number of Turns to execute and the initial World
type Request struct {
	ImageWidth  int
	ImageHeight int
	Turns       int
	World       [][]byte
}

// Response From the broker, the client expects: the number of CompletedTurns, the current state of the World, all the AliveCells, the NumAliveCells and the number of turns executed on termination (TerminateTurns)
type Response struct {
	CompletedTurns int
	World          [][]byte
	AliveCells     []util.Cell
	NumAliveCells  int
	TerminateTurns int
}

// ServerRequest To process a GOL turn, an individual server needs: the previous World, the ImageWidth and ImageHeight, and the NoOfServers and ServerNumber (to calculate start and end indices)
type ServerRequest struct {
	World        [][]byte
	ImageWidth   int
	ImageHeight  int
	NoOfServers  int
	ServerNumber int
}

// ServerResponse From the server, the broker expects the rows of the new World that the server processed.
type ServerResponse struct {
	World [][]byte
}
