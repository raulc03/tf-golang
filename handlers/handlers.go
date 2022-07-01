package handlers

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/raulc03/tf-golang/utils"
)

func Send(remote string, frame utils.Frame, callback func(net.Conn)) bool {
	if cn, err := net.Dial("tcp", remote); err == nil {
		defer cn.Close()
		enc := json.NewEncoder(cn)
		enc.Encode(frame)
		if callback != nil {
			callback(cn)
		}
		return true
	} else {
		log.Printf("%s -> can't connect to %s\n", utils.Host, remote)
		idx := -1
		remotes := <-utils.ChRemotes
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
		utils.ChRemotes <- remotes
		SetupCloseHandler()
		return false
	}
}

func responseNewContact(enc *json.Encoder, remotes []string) {
	data := remotes
	bc := <-utils.ChBlockchain
	out, err := json.Marshal(bc)
	if err == nil {
		data = append(data, string(out))
	}
	utils.ChBlockchain <- bc
	// Añadir la blockchain como elemento del slice de strings
	enc.Encode(utils.Frame{Cmd: "<response>", Sender: utils.Host, Data: data})
}

// Función que maneja la nueva conexión con un nuevo nodo
func HandleGreeting(cn net.Conn, frame *utils.Frame) {
	enc := json.NewEncoder(cn)
	remotes := <-utils.ChRemotes
	// Le respondemos con la lista de mis contactos y la blockchain
	responseNewContact(enc, remotes)
	// Notificamos a mis contactos para que agreguen al nuevo nodo
	notification := utils.Frame{Cmd: "add", Sender: utils.Host,
		Data: []string{frame.Sender}}
	for _, remote := range remotes {
		Send(remote, notification, nil)
	}
	// Agrego al nuevo nodo a mis contactos
	remotes = append(remotes, frame.Sender)
	log.Printf("%s -> Known nodes: %s\n", utils.Host, remotes)
	utils.ChRemotes <- remotes
}

// Funcion que maneja la consulta sobre agregar un nuevo nodo a los contactos
func HandleAdd(frame *utils.Frame) {
	remotes := <-utils.ChRemotes
	remotes = append(remotes, frame.Data...)
	log.Printf("%s -> Known nodes: %s\n", utils.Host, remotes)
	utils.ChRemotes <- remotes
}

// Obtenemos la data para construir el bloque que se añadirá al blockchain
func HandleCreateBlock(frame *utils.Frame) {
	bc := <-utils.ChBlockchain
	data := strings.Join(frame.Data, ",")
	bc.AddBlock(data)
	utils.ChBlockchain <- bc
}

// Hash Corrupto
func HandleCorruptHash(cn net.Conn, frame *utils.Frame) {
	HandleCreateBlock(&utils.Frame{Cmd: "create", Sender: frame.Sender,
		Data: []string{`"id":1`, `"name":"ruperto"`, `"category":"Roberto el que pega"`,
			`"brand":"Logitech"`, `"serie":"123456789"`}})
	// Enviamos respuesta
	enc := json.NewEncoder(cn)
	data := []string{string(`"status":"OK"`)}
	enc.Encode(utils.Frame{Cmd: "<response_post>", Sender: utils.Host, Data: data})

	// Enviamos información del bloque a los demás nodos
	notification := utils.Frame{Cmd: "create", Sender: utils.Host, Data: frame.Data}
	remotes := <-utils.ChRemotes
	for _, remote := range remotes {
		Send(remote, notification, nil)
	}
	utils.ChRemotes <- remotes

	// Empezamos consenso del último bloque añadido
	bc := <-utils.ChBlockchain
	pos_block := len(bc.Blocks) - 1
	utils.ChBlockchain <- bc
	startConsensus(pos_block)
}

func HandlePost(cn net.Conn, frame *utils.Frame) {
	// Añadimos el bloque a la blockchain
	HandleCreateBlock(frame)

	// Enviamos respuesta
	enc := json.NewEncoder(cn)
	data := []string{string(`"status":"OK"`)}
	enc.Encode(utils.Frame{Cmd: "<response_post>", Sender: utils.Host, Data: data})

	// Enviamos información del bloque a los demás nodos
	notification := utils.Frame{Cmd: "create", Sender: utils.Host, Data: frame.Data}
	remotes := <-utils.ChRemotes
	for _, remote := range remotes {
		Send(remote, notification, nil)
	}
	utils.ChRemotes <- remotes

	// Empezamos consenso del último bloque añadido
	bc := <-utils.ChBlockchain
	pos_block := len(bc.Blocks) - 1
	utils.ChBlockchain <- bc
	startConsensus(pos_block)
}

func HandleGet(cn net.Conn, frame *utils.Frame) {
	enc := json.NewEncoder(cn)
	bc := <-utils.ChBlockchain

	out, err := json.Marshal(bc)
	utils.ChBlockchain <- bc
	if err != nil {
		enc.Encode(utils.Frame{Cmd: "<response_get>", Sender: utils.Host,
			Data: []string{`"status":` + err.Error()}})
		return
	}

	// Enviando la blockchain como respuesta
	data := []string{string(out)}
	rsp_get := utils.Frame{Cmd: "<response_get>", Sender: utils.Host, Data: data}
	enc.Encode(rsp_get)
}

func HandleUpdateBlock(frame *utils.Frame) int {
	pos_block, _ := strconv.Atoi(strings.Split(frame.Data[0], ":")[1])
	data := strings.Join(frame.Data, ",")
	bc := <-utils.ChBlockchain
	bc.UpdateBlock(data, pos_block)
	utils.ChBlockchain <- bc

	return pos_block
}

func HandlePut(cn net.Conn, frame *utils.Frame) {
	// Obteniendo la posicion del bloque a actualizar
	pos_block := HandleUpdateBlock(frame)

	// Enviamos respuesta
	enc := json.NewEncoder(cn)
	data := []string{string(`"status":"OK"`)}
	enc.Encode(utils.Frame{Cmd: "<response_post>", Sender: utils.Host, Data: data})

	// Enviamos información del bloque a los demás nodos
	notification := utils.Frame{Cmd: "update", Sender: utils.Host, Data: frame.Data}
	remotes := <-utils.ChRemotes
	for _, remote := range remotes {
		Send(remote, notification, nil)
	}
	utils.ChRemotes <- remotes

	// Empezamos consenso del último bloque añadido
	startConsensus(pos_block)
}

func startConsensus(pos_block int) {
	remotes := <-utils.ChRemotes
	for _, remote := range remotes {
		log.Printf("%s -> Notifiying about Consensus to %s\n", utils.Host, remote)
		Send(remote, utils.Frame{Cmd: "consensus", Sender: utils.Host,
			Data: []string{strconv.Itoa(pos_block)}}, nil)
	}
	utils.ChRemotes <- remotes
	HandleConsensus(pos_block)
}

func HandleConsensus(pos_block int) {
	bc := <-utils.ChBlockchain
	vote_hash := hex.EncodeToString(bc.Blocks[pos_block].Hash)

	utils.ChBlockchain <- bc

	m := <-utils.ChCons
	m = make(map[string]int)
	m[vote_hash]++
	utils.ChCons <- m

	remotes := <-utils.ChRemotes
	utils.Participants = len(remotes) + 1
	for _, remote := range remotes {
		log.Printf("%s -> Sending vote [%s] to %s\n", utils.Host, vote_hash, remote)
		Send(remote, utils.Frame{Cmd: "vote", Sender: utils.Host,
			Data: []string{vote_hash, strconv.Itoa(pos_block)}}, nil)
	}
	utils.ChRemotes <- remotes
}

func HandleVote(frame *utils.Frame) {
	vote := frame.Data[0]
	pos_block, _ := strconv.Atoi(frame.Data[1])
	m := <-utils.ChCons
	m[vote]++
	utils.ChCons <- m

	var total_value int = 0
	for _, value := range m {
		total_value += value
	}
	if total_value == utils.Participants {
		if len(m) == 1 {
			log.Printf("%s -> Block Added Succesfully - Hash [%s]", utils.Host, vote)
		} else {
			bc := <-utils.ChBlockchain
			vote_hash := hex.EncodeToString(bc.Blocks[pos_block].Hash)
			utils.ChBlockchain <- bc
			m := <-utils.ChCons
			if m[vote_hash] < (utils.Participants/2) + 1 {
				log.Printf("%s -> I have a problem, I need to build the block again \n", utils.Host)

				Send(frame.Sender, utils.Frame{Cmd: "help", Sender: utils.Host,
					Data: []string{strconv.Itoa(pos_block)}}, func(cn net.Conn) {

					dec := json.NewDecoder(cn)
					var frame utils.Frame
					dec.Decode(&frame)
					block_net := []byte(frame.Data[0])
					bc := <-utils.ChBlockchain
					// Obtengo el bloque del nodo al que se lo pedí y lo
					// reemplazo por mi bloque corrupto
					json.Unmarshal(block_net, bc.Blocks[pos_block])
					utils.ChBlockchain <- bc
					go startConsensus(pos_block)
				})
			}
			utils.ChCons <- m
		}
	}
}

func Help(cn net.Conn, frame *utils.Frame) {
	bc := <-utils.ChBlockchain
	enc := json.NewEncoder(cn)
	last_block := bc.Blocks[len(bc.Blocks)-1]
	out, err := json.Marshal(last_block)
	if err == nil {
		data := []string{string(out)}
		enc.Encode(utils.Frame{Cmd: "<response>", Sender: utils.Host, Data: data})
	}
}
func HandleGoodbye(frame *utils.Frame) {
	remotes := <-utils.ChRemotes
	node_down := frame.Sender
	for i, remote := range remotes {
		if remote == node_down {
			remotes = append(remotes[:i], remotes[i+1:]...)
		}
	}
	log.Printf("%s -> Known nodes: %s\n", utils.Host, remotes)
	utils.ChRemotes <- remotes
}

func HandleClose() {
	remotes := <-utils.ChRemotes
	notification := utils.Frame{Cmd: "goodbye", Sender: utils.Host, Data: []string{}}
	for _, remote := range remotes {
		Send(remote, notification, nil)
	}
	utils.ChRemotes <- remotes
}

func SetupCloseHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		HandleClose()
		os.Exit(1)
	}()
}
