package main

import (
	"chatroom/server/model"
	"chatroom/server/utils"
	"crypto/tls"
	"log"
	"net"
	"syscall"
	"time"
)

func initUserDao() {
	model.MyUserDao = model.NewUserDao(pl)
}

func main() {
	initPool("localhost:6379", 16, 0, 300*time.Second)
	initUserDao()

	cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		log.Fatal("加载服务器的证书和私钥失败：", err)
	}
	//创建TLS配置
	config := &tls.Config{Certificates: []tls.Certificate{cert}}

	listener, err := net.Listen("tcp", "0.0.0.0:8889")
	if err != nil {
		utils.Log("net.Listen err = %v", err)
		return
	}
	defer listener.Close()

	utils.Log("服务器在8889端口监听......")
	listenFD, err := listener.(*net.TCPListener).File()
	if err != nil {
		utils.Log("获取tlslistener的文件描述符失败： %v", err)
		return
	}

	epfd, err := syscall.EpollCreate(1)
	if err != nil {
		utils.Log("创建epoll实例失败： %v", err)
		return
	}
	defer syscall.Close(epfd)

	if err := syscall.SetNonblock(int(listenFD.Fd()), true); err != nil {
		utils.Log("设置监听套接字为非阻塞失败%v", err)
		return
	}

	ev := syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(listenFD.Fd()),
	}

	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, int(listenFD.Fd()), &ev); err != nil {
		utils.Log("注册epoll监听失败%v", err)
		return
	}

	events := make([]syscall.EpollEvent, 10)
	for {
		n, err := syscall.EpollWait(epfd, events, -1)
		if err != nil {
			utils.Log("epoll等待事件失败： %v", err)
			return
		}
		for i := 0; i < n; i++ {
			if events[i].Fd == int32(listenFD.Fd()) {
				conn, err := listener.Accept()
				if err != nil {
					utils.Log("接受连接失败： %v", err)
					continue
				}

				connFD, err := conn.(*net.TCPConn).File()
				if err != nil {
					utils.Log("获取连接文件描述符失败： %v", err)
					continue
				}
				if err := syscall.SetNonblock(int(connFD.Fd()), true); err != nil {
					utils.Log("设置连接为非阻塞失败失败： %v", err)
					continue
				}

				ev := syscall.EpollEvent{
					Events: uint32(syscall.EPOLLIN),
					Fd:     int32(connFD.Fd()),
				}
				if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, int(connFD.Fd()), &ev); err != nil {
					utils.Log("将连接注册到epoll失败： %v", err)
					continue
				}

				//将net.Conn升级为tls.Conn,客户端需要执行tls.Client
				tlsConn := tls.Server(conn, config)
				processor := &Processor{
					Conn:          tlsConn,
					RemoteAddress: tlsConn.RemoteAddr().String(),
				}
				fileDescriberToProcessor[int32(connFD.Fd())] = processor
			} else {
				connFD := events[i].Fd

				processor, exists := fileDescriberToProcessor[connFD]
				if !exists {
					utils.Log("接收到了发送给已关闭连接的消息")
					continue
				}
				err = processor.handleMesFromClient()
				if err == utils.ERROR_CLIENT_DISCONNECTED {
					syscall.Close(int(connFD))
					processor.Conn.Close()
					delete(fileDescriberToProcessor, connFD)
				} else if err != nil {
					utils.Log("处理客户端%s发送的信息失败：%v", processor.RemoteAddress, err)
				}
			}
		}
	}

}
