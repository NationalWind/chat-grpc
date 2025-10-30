package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	pb "chat-grpc/proto"

	"google.golang.org/grpc"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type MessageLog struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Type      string `json:"type"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

type Group struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// Toàn bộ dữ liệu lưu ra file
type ServerData struct {
	Users    []User       `json:"users"`
	Groups   []Group      `json:"groups"`
	Messages []MessageLog `json:"messages"`
}

type clientSession struct {
	username string
	send     chan *pb.ChatMessage
}

var dataFile = "data.json"

func loadData() *ServerData {
	f, err := os.Open(dataFile)
	if err != nil {
		log.Println("No existing data file, starting fresh...")
		return &ServerData{}
	}
	defer f.Close()
	var data ServerData
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		log.Println("Failed to parse data.json:", err)
		return &ServerData{}
	}
	log.Printf("Loaded %d users, %d groups, %d messages", len(data.Users), len(data.Groups), len(data.Messages))
	return &data
}

func saveData(data *ServerData) {
	f, err := os.Create(dataFile)
	if err != nil {
		log.Println("saveData error:", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		log.Println("encode error:", err)
	}
}

var (
	serverData *ServerData
	dataMu     sync.RWMutex // Mutex để bảo vệ serverData
)

// Server implementation
type chatServer struct {
	pb.UnimplementedChatServiceServer
	mu      sync.RWMutex
	clients map[string]*clientSession
	groups  map[string]map[string]bool // groupName -> set of usernames
}

func newServer() *chatServer {
	srv := &chatServer{
		clients: make(map[string]*clientSession),
		groups:  make(map[string]map[string]bool),
	}

	// Load existing groups from serverData vào memory
	dataMu.RLock()
	for _, g := range serverData.Groups {
		srv.groups[g.Name] = make(map[string]bool)
		for _, member := range g.Members {
			srv.groups[g.Name][member] = true
		}
	}
	dataMu.RUnlock()

	return srv
}

// Register unary
func (s *chatServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	dataMu.Lock()
	defer dataMu.Unlock()

	// Kiểm tra username đã tồn tại chưa
	for _, u := range serverData.Users {
		if u.Username == req.Username {
			return &pb.RegisterResponse{Ok: false, Message: "username already exists"}, nil
		}
	}

	// Tạo user mới
	serverData.Users = append(serverData.Users, User{
		Username: req.Username,
		Password: req.Password,
	})
	saveData(serverData)

	log.Printf("User registered: %s", req.Username)
	return &pb.RegisterResponse{Ok: true, Message: "registered successfully"}, nil
}

// Login
func (s *chatServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	dataMu.RLock()
	defer dataMu.RUnlock()

	for _, u := range serverData.Users {
		if u.Username == req.Username && u.Password == req.Password {
			log.Printf("User logged in: %s", req.Username)
			return &pb.LoginResponse{Ok: true, Message: "login successful"}, nil
		}
	}
	log.Printf("Failed login attempt: %s", req.Username)
	return &pb.LoginResponse{Ok: false, Message: "invalid credentials"}, nil
}

// List users (chỉ những user đang online)
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
	if req.GroupName == "" {
		return &pb.CreateGroupResponse{Ok: false, Message: "empty group name"}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Kiểm tra group đã tồn tại chưa
	if _, ok := s.groups[req.GroupName]; ok {
		return &pb.CreateGroupResponse{Ok: false, Message: "group already exists"}, nil
	}

	// Tạo group trong memory
	s.groups[req.GroupName] = make(map[string]bool)
	for _, m := range req.Members {
		s.groups[req.GroupName][m] = true
	}

	// Lưu vào persistent storage
	dataMu.Lock()
	serverData.Groups = append(serverData.Groups, Group{
		Name:    req.GroupName,
		Members: req.Members,
	})
	saveData(serverData)
	dataMu.Unlock()

	log.Printf("Group created: %s with %d members", req.GroupName, len(req.Members))
	return &pb.CreateGroupResponse{Ok: true, Message: "group created"}, nil
}

func (s *chatServer) JoinGroup(ctx context.Context, req *pb.JoinGroupRequest) (*pb.JoinGroupResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Tạo group nếu chưa tồn tại (hoặc có thể return error)
	if _, ok := s.groups[req.GroupName]; !ok {
		s.groups[req.GroupName] = make(map[string]bool)
	}

	// Thêm user vào group
	s.groups[req.GroupName][req.Username] = true

	// Cập nhật persistent storage
	dataMu.Lock()
	found := false
	for i, g := range serverData.Groups {
		if g.Name == req.GroupName {
			// Kiểm tra user đã có trong members chưa
			memberExists := false
			for _, m := range g.Members {
				if m == req.Username {
					memberExists = true
					break
				}
			}
			if !memberExists {
				serverData.Groups[i].Members = append(serverData.Groups[i].Members, req.Username)
			}
			found = true
			break
		}
	}
	// Nếu group chưa có trong persistent storage, tạo mới
	if !found {
		serverData.Groups = append(serverData.Groups, Group{
			Name:    req.GroupName,
			Members: []string{req.Username},
		})
	}
	saveData(serverData)
	dataMu.Unlock()

	log.Printf("User %s joined group: %s", req.Username, req.GroupName)
	return &pb.JoinGroupResponse{Ok: true, Message: "joined successfully"}, nil
}

// ChatStream bi-directional
func (s *chatServer) ChatStream(stream pb.ChatService_ChatStreamServer) error {
	// Nhận message đầu tiên để lấy username
	initMsg, err := stream.Recv()
	if err != nil {
		return err
	}
	username := initMsg.From
	if username == "" {
		return errors.New("must send initial message with From=username")
	}

	// Tạo session
	sess := &clientSession{
		username: username,
		send:     make(chan *pb.ChatMessage, 100),
	}

	// Đăng ký client
	s.mu.Lock()
	if _, exists := s.clients[username]; exists {
		s.mu.Unlock()
		return fmt.Errorf("username %s already connected", username)
	}
	s.clients[username] = sess
	s.mu.Unlock()

	log.Printf("User connected: %s", username)

	// Goroutine để gửi messages đến client
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range sess.send {
			if err := stream.Send(msg); err != nil {
				log.Printf("send error to %s: %v", username, err)
				return
			}
		}
	}()

	// Xử lý initial message nếu có text
	if initMsg.Text != "" && initMsg.Type != "connect" {
		initMsg.Timestamp = time.Now().Unix()
		s.handleIncoming(initMsg)
	}

	// Read loop
	for {
		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Printf("client %s closed connection", username)
			} else {
				log.Printf("client %s error: %v", username, err)
			}
			s.removeClient(username)
			<-done // Đợi goroutine gửi kết thúc
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
		delete(s.clients, username)
		log.Printf("User disconnected: %s", username)
	}
	// Note: Không xóa user khỏi groups khi disconnect, giữ membership
}

func (s *chatServer) handleIncoming(msg *pb.ChatMessage) {
	// Lưu message vào persistent storage
	dataMu.Lock()
	serverData.Messages = append(serverData.Messages, MessageLog{
		From:      msg.From,
		To:        msg.To,
		Type:      msg.Type,
		Text:      msg.Text,
		Timestamp: msg.Timestamp,
	})
	saveData(serverData)
	dataMu.Unlock()

	switch msg.Type {
	case "private":
		s.mu.RLock()
		target, ok := s.clients[msg.To]
		s.mu.RUnlock()

		if ok {
			select {
			case target.send <- msg:
				log.Printf("[PM] %s -> %s: %s", msg.From, msg.To, msg.Text)
			default:
				log.Printf("user %s buffer full, cannot deliver private msg", msg.To)
			}
		} else {
			log.Printf("user %s offline, cannot deliver private msg from %s", msg.To, msg.From)
		}

	case "group":
		s.mu.RLock()
		members, ok := s.groups[msg.To]
		clientsSnapshot := make(map[string]*clientSession)
		if ok {
			for member := range members {
				if member == msg.From {
					continue // Không gửi lại cho người gửi
				}
				if c, online := s.clients[member]; online {
					clientsSnapshot[member] = c
				}
			}
		}
		s.mu.RUnlock()

		log.Printf("[GROUP %s] %s: %s (to %d members)", msg.To, msg.From, msg.Text, len(clientsSnapshot))
		for _, c := range clientsSnapshot {
			select {
			case c.send <- msg:
			default:
				log.Printf("member buffer full in group %s", msg.To)
			}
		}

	default:
		log.Printf("unknown msg type: %s from %s", msg.Type, msg.From)
	}
}

func main() {
	serverData = loadData()
	defer func() {
		dataMu.Lock()
		saveData(serverData)
		dataMu.Unlock()
	}()

	f, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("cannot open log file: %v", err)
	}
	defer f.Close()

	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcSrv := grpc.NewServer()
	pb.RegisterChatServiceServer(grpcSrv, newServer())

	log.Println("=================================")
	log.Println("gRPC Chat Server listening on :50051")
	log.Println("=================================")

	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("serve error: %v", err)
	}
}
