package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// states - Custom type to hold value for Released, Wanted and Held
type State int

const (
	RELEASED State = iota // EnumIndex = 0
	WANTED                // EnumIndex = 1
	HELD                  // EnumIndex = 2
)

// Variáveis globais interessantes para o processo
var err string             // Error string
var myPort string          // porta do meu servidor
var nServers int           // qtde de outros processo
var CliConn []*net.UDPConn // vetor com conexões para os servidores dos outros processos
var ServConn *net.UDPConn  // conexão do meu servidor (onde recebo mensagens dos outros processos)
var replyQueue []int       // Reply queue
var logicalClock int       // Relogio logico escalar interno
var state State            // Estado da requisicao do processo
var numOfReply int         // Numero de reply recebidos
var T int                  // Time when this process requests CS

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

func parceMsg(buf []byte, n int) (int, string, string) {
	var i int
	for i = 0; i < n; i++ {
		if buf[i] == ':' {
			break
		}
	}
	var j int
	for j = n - 1; j >= 0; j-- {
		if buf[j] == ':' {
			break
		}
	}
	otherclock, err := strconv.Atoi(string(buf[0:i]))
	CheckError(err)
	port := string(buf[i+1 : j])
	R := string(buf[j+1 : n])
	return otherclock, port, R
}

func updateLClock(otherclock int) {
	if otherclock > logicalClock {
		logicalClock = otherclock
	}
	logicalClock++
}

func compareLClock(otherClock int, otherPort string) bool {
	myPortINT, err := strconv.Atoi(myPort[1:])
	CheckError(err)
	otherPortINT, err := strconv.Atoi(otherPort)
	CheckError(err)
	if T < otherClock {
		return true
	} else if T == otherClock && myPortINT <= otherPortINT {
		return true
	}
	return false
}

func checkMsg(buf []byte, n int) {
	otherClock, otherPort, R := parceMsg(buf, n)
	updateLClock(otherClock)

	CliConnMAP := make(map[string]int) //
	CliConnMAP = CliConnMapper()

	switch R {
	case "REPLY":
		fmt.Println("Time", logicalClock, ":", otherPort, ", I got your REPLY!")
		numOfReply++
	case "REQUEST":
		fmt.Println("Time", logicalClock, ":", otherPort, "do you want my reply?")
		if (state == HELD) || (state == WANTED && compareLClock(otherClock, otherPort)) {
			fmt.Println("Time", logicalClock, ":", otherPort, "you can't have my reply.")
			replyQueue = append(replyQueue, CliConnMAP[otherPort])
		} else {
			fmt.Println("Time", logicalClock, ":", otherPort, "! You have my REPLY!")
			go doClientJob(CliConnMAP[otherPort], logicalClock, "REPLY")
		}
	default:
		fmt.Println("Received: ", R)
		fmt.Println("Wrong protocol.")

	}
}

func checkNumReply() {
	if numOfReply == nServers {
		fmt.Println("Time", logicalClock, ":Now I, ID:", myPort[1:], ", have CS.")
		state = HELD
	}
}

func multiCastRequest() {
	fmt.Println("Time", logicalClock, ":Guys, I need CS!")
	for j := 0; j < nServers; j++ {
		go doClientJob(j, logicalClock, "REQUEST")
	}
}

func multiCastReply(replyQueue []int) {
	for len(replyQueue) > 0 {
		go doClientJob(replyQueue[0], logicalClock, "REPLY")
		replyQueue = replyQueue[1:]
	}
}

func doServerJob() {

	//Loop infinito mesmo
	for {
		//Ler (uma vez somente) da conexão UDP a mensagem
		//Escrever na tela a msg recebida (indicando o endereço de quem enviou)
		buf := make([]byte, 1024)
		n, _, err := ServConn.ReadFromUDP(buf)
		checkMsg(buf, n)
		// fmt.Println("Received ", string(buf[0:n]), " from ", addr, " at my clock ", logicalClock)
		CheckError(err)
	}
}
func doClientJob(otherProcess int, clock int, R string) {
	//Enviar msg <T,pi> para o servidor do processo otherServer.
	msg := strconv.Itoa(clock) + myPort + ":" + R
	buf := []byte(msg)
	_, err := CliConn[otherProcess].Write(buf)
	CheckError(err)
}

func CliConnMapper() map[string]int {
	CliConnMAP := make(map[string]int) //Mapa que associa o nome dos outros processos com index do vetor CliConn
	for i := 0; i < nServers; i++ {
		CliConnMAP[os.Args[i+2][1:len(os.Args[i+2])]] = i
	}
	return CliConnMAP
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

	fmt.Println("To request critical section type: REQ")
	fmt.Println("To exit critical section type: REL")
	fmt.Println("To check state type: STA")
	state = RELEASED

	ch := make(chan string) //canal que guarda itens lidos do teclado
	go readInput(ch)        //chamar rotina que ”escuta” o teclado
	go doServerJob()
	for {
		// Verificar (de forma não bloqueante) se tem algo no stdin (input do terminal)
		select {
		case x, valid := <-ch:
			if valid {
				if x == "REQ" {
					state = WANTED
					multiCastRequest()
					T = logicalClock // Time when I requested CS
				}
				if x == "REL" {
					state = RELEASED
					fmt.Println("Time", logicalClock, ":Guys, I no longer need CS!")
					numOfReply = 0
					multiCastReply(replyQueue)
				}
				if x == "STA" {
					var stateName string
					switch state {
					case RELEASED:
						stateName = "RELEASED"
					case WANTED:
						stateName = "WANTED"
					case HELD:
						stateName = "HELD"
					}
					fmt.Println("Time", logicalClock, ":My ID(", myPort[1:], ") current state is", stateName)
				}
			} else {
				fmt.Println("Canal fechado!")
			}
		default:
			// Mas não fica bloqueado esperando o teclado
			checkNumReply() // Checa se recebeu todos os REPLY
			time.Sleep(time.Second * 1)
		}
		// Esperar um pouco
		time.Sleep(time.Second * 1)

	}

}
