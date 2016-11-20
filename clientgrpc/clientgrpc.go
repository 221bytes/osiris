package clientgrpc

import (
	"fmt"

	pb "github.com/221bytes/osiris/fileguide"
	"github.com/221bytes/osiris/fileutils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

type ClientGRPC struct {
	client pb.FileGuideClient
	i      int64
}

func (c *ClientGRPC) runSaveFileRoute(filename string) {
	stream, err := c.client.SaveFile(context.Background())
	if err != nil {
		grpclog.Fatalf("%v.RecordRoute(_) = _, %v", c.client, err)
	}

	if err := fileutils.SendFileToStream(filename, stream); err != nil {
		grpclog.Fatalf("%v.RecordRoute(_) = _, %v", c.client, err)
	}

	reply, err := stream.CloseAndRecv()
	if err != nil {
		grpclog.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	grpclog.Printf("reply: %v", reply)
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

func waitForCommands(cmdsc chan []string, clientGRPC *ClientGRPC) {
	for {
		select {
		case input := <-cmdsc:
			fmt.Println(input)
			if input[0] == "GetFile" {
				go clientGRPC.runGetFileRoute(input[1])
			}
			if input[0] == "SaveFile" {
				go clientGRPC.runSaveFileRoute(input[1])
			}
		}
	}
}

func StartClient(cmdsc chan []string, serverAddr string) {
	grpclog.Printf("Starting ClientGRPC on %s\n", serverAddr)
	var opts []grpc.DialOption

	opts = append(opts, grpc.WithInsecure())

	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		grpclog.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewFileGuideClient(conn)
	clientGRPC := &ClientGRPC{client: client, i: 0}
	waitForCommands(cmdsc, clientGRPC)
}
