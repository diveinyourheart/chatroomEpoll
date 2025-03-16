package main

import (
	"chatroom/server/model"
	"chatroom/server/utils"
	"crypto/tls"
	"log"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

func initUserDao() {
	model.MyUserDao = model.NewUserDao(pl)
}

func main() {
	initPool("localhost:6379", 16, 0, 300*time.Second)
	initUserDao()

	tasks = make(chan int32, MAX_TASKS_NUMBER)

	fileDescriberToProcessor = make(map[int32]*Processor, 1024)

	for i := 0; i < MAX_THREADS_NUMBER; i++ {
		wg.Add(1)
		go worker(i+1, tasks, &wg, processConn)
	}
	defer wg.Wait()
	defer close(tasks)

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

	epfd, err := unix.EpollCreate(1)
	if err != nil {
		utils.Log("创建epoll实例失败： %v", err)
		return
	}
	defer unix.Close(epfd)

	if err := unix.SetNonblock(int(listenFD.Fd()), true); err != nil {
		utils.Log("设置监听套接字为非阻塞失败%v", err)
		return
	}

	ev := unix.EpollEvent{
		Events: unix.EPOLLIN | unix.EPOLLET,
		Fd:     int32(listenFD.Fd()),
	}

	if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, int(listenFD.Fd()), &ev); err != nil {
		utils.Log("注册epoll监听失败%v", err)
		return
	}

	count := 0

	events := make([]unix.EpollEvent, 20)
	for {
		n, err := unix.EpollWait(epfd, events, -1)
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
				if err := unix.SetNonblock(int(connFD.Fd()), true); err != nil {
					utils.Log("设置连接为非阻塞失败失败： %v", err)
					continue
				}

				ev := unix.EpollEvent{
					Events: unix.EPOLLIN | unix.EPOLLET,
					Fd:     int32(connFD.Fd()),
				}
				if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, int(connFD.Fd()), &ev); err != nil {
					utils.Log("将连接注册到epoll失败： %v", err)
					continue
				}

				//将net.Conn升级为tls.Conn,客户端需要执行tls.Client
				tlsConn := tls.Server(conn, config)
				processor := &Processor{
					Conn:          tlsConn,
					RemoteAddress: tlsConn.RemoteAddr().String(),
				}
				err = tlsConn.Handshake()
				if err != nil {
					utils.Log("与客户端TLS握手失败：%v", err)
				}
				fileDescriberToProcessor[int32(connFD.Fd())] = processor
			} else {
				connFD := events[i].Fd

				tasks <- connFD
			}
		}
		count++
	}
}
