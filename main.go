package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

type Config struct {
	Token string
}

type Payload struct {
	T  interface{}            `json:"t"`
	S  interface{}            `json:"s"`
	Op int                    `json:"op"`
	D  map[string]interface{} `json:"d"`
}

type Heartbeat struct {
	op int
	d  interface{}
}

type Identify struct {
	Op int
	D  IdentifyData
}

type IdentifyData struct {
	Token      string
	Properties map[string]string
}

var addr = flag.String("addr", "gateway.discord.gg", "service address")

func main() {
	flag.Parse()
	log.SetFlags(0)

	token := os.Getenv("DISCORDTOKEN")
	config := Config{Token: token}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/?v=6&encoding=json"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{"Authorization": []string{config.Token}})
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	func() {
		defer close(done)
		for {
			fmt.Println("reading...")
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("err read: ", err)
				return
			}
			var p Payload
			log.Printf("recv: %s", message)
			json.Unmarshal(message, &p)
			fmt.Println(p.Op)
			if p.Op == 10 {
				go sendIdentify(config, c)
				go sendHeartbeat(p, c)
			}

		}
	}()

}
func sendIdentify(config Config, c *websocket.Conn) {
	fmt.Println("Sending Identify")

	properties := make(map[string]string)
	properties["$os"] = "linux"
	properties["$browser"] = "mybot"
	properties["$device"] = "mybot"

	identifyData := IdentifyData{
		Token:      config.Token,
		Properties: properties,
	}

	identify := Identify{
		Op: 2,
		D:  identifyData,
	}

	identifyJson, err := json.Marshal(identify)
	if err != nil {
		log.Println("error marshalling:", err)
	}
	err = c.WriteMessage(websocket.TextMessage, []byte(identifyJson))
	if err != nil {
		log.Println("error:", err)
	}
	fmt.Println("Sent Identify", string(identifyJson))
}

func sendHeartbeat(p Payload, c *websocket.Conn) {
	for {
		fmt.Println("Sending Heartbeat")
		fmt.Println("Received opcode10, will send heartbeat after sleeping for", p.D["heartbeat_interval"])
		time.Sleep(time.Duration(p.D["heartbeat_interval"].(float64)) * time.Millisecond)
		hb := Heartbeat{op: 1, d: p.S}
		hbJson, _ := json.Marshal(hb)
		fmt.Println("received op10, sending: ", hbJson)
		err := c.WriteMessage(websocket.TextMessage, []byte(hbJson))
		if err != nil {
			log.Println("error:", err)
		}
	}
}
