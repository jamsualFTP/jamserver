# jamsual server

my approach for ftp server-client implementation according to rfc 959, current goal is to make it sftp

## how to use (if someone ever wants) | it should still works or ill update later

1. git clone
2. go mod tidy
3. now, `go run cmd/main.go` or `go build cmd/main.go` then run as other bin
   (tested on macos, on linux should works, on windows no idea)

- if u want *docker*🐳:

1. change IP in `internal/server/server.go` from *127.0.0.1* to *0.0.0.0*
   (port currently hardcoded, but no worries, other will work too 2121 because 21 is ftp port for system)
2. `docker build -t <your_custom_name> .`
3. `docker run <your_custom_name>`
   or `docker run --name <your_custom_ame> -d -p 2121:2121 jamsualftp`
   (check 2nd method when encounter problem with ports)

- use some tcp client: *telnet*, *netcat* etc. with specified *ip* and *port*
  try: `echo <message>`, `hello` (just hello), `register <login> <password>`
