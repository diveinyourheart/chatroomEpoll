package model

import (
	"chatroom/common/message"
	"crypto/tls"
)

var (
	CurUsr CurUser
)

type CurUser struct {
	Conn *tls.Conn
	Usr  message.User
}
