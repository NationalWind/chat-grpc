package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	pb "chat-grpc/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Welcome to gRPC Chat!")
	fmt.Println("1) Register")
	fmt.Println("2) Login")
	fmt.Print("Choose: ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	fmt.Print("Enter username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Enter password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	// Setup logging cho client với username trong tên file
	logFileName := "client.log"
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("cannot open log file: %v", err)
	}
	defer logFile.Close()

	// Custom logger với prefix là username
	logger := log.New(io.MultiWriter(os.Stdout, logFile), fmt.Sprintf("[%s] ", username), log.LstdFlags|log.Lmicroseconds)

	logger.Printf("Client starting for user: %s", username)

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer conn.Close()
	log.Println("Connected to server at localhost:50051")
	client := pb.NewChatServiceClient(conn)

	// Xử lý Register hoặc Login dựa trên choice
	if choice == "1" {
		log.Printf("Attempting to register user: %s", username)
		res, err := client.Register(context.Background(), &pb.RegisterRequest{Username: username, Password: password})
		if err != nil || !res.Ok {
			log.Fatalf("register failed: %v", err)
		}
		log.Printf("Registration successful: %s", res.Message)
	} else {
		log.Printf("Attempting to login user: %s", username)
		res, err := client.Login(context.Background(), &pb.LoginRequest{Username: username, Password: password})
		if err != nil || !res.Ok {
			log.Fatalf("login failed: %v", err)
		}
		log.Println("Login successful")
	}

	// Open ChatStream
	log.Println("Opening chat stream...")
	stream, err := client.ChatStream(context.Background())
	if err != nil {
		log.Fatalf("open stream: %v", err)
	}

	// Send initial connect message
	init := &pb.ChatMessage{From: username, Type: "connect", Text: ""}
	if err := stream.Send(init); err != nil {
		log.Fatalf("send init: %v", err)
	}
	log.Println("Chat stream established")

	// Goroutine: receive messages from server
	go func() {
		for {
			in, err := stream.Recv()
			if err != nil {
				logger.Printf("recv error: %v", err)
				return
			}
			// Display message
			ts := time.Unix(in.Timestamp, 0).Format("15:04:05")
			switch in.Type {
			case "private":
				fmt.Printf("[%s][PM][%s -> you]: %s\n", ts, in.From, in.Text)
				logger.Printf("Received PM from %s: %s", in.From, in.Text)
			case "group":
				fmt.Printf("[%s][GROUP %s][%s]: %s\n", ts, in.To, in.From, in.Text)
				logger.Printf("Received group message in %s from %s: %s", in.To, in.From, in.Text)
			default:
				fmt.Printf("[%s][%s]: %s\n", ts, in.From, in.Text)
				logger.Printf("Received message from %s: %s", in.From, in.Text)
			}
		}
	}()

	fmt.Println("\nCommands:")
	fmt.Println("/pm <user> <message>  -- private message")
	fmt.Println("/group <group> <message> -- send to group")
	fmt.Println("/create_group <group>  -- create group")
	fmt.Println("/join_group <group>  -- join group")
	fmt.Println("/my_groups  -- list of your groups")
	fmt.Println("/list_users  -- list of online users")

	// Read stdin commands
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/pm ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) < 3 {
				fmt.Println("usage /pm <user> <message>")
				continue
			}
			msg := &pb.ChatMessage{
				From:      username,
				To:        parts[1],
				Type:      "private",
				Text:      parts[2],
				Timestamp: time.Now().Unix(),
			}
			if err := stream.Send(msg); err != nil {
				logger.Printf("Error sending private message to %s: %v", parts[1], err)
				fmt.Println("send error:", err)
			} else {
				logger.Printf("Sent private message to %s: %s", parts[1], parts[2])
			}
		} else if strings.HasPrefix(line, "/group ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) < 3 {
				fmt.Println("usage /group <group> <message>")
				continue
			}
			msg := &pb.ChatMessage{
				From:      username,
				To:        parts[1],
				Type:      "group",
				Text:      parts[2],
				Timestamp: time.Now().Unix(),
			}
			if err := stream.Send(msg); err != nil {
				logger.Printf("Error sending group message to %s: %v", parts[1], err)
				fmt.Println("send error:", err)
			} else {
				logger.Printf("Sent group message to %s: %s", parts[1], parts[2])
			}
		} else if strings.HasPrefix(line, "/create_group ") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) < 2 {
				fmt.Println("usage /create_group <group>")
				continue
			}
			grp := parts[1]
			logger.Printf("Creating group: %s", grp)
			_, err := client.CreateGroup(context.Background(), &pb.CreateGroupRequest{
				GroupName: grp,
				Members:   []string{username}, // Thêm creator vào group
			})
			if err != nil {
				logger.Printf("Error creating group %s: %v", grp, err)
				fmt.Println("create group err:", err)
			} else {
				logger.Printf("Group created successfully: %s", grp)
				fmt.Printf("Group '%s' created and you've joined it!\n", grp)
			}
		} else if strings.HasPrefix(line, "/join_group ") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) < 2 {
				fmt.Println("usage /join_group <group>")
				continue
			}
			grp := parts[1]
			logger.Printf("Joining group: %s", grp)
			_, err := client.JoinGroup(context.Background(), &pb.JoinGroupRequest{GroupName: grp, Username: username})
			if err != nil {
				logger.Printf("Error joining group %s: %v", grp, err)
				fmt.Println("join group err:", err)
			} else {
				logger.Printf("Joined group successfully: %s", grp)
				fmt.Println("joined group:", grp)
			}
		} else if line == "/my_groups" {
			logger.Println("Requesting user groups list")
			res, err := client.GetUserGroups(context.Background(), &pb.GetUserGroupsRequest{Username: username})
			if err != nil {
				logger.Printf("Error getting groups: %v", err)
				fmt.Println("get groups err:", err)
			} else {
				logger.Printf("Retrieved %d groups", len(res.Groups))
				if len(res.Groups) == 0 {
					fmt.Println("You haven't joined any groups yet.")
				} else {
					fmt.Printf("Your groups (%d):\n", len(res.Groups))
					for _, grp := range res.Groups {
						fmt.Printf("  - %s (%d members)\n", grp.Name, len(grp.Members))
					}
				}
			}
		} else if line == "/list_users" {
			logger.Println("Requesting online users list")
			res, err := client.ListUsers(context.Background(), &pb.Empty{})
			if err != nil {
				logger.Printf("Error listing users: %v", err)
				fmt.Println("list users err:", err)
			} else {
				logger.Printf("Retrieved %d online users", len(res.Users))
				if len(res.Users) == 0 {
					fmt.Println("No users online.")
				} else {
					fmt.Printf("Online users (%d):\n", len(res.Users))
					for _, u := range res.Users {
						if u.Username == username {
							fmt.Printf("  - %s (you)\n", u.Username)
						} else {
							fmt.Printf("  - %s\n", u.Username)
						}
					}
				}
			}
		} else {
			fmt.Println("unknown command")
		}
	}
}
