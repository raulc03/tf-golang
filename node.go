package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Frame struct {
	Cmd    string   `json:"cmd"`
	Sender string   `json:"sender"`
	Data   []string `json:"data"`
}

type Info struct {
	nextNode string
	nextNum  int
	imFirst  bool
	cont     int
}

type InfoCons struct {
	contA, contB int
}

var (
	host         string
	myNum        int
	chRemotes    chan []string
	chInfo       chan Info
	chCons       chan InfoCons
	participants int
)

func main() {
	rand.Seed(time.Now().UnixNano())
	if len(os.Args) == 1 {
		log.Println("Hostname not given")
	} else {
		host = os.Args[1]
		chRemotes = make(chan []string, 1)
		chInfo = make(chan Info, 1)
		chCons = make(chan InfoCons, 1)

		chRemotes <- []string{}
		if len(os.Args) >= 3 {
			connectToNode(os.Args[2])
		}
		server()
	}
}

func connectToNode(remote string) {
	remotes := <-chRemotes
	remotes = append(remotes, remote)
	chRemotes <- remotes
	if !send(remote, Frame{"greeting", host, []string{}}, func(cn net.Conn) {
		dec := json.NewDecoder(cn)
		var frame Frame
		dec.Decode(&frame)
		remotes := <-chRemotes
		// Al conectarme al nodo destino, este me da la lista de todos los
		// nodos a los que está conectado
		remotes = append(remotes, frame.Data...)
		chRemotes <- remotes
		log.Printf("%s -> Connections: %s\n", host, remotes)
	}) {
		log.Printf("%s -> unable to connect to %s\n", host, remote)
	}
}

func send(remote string, frame Frame, callback func(net.Conn)) bool {
	if cn, err := net.Dial("tcp", remote); err == nil {
		defer cn.Close()
		enc := json.NewEncoder(cn)
		enc.Encode(frame)
		if callback != nil {
			callback(cn)
		}
		return true
	} else {
		log.Printf("%s -> can't connect to %s\n", host, remote)
		idx := -1
		remotes := <-chRemotes
		for i, rem := range remotes {
			if remote == rem {
				idx = i
				break
			}
		}
		if idx >= 0 {
			remotes[idx] = remotes[len(remotes)-1]
			remotes = remotes[:len(remotes)-1]
		}
		chRemotes <- remotes
		return false
	}
}

func server() {
	if ln, err := net.Listen("tcp", host); err == nil {
		defer ln.Close()
		setupCloseHandler()
		log.Printf("Listening on %s\n", host)
		for {
			if cn, err := ln.Accept(); err == nil {
				go fauxDispatcher(cn)
			} else {
				log.Printf("%s -> Can't accept connection.\n", host)
			}
		}
	} else {
		log.Printf("Can't listen on %s\n", host)
	}
}

func fauxDispatcher(cn net.Conn) {
	defer cn.Close()
	dec := json.NewDecoder(cn)
	frame := &Frame{}
	dec.Decode(frame)
	switch frame.Cmd {
	case "greeting":
		handleGreeting(cn, frame)
	case "add":
		handleAdd(frame)
	case "goodbye":
		handleGoodbye(frame)
	}
}

func handleGreeting(cn net.Conn, frame *Frame) {
	enc := json.NewEncoder(cn)
	remotes := <-chRemotes
	enc.Encode(Frame{"<response>", host, remotes})
	notification := Frame{"add", host, []string{frame.Sender}}
	for _, remote := range remotes {
		send(remote, notification, nil)
	}
	remotes = append(remotes, frame.Sender)
	log.Printf("%s -> Known nodes: %s\n", host, remotes)
	chRemotes <- remotes
}

func handleAdd(frame *Frame) {
	remotes := <-chRemotes
	remotes = append(remotes, frame.Data...)
	log.Printf("%s -> Known nodes: %s\n", host, remotes)
	chRemotes <- remotes
}

func setupCloseHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		handleClose()
		os.Exit(1)
	}()
}

func handleClose() {
	remotes := <-chRemotes
	notification := Frame{"goodbye", host, []string{}}
	for _, remote := range remotes {
		send(remote, notification, nil)
	}
	chRemotes <- remotes
}

func handleGoodbye(frame *Frame) {
	remotes := <-chRemotes
	node_down := frame.Sender
	for i, remote := range remotes {
		if remote == node_down {
			remotes = append(remotes[:i], remotes[i+1:]...)
		}
	}
	log.Printf("%s -> Known nodes: %s\n", host, remotes)
	chRemotes <- remotes
}
