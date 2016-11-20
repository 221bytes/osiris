package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/221bytes/osiris/clientgrpc"
	"github.com/221bytes/osiris/fileutils"
	"github.com/221bytes/osiris/servergrpc"
)

var (
	tls                = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	caFile             = flag.String("ca_file", "testdata/ca.pem", "The file containning the CA root cert file")
	serverAddr         = flag.String("server_addr", "127.0.0.1:10000", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "x.test.youtube.com", "The server name use to verify the hostname returned by TLS handshake")
	functions          = map[string]func(opts []string){"help": help}
)

func testFile() error {
	ciphertext, err := fileutils.Encrypt("test.mkv")
	if err != nil {
		return err
	}

	if err := fileutils.NewFile(ciphertext, "test.locked"); err != nil {
		return err
	}
	fileDecrypted, err := fileutils.Decrypt("test.locked")
	if err != nil {
		return err
	}

	if err := fileutils.NewFile(fileDecrypted, "test.unlocked"); err != nil {
		return err
	}
	return nil
}

func help(opts []string) {
	fmt.Println("Available options")
}

func getSTDIN(cmdsc chan []string) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		ints := strings.Split(input, " ")
		fn := functions[ints[0]]
		cmdsc <- ints
		if fn == nil {
			fmt.Printf("unknow command %v\n", ints[0])
			continue
		}
		fn(ints[0:])
	}
}

func main() {
	flag.Parse()

	cmdsc := make(chan []string)
	go func() {
		servergrpc.StartServer()
	}()
	go func() {
		clientgrpc.StartClient(cmdsc, *serverAddr)
	}()

	getSTDIN(cmdsc)
}
