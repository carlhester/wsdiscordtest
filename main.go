// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

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

	go func() {
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

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
			/*	case t := <-ticker.C:
				err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
				if err != nil {
					log.Println("write:", err)
					return
				}
			*/
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

type Identify struct {
	Op int
	D  IdentifyData
}

type IdentifyData struct {
	token      string
	properties map[string]string
}

func sendIdentify(config Config, c *websocket.Conn) {
	fmt.Println("Sending Identify")

	p := make(map[string]string)
	p["$os"] = "linux"
	p["$browser"] = "mybot"
	p["$device"] = "mybot"

	identifyData := IdentifyData{
		token:      config.Token,
		properties: p,
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
