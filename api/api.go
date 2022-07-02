package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	cors "github.com/itsjamie/gin-cors"
)

type Block struct {
	Hash     []byte
	Data     []byte
	PrevHash []byte
}

type BlockChain struct {
	Blocks []*Block
}

//INICIALIZANDO DATOS
type album struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Price  float64 `json:"price"`
}

type Frame struct {
	Cmd    string   `json:"cmd"`
	Sender string   `json:"sender"`
	Data   []string `json:"data"`
}

type Product struct {
	Id       int
	Name     string
	Category string
	Brand    string
	Serie    string
}

type Response struct {
	Status string
}

var countId int = 1

// albums slice to seed record album data.
var products = []Product{}

func sendPostTCP(prod Product, cmd string) string {
	// connect to the server
	var msg Frame
	if _, err := strconv.Atoi(prod.Serie); err != nil {
		return `{"status":"El número de serie no debe contener letras"}`
	}

	if len(prod.Serie) < 9 || len(prod.Serie) >= 10 {
		return `{"status":"El número de serie no es válido"}`
	}

	msg = Frame{Cmd: cmd, Sender: "localhost:8080",
		Data: []string{fmt.Sprintf(`"id":%d`, prod.Id),
			`"name":"` + prod.Name + `"`, `"category":"` + prod.Category + `"`,
			`"brand":"` + prod.Brand + `"`, `"serie":"` + prod.Serie + `"`}}
	var rsp string
	send("localhost:9000", msg, func(c net.Conn) {
		d := json.NewDecoder(c)
		var frame Frame
		d.Decode(&frame)
		rsp = frame.Data[0]
	})
	ex := "{" + rsp + "}"
	return ex
}

func send(remote string, frame Frame, callback func(net.Conn)) {
	if cn, err := net.Dial("tcp", remote); err == nil {
		defer cn.Close()
		enc := json.NewEncoder(cn)
		enc.Encode(frame)
		if callback != nil {
			callback(cn)
		}
	} else {
		fmt.Printf("Can't connect to %s\n", remote)
	}
}

func sendGetByIdTCP(id int) string {
	msg := Frame{Cmd: "get", Sender: "localhost:8080", Data: []string{}}
	var rsp string
	var prod string
	send("localhost:9000", msg, func(c net.Conn) {
		d := json.NewDecoder(c)
		var frame Frame
		d.Decode(&frame)
		rsp = frame.Data[0]
		prod = responseGetByIdHandler(rsp, id)
	})

	return prod
}

func sendGetTCP() []Product {
	msg := Frame{Cmd: "get", Sender: "localhost:8080", Data: []string{}}
	var rsp string
	var prods []Product
	send("localhost:9000", msg, func(c net.Conn) {
		d := json.NewDecoder(c)
		var frame Frame
		d.Decode(&frame)
		rsp = frame.Data[0]

		fmt.Println("LA DATA:")
		fmt.Println(frame.Data[0])
		prods = responseGetHandler(rsp)
	})
	fmt.Println(prods)
	return prods
}

func responseGetByIdHandler(data string, id int) string {
	var blockchain BlockChain
	json.Unmarshal([]byte(data), &blockchain)

	for _, block := range blockchain.Blocks {
		product := "{" + string(block.Data) + "}"

		var prod Product

		json.Unmarshal([]byte(product), &prod)

		if prod.Id == id {
			prodStr, _ := json.Marshal(&prod)
			return string(prodStr)
		}
	}

	return "Error"
}

func postProduct(c *gin.Context) {
	var product Product

	if err := c.BindJSON(&product); err != nil {
		return
	}

	var response Response

	product.Id = countId

	responseTcp := sendPostTCP(product, "post")
	products = append(products, product)
	json.Unmarshal([]byte(responseTcp), &response)
	if response.Status == "OK" {
		countId++
	}
	c.IndentedJSON(http.StatusCreated, response)
}

func testConsensusProduct(c *gin.Context) {
	var product Product

	if err := c.BindJSON(&product); err != nil {
		return
	}

	var response Response

	product.Id = countId

	responseTcp := sendPostTCP(product, "test_consensus")
	products = append(products, product)
	json.Unmarshal([]byte(responseTcp), &response)
	if response.Status == "OK" {
		countId++
	}
	c.IndentedJSON(http.StatusCreated, response)
}

func putProduct(c *gin.Context) {
	var response Response
	id, _ := strconv.Atoi(c.Query("id"))
	var product Product

	if id >= countId {
		responseTcp := `{"status":"El id no existe"}`
		json.Unmarshal([]byte(responseTcp), &response)
		c.IndentedJSON(http.StatusCreated, response)
		return
	}

	if err := c.BindJSON(&product); err != nil {
		return
	}

	product.Id = id

	responseTcp := sendPostTCP(product, "put")
	products = append(products, product)
	json.Unmarshal([]byte(responseTcp), &response)
	c.IndentedJSON(http.StatusCreated, response)
}

func getProductById(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id"))
	responseTcp := sendGetByIdTCP(id)
	if responseTcp == "Error" {
		var response Response
		json.Unmarshal([]byte("{"+`"status":"Error"`+"}"), &response)
		c.IndentedJSON(http.StatusCreated, response)
	} else {
		var prod Product
		json.Unmarshal([]byte(responseTcp), &prod)
		c.IndentedJSON(http.StatusCreated, prod)
	}
}

func responseGetHandler(data string) []Product {
	var blockchain BlockChain
	var products []Product
	json.Unmarshal([]byte(data), &blockchain)
	fmt.Println("Bloques:")
	fmt.Println(blockchain.Blocks)
	for i, block := range blockchain.Blocks {
		if i > 0 {

			fmt.Println("Bloque x:")
			fmt.Println(string(block.Data))
			product := "{" + string(block.Data) + "}"

			var prod Product

			json.Unmarshal([]byte(product), &prod)
			products = append(products, prod)
		}
	}
	fmt.Println(products)
	return products
}

func getProducts(c *gin.Context) {
	responseTcp := sendGetTCP()
	c.JSON(http.StatusCreated, responseTcp)
}

func main() {
	router := gin.Default()
	router.Use(cors.Middleware(cors.Config{
		Origins:         "*",
		Methods:         "GET, PUT, POST, DELETE",
		RequestHeaders:  "Origin, Authorization, Content-Type",
		ExposedHeaders:  "",
		MaxAge:          50 * time.Second,
		Credentials:     false,
		ValidateHeaders: false,
	}))
	router.GET("/api/products", getProducts)
	router.GET("/api/products/getById", getProductById)
	router.POST("/api/products", postProduct)
	router.PUT("/api/products", putProduct)
	router.POST("/api/test_consensus", testConsensusProduct)
	router.Run()
}
