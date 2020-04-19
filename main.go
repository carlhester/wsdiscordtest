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
	Op int         `json:"op"`
	D  interface{} `json:"d"`
}

type Identify struct {
	Op int          `json:"op"`
	D  IdentifyData `json:"d"`
}

type IdentifyData struct {
	Token      string            `json:"token"`
	Properties map[string]string `json:"properties"`
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
		log.Fatal("dial error:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			fmt.Println("reading...")
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read error: ", err)
				return
			}
			var p Payload
			err = json.Unmarshal(message, &p)
			if err != nil {
				log.Println("Unmarshal error: ", err)
				log.Printf("Received Raw Message: %s\n", message)
				log.Fatal(err)
			}
			fmt.Printf("Received Payload: %+v\n", p)
			switch p.Op {
			case 10:
				go sendIdentify(config, c)
				go sendHeartbeat(p, c)
			case 7:
				return
			case 9:
				return
			}
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			//err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
			log.Println("Ticker: ", t)
			return
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
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
		log.Println("error WriteMessage:", err)
	}
	fmt.Println("Sent Identify", string(identifyJson))
}

func sendHeartbeat(p Payload, c *websocket.Conn) {
	fmt.Println("Starting Heartbeat Cycle")
	for {
		fmt.Println("Received opcode10, will send heartbeat after sleeping for", p.D["heartbeat_interval"])
		time.Sleep(time.Duration(p.D["heartbeat_interval"].(float64)) * time.Millisecond)
		hb := Heartbeat{Op: 1, D: p.S}
		hbJson, err := json.Marshal(hb)
		if err != nil {
			log.Println("error marshalling:", err)
		}
		fmt.Println("Sending heartbeat: ", string(hbJson))
		err = c.WriteMessage(websocket.TextMessage, []byte(hbJson))
		if err != nil {
			log.Println("error:", err)
		}
	}
}
