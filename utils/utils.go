package utils

import (
	"github.com/raulc03/tf-golang/blockchain"
)

var (
	Host         string
	MyNum        int
	ChRemotes    chan []string
	ChCons       chan map[string]int
	Participants int
	ChBlockchain chan blockchain.BlockChain
)

type Frame struct {
	Cmd    string   `json:"cmd"`
	Sender string   `json:"sender"`
	Data   []string `json:"data"`
}
