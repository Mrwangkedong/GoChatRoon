package main

import (
	"fmt"
	"runtime"
	"time"
)

func aaa() {
	for true {
		fmt.Println("aaaaa")
		time.Sleep(1 * time.Second)
	}
}

func main() {
	go func() {
		fmt.Println("-------1")
		go aaa()
		fmt.Println("-------2")
		runtime.Goexit()
	}()

	for true {

	}
}
