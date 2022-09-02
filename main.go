package main

import (
	"fmt"
	"log"
	"net/http"
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
	s.Every(30).Second().Do(func() {
		//log.Printf("Running Cron Job..")
		WaitQueueCron()
	})

	s.StartBlocking()
}

func main() {
	router := chi.NewRouter()
	go runCronJobs()
	userConns = make(map[string]*websocket.Conn)
	userMsgs = make(map[string]UserMsgs)
	router.Route("/ws", func(ws chi.Router) {
		ws.Get("/alive", alive)
		ws.Get("/read", ReadMsg)
	})
	httpPort := "8081"
	log.Fatal(http.ListenAndServe(":"+httpPort, router))
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func alive(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Here It is"))
	log.Printf("here it is")
	return
}
func ReadMsg(w http.ResponseWriter, r *http.Request) {

	log.Printf("we go")
	conn, err := upgrader.Upgrade(w, r, nil) // error ignored for sake of simplicity
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	userConns[r.URL.Query().Get("in_id")] = conn
	go func() {
		for {
			msgType, msgs, err := userConns[r.URL.Query().Get("in_id")].ReadMessage()
			if err != nil {
				return
			}
			//	sEnc := b64.StdEncoding.EncodeToString(msg)

			//	fmt.Printf("%s Recieved: %s Endoded Message: %s \n", userConns[r.URL.Query().Get("in_id")].RemoteAddr(), string(msg), sEnc)
			oid := r.URL.Query().Get("out_id")
			out, ok := userConns[oid]
			if ok {
				if len(userMsgs[oid].msgs) > 0 {
					mut := userMsgs[oid].mu
					mut.Lock()
					for _, msg := range userMsgs[oid].msgs {
						//	sEnc := b64.StdEncoding.EncodeToString([]byte(msg.msg))
						if err := out.WriteMessage(msg.msgType, []byte(msg.msg)); err != nil {
							return
						}
						//		fmt.Printf("%s Send: %s Endoded Message: %s \n", out.RemoteAddr(), msg.msg, sEnc)

					}
					mut.Unlock()
					delete(userMsgs, r.URL.Query().Get("out_id"))

				}
				if err := out.WriteMessage(msgType, msgs); err != nil {
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
	}()
}
func WaitQueueCron() {
	for key, ele := range userMsgs {
		outUser, ok := userConns[key]
		fmt.Printf("%v \n", key)
		if ok {
			ele.mu.Lock()
			for _, msg := range ele.msgs {
				//	sEnc := b64.StdEncoding.EncodeToString([]byte(msg.msg))
				if err := outUser.WriteMessage(msg.msgType, []byte(msg.msg)); err != nil {
					return
				}
				//	fmt.Printf("%s Send: %s Endoded Message: %s \n", outUser.RemoteAddr(), msg.msg, sEnc)

			}
			ele.mu.Unlock()
			delete(userMsgs, key)
		}
	}
}
