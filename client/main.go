package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	pb "chat-grpc/chat-grpc/proto"

	"google.golang.org/grpc"
)

type clientSession struct {
	username string
	send     chan *pb.ChatMessage
}

// Server implementation
type chatServer struct {
	pb.UnimplementedChatServiceServer
	mu      sync.RWMutex
	clients map[string]*clientSession
	groups  map[string]map[string]bool // groupName -> set of usernames
}

func newServer() *chatServer {
	return &chatServer{
		clients: make(map[string]*clientSession),
		groups:  make(map[string]map[string]bool),
	}
}

// Register unary
func (s *chatServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Username == "" {
		return &pb.RegisterResponse{Ok: false, Message: "empty username"}, nil
	}
	if _, exists := s.clients[req.Username]; exists {
		return &pb.RegisterResponse{Ok: false, Message: "username already used"}, nil
	}
	// Note: we don't create session here; session created when client opens ChatStream
	return &pb.RegisterResponse{Ok: true, Message: "registered (now open ChatStream)"}, nil
}

// List users
func (s *chatServer) ListUsers(ctx context.Context, _ *pb.Empty) (*pb.ListUsersResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	resp := &pb.ListUsersResponse{}
	for name := range s.clients {
		resp.Users = append(resp.Users, &pb.UserInfo{Username: name})
	}
	return resp, nil
}

func (s *chatServer) CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.CreateGroupResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.GroupName == "" {
		return &pb.CreateGroupResponse{Ok: false, Message: "empty group name"}, nil
	}
	if _, ok := s.groups[req.GroupName]; ok {
		return &pb.CreateGroupResponse{Ok: false, Message: "group exists"}, nil
	}
	s.groups[req.GroupName] = make(map[string]bool)
	for _, m := range req.Members {
		s.groups[req.GroupName][m] = true
	}
	return &pb.CreateGroupResponse{Ok: true, Message: "group created"}, nil
}

func (s *chatServer) JoinGroup(ctx context.Context, req *pb.JoinGroupRequest) (*pb.JoinGroupResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[req.GroupName]; !ok {
		s.groups[req.GroupName] = make(map[string]bool)
	}
	s.groups[req.GroupName][req.Username] = true
	return &pb.JoinGroupResponse{Ok: true, Message: "joined"}, nil
}

// ChatStream bi-directional
func (s *chatServer) ChatStream(stream pb.ChatService_ChatStreamServer) error {
	// First message from client should be a "connect" with from=username and type maybe "connect"
	// But for simplicity let's require client to first send a ChatMessage with from set.
	// We'll read first message to know username.

	// Receive initial message to establish username
	initMsg, err := stream.Recv()
	if err != nil {
		return err
	}
	username := initMsg.From
	if username == "" {
		return errors.New("must send initial message with From=username")
	}

	// create session
	sess := &clientSession{
		username: username,
		send:     make(chan *pb.ChatMessage, 100),
	}

	// register client
	s.mu.Lock()
	if _, exists := s.clients[username]; exists {
		s.mu.Unlock()
		return fmt.Errorf("username %s already connected", username)
	}
	s.clients[username] = sess
	s.mu.Unlock()

	// start goroutine to send messages to client
	go func() {
		for msg := range sess.send {
			if err := stream.Send(msg); err != nil {
				log.Printf("send error to %s: %v", username, err)
				return
			}
		}
	}()

	// If initial message also contains payload (not just connect), handle it
	if initMsg.Text != "" {
		initMsg.Timestamp = time.Now().Unix()
		s.handleIncoming(initMsg)
	}

	// read loop
	for {
		msg, err := stream.Recv()
		if err != nil {
			// stream closed or error -> cleanup
			log.Printf("client %s disconnected: %v", username, err)
			s.removeClient(username)
			return nil
		}
		msg.Timestamp = time.Now().Unix()
		s.handleIncoming(msg)
	}
}

func (s *chatServer) removeClient(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.clients[username]; ok {
		close(c.send)
	}
	delete(s.clients, username)
	// Optionally remove from groups or keep membership
	for g := range s.groups {
		delete(s.groups[g], username)
	}
}

func (s *chatServer) handleIncoming(msg *pb.ChatMessage) {
	switch msg.Type {
	case "private":
		s.mu.RLock()
		target, ok := s.clients[msg.To]
		s.mu.RUnlock()
		if ok {
			target.send <- msg
		} else {
			log.Printf("user %s offline, cannot deliver private msg to %s", msg.From, msg.To)
		}
	case "group":
		s.mu.RLock()
		members, ok := s.groups[msg.To]
		clientsSnapshot := make(map[string]*clientSession)
		if ok {
			for member := range members {
				if member == msg.From {
					continue
				}
				if c, online := s.clients[member]; online {
					clientsSnapshot[member] = c
				}
			}
		}
		s.mu.RUnlock()
		for _, c := range clientsSnapshot {
			c.send <- msg
		}
	default:
		log.Printf("unknown msg type: %s", msg.Type)
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcSrv := grpc.NewServer()
	pb.RegisterChatServiceServer(grpcSrv, newServer())
	log.Println("server listening :50051")
	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("serve error: %v", err)
	}
}
