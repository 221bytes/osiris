package servergrpc

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	context "golang.org/x/net/context"

	pb "github.com/221bytes/osiris/fileguide"
	"github.com/221bytes/osiris/fileutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

var (
	port = flag.Int64("port", 10000, "The server port")
)

type fileGuideServer struct {
	fileSummariesc chan *pb.FileSummary
}

var progressMap = make(map[string]int64)

func (s *fileGuideServer) SaveFile(stream pb.FileGuide_SaveFileServer) error {
	fileSummary, err := fileutils.SaveFileFromStream(stream)
	if err != nil {
		grpclog.Fatalf("%v.Server SaveFile= %v", stream, err)
	}
	grpclog.Printf("file download in %v seconds", fileSummary.ElapsedTime)
	s.fileSummariesc <- fileSummary

	return stream.SendAndClose(fileSummary)
}

func (s *fileGuideServer) GetFile(filename *pb.Filename, stream pb.FileGuide_GetFileServer) error {
	file, err := os.Open(filename.Name)
	if err != nil {
		grpclog.Fatalf("%v.runSaveFileRoute() got error %v, want %v", stream, err, nil)
	}

	if err := fileutils.SendFileToStream(file, stream); err != nil {
		grpclog.Fatalf("%v.RecordRoute(_) = _, %v", stream, err)
	}
	return nil
}
func random(min, max int) int64 {
	rand.Seed(time.Now().Unix())
	return int64(rand.Intn(max-min) + min)
}

func (s *fileGuideServer) InitConnection(ctx context.Context, ini *pb.InitConnectionData) (*pb.InitConnectionData, error) {
	myrand := random(1024, 65535)
	ports := make([]int64, ini.NBPort)
	ini.Ports = ports
	for i := int64(0); i < ini.NBPort; i++ {
		go runServer(myrand + i)
		ports[i] = myrand + i
	}
	return ini, nil
}

func newServer() *fileGuideServer {
	s := new(fileGuideServer)
	return s
}

func runServer(port int64) {
	var mutex = &sync.Mutex{}

	grpclog.Printf("Starting ServerGRPC on %v\n", port)
	fileSummariesc := make(chan *pb.FileSummary)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	grpcServer := grpc.NewServer(opts...)
	fileGuideSvr := newServer()
	fileGuideSvr.fileSummariesc = fileSummariesc
	pb.RegisterFileGuideServer(grpcServer, fileGuideSvr)
	go grpcServer.Serve(lis)
	for {
		select {
		case fileSummary := <-fileSummariesc:
			fmt.Println("fileSummary")
			fmt.Println(fileSummary)
			mutex.Lock()
			progressMap[fileSummary.Name]++
			if progressMap[fileSummary.Name] == fileSummary.TotalBlock {
				fmt.Println("all blocks has been downloaded")
				fileutils.MergeFile(fileSummary.Name)
			}
			mutex.Unlock()
		}
	}
}

func StartServer() {
	flag.Parse()
	runServer(*port)
}
