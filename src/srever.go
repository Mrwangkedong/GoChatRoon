package main

import (
	"fmt"
	"net"
	"runtime"
)

/*
1.创建服务器地址
2.接收新的Client，HandleConnect()
	2.1 新上线用户存储
	2.2	用户消息的读取，发送
	2.3	用户改名
	2.4 下线处理、超时处理
3.处理Client的消息（接收并发送到公屏上） Manager()
4.监听用户沉默时间，到达一定时间踢出去
*/

/*
map: 存储所有用户的 key--ip+port，value--Client结构体
Client结构体：成员Name，网络地址ip+port，发送消息的通道C(channel)
通道message：协调并发go程将消息的传递（广播）
*/

//用户结构体
type User struct {
	name string      //用户名称
	addr string      //用户ip+port
	C    chan string //用户处理消息的通道
}

//定义全局变量
var user_map = make(map[string]User) //存储client个体信息
var MesChan = make(chan string)      //接收来自client的消息

func main() {
	//1.指定服务器，通信协议，IP地址，port。创建一个用于监听的socket
	myListener, err := net.Listen("tcp", "localhost:8000")
	if err != nil {
		fmt.Println("net Listener err: ", err)
		return
	}
	defer myListener.Close()
	fmt.Println("服务端等待客户端建立连接")

	//创建一个管理者go程，管理map和全局channel
	go Manager()

	//阻塞监听客户端连接请求,获取客户端连接信号.若成功，返回用于通信的socket
	//利用循环多重复监听
	for {
		conn, err := myListener.Accept()
		if err != nil {
			fmt.Println("listener.Accept() err: ", err)
			return
		}
		fmt.Println("服务器与客户端成功建立连接")

		//封装函数，具体完成服务器和客户端的数据通信
		go HandlerConnect(conn)
	}
}

//将要全局发送的消息，放置在各个client各自的Channel中
func Manager() {
	//初始化 map_user
	user_map = make(map[string]User)
	//循环监听...
	for true {
		//监听全局channel中是否有数据,有数据读出来，无数据则阻塞
		msg := <-MesChan
		//循环发送消息
		for _, client := range user_map {

			client.C <- msg

		}
	}

}

func HandlerConnect(conn net.Conn) {
	defer conn.Close()

	//1.将client存在map中,name初始为client的addr
	//要初始化user.C，否则channel无地址穿不进去value
	user := User{name: conn.RemoteAddr().String(), addr: conn.RemoteAddr().String(), C: make(chan string)}
	user_map[conn.RemoteAddr().String()] = user

	//2.创建专门用来给当前用户发送消息的go程
	go WriteMsgToClient(user, conn)

	//3.发送用户上线通道到全局MesChan
	MesChan <- "[" + conn.RemoteAddr().String() + "] 上线啦！！！！"

	//创建一个匿名go程，专门处理用户发送的消息
	go func() {
		buf := make([]byte, 4096)
		for true {
			//从客户端读取信息
			n, err := conn.Read(buf) //返回读到的个数
			if n == 0 {
				fmt.Printf("服务器检测到客户端[%s]已经关闭，断开连接...\n", user.name)
				runtime.Goexit()
			}
			if err != nil {
				fmt.Println("conn.Read(buf) err: ", err)
				return
			}
			//写入全局msg
			MesChan <- user.name + ": " + string(buf[:n])
		}
	}()

	//4.保证不退出
	for true {

	}

}

//利用当前连接，给当前用户发送消息
func WriteMsgToClient(user User, conn net.Conn) {
	//循环去写广播数据给Client
	for true {
		msg := <-user.C //监听用户自带channel是否有数据
		_, err := conn.Write([]byte(msg + "\n"))
		if err != nil {
			fmt.Println("Client ["+user.name+"] conn.Write(buf) err: ", err)
			return
		}
	}
}
