package clientgrpc

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"io/ioutil"

	pb "github.com/221bytes/osiris/fileguide"
	"github.com/221bytes/osiris/fileutils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

type ClientGRPC struct {
	client pb.FileGuideClient
	conn   *grpc.ClientConn
}

var (
	numCPU     = int64(1)
	serverAddr = flag.String("server_addr", "127.0.0.1", "The server address in the format of host")
	serverPort = flag.String("server_port", ":10000", "The server address in the format of :port")
)

func init() {
	numCPU = int64(runtime.NumCPU())
}

func (c *ClientGRPC) runGetFileRoute(name string) {
	filename := &pb.Filename{Name: name}
	stream, err := c.client.GetFile(context.Background(), filename)
	if err != nil {
		grpclog.Fatalf("%v.RecordRoute(_) = _, %v", c.client, err)
	}
	fileSummary, err := fileutils.SaveFileFromStream(stream)
	if err != nil {
		grpclog.Fatalf("%v.Send= %v", stream, err)
	}
	grpclog.Printf("file download in %v seconds", fileSummary.ElapsedTime)
}

func closeStream(stream pb.FileGuide_SaveFileClient) {
	reply, err := stream.CloseAndRecv()
	if err != nil {
		grpclog.Fatalf("%v.runSaveFileRoute() got error %v, want %v", stream, err, nil)
	}
	grpclog.Printf("reply: %v", reply)
}

func readByteFromFile(buf []byte, blockID int64, filename string, totalBlock int64, stream grpc.ClientStream) error {
	lenBytesToSend := int64(len(buf))
	start, end := int64(0), int64(1000000)
	if 1000000 >= lenBytesToSend {
		end = lenBytesToSend - 1
	}
	for start < lenBytesToSend-1 {

		fileChunk := &pb.FileChunk{
			Chunk:      buf[start:end],
			Filename:   filename,
			BlockID:    blockID,
			TotalBlock: totalBlock,
		}

		if err := stream.SendMsg(fileChunk); err != nil {
			grpclog.Fatalf("%v.Send(%v) = %v", stream, fileChunk, err)
		}

		start = end
		end += 1000000
		if end >= lenBytesToSend {
			end = lenBytesToSend - 1
		}
	}
	// wg.Done()
	// }
	// nb := int64(len(n)) / int64(numCPU)
	// t := int64(0)
	// for i := int64(0); i < numCPU; i++ {
	// 	if i < numCPU-1 {
	// 		go test(n[t:t+nb], i, streams[i])
	// 		t += nb - 1
	// 	} else {
	// 		go test(n[t:], i, streams[i])
	// 	}
	// 	wg.Add(1)
	// }

	// wg.Wait()
	return nil
}

func (c *ClientGRPC) runSaveFileRoute(buf []byte, blockID int64, totalBlock int64, filename string) {
	stream, err := c.client.SaveFile(context.Background())
	if err != nil {
		grpclog.Fatalf("%v.runSaveFileRoute(_) = _, %v", c.client, err)
	}
	defer closeStream(stream)

	if err := readByteFromFile(buf, blockID, filename, totalBlock, stream); err != nil {
		grpclog.Fatalf("%v.runSaveFileRoute(_) = _, %v", c.client, err)
	}
}

func getByteFromFilename(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		grpclog.Fatalf("runSaveFileRoute() got error %v, want %v", err, nil)
	}

	filename = filepath.Base(file.Name())
	r := bufio.NewReader(file)
	n, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatal(err)
	}
	return n, nil
}

func (c *ClientGRPC) runInitConnectionRoute(name string) {
	var wg sync.WaitGroup

	initData := &pb.InitConnectionData{NBPort: numCPU}
	initConnectionData, err := c.client.InitConnection(context.Background(), initData)
	if err != nil {
		grpclog.Fatalf("%v.RecordRoute(_) = _, %v", c.client, err)
	}
	fmt.Println(initConnectionData.Ports)
	buf, err := getByteFromFilename(name)
	if err != nil {
		grpclog.Fatalf("%v.getByteFromFilename(_) = _, %v", c.client, err)
	}
	nbPorts := int64(len(initConnectionData.Ports))
	nb := int64(len(buf)) / nbPorts
	t := int64(0)
	name = filepath.Base(name)
	for index, port := range initConnectionData.Ports {
		run := func(port int64, n []byte, blockID int64) {
			wg.Add(1)
			s := strconv.FormatInt(port, 10)
			client := runClient(*serverAddr + ":" + s)
			client.runSaveFileRoute(n, blockID, nbPorts, name)
			wg.Done()
		}
		if int64(index) < numCPU-1 {
			go run(port, buf[t:t+nb], int64(index))
			t += nb - 1
		} else {
			go run(port, buf[t:], int64(index))
		}
	}
	wg.Wait()
}

func waitForCommands(cmdsc chan []string, clientGRPC *ClientGRPC) {
	for {
		select {
		case input := <-cmdsc:
			fmt.Println(input)
			if input[0] == "GetFile" {
				go clientGRPC.runGetFileRoute(input[1])
			}
			// if input[0] == "SaveFile" {
			// 	go clientGRPC.runSaveFileRoute(input[1])
			// }
			if input[0] == "Init" {
				go clientGRPC.runInitConnectionRoute(input[1])
			}
		}
	}
}

func runClient(serverAddr string) *ClientGRPC {
	grpclog.Printf("Starting ClientGRPC on %s\n", serverAddr)
	var opts []grpc.DialOption

	opts = append(opts, grpc.WithInsecure())

	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		grpclog.Fatalf("fail to dial: %v", err)
	}
	client := pb.NewFileGuideClient(conn)
	return &ClientGRPC{client: client, conn: conn}
}

func StartClient(cmdsc chan []string) {

	clientGRPC := runClient(*serverAddr + *serverPort)
	defer clientGRPC.conn.Close()
	waitForCommands(cmdsc, clientGRPC)
}
