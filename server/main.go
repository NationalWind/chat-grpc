package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pb "chat-grpc/chat-grpc/proto"

	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: client <username>")
		return
	}
	username := os.Args[1]
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewChatServiceClient(conn)

	// Optional: register (unary)
	reg, err := client.Register(context.Background(), &pb.RegisterRequest{Username: username})
	if err != nil || !reg.Ok {
		log.Fatalf("register failed: %v %v", reg, err)
	}
	fmt.Println("Registered:", reg.Message)

	// open ChatStream
	stream, err := client.ChatStream(context.Background())
	if err != nil {
		log.Fatalf("open stream: %v", err)
	}

	// send initial connect message
	init := &pb.ChatMessage{From: username, Type: "connect", Text: ""}
	if err := stream.Send(init); err != nil {
		log.Fatalf("send init: %v", err)
	}

	// goroutine: receive messages from server
	go func() {
		for {
			in, err := stream.Recv()
			if err != nil {
				log.Printf("recv error: %v", err)
				return
			}
			// display
			ts := time.Unix(in.Timestamp, 0).Format("15:04:05")
			if in.Type == "private" {
				fmt.Printf("[%s][PM][%s -> you]: %s\n", ts, in.From, in.Text)
			} else if in.Type == "group" {
				fmt.Printf("[%s][GROUP %s][%s]: %s\n", ts, in.To, in.From, in.Text)
			} else {
				fmt.Printf("[%s][%s]: %s\n", ts, in.From, in.Text)
			}
		}
	}()

	// read stdin commands
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Commands:")
	fmt.Println("/pm <user> <message>  -- private message")
	fmt.Println("/group <group> <message> -- send to group")
	fmt.Println("/create_group <group>  -- create group (via unary)")
	fmt.Println("/join_group <group>  -- join group (via unary)")
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
				From: username, To: parts[1], Type: "private", Text: parts[2], Timestamp: time.Now().Unix(),
			}
			if err := stream.Send(msg); err != nil {
				fmt.Println("send error:", err)
			}
		} else if strings.HasPrefix(line, "/group ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) < 3 {
				fmt.Println("usage /group <group> <message>")
				continue
			}
			msg := &pb.ChatMessage{
				From: username, To: parts[1], Type: "group", Text: parts[2], Timestamp: time.Now().Unix(),
			}
			if err := stream.Send(msg); err != nil {
				fmt.Println("send error:", err)
			}
		} else if strings.HasPrefix(line, "/create_group ") {
			parts := strings.SplitN(line, " ", 2)
			grp := parts[1]
			_, err := client.CreateGroup(context.Background(), &pb.CreateGroupRequest{GroupName: grp})
			if err != nil {
				fmt.Println("create group err:", err)
			} else {
				fmt.Println("group created:", grp)
			}
		} else if strings.HasPrefix(line, "/join_group ") {
			parts := strings.SplitN(line, " ", 2)
			grp := parts[1]
			_, err := client.JoinGroup(context.Background(), &pb.JoinGroupRequest{GroupName: grp, Username: username})
			if err != nil {
				fmt.Println("join group err:", err)
			} else {
				fmt.Println("joined group:", grp)
			}
		} else {
			fmt.Println("unknown command")
		}
	}
}
