package servergrpc

import (
	"flag"
	"fmt"
	"net"

	pb "github.com/221bytes/osiris/fileguide"
	"github.com/221bytes/osiris/fileutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

var (
	port = flag.Int("port", 10000, "The server port")
)

type fileGuideServer struct {
}

func (s *fileGuideServer) SaveFile(stream pb.FileGuide_SaveFileServer) error {
	fileSummary, err := fileutils.SaveFileFromStream(stream)
	if err != nil {
		grpclog.Fatalf("%v.Server SaveFile= %v", stream, err)
	}
	grpclog.Printf("file download in %v seconds", fileSummary.ElapsedTime)
	return stream.SendAndClose(fileSummary)
}

func (s *fileGuideServer) GetFile(filename *pb.Filename, stream pb.FileGuide_GetFileServer) error {
	if err := fileutils.SendFileToStream(filename.Name, stream); err != nil {
		grpclog.Fatalf("%v.RecordRoute(_) = _, %v", stream, err)
	}
	return nil
}

func newServer() *fileGuideServer {
	s := new(fileGuideServer)
	return s
}

func StartServer() {
	grpclog.Println("Starting ServerGRPC")

	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterFileGuideServer(grpcServer, newServer())
	grpcServer.Serve(lis)
}
