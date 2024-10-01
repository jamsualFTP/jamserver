package server

import (
	"fmt"
	"io"
	"jamsual/pkg/utils"
	"log"
	"net"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// NOTE: https://www.rfc-editor.org/rfc/rfc959
// TODO : implement auth system,

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

func HandleConnection(conn *net.TCPConn, id int) {
	defer func() {
		conn.Close()
		mu.Lock()
		delete(activeConnections, id)
		fmt.Printf("Connection %v closed and removed from active list\n", id)
		mu.Unlock()
	}()

	conn.Write([]byte(fmt.Sprintf("220 Welcome to jamsualFTP server, user %v! \n\n", id)))

	for {
		buffer := make([]byte, 128)
		n, err := conn.Read(buffer)

		if err == io.EOF {
			return
		}
		str := strings.TrimSpace(string(buffer[:n]))
		part := strings.Split(str, " ")
		command := strings.ToUpper(part[0])
		args := part[1:]
		HandleCommands(conn, command, args)

	}
}

// using command pattern for a while, maybe will refactor to COR
func HandleCommands(conn *net.TCPConn, command string, args []string) {
	commands := map[string]func(*net.TCPConn, []string){
		"ECHO":     handleEcho,
		"HELLO":    handleHello,
		"REGISTER": handleRegister,
		"LOGIN":    handleLogin,
	}

	if result, ok := commands[command]; ok {
		result(conn, args)
	} else {
		conn.Write([]byte("502, command not implemented \n\n"))
	}
}

func handleEcho(conn *net.TCPConn, value []string) {
	conn.Write([]byte(fmt.Sprintf("%v \n\n", strings.Join(value, " "))))
}

func handleHello(conn *net.TCPConn, value []string) {
	conn.Write([]byte("Hello \n\n"))
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

func handleRegister(conn *net.TCPConn, value []string) {
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
	conn.Write([]byte(fmt.Sprintf("Successfully registered. Your login: %v \n\n", newUser.Login)))
}

func handleLogin(conn *net.TCPConn, value []string) {
	// bcrypt.CompareHashAndPassword
	// TODO: make some cool custom tcp client: auto highlighting keywords, more cool sh!
}
