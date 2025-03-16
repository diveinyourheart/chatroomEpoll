package main

import (
	"chatroom/server/utils"
	"sync"

	"golang.org/x/sys/unix"
)

const (
	MAX_THREADS_NUMBER = 10
	MAX_TASKS_NUMBER   = 10
)

var (
	wg    sync.WaitGroup
	tasks chan int32
)

func processConn(connFD int32) {
	processor, exists := fileDescriberToProcessor[connFD]
	if !exists {
		utils.Log("接收到了发送给已关闭连接的消息")
		return
	}
	err := processor.handleMesFromClient()
	if err == utils.ERROR_CLIENT_DISCONNECTED {
		unix.Close(int(connFD))
		processor.Conn.Close()
		delete(fileDescriberToProcessor, connFD)
	} else if err != nil {
		utils.Log("处理客户端%s发送的信息失败：%v", processor.RemoteAddress, err)
	}
}

func worker(id int, tasks <-chan int32, wg *sync.WaitGroup, execFun interface{}) {
	defer wg.Done()
	execF, ok := execFun.(func(int32))
	if !ok {
		utils.Log("传入了非期望的执行函数类型")
		return
	}
	for task := range tasks {
		execF(task)
	}
}
