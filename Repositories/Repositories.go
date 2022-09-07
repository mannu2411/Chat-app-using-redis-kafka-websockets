package Repositories

import (
	"Websockets/Models"
	"Websockets/Provider"
	"github.com/gorilla/websocket"
	"sync"
)

type UserMsgMap struct {
	mu   sync.Mutex
	uMsg map[string]Models.UserMsg
}

func NewUserMsgMapProvider() Provider.UserMsgMapProvider {
	return &UserMsgMap{
		mu:   sync.Mutex{},
		uMsg: make(map[string]Models.UserMsg),
	}
}
func (uMsgMap *UserMsgMap) GetMsgMap(key string) (Models.UserMsg, bool) {
	msgs, ok := uMsgMap.uMsg[key]
	return msgs, ok
}
func (uMsgMap *UserMsgMap) InsertMsgMap(key string, msg Models.UserMsg) {
	uMsgMap.mu.Lock()
	uMsgMap.uMsg[key] = msg
	uMsgMap.mu.Unlock()
	return
}
func (uMsgMap *UserMsgMap) WaitQueueCron(key string, userCons map[string]*websocket.Conn) {
	uMsgMap.mu.Lock()
	messages := uMsgMap.uMsg[key].Msg
	for _, msg := range messages {
		if err := userCons[key].WriteMessage(msg.MsgType, []byte(msg.Msg)); err != nil {
			return
		}
	}
	uMsgMap.mu.Unlock()
	delete(uMsgMap.uMsg, key)
}
