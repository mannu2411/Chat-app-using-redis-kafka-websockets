package main

import (
	"Websockets/Provider"
	"Websockets/Repositories"
	"context"
	"github.com/go-chi/chi"
	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	"github.com/segmentio/kafka-go"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Server struct {
	userCons    map[string]*websocket.Conn
	userMsgMap  Provider.UserMsgMapProvider
	ctx         context.Context
	reader      *kafka.Reader
	writer      *kafka.Writer
	mut         sync.Mutex
	RedisClient *redis.Client
}

const (
	topic          = "my-kafka-topic"
	broker1Address = "localhost:9093"
)

func InitService() *Server {
	userMsgMapProvider := Repositories.NewUserMsgMapProvider()
	return &Server{
		userCons:   make(map[string]*websocket.Conn),
		userMsgMap: userMsgMapProvider,
		ctx:        context.Background(),
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{broker1Address},
			Topic:   topic,
		}),
		writer: &kafka.Writer{
			Addr:     kafka.TCP(broker1Address),
			Topic:    topic,
			Balancer: &kafka.Murmur2Balancer{},
		},
		mut: sync.Mutex{},
		RedisClient: redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		}),
	}
}
func main() {
	srv := InitService()
	srv.reader.SetOffset(kafka.LastOffset)
	router := chi.NewRouter()
	router.Route("/", func(ws chi.Router) {
		ws.Get("/", srv.alive)
	})
	router.Route("/ws", func(ws chi.Router) {
		ws.Get("/read", srv.ReadMsg)
	})

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), router))
}

var upgrades = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (srv *Server) alive(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("<h4>Welcome to chatting app</h4>"))
	if err != nil {
		return
	}
	log.Printf("Welcome to chatting app")
	return
}

func (srv *Server) ReadMsg(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrades.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	inID := r.URL.Query().Get("in_id")
	oID := r.URL.Query().Get("out_id")
	srv.userCons[inID] = conn
	srv.RedisClient.Set(inID, "true", time.Hour*30)
	val, err := srv.RedisClient.Get(inID + "msg").Result()
	if err == nil {
		go srv.SendOldMsg(inID, val)
	}
	go func() {
		for {
			msg, err := srv.reader.ReadMessage(srv.ctx)
			if err != nil {
				panic("could not read message " + err.Error())
			}
			str := string(msg.Value)
			config := strings.Split(str, "|")
			srv.mut.Lock()
			if conn, ok := srv.userCons[config[0]]; ok {
				if err := conn.WriteMessage(1, []byte(config[1])); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			srv.mut.Unlock()
		}
	}()
	for {
		_, body, err := conn.ReadMessage()
		if err != nil {
			break
		}
		_, err = srv.RedisClient.Get(oID).Result()
		if err != nil {
			val, err := srv.RedisClient.Get(oID + "msg").Result()
			if err != nil {
				val = string(body)
			} else {
				val = val + "|" + string(body)
			}
			srv.RedisClient.Set(oID+"msg", val, time.Hour*30)
			log.Printf("Succes: Msg written in redis for user: %v and msg: %v", oID, string(body))
		} else {
			tmp := oID + "|" + string(body)
			msg := kafka.Message{
				Key:   []byte(oID),
				Value: []byte(tmp),
			}

			err = srv.writer.WriteMessages(srv.ctx, msg)
			if err != nil {
				log.Printf("Error: %v", err)
			}
			log.Printf("Succes: Msg written in kafka for user: %v and msg: %v", oID, string(msg.Value))
		}
	}
	delete(srv.userCons, inID)
	srv.RedisClient.Del(inID)
}
func (srv *Server) SendOldMsg(key string, uMsgs string) {
	//uMsgMap.mu.Lock()
	messages := strings.Split(uMsgs, "|")
	for _, msg := range messages {
		if err := srv.userCons[key].WriteMessage(1, []byte(msg)); err != nil {
			return
		}
	}
	srv.RedisClient.Del(key)
}
