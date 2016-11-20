package fileutils

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	pb "github.com/221bytes/osiris/fileguide"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

var downloadFolder = "files_download"

func Encrypt(filename string) ([]byte, error) {
	// read content from your file
	plaintext, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	// this is a key
	key := []byte("example key 1234")

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)
	return ciphertext, nil

}

func Decrypt(filename string) ([]byte, error) {
	ciphertext, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	key := []byte("example key 1234")

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)

	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}

func NewFile(ciphertext []byte, filename string) error {

	// create a new file for saving the encrypted data.
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, bytes.NewReader(ciphertext))
	if err != nil {
		return err
	}
	return nil
}

func Recv(s grpc.Stream) (*pb.FileChunk, error) {
	m := new(pb.FileChunk)
	if err := s.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func SaveFileFromStream(stream grpc.Stream) (*pb.FileSummary, error) {
	nBytes, nChunks := int64(0), int64(0)
	once := false
	var out *os.File
	startTime := time.Now()
	for {
		fileChunk, err := Recv(stream)
		if err == io.EOF {
			endTime := time.Now()
			elapsedTime := int64(endTime.Sub(startTime).Seconds())
			return &pb.FileSummary{
				ByteCount:   nBytes,
				ChunkCount:  nChunks,
				ElapsedTime: elapsedTime,
			}, nil
		}
		if err != nil {
			return nil, err
		}
		if once == false {
			once = true
			if _, err := os.Stat(downloadFolder); os.IsNotExist(err) {
				os.Mkdir(downloadFolder, 0771)
			}
			out, err = os.Create(downloadFolder + "/" + fileChunk.Filename)
			if err != nil {
				return nil, fmt.Errorf("SaveFileFromStream: os.create error : %v", err)
			}
			defer out.Close()
		}
		n, err := out.Write(fileChunk.Chunk)
		if err != nil {
			return nil, fmt.Errorf("SaveFileFromStream: file write error : %v", err)
		}
		nChunks++
		nBytes += int64(n)
	}
}

func SendFileToStream(filePath string, stream grpc.Stream) error {
	nBytes, nChunks := int64(0), int64(0)
	buf := make([]byte, 0, 4000000)
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	r := bufio.NewReader(file)
	for {
		n, err := r.Read(buf[:cap(buf)])
		buf = buf[:n]
		if n == 0 {
			if err == nil {
				continue
			}
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		nChunks++
		nBytes += int64(len(buf))
		// process buf
		filename := filepath.Base(filePath)
		fileChunk := &pb.FileChunk{Chunk: buf, Filename: filename}

		if err := stream.SendMsg(fileChunk); err != nil {
			grpclog.Fatalf("%v.Send(%v) = %v", stream, fileChunk, err)
		}
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}
	}
	grpclog.Printf("Sent: chunks %v\tbytes:%v", nChunks, nBytes)

	return nil
}
