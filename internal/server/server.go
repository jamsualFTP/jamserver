package server

import (
	"fmt"
	"io"
	"jamserver/pkg/utils"
	"log"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// NOTE: https://www.rfc-editor.org/rfc/rfc959
// TODO : implement auth system;

type Session struct {
	Login         string
	Authenticated bool
}

type Client struct {
	Conn    *net.TCPConn
	Session *Session
}

var (
	connectionCounter int
	activeConnections = make(map[int]*Client)
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

		client := &Client{
			Conn:    conn,
			Session: &Session{},
		}

		mu.Lock()
		connectionCounter++
		id := connectionCounter
		activeConnections[id] = client
		mu.Unlock()

		addr := conn.RemoteAddr().String()
		fmt.Printf("Accepted new connection: id = %v! %v \n\n", id, addr)
		go HandleConnection(client, id)
	}
}

func HandleDisconnect(client *Client, id int) {
	mu.Lock()
	defer mu.Unlock()

	if err := client.Conn.Close(); err != nil {
		fmt.Printf("Error closing connection %v: %v\n", id, err)
	}

	delete(activeConnections, id)
	fmt.Printf("Connection %v closed and removed from active list\n", id)
}

func HandleConnection(client *Client, id int) {
	quitChan := make(chan bool)
	defer HandleDisconnect(client, id)

	time.Sleep(time.Second)
	fmt.Fprintf(client.Conn, "\033[36m220  \033[0mWelcome to jamsualFTP server, user %v! \n\n", id)

	for {
		select {
		case <-quitChan:
			fmt.Println("SADASD")
			return
		default:
			buffer := make([]byte, 1024) // request buffer
			n, err := client.Conn.Read(buffer)

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
			go HandleCommands(client, command, args, quitChan)
		}
	}
}

// using command pattern for a while, maybe will refactor to COR
func HandleCommands(client *Client, command string, args []string, quitChan chan bool) {
	commands := map[string]func(*Client, []string, chan<- bool){
		"ECHO": handleEcho,
		"HLLO": handleHello,
		"RGSR": handleRegister,
		"USER": handleLogin,
		"PASS": handlePass,
		"QUIT": handleQuit,
	}

	if result, ok := commands[command]; ok {
		result(client, args, quitChan)
	} else {
		client.Conn.Write([]byte("\033[31m502  \033[0mCommand not implemented.\n\n"))
	}
}

func handleEcho(client *Client, value []string, _ chan<- bool) {
	fmt.Fprintf(client.Conn, "\033[32m200  \033[0m%v \n\n", strings.Join(value, " "))
}

func handleHello(client *Client, value []string, _ chan<- bool) {
	client.Conn.Write([]byte("\033[32m200  \033[0mHello\n\n"))
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

func handleRegister(client *Client, value []string, _ chan<- bool) {
	if len(value) < 2 {
		client.Conn.Write([]byte("Lack of arguments, exit."))
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(value[1]), 10)
	if err != nil {
		client.Conn.Write([]byte("Error generating password hash, maybe password is too long?"))
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
		client.Conn.Write([]byte("Username exists, try again with different login. \n"))
		return
	}

	users = append(users, *newUser)

	err = utils.SaveJSON("internal/server/db.json", users)
	if err != nil {
		log.Printf("Error saving file: %v\n", err)
		client.Conn.Write([]byte("Server error, please try again later.\n"))
		return
	}

	fmt.Printf("New user registered: %v \n\n", newUser)
	fmt.Fprintf(client.Conn, "\033[32m200  \033[0mSuccessfully registered. Your login: %v \n\n", newUser.Login)
}

func handleLogin(client *Client, value []string, _ chan<- bool) {
	// TODO: make some cool custom tcp client: auto highlighting keywords, prefix when logged in!

	if client.Session.Authenticated {
		fmt.Fprintf(client.Conn, "\033[33m435  \033[0mYou are already logged in.. \n\n")
		return
	}

	if len(value) < 1 {
		client.Conn.Write([]byte("\033[31m501  \033[0mNo username provided, try again.\n\n"))
		return
	}

	if len(value) > 1 {
		client.Conn.Write([]byte("\033[31m501  \033[0mUse one username..\n\n"))
		return
	}

	users, err := utils.LoadJSON[[]Credentials]("internal/server/db.json")
	if err != nil {
		log.Fatal("something went wrong with loading file. ", err)
	}

	login := value[0]
	if len(login) > 0 {
		// preventing panic with idx out of range
		client.Session.Login = ""
		for _, user := range users {
			if login == user.Login {
				client.Session.Login = login
				client.Conn.Write([]byte("\033[33m331  \033[0mUser okay, need password.  \n\n"))
				return
			}
		}
		fmt.Fprintf(client.Conn, "\033[33m332 \033[0mNeed account for login. \n\n")
		client.Session.Login = ""
	}
}

func handlePass(client *Client, value []string, quitChan chan<- bool) {
	if client.Session.Authenticated {
		fmt.Fprintf(client.Conn, "\033[33m435  \033[0mYou are already logged in.. \n\n")
		return
	}

	if len(value) < 1 {
		client.Conn.Write([]byte("\033[31m501  \033[0mNo password provided, try again.\n\n"))
		return
	}

	if len(value) > 1 {
		client.Conn.Write([]byte("\033[31m501  \033[0mUse one password..\n\n"))
		return
	}

	password := value[0]
	if len(password) > 0 {
		users, err := utils.LoadJSON[[]Credentials]("internal/server/db.json")
		if err != nil {
			log.Fatal("something went wrong with loading file. ", err)
		}

		if len(client.Session.Login) > 0 {

			idx := slices.IndexFunc(users, func(u Credentials) bool { return u.Login == client.Session.Login })

			if idx >= 0 {
				err := bcrypt.CompareHashAndPassword([]byte(users[idx].Password), []byte(password))
				if err != nil {
					client.Conn.Write([]byte("\033[31m503  \033[0mNot logged in. \n\n"))
					return
				} else {
					client.Session.Authenticated = true
					fmt.Fprintf(client.Conn, "\033[32m230  \033[0mUser logged in, proceed. \n\n")
					return
				}
			}
		} else {
			client.Conn.Write([]byte("\033[31m503  \033[0mNot user specified. \n\n"))
			return
		}
	}
}

func handleQuit(client *Client, _ []string, quitChan chan<- bool) {
	// BUG: check client side
	// if client.Session.Authenticated {
	// 	client.Session.Authenticated = false
	// 	client.Session.Login = ""
	// 	client.Conn.Write([]byte("\033[32m221  \033[0mSuccessfully logged out.\n\n"))
	// 	return
	// }
	client.Conn.Write([]byte("\033[32m221  \033[0mConnection closed.\n\n"))
	client.Session.Authenticated = false
	client.Session.Login = ""
	quitChan <- true
	close(quitChan)
}
