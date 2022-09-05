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

type Msg struct {
	msg     string
	msgType int
}
type UserMsg struct {
	mu  sync.Mutex
	msg []Msg
}

var userCons map[string]*websocket.Conn
var userMsg map[string]UserMsg

func runCronJobs() {
	s := gocron.NewScheduler(time.UTC)
	s.Every(10).Second().Do(func() {
		log.Printf("Running Cron Job..")
		for key, ele := range userMsg {
			_, ok := userCons[key]
			if ok {
				WaitQueueCron(key, ele)
			}
		}
	})
	s.StartBlocking()
}

func main() {
	router := chi.NewRouter()
	go runCronJobs()
	userCons = make(map[string]*websocket.Conn)
	userMsg = make(map[string]UserMsg)
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
	ele, ok := userMsg[r.URL.Query().Get("in_id")]
	if ok {
		go WaitQueueCron(r.URL.Query().Get("in_id"), ele)
	}
	userCons[r.URL.Query().Get("in_id")] = conn
	log.Printf("Here: %v and %v", conn.RemoteAddr(), r.URL.Query().Get("in_id"))
	for {
		msgType, msgs, err := userCons[r.URL.Query().Get("in_id")].ReadMessage()
		if err != nil {
			break
		}
		oid := r.URL.Query().Get("out_id")
		_, ok := userCons[oid]
		if ok {
			if len(userMsg[oid].msg) > 0 {
				WaitQueueCron(oid, userMsg[oid])
			}
			//if len(userMsgs[oid].msgs) > 0 {
			//	mut := userMsgs[oid].mu
			//	mut.Lock()
			//	for _, msg := range userMsgs[oid].msgs {
			//		if err := userConns[oid].WriteMessage(msg.msgType, []byte(msg.msg)); err != nil {
			//			return
			//		}
			//	}
			//	mut.Unlock()
			//	delete(userMsgs, r.URL.Query().Get("out_id"))
			//
			//}
			if err := userCons[oid].WriteMessage(msgType, msgs); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			if ele, ok := userMsg[oid]; !ok {
				ele.msg = make([]Msg, 0)
				ele.mu = sync.Mutex{}
				userMsg[oid] = ele
			}
			usrMsg := userMsg[oid]
			mut := usrMsg.mu
			mut.Lock()
			messges := userMsg[oid].msg
			messges = append(messges, Msg{msg: string(msgs), msgType: msgType})
			usrMsg.msg = messges
			mut.Unlock()
			userMsg[oid] = usrMsg
		}
	}
	delete(userCons, r.URL.Query().Get("in_id"))
	log.Printf("user %v deleted", r.URL.Query().Get("in_id"))
}
func WaitQueueCron(key string, ele UserMsg) {
	ele.mu.Lock()
	for _, msg := range ele.msg {
		if err := userCons[key].WriteMessage(msg.msgType, []byte(msg.msg)); err != nil {
			return
		}
	}
	ele.mu.Unlock()
	delete(userMsg, key)

}
