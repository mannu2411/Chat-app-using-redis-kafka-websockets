package Models

type Msg struct {
	Msg     string
	MsgType int
}
type UserMsg struct {
	Msg []Msg
}
