package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/raulc03/tf-golang/blockchain"
	"github.com/raulc03/tf-golang/handlers"
	"github.com/raulc03/tf-golang/utils"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	if len(os.Args) == 1 {
		log.Println("Hostname not given")
	} else {
		utils.Host = os.Args[1]
		utils.ChRemotes = make(chan []string, 1)
		utils.ChCons = make(chan map[string]int, 1)
		utils.ChBlockchain = make(chan blockchain.BlockChain, 1)
		utils.ChRemotes <- []string{}
		utils.ChCons <- make(map[string]int)
		aux_bc := blockchain.InitBlockChain()
		utils.ChBlockchain <- *aux_bc
		if len(os.Args) >= 3 {
			connectToNode(os.Args[2])
		}
		server()
	}
}

func connectToNode(remote string) {
	remotes := <-utils.ChRemotes
	remotes = append(remotes, remote)
	utils.ChRemotes <- remotes
	if !handlers.Send(remote, utils.Frame{Cmd: "greeting", Sender: utils.Host, Data: []string{}}, func(cn net.Conn) {
		dec := json.NewDecoder(cn)
		var frame utils.Frame
		dec.Decode(&frame)
		remotes := <-utils.ChRemotes
		// Al conectarme al nodo destino, este me da la lista de todos los
		// nodos a los que está conectado
		for i := 0; i < len(frame.Data)-1; i++ {
			remotes = append(remotes, frame.Data[i])
		}
		utils.ChRemotes <- remotes
		// Y la blockchain
		bc := <-utils.ChBlockchain
		bc_net := []byte(frame.Data[len(frame.Data)-1])
		json.Unmarshal(bc_net, &bc)
		utils.ChBlockchain <- bc
		log.Printf("%s -> Connections: %s\n", utils.Host, remotes)
		log.Printf("%s -> Blockchain added\n", utils.Host)
	}) {
		log.Printf("%s -> Unable to connect to %s\n", utils.Host, remote)
		handlers.SetupCloseHandler()
	}
}

func server() {
	if ln, err := net.Listen("tcp", utils.Host); err == nil {
		defer ln.Close()
		handlers.SetupCloseHandler()
		log.Printf("Listening on %s\n", utils.Host)
		for {
			if cn, err := ln.Accept(); err == nil {
				go fauxDispatcher(cn)
			} else {
				log.Printf("%s -> Can't accept connection.\n", utils.Host)
			}
		}
	} else {
		log.Printf("Can't listen on %s\n", utils.Host)
		handlers.SetupCloseHandler()
	}
}

func fauxDispatcher(cn net.Conn) {
	defer cn.Close()
	dec := json.NewDecoder(cn)
	frame := &utils.Frame{}
	dec.Decode(frame)
	switch frame.Cmd {
	case "greeting":
		handlers.HandleGreeting(cn, frame)
	case "add":
		handlers.HandleAdd(frame)
	case "goodbye":
		handlers.HandleGoodbye(frame)
	case "post":
		handlers.HandlePost(cn, frame)
	case "get":
		handlers.HandleGet(cn, frame)
	case "create":
		handlers.HandleCreateBlock(frame)
	case "consensus":
		handlers.HandleConsensus()
	case "vote":
		handlers.HandleVote(frame)
	case "help":
		handlers.Help(cn, frame)
	}
}
