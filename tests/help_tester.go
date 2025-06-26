package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	logFileName := "help_tester.log"
	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Błąd otwierania pliku logów %s: %v", logFileName, err)
	}
	defer logFile.Close()

	mw := io.MultiWriter(os.Stderr, logFile)
	log.SetOutput(mw)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds) // Use microseconds for better timing

	serverHost := flag.String("host", "localhost", "Server hostname or IP address")
	controlPort := flag.Int("cport", 2121, "Control connection port")
	helpPort := flag.Int("hport", 2222, "HELP connection port")
	numClients := flag.Int("n", 10, "Number of concurrent clients")
	username := flag.String("user", "testuser", "FTP username")
	password := flag.String("pass", "testpass", "FTP password")
	testDuration := flag.Duration("duration", 30*time.Second, "Duration of the test (e.g., 30s, 1m)")

	flag.Parse()

	serverControlAddr := fmt.Sprintf("%s:%d", *serverHost, *controlPort)
	serverHelpAddr := fmt.Sprintf("%s:%d", *serverHost, *helpPort)

	log.Printf("Starting HELP connection test...")
	log.Printf("Target Server: %s (Control: %d, HELP: %d)", *serverHost, *controlPort, *helpPort)
	log.Printf("Number of concurrent clients: %d", *numClients)
	log.Printf("Test duration: %v", *testDuration)
	log.Printf("Using credentials: User=%s", *username)

	var wg sync.WaitGroup
	startTime := time.Now()

	done := make(chan struct{})

	for i := 0; i < *numClients; i++ { // Corrected loop range
		wg.Add(1)
		go simulateClient(i, serverControlAddr, serverHelpAddr, *username, *password, &wg, done)
	}

	go func() {
		<-time.After(*testDuration)
		log.Println("Test duration elapsed. Signaling clients to stop...")
		close(done)
	}()

	wg.Wait()

	log.Printf("Test finished after %v. All clients completed.", time.Since(startTime))
}

// ... (Funkcja main - zakładam, że jest poprawna i zawiera inicjalizację logowania do pliku) ...

func simulateClient(id int, controlAddr, helpAddr, user, pass string, wg *sync.WaitGroup, done <-chan struct{}) {
	defer wg.Done()
	logPrefix := fmt.Sprintf("[Client %d] ", id)

	// --- Połączenie Kontrolne ---
	controlConn, err := net.DialTimeout("tcp", controlAddr, 5*time.Second)
	if err != nil {
		log.Printf("%sError connecting to control port %s: %v", logPrefix, controlAddr, err)
		return
	}
	defer controlConn.Close()
	log.Printf("%sConnected to control port.", logPrefix)
	controlReader := bufio.NewReader(controlConn)

	// --- Połączenie HELP ---
	helpConn, err := net.DialTimeout("tcp", helpAddr, 5*time.Second)
	if err != nil {
		log.Printf("%sError connecting to HELP port %s: %v", logPrefix, helpAddr, err)
		controlConn.Close() // Zamknij też controlConn
		return
	}
	defer helpConn.Close()
	log.Printf("%sConnected to HELP port.", logPrefix)

	helpDataChan := make(chan string, 10)
	var helpWg sync.WaitGroup
	helpWg.Add(1)
	go readHelpConnection(logPrefix, helpConn, helpDataChan, &helpWg)

	err = controlConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		log.Printf("%sError setting initial read deadline: %v", logPrefix, err)
		helpConn.Close()
		return
	}

	var welcomeLines []string
	expectedWelcomeLines := 4
	linesRead := 0
	for linesRead < expectedWelcomeLines {
		if linesRead > 0 {
			err = controlConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			if err != nil {
				log.Printf("%sError setting short read deadline: %v", logPrefix, err)
				break
			}
		}

		line, err := readResponse(logPrefix+"Control", controlReader)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() && linesRead > 0 {
				log.Printf("%sTimeout reading further welcome lines, assuming complete.", logPrefix)
				break // Wyjdź z pętli
			}
			log.Printf("%sError reading welcome line %d: %v", logPrefix, linesRead+1, err)
			helpConn.Close()
			return
		}
		if line != "" {
			log.Printf("%sReceived Welcome Line %d: %s", logPrefix, linesRead+1, line)
			welcomeLines = append(welcomeLines, line)
		} else {
			log.Printf("%sReceived Empty Welcome Line %d", logPrefix, linesRead+1)
		}
		linesRead++
	}

	controlConn.SetReadDeadline(time.Time{})

	if len(welcomeLines) == 0 || !strings.HasPrefix(welcomeLines[0], "220") {
		log.Printf("%sDid not receive expected 220 welcome code.", logPrefix)
	}

	// --- Sprawdzenie initial HELP ---
	select {
	case initialHelp, ok := <-helpDataChan:
		if !ok {
			log.Printf("%sHELP channel closed before initial data.", logPrefix)
			return
		}
		log.Printf("%sReceived initial HELP data: %s", logPrefix, initialHelp)
	case <-time.After(3 * time.Second):
		log.Printf("%sDid not receive initial HELP data within timeout.", logPrefix)
	case <-done:
		log.Printf("%sReceived stop signal before initial HELP.", logPrefix)
		return
	}

	// --- Losowy sleep przed logowaniem ---
	initialSleep := time.Duration(50+rand.Intn(500)) * time.Millisecond
	select {
	case <-time.After(initialSleep):
		// Kontynuuj
	case <-done:
		log.Printf("%sReceived stop signal during initial sleep.", logPrefix)
		return
	}

	// --- Logowanie ---
	if err := sendCommand(logPrefix+"Control", controlConn, fmt.Sprintf("USER %s", user)); err != nil {
		return
	}
	err = controlConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		log.Printf("%sError setting USER read deadline: %v", logPrefix, err)
		return
	}
	userResp, err := readResponse(logPrefix+"Control", controlReader)
	if err != nil {
		return
	}
	controlConn.SetReadDeadline(time.Time{})
	log.Printf("%sUSER response: %s", logPrefix, userResp)

	// time.Sleep(100 * time.Millisecond) // Opcjonalny sleep

	if err := sendCommand(logPrefix+"Control", controlConn, fmt.Sprintf("PASS %s", pass)); err != nil {
		return
	}
	err = controlConn.SetReadDeadline(time.Now().Add(15 * time.Second))
	if err != nil {
		log.Printf("%sError setting PASS read deadline: %v", logPrefix, err)
		return
	}
	loginResp, err := readResponse(logPrefix+"Control", controlReader)

	controlConn.SetReadDeadline(time.Time{})

	loginSucceeded := false
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Printf("%sTimeout waiting for PASS response.", logPrefix)
		} else if err != io.EOF { // Nie loguj EOF jako błędu krytycznego
			log.Printf("%sError reading PASS response: %v", logPrefix, err)
		}
		// Nawet jeśli był błąd odczytu odpowiedzi PASS, logowanie mogło się udać na serwerze.
		// Sprawdzimy kanał HELP poniżej.
	} else {
		log.Printf("%sPASS response: %s", logPrefix, loginResp)
		if strings.HasPrefix(loginResp, "230") {
			loginSucceeded = true
			log.Printf("%sLogin successful based on response.", logPrefix)
		} else {
			log.Printf("%sLogin failed based on response.", logPrefix)
		}
	}

	select {
	case updatedHelp, ok := <-helpDataChan:
		if !ok {
			log.Printf("%sHELP channel closed before checking for update.", logPrefix)
		} else {
			log.Printf("%sReceived potential updated HELP data: %s", logPrefix, updatedHelp)
			if strings.Contains(updatedHelp, "pasv") { // Proste sprawdzenie
				log.Printf("%sHELP data confirms logged-in state.", logPrefix)
				if !loginSucceeded {
					log.Printf("%sWarning: HELP data updated despite non-230 PASS response or read error.", logPrefix)
				}
				loginSucceeded = true
			} else if loginSucceeded {
				log.Printf("%sWarning: Login response was 230, but HELP data doesn't seem updated.", logPrefix)
			}
		}
	case <-time.After(5 * time.Second):
		log.Printf("%sDid not receive updated HELP data within timeout after PASS attempt.", logPrefix)
		if loginSucceeded {
			log.Printf("%sWarning: Login response was 230, but no HELP update received.", logPrefix)
		}
	case <-done:
		log.Printf("%sReceived stop signal before checking updated HELP.", logPrefix)
		return
	}

	// --- Pętla oczekiwania ---
	log.Printf("%sEntering wait loop (Login success: %t)...", logPrefix, loginSucceeded)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			log.Printf("%sReceived stop signal. Sending QUIT.", logPrefix)
			if controlConn != nil {
				_ = sendCommand(logPrefix+"Control", controlConn, "QUIT")
				err = controlConn.SetReadDeadline(time.Now().Add(5 * time.Second))
				if err != nil {
				}
				quitResp, _ := readResponse(logPrefix+"Control", controlReader)
				controlConn.SetReadDeadline(time.Time{})
				log.Printf("%sQUIT response: %s", logPrefix, quitResp)
			}
			// Poczekaj na zakończenie gorutyny HELP przed wyjściem z simulateClient
			helpConn.Close() // Zamknięcie połączenia powinno zakończyć readHelpConnection
			helpWg.Wait()    // Czekaj na zakończenie gorutyny readHelpConnection
			return
		case helpMsg, ok := <-helpDataChan:
			if !ok {
				log.Printf("%sHELP connection closed unexpectedly during wait loop.", logPrefix)
				return // Zakończ, jeśli kanał HELP został zamknięty
			}
			log.Printf("%sReceived subsequent HELP data: %s", logPrefix, helpMsg)
		case <-ticker.C:
		}
	}
}

func readHelpConnection(logPrefix string, conn net.Conn, dataChan chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()       // Zasygnalizuj zakończenie
	defer close(dataChan) // Zamknij kanał, gdy funkcja się zakończy
	reader := bufio.NewReader(conn)
	for {
		err := conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err != nil {
			return
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return // Inny błąd (EOF)
		}
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			select {
			case dataChan <- trimmedLine:
			case <-time.After(100 * time.Millisecond): // Zwiększony timeout dla kanału
				log.Printf("%sHELP Channel full or blocked, discarding message: %s", logPrefix, trimmedLine)
			}
		}
	}
}

// Funkcja sendCommand - bez zmian
func sendCommand(logPrefix string, conn net.Conn, command string) error {
	// log.Printf("%sSending: %s", logPrefix, command) // Uncomment for debugging
	_, err := fmt.Fprintf(conn, "%s\r\n", command)
	if err != nil {
		log.Printf("%sError sending command '%s': %v", logPrefix, command, err)
	}
	return err
}

// Funkcja readResponse - bez zmian (bez deadline'u wewnątrz)
func readResponse(logPrefix string, reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		if err != io.EOF {
			log.Printf("%sError reading response: %v", logPrefix, err)
		}
		return "", err
	}
	trimmedLine := strings.TrimSpace(line)
	// log.Printf("%sReceived: %s", logPrefix, trimmedLine) // Uncomment for debugging
	return trimmedLine, nil
}
