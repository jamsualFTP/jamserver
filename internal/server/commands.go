package server

import (
	"bytes"
	"fmt"
	"io"
	"jamserver/pkg/utils"
	"log"
	"net"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func closeDTPConnection(client *Client) {
	if client.Session.DTPConnection != nil {
		client.Session.DTPConnection.Close()
		client.Session.DTPConnection = nil
	}
	client.Session.Passive = false // Reset passive mode state
}

// using command pattern for a while, maybe will refactor to COR when annoying
func HandleCommands(client *Client, command string, args []string) {
	commands := map[string]func(*Client, []string){
		"ECHO": handleEcho,
		"HLLO": handleHello,
		"RGSR": handleRegister,
		"USER": handleLogin,
		"PASS": handlePass,
		"QUIT": handleQuit,
		"HELP": handleHelp,
		"PASV": handlePassive,
		"LIST": handleList,
		"RETR": handleRetrieve,
		"STOR": handleStore,
	}

	if result, ok := commands[command]; ok {
		if command != "LIST" && command != "RETR" && command != "STOR" {
			closeDTPConnection(client) // Ensure no residual DTP state
		}
		result(client, args)
	} else {
		client.Conn.Write([]byte("\033[31m502  \033[0mCommand not implemented.\n\n"))
	}
}

func handleEcho(client *Client, value []string) {
	fmt.Fprintf(client.Conn, "\033[32m200  \033[0m%v \n\n", strings.Join(value, " "))
}

func handleHello(client *Client, _ []string) {
	fmt.Fprintf(client.Conn, "\033[32m200  \033[0mHello\n\n")
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

func handleRegister(client *Client, value []string) {
	if len(value) < 2 {
		client.Conn.Write([]byte("\033[31m501  \033[0mLack of arguments, exit.\n\n"))
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(value[1]), 10)
	if err != nil {
		client.Conn.Write([]byte("\033[31m451  \033[0mError generating password hash, maybe password is too long?\n\n"))
		return
	}

	newUser := new(Credentials)
	newUser.Login = value[0]
	newUser.Password = string(hashedPassword)

	users, err := utils.LoadJSON[[]Credentials]("app/db.json")
	if err != nil {
		log.Fatal("something went wrong with loading file. ", err)
		client.Conn.Write([]byte("\033[31m451  \033[0mLocal server error.\n\n"))
		return
	}

	if !isLoginUnique(users, newUser.Login) {
		client.Conn.Write([]byte("\033[31m530  \033[0mUsername exists, try again with different login. \n\n"))
		return
	}

	users = append(users, *newUser)

	err = utils.SaveJSON("app/db.json", users)
	if err != nil {
		log.Printf("Error saving file: %v\n", err)
		client.Conn.Write([]byte("\033[31m451  \033[0mServer error, please try again later.\n\n"))
		return
	}

	fmt.Printf("New user registered: %v \n\n", newUser)
	fmt.Fprintf(client.Conn, "\033[32m200  \033[0mSuccessfully registered. Your login: %v \n\n", newUser.Login)
}

func handleLogin(client *Client, value []string) {
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

	users, err := utils.LoadJSON[[]Credentials]("app/db.json")
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

func handlePass(client *Client, value []string) {
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
		users, err := utils.LoadJSON[[]Credentials]("app/db.json")
		if err != nil {
			log.Fatal("something went wrong with loading file. ", err)
		}

		if len(client.Session.Login) > 0 {

			idx := slices.IndexFunc(users, func(u Credentials) bool { return u.Login == client.Session.Login })

			if idx >= 0 {
				err := bcrypt.CompareHashAndPassword([]byte(users[idx].Password), []byte(password))
				if err != nil {
					client.Conn.Write([]byte("\033[31m530  \033[0mNot logged in. \n\n"))
					return
				} else {
					client.Session.Authenticated = true

					fmt.Fprintf(client.Conn, "\033[32m230  \033[0mUser logged in, proceed. \n\n")
					// Update help connection with expanded commands
					if client.Session.HelpConnection != nil {
						availableCommands := getAvailableCommands(client) // Expanded commands after login
						commandList := strings.Join(availableCommands, " ") + "\n"

						if _, err := client.Session.HelpConnection.Write([]byte(commandList)); err != nil {
							fmt.Printf("Error updating commands on help connection: %v\n", err)
						}
					}
					return
				}
			}
		} else {
			client.Conn.Write([]byte("\033[31m503  \033[0mNot user specified. \n\n"))
			return
		}
	}
}

func handleQuit(client *Client, _ []string) {
	if client.Session.DTPConnection != nil {
		client.Session.DTPConnection.Close()
	}
	client.Session.DTPConnection = nil

	client.Conn.Write([]byte("\033[32m221  \033[0mConnection closed.\n\n"))
	client.Session.Authenticated = false

	client.Session.Login = ""
}

func handleHelp(client *Client, _ []string) {
	if client.Session.Authenticated {
		fmt.Fprintf(client.Conn, "\033[32m200  \033[0mAvailable commands: \n     help, echo, hllo, rgsr, user, pass, quit, pasv, list, retr, stor  \n\n")
		return
	} else {
		fmt.Fprintf(client.Conn, "\033[32m200  \033[0mAvailable commands: \n     help, echo, hllo, rgsr, user, pass, quit  \n\n")
		return
	}
}

func handlePassive(client *Client, _ []string) {
	if !client.Session.Authenticated {
		client.Conn.Write([]byte("\033[31m503  \033[0mNot logged in.\n\n"))
		return
	}

	if client.Session.Passive {
		client.Conn.Write([]byte("\033[31m527  \033[0mAlready in Passive Mode.\n\n"))
		return
	}

	if client.Session.DTPListener != nil {
		client.Conn.Write([]byte("\033[31m527  \033[0mAlready in Passive Mode.\n\n"))
		return
	}

	// Always create a new listener, even if already in passive mode
	dtpListener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		fmt.Printf("Error creating listener: %v\n", err)
		client.Conn.Write([]byte("\033[31m425  \033[0mCan't open data connection.\n\n"))
		return
	}

	// Close any existing DTP listener and connection
	if client.Session.DTPListener != nil {
		client.Session.DTPListener.Close()
	}
	if client.Session.DTPConnection != nil {
		client.Session.DTPConnection.Close()
	}

	// Store the listener for potential future closure
	client.Session.DTPListener = dtpListener

	addr := dtpListener.Addr().(*net.TCPAddr)
	port := addr.Port
	port1 := port / 256
	port2 := port % 256
	ipParts := addr.IP.To4()
	if ipParts == nil {
		client.Conn.Write([]byte("\033[31m425  \033[0mCan't open data connection.\n\n"))
		dtpListener.Close()
		return
	}

	fmt.Fprintf(client.Conn, "\033[32m227  \033[0mEntering Passive Mode (%d,%d,%d,%d,%d,%d).\n\n",
		ipParts[0], ipParts[1], ipParts[2], ipParts[3], port1, port2)

	go func() {
		defer func() {
			client.Session.mu.Lock()
			defer client.Session.mu.Unlock()
			fmt.Println("DTP Listener is being closed")
			if err := dtpListener.Close(); err != nil {
				fmt.Printf("Error closing DTP listener: %v\n", err)
			}
			client.Session.DTPListener = nil
		}()

		// Set a timeout for accepting the connection
		if tcpListener, ok := dtpListener.(*net.TCPListener); ok {
			if err := tcpListener.SetDeadline(time.Now().Add(2 * time.Minute)); err != nil {
				fmt.Printf("Error setting deadline for DTP listener: %v\n", err)
				return
			}
		} else {
			fmt.Println("Listener is not a TCP listener; skipping deadline setup.")
		}

		// Accept a connection
		dtpConn, acceptErr := dtpListener.Accept()
		if acceptErr != nil {
			fmt.Printf("Error accepting DTP connection: %v\n", acceptErr)
			return
		}

		// Safely update the session state with the new DTP connection
		client.Session.mu.Lock()
		fmt.Printf("DTP connection established: %v\n", dtpConn.RemoteAddr())
		client.Session.DTPConnection = dtpConn
		client.Session.Passive = true
		client.Session.mu.Unlock()
	}()
}

func handleList(client *Client, args []string) {
	if !client.Session.Authenticated {
		client.Conn.Write([]byte("\033[31m530  \033[0mNot logged in. \n\n"))
		return
	}

	if !client.Session.Passive {
		client.Conn.Write([]byte("\033[31m527  \033[0mYou are not in Passive Mode.\n\n"))
		return
	}

	// Ensure we have an active DTP connection
	if client.Session.DTPConnection == nil {
		client.Conn.Write([]byte("\033[31m425  \033[0mNo data connection. Re-enter Passive Mode.\n\n"))
		client.Session.Passive = false
		return
	}

	files, err := globalFileSystem.ListFiles()
	if err != nil {
		client.Conn.Write([]byte("\033[31m550 \033[0mCould not list directory. \n\n"))
		client.Session.DTPConnection.Close()
		client.Session.DTPConnection = nil
		client.Session.Passive = false
		return
	}

	// Send 150 response before data transfer
	client.Conn.Write([]byte("\033[32m150  \033[0mHere comes the directory listing.\n\n"))

	// Check if files list is empty
	if len(files) == 0 {
		client.Session.DTPConnection.Write([]byte("\033[32m226  \033[0mDirectory is empty.\n\n"))
		client.Session.DTPConnection.Close()
		client.Session.DTPConnection = nil
		client.Session.Passive = false
		return
	}

	// Prepare file list for transmission
	filesList := utils.FormatFileList(files)

	// Send actual listing via DTP connection
	_, err = client.Session.DTPConnection.Write([]byte(filesList))
	if err != nil {
		client.Conn.Write([]byte("\033[31m426  \033[0mConnection closed due to network error.\n\n"))
		client.Session.DTPConnection.Close()
		client.Session.DTPConnection = nil
		client.Session.Passive = false
		return
	}

	// Close the DTP connection
	client.Session.DTPConnection.Close()
	client.Session.DTPConnection = nil
	client.Session.Passive = false

	// Send transfer complete message
	client.Conn.Write([]byte("\033[32m226  \033[0mDirectory send OK. \n\n"))
}

func handleRetrieve(client *Client, args []string) {
	if len(args) < 1 {
		client.Conn.Write([]byte("\033[31m501 \033[0mSyntax error in parameters or arguments.\n   Usage: RETR <filename>\n\n"))
		return
	}

	filename := args[0]

	fileData, err := globalFileSystem.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(client.Conn, "\033[31m550 \033[0mFile not found or access denied: %s\n\n", filename)
		return
	}

	// Check if client is in passive mode
	if client.Session.DTPConnection == nil {
		client.Conn.Write([]byte("\033[31m425 \033[0mUse PASV first.\n\n"))
		return
	}

	fmt.Fprintf(client.Conn, "\033[32m150 \033[0mOpening data connection for %s.\n\n", filename)

	// Write the file data to the data connection
	n, err := client.Session.DTPConnection.Write(fileData)
	if err != nil {
		fmt.Fprintf(client.Conn, "\033[31m426 \033[0mConnection closed; transfer aborted: %v\n\n", err)
		return
	}

	fmt.Fprintf(client.Conn, "\033[32m226 \033[0mTransfer complete. Total bytes sent: %d.\n\n", n)

	// Close the data connection
	client.Session.DTPConnection.Close()
	client.Session.DTPConnection = nil
	client.Session.Passive = false
}

func handleStore(client *Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(client.Conn, "\033[31m501 \033[0mSyntax error in parameters or arguments. Usage: STOR <filename>\n\n")
		return
	}

	filename := args[0]

	if client.Session.DTPConnection == nil {
		fmt.Fprintf(client.Conn, "\033[31m425 \033[0mUse PASV first.\n\n")
		return
	}

	fmt.Fprintf(client.Conn, "\033[32m150 \033[0mOpening data connection for %s.\n\n", filename)

	client.Session.DTPConnection.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Read all data from the connection
	var buffer bytes.Buffer
	buf := make([]byte, 1024)
	totalBytes := 0

	for {
		n, err := client.Session.DTPConnection.Read(buf)
		print(n, err)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Fprintf(client.Conn, "\033[31m426 \033[0mData connection timed out.\n\n")
				return
			}
			if err == io.EOF {
				break // Exit loop on EOF
			}

			fmt.Fprintf(client.Conn, "\033[31m426 \033[0mConnection closed; transfer aborted: %v\n\n", err)
			return
		}

		// Write to buffer
		_, writeErr := buffer.Write(buf[:n])
		if writeErr != nil {
			fmt.Fprintf(client.Conn, "\033[31m426 \033[0mError buffering data: %v\n\n", writeErr)
			return
		}

		totalBytes += n
	}

	client.Session.DTPConnection.SetReadDeadline(time.Time{})

	err := globalFileSystem.WriteFile(filename, buffer.Bytes())
	if err != nil {
		fmt.Fprintf(client.Conn, "\033[31m550 \033[0mCould not write file: %s - %v\n\n", filename, err)
		return
	}

	// Send success response
	fmt.Fprintf(client.Conn, "\033[32m226 \033[0mTransfer complete. Total bytes received: %d.\n\n", totalBytes)

	// Properly close the data connection
	if client.Session.DTPConnection != nil {
		client.Session.DTPConnection.Close()
		client.Session.DTPConnection = nil
	}
	client.Session.Passive = false
}
