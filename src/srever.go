package main

import (
	"fmt"
	"net"
	"runtime"
	"time"
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
		//循环向在线所有用户发送消息
		for _, client := range user_map {
			client.C <- msg
		}
	}

}

//处理与客户端的连接需求
func HandlerConnect(conn net.Conn) {
	defer conn.Close()
	exitFlag := make(chan int) //exitFlag 有值就退出
	talkFlag := make(chan int) //talkFlag 监听是否在活跃

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
			//处理不同指令的消息
			msg := HandleMsgSort(string(buf[:n-1]), &user, exitFlag, talkFlag, conn)
			//写入全局msg,如果等于空值，则表明用户要退出，提前已经向全局channel中注入消息
			if msg != "" {
				MesChan <- msg //发出当前聊天信息
			}
		}
	}()

	//4.保证不退出
	for true {
		select {
		case <-talkFlag:
			fmt.Printf("")
		case <-exitFlag:
			user.C <- "exit" //关闭子go程
			runtime.Goexit() //结束当前进程   //会导致子go程还在运行
			//return     //会导致子go程还在运行
		case <-time.After(time.Second * 10):
			user.C <- "exit"                         //关闭子go程
			delete(user_map, user.addr)              //更新当前在线列表
			MesChan <- "[" + user.name + "] Exit..." //通知大厅有人退出
			runtime.Goexit()                         //不活跃超过10秒，退出
		}
	}

}

/***
s:消息
client：当前客户端
*flag：当前客户端状态信息地址
*/
func HandleMsgSort(s string, client *User, exitFlag chan int, talkFlag chan int, conn net.Conn) string {
	//处理查询所有在线用户
	//查询在线用户
	if s == "who" {
		clients := "user list: \n\n"
		for _, client := range user_map {
			clients += "\t[" + client.name + "] \n "
		}
		_, _ = conn.Write([]byte(clients + "\n"))
		//向活跃channel中注入
		talkFlag <- 1
		return ""
	} else if s == "exit" { //处理退出
		//发布用户退出消息
		MesChan <- "[" + client.name + "] Exit..."
		//更新map
		delete(user_map, client.addr)
		//向退出channel中注入
		exitFlag <- 1
		//返回空值
		return ""
	} else if len(s) > 7 && s[:7] == "rename|" { //改名
		preName := client.name
		client.name = s[7:] //提取后面的名称，进行改名
		user_map[client.addr] = *client
		_, _ = conn.Write([]byte("[" + preName + "]Rename[" + client.name + "]\n"))
		//向活跃channel中注入
		talkFlag <- 1
		return "" //改名后不进行提示
	} else {
		//处理向大厅中发送的消息
		//向活跃channel中注入
		talkFlag <- 1
		return "[" + client.name + "]: " + s
	}

}

//利用当前连接，给当前用户发送消息
func WriteMsgToClient(user User, conn net.Conn) {
	//循环去写广播数据给Client
	for {
		msg := <-user.C //监听用户自带channel是否有数据
		if msg == "exit" {
			runtime.Goexit()
		}
		_, err := conn.Write([]byte(msg + "\n"))
		if err != nil {
			fmt.Println("Client ["+user.name+"] conn.Write(buf) err: ", err)
			return
		}
	}
}
