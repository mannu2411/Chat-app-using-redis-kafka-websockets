package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-co-op/gocron"
	"github.com/gorilla/websocket"
)

type Msgs struct {
	msg     string
	msgType int
}
type UserMsgs struct {
	mu   sync.Mutex
	msgs []Msgs
}

var userConns map[string]*websocket.Conn
var userMsgs map[string]UserMsgs

func runCronJobs() {
	s := gocron.NewScheduler(time.UTC)
	s.Every(10).Second().Do(func() {
		log.Printf("Running Cron Job..")
		WaitQueueCron()
	})

	s.StartBlocking()
}

func main() {
	router := chi.NewRouter()
	go runCronJobs()
	userConns = make(map[string]*websocket.Conn)
	userMsgs = make(map[string]UserMsgs)
	router.Route("/", func(ws chi.Router) {
		ws.Get("/", alive)
	})
	router.Route("/ws", func(ws chi.Router) {
		ws.Get("/read", ReadMsg)
	})
	port := os.Getenv("PORT")
	log.Fatal(http.ListenAndServe(":"+port, router))
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func alive(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<h4>Welcome to chatting app</h4>"))
	log.Printf("Welcome to chatting app")
	return
}

func ReadMsg(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	userConns[r.URL.Query().Get("in_id")] = conn
	log.Printf("Here: %v and %v", conn.RemoteAddr(), r.URL.Query().Get("in_id"))
	for {
		msgType, msgs, err := userConns[r.URL.Query().Get("in_id")].ReadMessage()
		if err != nil {
			break
		}
		oid := r.URL.Query().Get("out_id")
		_, ok := userConns[oid]
		if ok {
			if len(userMsgs[oid].msgs) > 0 {
				mut := userMsgs[oid].mu
				mut.Lock()
				for _, msg := range userMsgs[oid].msgs {
					if err := userConns[oid].WriteMessage(msg.msgType, []byte(msg.msg)); err != nil {
						return
					}
				}
				mut.Unlock()
				delete(userMsgs, r.URL.Query().Get("out_id"))

			}
			if err := userConns[oid].WriteMessage(msgType, msgs); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			if ele, ok := userMsgs[oid]; !ok {
				ele.msgs = make([]Msgs, 0)
				ele.mu = sync.Mutex{}
				userMsgs[oid] = ele
			}
			usrMsg := userMsgs[oid]
			mut := usrMsg.mu
			mut.Lock()
			messges := userMsgs[oid].msgs
			messges = append(messges, Msgs{msg: string(msgs), msgType: msgType})
			usrMsg.msgs = messges
			mut.Unlock()
			userMsgs[oid] = usrMsg
		}
	}
	delete(userConns, r.URL.Query().Get("in_id"))
	log.Printf("user %v deleted", r.URL.Query().Get("in_id"))
}
func WaitQueueCron() {
	for key, ele := range userMsgs {
		_, ok := userConns[key]
		if ok {
			ele.mu.Lock()
			for _, msg := range ele.msgs {
				if err := userConns[key].WriteMessage(msg.msgType, []byte(msg.msg)); err != nil {
					return
				}
			}
			ele.mu.Unlock()
			delete(userMsgs, key)
		}
	}
}
