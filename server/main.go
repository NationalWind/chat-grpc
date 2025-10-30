package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	pb "chat-grpc/proto"
	"chat-grpc/database"

	"google.golang.org/grpc"
)

type clientSession struct {
	username string
	send     chan *pb.ChatMessage
}

var (
	db *database.DB
)

// Server implementation
type chatServer struct {
	pb.UnimplementedChatServiceServer
	mu      sync.RWMutex
	clients map[string]*clientSession
}

func newServer() *chatServer {
	return &chatServer{
		clients: make(map[string]*clientSession),
	}
}

// Register unary
func (s *chatServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// Kiểm tra username đã tồn tại chưa
	exists, err := db.UserExists(req.Username)
	if err != nil {
		log.Printf("Error checking user existence: %v", err)
		return &pb.RegisterResponse{Ok: false, Message: "database error"}, nil
	}

	if exists {
		return &pb.RegisterResponse{Ok: false, Message: "username already exists"}, nil
	}

	// Tạo user mới với password hash
	_, err = db.CreateUser(req.Username, req.Password)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return &pb.RegisterResponse{Ok: false, Message: "failed to create user"}, nil
	}

	log.Printf("User registered: %s", req.Username)
	return &pb.RegisterResponse{Ok: true, Message: "registered successfully"}, nil
}

// Login
func (s *chatServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// Authenticate user với bcrypt password check
	_, err := db.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		log.Printf("Failed login attempt for %s: %v", req.Username, err)
		return &pb.LoginResponse{Ok: false, Message: "invalid credentials"}, nil
	}

	log.Printf("User logged in: %s", req.Username)
	return &pb.LoginResponse{Ok: true, Message: "login successful"}, nil
}

// List users (chỉ những user đang online)
func (s *chatServer) ListUsers(ctx context.Context, _ *pb.Empty) (*pb.ListUsersResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := &pb.ListUsersResponse{}
	for name := range s.clients {
		resp.Users = append(resp.Users, &pb.UserInfo{
			Username:    name,
			DisplayName: name,
			IsOnline:    true,
		})
	}
	return resp, nil
}

// SearchUsers - Tìm kiếm user với fuzzy search
func (s *chatServer) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest) (*pb.SearchUsersResponse, error) {
	resp := &pb.SearchUsersResponse{}

	// Validate query
	if req.Query == "" {
		return resp, nil
	}

	// Set default limit
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}

	// Tìm kiếm trong database với fuzzy search
	users, err := db.SearchUsers(req.Query, limit)
	if err != nil {
		log.Printf("Error searching users for query '%s': %v", req.Query, err)
		return resp, nil
	}

	// Chuyển đổi sang protobuf response
	for _, user := range users {
		resp.Users = append(resp.Users, &pb.UserInfo{
			Username:    user.Username,
			DisplayName: user.DisplayName,
			IsOnline:    user.IsOnline,
		})
	}

	log.Printf("Search query '%s' returned %d users", req.Query, len(resp.Users))
	return resp, nil
}

// GetUserGroups - Lấy danh sách groups mà user đã join
func (s *chatServer) GetUserGroups(ctx context.Context, req *pb.GetUserGroupsRequest) (*pb.GetUserGroupsResponse, error) {
	resp := &pb.GetUserGroupsResponse{}

	// Lấy groups từ database
	groups, err := db.GetUserGroups(req.Username)
	if err != nil {
		log.Printf("Error getting user groups for %s: %v", req.Username, err)
		return resp, nil
	}

	// Chuyển đổi sang protobuf response
	for _, group := range groups {
		// Lấy members của group
		members, err := db.GetGroupMembers(group.Name)
		if err != nil {
			log.Printf("Error getting group members for %s: %v", group.Name, err)
			continue
		}

		resp.Groups = append(resp.Groups, &pb.GroupInfo{
			Name:    group.Name,
			Members: members,
		})
	}

	return resp, nil
}

func (s *chatServer) CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.CreateGroupResponse, error) {
	if req.GroupName == "" {
		return &pb.CreateGroupResponse{Ok: false, Message: "empty group name"}, nil
	}

	// Kiểm tra group đã tồn tại chưa
	exists, err := db.GroupExists(req.GroupName)
	if err != nil {
		log.Printf("Error checking group existence: %v", err)
		return &pb.CreateGroupResponse{Ok: false, Message: "database error"}, nil
	}

	if exists {
		return &pb.CreateGroupResponse{Ok: false, Message: "group already exists"}, nil
	}

	// Đảm bảo có ít nhất creator trong group
	membersList := req.Members
	if len(membersList) == 0 {
		return &pb.CreateGroupResponse{Ok: false, Message: "creator must be included in members"}, nil
	}

	// Tạo group trong database
	_, err = db.CreateGroup(req.GroupName)
	if err != nil {
		log.Printf("Error creating group: %v", err)
		return &pb.CreateGroupResponse{Ok: false, Message: "failed to create group"}, nil
	}

	// Thêm members vào group
	for _, m := range membersList {
		if err := db.AddGroupMember(req.GroupName, m); err != nil {
			log.Printf("Error adding member %s to group %s: %v", m, req.GroupName, err)
		}
	}

	log.Printf("Group created: %s with %d members (creator: %s)", req.GroupName, len(membersList), membersList[0])
	return &pb.CreateGroupResponse{Ok: true, Message: "group created and you've joined"}, nil
}

func (s *chatServer) JoinGroup(ctx context.Context, req *pb.JoinGroupRequest) (*pb.JoinGroupResponse, error) {
	// Kiểm tra group đã tồn tại chưa
	exists, err := db.GroupExists(req.GroupName)
	if err != nil {
		log.Printf("Error checking group existence: %v", err)
		return &pb.JoinGroupResponse{Ok: false, Message: "database error"}, nil
	}

	// Tạo group nếu chưa tồn tại
	if !exists {
		_, err := db.CreateGroup(req.GroupName)
		if err != nil {
			log.Printf("Error creating group: %v", err)
			return &pb.JoinGroupResponse{Ok: false, Message: "failed to create group"}, nil
		}
	}

	// Thêm user vào group
	if err := db.AddGroupMember(req.GroupName, req.Username); err != nil {
		log.Printf("Error adding user %s to group %s: %v", req.Username, req.GroupName, err)
		return &pb.JoinGroupResponse{Ok: false, Message: "failed to join group"}, nil
	}

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

	// Cập nhật user status thành online
	if err := db.UpdateUserOnlineStatus(username, true); err != nil {
		log.Printf("Error updating user online status: %v", err)
	}

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

		// Cập nhật user status thành offline
		if err := db.UpdateUserOnlineStatus(username, false); err != nil {
			log.Printf("Error updating user offline status: %v", err)
		}

		log.Printf("User disconnected: %s", username)
	}
	// Note: Không xóa user khỏi groups khi disconnect, giữ membership
}

func (s *chatServer) handleIncoming(msg *pb.ChatMessage) {
	// Lưu message vào database
	if err := db.SaveMessage(msg.From, msg.To, msg.Type, msg.Text); err != nil {
		log.Printf("Error saving message: %v", err)
	}

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
		// Lấy members từ database
		members, err := db.GetGroupMembers(msg.To)
		if err != nil {
			log.Printf("Error getting group members for %s: %v", msg.To, err)
			return
		}

		s.mu.RLock()
		clientsSnapshot := make(map[string]*clientSession)
		for _, member := range members {
			if member == msg.From {
				continue // Không gửi lại cho người gửi
			}
			if c, online := s.clients[member]; online {
				clientsSnapshot[member] = c
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
	// Setup logging
	f, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("cannot open log file: %v", err)
	}
	defer f.Close()

	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Connect to database
	var dbErr error
	db, dbErr = database.Connect(database.DefaultConfig())
	if dbErr != nil {
		log.Fatalf("failed to connect to database: %v", dbErr)
	}
	defer db.Close()

	log.Println("Database connection established")

	// Setup gRPC server
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
