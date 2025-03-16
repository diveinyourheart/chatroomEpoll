package main

import (
	"chatroom/common/message"
	"chatroom/server/processes"
	"chatroom/server/utils"
	"fmt"
	"net"
)

type Processor struct {
	Conn          net.Conn
	UsrId         int
	RemoteAddress string
}

var (
	fileDescriberToProcessor map[int32]*Processor
)

// 编写一个ServerProcessMes函数
// 功能：根据客户端发送消息的种类不同，决定调用哪个函数来处理
func (p *Processor) serverProcessMes(mes *message.Message) (err error) {
	switch mes.Type {
	case message.LoginMesType:
		//处理登录
		userP := &processes.UserProcess{
			Conn: p.Conn,
		}
		err = userP.ServerProcessLogin(mes)
		if err == nil {
			p.UsrId = userP.UsrId
			userP.NotifyOtherOnlineFriends(message.UserOnline)
			err = userP.PushOfflineUserMessages()
			if err != nil {
				utils.Log("推送离线消息错误：%v", err)
			}
		} else {
			utils.Log("为用户登录错误:%v", err)
		}
	case message.RegisterMesType:
		userP := &processes.UserProcess{
			Conn: p.Conn,
		}
		err = userP.ServerProcessRegister(mes)
	default:
		up, er := processes.UsrMgr.GetOnlineUserByID(p.UsrId)
		if p.UsrId == 0 || er != nil {
			utils.Log("出现意料之外的错误:%v,消息类型：%s", er, mes.Type)
			err = er
			return
		}
		switch mes.Type {
		case message.GroupChatMesType:
			smsPrcs, _ := processes.UsrMgr.GetOnlineUserSmsPrcsByUserPrcs(up)
			err = smsPrcs.ForwardGroupChatMes(mes)
		case message.OneToOneMesType:
			err = up.SendOneToOneMes(mes)
		case message.AddFriendResMesType:
			err = up.ForwardAddFriendResMes(mes)
		case message.AddFriendMesType:
			err = up.ForwardAddFriendRequestMes(mes)
		case message.GroupManageMesType:
			err = up.ProcessGroupChatManageMes(mes)
		//群管理员处理用户的群聊加入申请，然后转发给服务器，服务器根据处理结果做后续操作
		case message.GroupManageResMesType:
			err = up.ProcessGroupChatManageResMes(mes)
		default:
			err = fmt.Errorf("出现未知的消息类型，无法处理：%s", mes.Type)
		}
	}
	return
}

func (p *Processor) handleMesFromClient() error {
	tf := utils.Transfer{
		Conn: p.Conn,
	}
	mes, err := tf.ReadPkg()
	if err != nil {
		if err == utils.ERROR_CLIENT_DISCONNECTED {
			utils.Log("判定客户端%s断开连接，执行用户登出逻辑", p.RemoteAddress)
			if p.UsrId != 0 {
				up, err := processes.UsrMgr.GetOnlineUserByID(p.UsrId)
				if err == nil {
					if p.RemoteAddress != up.Conn.RemoteAddr().String() {
						utils.Log("%d用户所在的端口%s发生改变，processor无效", p.UsrId, up.Conn.RemoteAddr().String())
						return nil
					}
					up.HandleForOffline()
				}
			}
		}
		return err
	}
	return p.serverProcessMes(mes)
}
