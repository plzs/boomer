// +build !goczmq

package boomer

import (
	"fmt"
	"log"
	"testing"

	"github.com/zeromq/gomq"
	"github.com/zeromq/gomq/zmtp"
)

type testServer struct {
	bindHost       string
	pushPort       int
	pullPort       int
	fromClient     chan *message
	toClient       chan *message
	pushSocket     *gomq.PushSocket
	pullSocket     *gomq.PullSocket
	shutdownSignal chan bool
}

func newTestServer(bindHost string, pushPort, pullPort int) (server *testServer) {
	return &testServer{
		bindHost:       bindHost,
		pushPort:       pushPort,
		pullPort:       pullPort,
		fromClient:     make(chan *message, 100),
		toClient:       make(chan *message, 100),
		shutdownSignal: make(chan bool, 1),
	}
}

func (s *testServer) send() {
	for {
		select {
		case <-s.shutdownSignal:
			s.pushSocket.Close()
			return
		case msg := <-s.toClient:
			s.sendMessage(msg)
		}
	}
}

func (s *testServer) sendMessage(msg *message) {
	err := s.pushSocket.Send(msg.serialize())
	if err != nil {
		log.Printf("Error sending to client: %v\n", err)
	}
}

func (s *testServer) recv() {
	for {
		select {
		case <-s.shutdownSignal:
			s.pullSocket.Close()
			return
		default:
			msg, err := s.pullSocket.Recv()
			if err != nil {
				log.Printf("Error reading: %v\n", err)
			} else {
				msgFromClient := newMessageFromBytes(msg)
				s.fromClient <- msgFromClient
			}
		}
	}
}

func (s *testServer) close() {
	close(s.shutdownSignal)
}

func (s *testServer) start() {
	pushAddr := fmt.Sprintf("tcp://%s:%d", s.bindHost, s.pushPort)
	pullAddr := fmt.Sprintf("tcp://%s:%d", s.bindHost, s.pullPort)

	pushSocket := gomq.NewPush(zmtp.NewSecurityNull())
	pullSocket := gomq.NewPull(zmtp.NewSecurityNull())

	go pushSocket.Bind(pushAddr)
	go pullSocket.Bind(pullAddr)
	s.pushSocket = pushSocket
	s.pullSocket = pullSocket

	go s.recv()
	go s.send()
}

func TestPingPong(t *testing.T) {
	masterHost := "127.0.0.1"
	masterPort := 5557
	toMaster = make(chan *message, 10)

	server := newTestServer(masterHost, masterPort+1, masterPort)
	// defer server.close()
	server.start()

	// start client
	newZmqClient(masterHost, masterPort)
	// defer client.close()

	toMaster <- newMessage("ping", nil, "testing ping pong")
	msg := <-server.fromClient
	if msg.Type != "ping" || msg.NodeID != "testing ping pong" {
		t.Error("server doesn't recv ping message")
	}

	server.toClient <- newMessage("pong", nil, "testing ping pong")
	msg = <-fromMaster
	if msg.Type != "pong" || msg.NodeID != "testing ping pong" {
		t.Error("client doesn't recv pong message")
	}
}
