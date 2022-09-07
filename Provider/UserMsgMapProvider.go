package Provider

import (
	"Websockets/Models"
	"github.com/gorilla/websocket"
)

type UserMsgMapProvider interface {
	WaitQueueCron(key string, userCons map[string]*websocket.Conn)
	GetMsgMap(key string) (Models.UserMsg, bool)
	InsertMsgMap(key string, msg Models.UserMsg)
}
