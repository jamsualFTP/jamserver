package server

import (
	"fmt"
	"io"
	"jamserver/pkg/utils"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// NOTE: https://www.rfc-editor.org/rfc/rfc959
// TODO : implement auth system;
//        add dns (optional)

var (
	connectionCounter int
	activeConnections = make(map[int]*net.TCPConn)
	mu                sync.Mutex // mutex for  handle concurrent connections
)

func Run() error {
	// address := "0.0.0.0:2121"
	address := "127.0.0.1:2121"

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return fmt.Errorf("resolving address error %w", err)
	}
	listener, listenErr := net.ListenTCP("tcp", tcpAddr)
	if listenErr != nil {
		return fmt.Errorf("listening error %w", listenErr)
	}
	defer listener.Close()

	fmt.Println("jamsualFTP started!")
	fmt.Printf("Listening on %v at port %v \n", tcpAddr.IP, tcpAddr.Port)

	for {
		fmt.Print("Waiting for upcoming connections... \n\n")
		conn, acceptErr := listener.AcceptTCP()
		if acceptErr != nil {
			log.Printf("connection error %v", acceptErr)
			continue
		}

		mu.Lock()
		connectionCounter++
		id := connectionCounter
		activeConnections[id] = conn
		mu.Unlock()

		addr := conn.RemoteAddr().String()
		fmt.Printf("Accepted new connection: id = %v! %v \n\n", id, addr)
		go HandleConnection(conn, id)
	}
}

func HandleDisconnect(conn *net.TCPConn, id int) {
	mu.Lock()
	defer mu.Unlock()

	if err := conn.Close(); err != nil {
		fmt.Printf("Error closing connection %v: %v\n", id, err)
	}

	delete(activeConnections, id)
	fmt.Printf("Connection %v closed and removed from active list\n", id)
}

func HandleConnection(conn *net.TCPConn, id int) {
	quitChan := make(chan bool)
	defer HandleDisconnect(conn, id)

	time.Sleep(time.Second)
	fmt.Fprintf(conn, "\033[36m220 \033[0mWelcome to jamsualFTP server, user %v! \n\n", id)

	for {
		select {
		case <-quitChan:
			fmt.Println("SADASD")
			return
		default:
			buffer := make([]byte, 1024) // request buffer
			n, err := conn.Read(buffer)

			if err == io.EOF {
				return
			}

			if err != nil {
				fmt.Printf("Error reading from connection %v: %v\n", id, err)
				return
			}

			str := strings.TrimSpace(string(buffer[:n]))
			part := strings.Split(str, " ")
			command := strings.ToUpper(part[0])
			args := part[1:]
			go HandleCommands(conn, command, args, quitChan)
		}
	}
}

// using command pattern for a while, maybe will refactor to COR
func HandleCommands(conn *net.TCPConn, command string, args []string, quitChan chan bool) {
	commands := map[string]func(*net.TCPConn, []string, chan<- bool){
		"ECHO": handleEcho,
		"HLLO": handleHello,
		"RGSR": handleRegister,
		"USER": handleLogin,
		"QUIT": handleQuit,
	}

	if result, ok := commands[command]; ok {
		result(conn, args, quitChan)
	} else {
		conn.Write([]byte("\033[31m502  \033[0mCommand not implemented.\n\n"))
	}
}

func handleEcho(conn *net.TCPConn, value []string, _ chan<- bool) {
	fmt.Fprintf(conn, "\033[32m200  \033[0m%v \n\n", strings.Join(value, " "))
}

func handleHello(conn *net.TCPConn, value []string, _ chan<- bool) {
	conn.Write([]byte("\033[32m200  \033[0mHello\n\n"))
}

type Credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func isLoginUnique(users []Credentials, login string) bool {
	for _, user := range users {
		if user.Login == login {
			return false
		}
	}
	return true
}

func handleRegister(conn *net.TCPConn, value []string, _ chan<- bool) {
	if len(value) < 2 {
		conn.Write([]byte("Lack of arguments, exit."))
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(value[1]), 10)
	if err != nil {
		conn.Write([]byte("Error generating password hash, maybe password is too long?"))
		return
	}

	newUser := new(Credentials)
	newUser.Login = value[0]
	newUser.Password = string(hashedPassword)

	users, err := utils.LoadJSON[[]Credentials]("internal/server/db.json")
	if err != nil {
		log.Fatal("something went wrong with loading file. ", err)
	}

	if !isLoginUnique(users, newUser.Login) {
		conn.Write([]byte("Username exists, try again with different login. \n"))
		return
	}

	users = append(users, *newUser)

	err = utils.SaveJSON("internal/server/db.json", users)
	if err != nil {
		log.Printf("Error saving file: %v\n", err)
		conn.Write([]byte("Server error, please try again later.\n"))
		return
	}

	fmt.Printf("New user registered: %v", newUser)
	fmt.Fprintf(conn, "\033[32m200  \033[0mSuccessfully registered. Your login: %v \n\n", newUser.Login)
}

func handleLogin(conn *net.TCPConn, value []string, _ chan<- bool) {
	// bcrypt.CompareHashAndPassword
	// TODO: make some cool custom tcp client: auto highlighting keywords, more cool sh!

	if len(value) < 1 {
		conn.Write([]byte("\033[31m501  \033[0mNo username provided, try again.\n\n"))
		return
	}

	if len(value) > 1 {
		conn.Write([]byte("\033[31m501  \033[0mUse one username..\n\n"))
		return
	}

	// TODO: handle more cases with codes. add correct login, add color pkg and replace with inline
	fmt.Print(value)
}

func handleQuit(conn *net.TCPConn, _ []string, quitChan chan<- bool) {
	conn.Write([]byte("\033[32m221  \033[0mConnection closed.\n\n"))
	quitChan <- true
	close(quitChan)
}
