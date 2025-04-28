package main

import (
	"fmt"
	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"log"
	"math/rand"
	"net/http"
	"os/exec"
	"sync"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func terminalHandler(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	challengeId := c.QueryParam("challengeId")
	if challengeId == "" {
		challengeId = "001" // default fallback
	}

	// Generate random container name
	containerName := fmt.Sprintf("%d", rand.Intn(1000000))

	cmd := exec.Command("docker", "run", "--name", containerName, "--rm", "-i",
		fmt.Sprintf("linux-learner:%s", challengeId),
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	var once sync.Once
	cleanup := func() {
		log.Println("Cleaning up terminal session...")
		cmd.Process.Kill()
		ptmx.Close()
		ws.Close()
		exec.Command("docker", "stop", containerName).Run()
	}

	// WebSocket -> PTY
	go func() {
		defer once.Do(cleanup)
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				break
			}
			_, err = ptmx.Write(msg)
			if err != nil {
				break
			}
		}
	}()

	// PTY -> WebSocket
	buf := make([]byte, 1024)
	for {
		n, err := ptmx.Read(buf)
		if err != nil {
			break
		}
		err = ws.WriteMessage(websocket.TextMessage, buf[:n])
		if err != nil {
			break
		}
	}

	once.Do(cleanup)
	return nil
}

func main() {
	e := echo.New()
	e.Static("/", "../client/")
	e.GET("/ws", terminalHandler)
	e.Logger.Fatal(e.Start(":8888"))
}
