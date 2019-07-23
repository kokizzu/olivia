package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/olivia-ai/olivia/modules"

	"github.com/gookit/color"
	"github.com/gorilla/websocket"
	"github.com/olivia-ai/olivia/analysis"
	"github.com/olivia-ai/olivia/training"
	"github.com/olivia-ai/olivia/user"
	"github.com/olivia-ai/olivia/util"
	gocache "github.com/patrickmn/go-cache"
)

var (
	model = training.CreateNeuralNetwork()
	cache = gocache.New(5*time.Minute, 5*time.Minute)
)

// Configure the upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type RequestMessage struct {
	Type        int              `json:"type"` // 0 is for a "handshake" to send informations, 1 is for chat
	Content     string           `json:"content"`
	Token       string           `json:"user_token"`
	Information user.Information `json:"information"`
}

type ResponseMessage struct {
	Content     string           `json:"content"`
	Tag         string           `json:"tag"`
	Information user.Information `json:"information"`
}

func main() {
	http.HandleFunc("/", Handle)

	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	magenta := color.FgMagenta.Render
	fmt.Printf("\nListening on the port %s...\n", magenta(port))
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}

func Handle(w http.ResponseWriter, r *http.Request) {
	conn, _ := upgrader.Upgrade(w, r, nil)

	for {
		// Read message from browser
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			continue
		}

		// Unserialize the json content of the message
		var request RequestMessage
		if err = json.Unmarshal(msg, &request); err != nil {
			continue
		}

		// Set the informations from the client into the cache
		if user.GetUserInformations(request.Token) == (user.Information{}) {
			user.SetUserInformations(request.Token, request.Information)
		}

		if request.Type == 0 {
			// Engage the conversation with the user
			for _, bytes := range Engage(request.Token) {
				if err = conn.WriteMessage(msgType, bytes); err != nil {
					continue
				}
			}
		} else if request.Type == 1 {
			// Write message back to browser
			response := Reply(request)
			if err = conn.WriteMessage(msgType, response); err != nil {
				continue
			}
		}
	}
}

func Reply(request RequestMessage) []byte {
	var responseSentence, responseTag string

	// Send a message from res/messages.json if it is too long
	if len(request.Content) > 500 {
		responseTag = "too long"
		responseSentence = util.GetMessage(responseTag)
	} else {
		responseTag, responseSentence = analysis.NewSentence(
			request.Content,
		).Calculate(*cache, model, request.Token)
	}

	// Marshall the response in json
	response := ResponseMessage{
		Content:     responseSentence,
		Tag:         responseTag,
		Information: user.GetUserInformations(request.Token),
	}

	bytes, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}

	return bytes
}

func Engage(userToken string) (output [][]byte) {
	for _, engageContent := range modules.EngageConversation(userToken) {
		response := ResponseMessage{
			Content:     engageContent,
			Tag:         "engage conversation",
			Information: user.GetUserInformations(userToken),
		}

		bytes, err := json.Marshal(response)
		if err != nil {
			panic(err)
		}

		output = append(output, bytes)
	}

	return
}
