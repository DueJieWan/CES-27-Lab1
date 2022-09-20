package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// Variáveis globais interessantes para o processo
var err string             //
var myPort string          //porta do meu servidor
var nServers int           //qtde de outros processo
var CliConn []*net.UDPConn //vetor com conexões para os servidores
// dos outros processos
var ServConn *net.UDPConn //conexão do meu servidor (onde recebo mensagens dos outros processos)
var logicalClock int      //Relogio logico escalar interno

func CheckError(err error) {
	if err != nil {
		fmt.Println("Erro: ", err)
		os.Exit(0)
	}
}
func PrintError(err error) {
	if err != nil {
		fmt.Println("Erro: ", err)
	}
}

func updateLClock(buf []byte, n int) {
	var i int
	for i = 0; i < n; i++ {
		if buf[i] == ':' {
			break
		}
	}

	otherclock, err := strconv.Atoi(string(buf[0:i]))
	CheckError(err)
	fmt.Println("Printed inside updateLClock", otherclock)
	if otherclock > logicalClock {
		logicalClock = otherclock
	}
	logicalClock++
}

func doServerJob() {

	//Loop infinito mesmo
	for {
		//Ler (uma vez somente) da conexão UDP a mensagem
		//Escrever na tela a msg recebida (indicando o endereço de quem enviou)
		buf := make([]byte, 1024)
		n, addr, err := ServConn.ReadFromUDP(buf)
		updateLClock(buf, n)
		fmt.Println("Received ", string(buf[0:n]), " from ", addr, " at my clock ", logicalClock)
		CheckError(err)
	}
}
func doClientJob(otherProcess int, clock int) {
	//Enviar msg <T,pi> para o servidor do processo otherServer.
	msg := strconv.Itoa(clock) + myPort
	buf := []byte(msg)
	_, err := CliConn[otherProcess].Write(buf)
	CheckError(err)
}

// ESSA FUNÇÃO ESTÁ PRONTA
func initConnections() {
	myPort = os.Args[1]
	nServers = len(os.Args) - 2
	/*Esse 2 tira o nome (no caso Process) e tira a primeira porta (que
	é a minha). As demais portas são dos outros processos*/
	CliConn = make([]*net.UDPConn, nServers)

	/*Outros códigos para deixar ok a conexão do meu servidor (onde re-
	cebo msgs). O processo já deve ficar habilitado a receber msgs.*/

	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)
	/*Outros códigos para deixar ok a minha conexão com cada servidor
	dos outros processos. Colocar tais conexões no vetor CliConn.*/
	for servidores := 0; servidores < nServers; servidores++ {
		ServerAddr, err := net.ResolveUDPAddr("udp",
			"127.0.0.1"+os.Args[2+servidores])
		CheckError(err)
		Conn, err := net.DialUDP("udp", nil, ServerAddr)
		CliConn[servidores] = Conn
		CheckError(err)
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
	initConnections()
	//O fechamento de conexões deve ficar aqui, assim só fecha conexão quando a main morrer
	defer ServConn.Close()
	for i := 0; i < nServers; i++ {
		defer CliConn[i].Close()
	}

	ch := make(chan string) //canal que guarda itens lidos do teclado
	go readInput(ch)        //chamar rotina que ”escuta” o teclado
	go doServerJob()
	for {
		// Verificar (de forma não bloqueante) se tem algo no stdin (input do terminal)
		select {
		case x, valid := <-ch:
			if valid {
				fmt.Printf("Recebi do teclado: %s \n", x)
				fmt.Printf("Meu relogio diz %d \n", logicalClock)
				if x != "1" {
					var msgVect [2]string
					msgVect[0] = x
					msgVect[1] = strconv.Itoa(logicalClock)
					for j := 0; j < nServers; j++ {
						go doClientJob(j, logicalClock)
					}
				}
			} else {
				fmt.Println("Canal fechado!")
			}
		default:
			// Fazer nada!
			// Mas não fica bloqueado esperando o teclado
			time.Sleep(time.Second * 1)
		}
		// Esperar um pouco
		time.Sleep(time.Second * 1)

	}

}
