package main

import (
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"os/user"
	"runtime"
	"strings"
)

var (
	serversListFile string = "./server_list.txt"
	commandsFile    string = "./commands_list.txt"
	privateKeyFile  string
	username        string
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	handleCommandLine()
	hosts := getListLines(serversListFile)
	commands := getListLines(commandsFile)

	key, err := getKey(privateKeyFile)
	if err != nil {
		log.Fatal("failed getting private key: ", err)
	}
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}
	results := make(chan string, 100)
	done := make(chan struct{}, len(hosts))

	for _, hostname := range hosts {
		go executeCmd(done, commands, hostname, config, results)
	}
	waitAndProcessResults(done, len(hosts), results)
}

func handleCommandLine() {
	usr, _ := user.Current()
	username = usr.Name
	privateKeyFile = usr.HomeDir + "/.ssh/id_rsa"

	flag.StringVar(&serversListFile, "srv", serversListFile, "file with list of servers")
	flag.StringVar(&commandsFile, "cmd", commandsFile, "a file containing a list of commands for execute")
	flag.StringVar(&privateKeyFile, "key", privateKeyFile, "file with private ssh key")
	flag.StringVar(&username, "u", username, "username for connect to servers")

	flag.Parse()
}

func getKey(file string) (key ssh.Signer, err error) {
	rawBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	key, err = ssh.ParsePrivateKey(rawBytes)
	if err != nil {
		return
	}
	return
}

func getListLines(filename string) []string {
	rawBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal("failed to finish reading the file: ", err)
	}
	text := string(rawBytes)
	lines := strings.Split(text, "\n")
	lines = lines[:len(lines)-1]
	return lines
}

func executeCmd(done chan<- struct{}, commands []string, hostname string, config *ssh.ClientConfig, results chan<- string) {
	conn, err := ssh.Dial("tcp", hostname+":22", config)
	if err != nil {
		log.Printf("failed to connect to %s: %s\n", hostname, err)
		done <- struct{}{}
		return
	}
	defer conn.Close()

	for _, cmd := range commands {
		session, err := conn.NewSession()
		if err != nil {
			log.Printf("failed to establish session with %s: %s\n", hostname, err)
			done <- struct{}{}
			return
		}
		defer session.Close()
		stdoutBuf, err := session.Output(cmd)
		if err != nil {
			log.Printf("failed to run commands '%s' on host '%s': %s\n", cmd, hostname, err.Error())
			continue
		}
		results <- hostname + ":\n" + string(stdoutBuf)
	}
	done <- struct{}{}
}

func waitAndProcessResults(done <-chan struct{}, num_hosts int, results <-chan string) {
	for working := num_hosts; working > 0; {
		select { // Blocking
		case result := <-results:
			fmt.Println(result)
		case <-done:
			working--
		}
	}
DONE:
	for {
		select { // Nonblocking
		case result := <-results:
			fmt.Println(result)
		default:
			break DONE
		}
	}
}
