package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func CheckError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func readInput(ch chan string) {
	//Rotina que "escuta" o stdin
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _, _ := reader.ReadLine()
		ch <- string(text)
	}
	println(ch)
}

func main() {
	Address, err := net.ResolveUDPAddr("udp", "127.0.0.1"+":10001")
	CheckError(err)
	Connection, err := net.ListenUDP("udp", Address)
	CheckError(err)
	defer Connection.Close()
	buf := make([]byte, 1024)

	for {
		n, _, err := Connection.ReadFromUDP(buf)
		fmt.Println("Received from " + string(buf[0:n]))

		if err != nil {
			fmt.Println("Error: ", err)
		}
	}
}
