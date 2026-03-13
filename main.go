package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/gin-contrib/cors"
)

var commandsMutex = sync.Mutex{}

// commands queue
var commands = make([]string, 0)

const ESP_HASH = "8538bca6895b078765e9ec5b659147cc79182fea98354550de439fbbfde6430a2e29c73def65fff106f281a73a1330d6a5904206dc40209e077cedea612f1b59"

const AI_SERVER_HASH = "001ca698898c60f835261b55cc2ac77b3a5560ba7da32b4292dc508d53e2a82946d6175d18a5760e982ad3f85285b29f3f8320c9be3d0aa5a4cff3ce45b74511"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow ESP32
	},
}

// connections := make(map[string]*websocket.Conn)

func sendCommand(command string) {
	commandsMutex.Lock()
	commands = append(commands, command)
	commandsMutex.Unlock()
}

var aiServerConnMutex = sync.Mutex{}

func ESPWSHandler(c *gin.Context) {

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	fmt.Println("ESP32 connected")

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		if msgType == websocket.BinaryMessage {
			fmt.Println("Received audio bytes:", len(data))
		}

		// send this message
		// light:true

		// conn.WriteMessage(websocket.TextMessage, []byte("light:true"))

		commandsMutex.Lock()

		if len(commands) > 0 {
			for _, command := range commands {
				fmt.Println("Sending command:", command)
				conn.WriteMessage(websocket.TextMessage, []byte(command))
			}
			commands = make([]string, 0)
		}

		commandsMutex.Unlock()

		if aiServerConn != nil {
			aiServerConnMutex.Lock()
			err := aiServerConn.WriteMessage(websocket.BinaryMessage, data)
			aiServerConnMutex.Unlock()

			if err != nil {
				log.Println("Error sending to AI Server:", err)
			}
		}

	}

}

var aiServerConn *websocket.Conn

func AIServerWSHandler(c *gin.Context) {

	fmt.Println("AI Server attempting to connect")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)

	if err != nil {
		log.Println("Upgrade error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade to WebSocket"})
		return
	}

	fmt.Println("AI Server connected")

	aiServerConn = conn

	defer func() {
		aiServerConn = nil
	}()

	defer conn.Close()

	fmt.Println("AI Server connected")

	// conn.WriteMessage(websocket.TextMessage, []byte("light:true"))

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		if msgType == websocket.TextMessage {
			fmt.Println("Received command from AI Server:", string(data))
		}

	}

}

func wsHandler(c *gin.Context) {

	connectionHash := c.Query("hash")

	commands = append(commands, "light:true")

	if connectionHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing connection hash"})
		return
	}

	if connectionHash == ESP_HASH {

		ESPWSHandler(c)
		return

	}

	if connectionHash == AI_SERVER_HASH {

		fmt.Println("AI Server attempting to connect")

		AIServerWSHandler(c)

		fmt.Println("AI Server connected")

	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid connection hash"})
		return
	}

}

func main() {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	router.Use(gin.LoggerWithWriter(os.Stdout))

	router.Use(gin.Recovery())

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// WebSocket route
	router.GET("/ws", wsHandler)

	// health check route
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	fmt.Println("WebSocket server listening on :42069")

	router.Run(":42069")

}
