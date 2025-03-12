package utils

import (
	"chatroom/common/message"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

var (
	ERROR_CLIENT_DISCONNECTED error = errors.New("error:客户端断开连接事件")
)

type Transfer struct {
	Conn net.Conn
	buf  [8096]byte //传输时使用的缓冲
}

func Log(format string, args ...interface{}) {
	fmt.Printf("[%s] ", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf(format, args...)
	fmt.Println()
}

// func caculateCRC(data []byte) uint32 {
// 	return crc32.ChecksumIEEE(data)
// }

// func verifyCRC(buffer []byte, n int) error {
// 	remoteCRC := binary.BigEndian.Uint32(buffer[n-4 : n])
// 	localCRC := caculateCRC(buffer[:n-4])
// 	if remoteCRC != localCRC {
// 		return fmt.Errorf("校验和不一致")
// 	}
// 	return nil
// }

func (tf *Transfer) ReadPkg() (*message.Message, error) {
	n, err := tf.Conn.Read(tf.buf[:4])
	if err != nil {
		if err == io.EOF {
			return nil, ERROR_CLIENT_DISCONNECTED
		} else {
			return nil, fmt.Errorf("读取客户端发送的消息失败：%v", err)
		}
	} else if n != 4 {
		return nil, fmt.Errorf("读取消息长度信息失败")
	}

	pkgLen := binary.BigEndian.Uint32(tf.buf[:4])

	n, err = tf.Conn.Read(tf.buf[:pkgLen])
	if err != nil {
		if err == io.EOF {
			return nil, ERROR_CLIENT_DISCONNECTED
		} else {
			return nil, fmt.Errorf("读取客户端发送的消息失败：%v", err)
		}
	} else if n != int(pkgLen) {
		return nil, fmt.Errorf("读取消息失败")
	}
	mes := &message.Message{}
	err = json.Unmarshal(tf.buf[:int(pkgLen)], mes)
	if err != nil {
		return nil, fmt.Errorf("反序列化失败：%v", err)
	}
	return mes, nil
}

func (tf *Transfer) WritePkg(data []byte) (er error) {
	intPkgLen := len(data)
	pkgLen := uint32(intPkgLen)
	binary.BigEndian.PutUint32(tf.buf[0:4], pkgLen)
	copy(tf.buf[4:4+intPkgLen], data)
	n, err := tf.Conn.Write(tf.buf[0 : 4+intPkgLen])
	if err != nil {
		if err == io.EOF {
			er = ERROR_CLIENT_DISCONNECTED
		} else {
			er = fmt.Errorf("向客户端发送消息失败：%v", err)
		}
	} else if n != 4+intPkgLen {
		er = fmt.Errorf("net.Conn.Write返回的长度与写入网络的字节流的长度不符合")
	}
	return
}
