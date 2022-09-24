package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
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
var id int                 // id do processo
var err string             // Error string
var myPort string          // porta do meu servidor
var nServers int           // qtde de outros processo
var CliConn []*net.UDPConn // vetor com conexões para os servidores dos outros processos
var ServConn *net.UDPConn  // conexão do meu servidor (onde recebo mensagens dos outros processos)
var replyQueue []int       // Reply queue
var state State            // Estado da requisicao do processo so eh atualizado em 1 goroutine
var T int                  // Time when this process requests CS so eh atualizado em 1 goroutine
var counter int            // To count the time spent in CS so eh atualizado em 1 goroutine
var CStimeuse int          // time spent in CS

// SafeCounter is safe to use concurrently.
type SafeCounter struct {
	mu           sync.Mutex
	logicalClock int // Relogio logico escalar interno
	numOfReply   int // Numero de reply recebidos
}

var c SafeCounter

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
	// This function parces the received msg from other ports into their logicalClock, their port string and either they REPLIED or REQUESTED
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
	// Updates safely the logical clock of this process
	c.mu.Lock()
	if otherclock > c.logicalClock {
		c.logicalClock = otherclock
	}
	c.logicalClock++
	c.mu.Unlock()
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
	// Check and process received message
	otherClock, otherPort, R := parceMsg(buf, n)
	updateLClock(otherClock)

	CliConnMAP := make(map[string]int) // Mapping port name to correct index in CliConn array so eh utilizado nesta funcao
	CliConnMAP = CliConnMapper()

	switch R {
	case "REPLY":
		fmt.Println("Time", c.logicalClock, ":", otherPort, ", I got your REPLY!")
		c.numOfReply++
	case "REQUEST":
		fmt.Println("Time", c.logicalClock, ":", otherPort, "do you want my reply?")
		if (state == HELD) || (state == WANTED && compareLClock(otherClock, otherPort)) {
			fmt.Println("Time", c.logicalClock, ":", otherPort, "you can't have my reply.")
			replyQueue = append(replyQueue, CliConnMAP[otherPort])
		} else {
			fmt.Println("Time", c.logicalClock, ":", otherPort, "! You have my REPLY!")
			go doClientJob(CliConnMAP[otherPort], c.logicalClock, "REPLY")
		}
	default:
		fmt.Println("Received: ", R)
		fmt.Println("Wrong protocol.")
	}
}

func checkNumReply() {
	// Checa se recebeu todos os reply
	for {
		c.mu.Lock() // Trava numOfReply
		if c.numOfReply == nServers {
			if counter == 0 {
				fmt.Println("Time", c.logicalClock, ":Now I, ID:", id, ", have CS for 5 sec.")
				useCS()
			} else if counter < CStimeuse {
				counter++
			} else {
				state = RELEASED
				fmt.Println("Time", c.logicalClock, ":Guys, I no longer need CS!")
				c.numOfReply = 0
				counter = 0
				multiCastReply(replyQueue)
			}
		}
		c.mu.Unlock() // Libera numOfReply
		time.Sleep(time.Second * 1)
	}

}

func multiCastRequest() {
	// Send request to everybody else
	c.mu.Lock()
	c.logicalClock++
	c.mu.Unlock()
	fmt.Println("Time", c.logicalClock, ":Guys, I need CS!")
	for j := 0; j < nServers; j++ {
		go doClientJob(j, c.logicalClock, "REQUEST")
	}
}

func multiCastReply(replyQueue []int) {
	// Send reply to everbody left in queue
	for len(replyQueue) > 0 {
		go doClientJob(replyQueue[0], c.logicalClock, "REPLY")
		replyQueue = replyQueue[1:]
	}
}

func doServerJob() {
	//Loop infinito mesmo
	for {
		//Ler (uma vez somente) da conexão UDP a mensagem
		buf := make([]byte, 1024)
		n, _, err := ServConn.ReadFromUDP(buf)
		checkMsg(buf, n) //Checa a mensagem recebida
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
		// This comment is necessary because windows defender is acting up
		CliConnMAP[os.Args[i+3][1:len(os.Args[i+3])]] = i
	}
	return CliConnMAP
}

func useCS() {
	// Enviar mensagem para CS
	CsConn := make([]*net.UDPConn, 1)
	CsAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+":10001")
	CheckError(err)
	Conn, err := net.DialUDP("udp", nil, CsAddr)
	CsConn[0] = Conn
	CheckError(err)

	msg := "ID:" + strconv.Itoa(id) + " clock:" + strconv.Itoa(c.logicalClock) + " Oi."
	buf := []byte(msg)
	_, err = CsConn[0].Write(buf)
	CheckError(err)
	counter++
	c.logicalClock++
}

// ESSA FUNÇÃO ESTÁ PRONTA
func initConnections() {
	id, _ = strconv.Atoi(os.Args[1])
	myPort = os.Args[2]
	nServers = len(os.Args) - 3
	CStimeuse = 5
	c.logicalClock = 1
	// Entrada tem padrao $ Process id :10002 :10003 :10004
	/*Esse 3 tira o nome (no caso Process), tira id e tira a primeira porta (que
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
			"127.0.0.1"+os.Args[3+servidores])
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
	fmt.Println("To check state type: STA")
	state = RELEASED

	ch := make(chan string) //canal que guarda itens lidos do teclado
	go readInput(ch)        //chamar rotina que ”escuta” o teclado
	go doServerJob()        //chamar rotina que "escuta" das portas
	go checkNumReply()      //chamar rotina que checa o numero de reply
	for {
		// Verificar (de forma não bloqueante) se tem algo no stdin (input do terminal)
		select {
		case x, valid := <-ch:
			if valid {
				if x == "REQ" {
					if state == RELEASED {
						state = WANTED
						multiCastRequest()
						T = c.logicalClock // Time when I requested CS
					} else {
						fmt.Println(x, "ignorado.")
					}
				}
				if x == strconv.Itoa(id) {
					c.logicalClock++
					fmt.Println("Time:", c.logicalClock)
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
					fmt.Println("Time", c.logicalClock, ":My ID(", id, ") current state is", stateName)
				}
			} else {
				fmt.Println("Canal fechado!")
			}
		default:
			// Mas não fica bloqueado esperando o teclado
			time.Sleep(time.Second * 1)
		}
		// Esperar um pouco
		time.Sleep(time.Second * 1)

	}

}
