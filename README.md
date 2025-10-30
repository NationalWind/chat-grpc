# ĐỒ ÁN CHAT SỬ DỤNG gRPC

**Môn học**: Chuyên đề hệ thống phân tán  
**Ngày hoàn thành**: 30/10/2025  
**Sinh viên thực hiện**: *22127327 - Trần Quốc Phong*, *Nguyễn Trọng Nghĩa - 22127480*

---

## MỤC LỤC

1. [Giới thiệu](#1-giới-thiệu)
2. [Yêu cầu hệ thống](#2-yêu-cầu-hệ-thống)
3. [Cài đặt và chạy chương trình](#3-cài-đặt-và-chạy-chương-trình)
4. [Kiến trúc hệ thống](#4-kiến-trúc-hệ-thống)
5. [Chi tiết thiết kế](#5-chi-tiết-thiết-kế)
6. [Các tính năng](#6-các-tính-năng)
7. [File log](#7-file-log)
8. [Video demo](#8-video-demo)
9. [Đánh giá và kết luận](#9-đánh-giá-và-kết-luận)

---

## 1. GIỚI THIỆU

### 1.1. Mục tiêu đồ án
- Tìm hiểu và sử dụng gRPC để giao tiếp giữa các tiến trình
- Xây dựng ứng dụng chat đa người dùng với đầy đủ tính năng
- Áp dụng kiến thức về lập trình mạng, xử lý đồng thời và persistent storage

### 1.2. Mô tả tổng quan
Đồ án xây dựng hệ thống chat client-server sử dụng gRPC, cho phép:
- Nhiều người dùng đăng ký, đăng nhập và chat với nhau
- Chat riêng tư 1-1 giữa 2 người dùng
- Tạo nhóm chat và gửi tin nhắn broadcast trong nhóm
- Lưu trữ dữ liệu persistent (users, groups, messages)

### 1.3. Công nghệ sử dụng
- **Ngôn ngữ**: Go (Golang)
- **Framework gRPC**: google.golang.org/grpc
- **Protocol**: Protocol Buffers (protobuf)
- **Storage**: JSON file-based storage

---

## 2. YÊU CẦU HỆ THỐNG

### 2.1. Phần mềm cần thiết
```
- Go version 1.20 trở lên
- Protocol Buffer Compiler (protoc)
- Go plugins cho protoc:
  + protoc-gen-go
  + protoc-gen-go-grpc
```

### 2.2. Cài đặt dependencies
```bash
# Cài đặt Go (nếu chưa có)
# Download từ: https://golang.org/dl/

# Cài đặt protoc
# Download từ: https://github.com/protocolbuffers/protobuf/releases

# Cài đặt Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Clone project và cài đặt dependencies
go mod tidy
```

---

## 3. CÀI ĐẶT VÀ CHẠY CHƯƠNG TRÌNH

### 3.1. Cấu trúc thư mục
```
chat-grpc/
├── proto/
│   ├── chat.proto          # Định nghĩa Protocol Buffer
│   ├── chat.pb.go          # Generated code
│   └── chat_grpc.pb.go     # Generated gRPC code
├── server/
│   └── server.go           # Server implementation
│   └── server.log          # Server log file
├── client/
│   └── client.go           # Client implementation
│   └── client.log          # Client log file
├── data.json               # Persistent storage
├── go.mod
├── go.sum
└── README.md
```

### 3.2. Compile proto file
```bash
protoc --go_out=. --go-grpc_out=. proto/chat.proto
```

### 3.3. Chạy Server
```bash
# Terminal 1: Server
cd server
go run server.go
```

### 3.4. Chạy Client (ít nhất 5 clients)
```bash
# Terminal 2: Client 1 (Alice)
cd client
go run client.go

# Terminal 3: Client 2 (Bob)
go run client.go

# Terminal 4: Client 3 (Charlie)
go run client.go

# Terminal 5: Client 4 (Diana)
go run client.go

# Terminal 6: Client 5 (Eve)
go run client.go
```

### 3.5. Flow đăng ký/đăng nhập
```
Welcome to gRPC Chat!
1) Register
2) Login
Choose: 1
Enter username: alice
Enter password: 123456

Registered: registered successfully
```

---

## 4. KIẾN TRÚC HỆ THỐNG

### 4.1. Sơ đồ tổng quan
```
┌─────────────┐         gRPC          ┌─────────────┐
│  Client 1   │◄─────────────────────►│             │
│  (Alice)    │                       │             │
├─────────────┤                       │             │
│  Client 2   │◄─────────────────────►│   Server    │
│  (Bob)      │                       │   :50051    │
├─────────────┤                       │             │
│  Client 3   │◄─────────────────────►│             │
│  (Charlie)  │                       │             │
├─────────────┤                       │             │
│  Client 4   │◄─────────────────────►│             │
│  (Diana)    │                       │             │
├─────────────┤                       │             │
│  Client 5   │◄─────────────────────►│             │
│  (Eve)      │                       │             │
└──────┬──────┘                       └──────┬──────┘
       │                                     │
       └──────────────┐         ┌────────────┘
                      ▼         ▼
                ┌──────────────────────┐
                │  data.json           │
                │  server.log          │
                │  client.log (shared) │
                └──────────────────────┘
```

### 4.2. Luồng dữ liệu

**Chat riêng (Private Message)**:
```
Client A ──[PM]──► Server ──[Forward]──► Client B
```

**Chat nhóm (Group Message)**:
```
Client A ──[Group Msg]──► Server ──[Broadcast]──► Clients B, C, D, E
```

---

## 5. CHI TIẾT THIẾT KẾ

### 5.1. Protocol Buffer

File `proto/chat.proto` định nghĩa các message và service:

```protobuf
service ChatService {
  // Unary RPCs
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc CreateGroup(CreateGroupRequest) returns (CreateGroupResponse);
  rpc JoinGroup(JoinGroupRequest) returns (JoinGroupResponse);
  rpc ListUsers(Empty) returns (ListUsersResponse);
  rpc GetUserGroups(GetUserGroupsRequest) returns (GetUserGroupsResponse);
  
  // Bidirectional Streaming RPC
  rpc ChatStream(stream ChatMessage) returns (stream ChatMessage);
}
```

### 5.2. Kiến trúc Server

**Cấu trúc dữ liệu chính**:

```go
// In-memory data structures
type chatServer struct {
    mu      sync.RWMutex
    clients map[string]*clientSession      // Online users
    groups  map[string]map[string]bool     // Groups and members
}

// Persistent storage
type ServerData struct {
    Users    []User       `json:"users"`
    Groups   []Group      `json:"groups"`
    Messages []MessageLog `json:"messages"`
}
```

**Xử lý đồng thời**:
- Sử dụng `sync.RWMutex` để đảm bảo thread-safety
- Mỗi client connection có goroutine riêng để nhận/gửi messages
- Channels được sử dụng cho communication giữa goroutines

### 5.3. Kiến trúc Client

**Flow hoạt động**:
1. Kết nối đến server qua gRPC
2. Đăng ký hoặc đăng nhập
3. Mở bidirectional stream
4. Gửi initial "connect" message
5. Goroutine nhận messages từ server
6. Main loop đọc commands từ stdin và gửi đi

---

## 6. CÁC TÍNH NĂNG

### 6.1. Đăng ký và Đăng nhập

**Register**:
```bash
Choose: 1
Enter username: alice
Enter password: 123456
✓ Registered: registered successfully
```

**Login**:
```bash
Choose: 2
Enter username: alice
Enter password: 123456
✓ Login success!
```

**Tính năng**:
- Kiểm tra username trùng lặp
- Lưu password (trong thực tế nên hash)
- Persistent storage vào `data.json`

### 6.2. Chat riêng (Private Message)

**Cú pháp**:
```bash
/pm <username> <message>
```

**Ví dụ**:
```bash
# Alice gửi tin nhắn cho Bob
/pm bob Hello Bob, how are you?

# Bob nhận được:
[14:30:25][PM][alice -> you]: Hello Bob, how are you?
```

**Luồng xử lý**:
1. Client gửi `ChatMessage` với `type="private"` và `to="bob"`
2. Server nhận message, tìm Bob trong `clients` map
3. Nếu Bob online, forward message qua channel `bob.send`
4. Nếu Bob offline, log warning

### 6.3. Chat nhóm (Group Chat)

**Tạo nhóm**:
```bash
/create_group project-team
✓ Group 'project-team' created and you've joined it!
```

**Join nhóm**:
```bash
/join_group project-team
✓ joined group: project-team
```

**Gửi tin nhắn nhóm**:
```bash
/group project-team Hello everyone in the team!

# Các members nhận được:
[14:35:10][GROUP project-team][alice]: Hello everyone in the team!
```

**Xem nhóm đã join**:
```bash
/my_groups

Your groups (2):
  - project-team (5 members)
  - general (3 members)
```

### 6.4. Liệt kê users online

```bash
/list_users

Online users (5):
  - alice (you)
  - bob
  - charlie
  - diana
  - eve
```

### 6.5. Danh sách commands

| Command | Mô tả |
|---------|-------|
| `/pm <user> <message>` | Gửi tin nhắn riêng |
| `/group <group> <message>` | Gửi tin nhắn nhóm |
| `/create_group <group>` | Tạo nhóm mới |
| `/join_group <group>` | Tham gia nhóm |
| `/my_groups` | Xem nhóm đã join |
| `/list_users` | Xem users online |

---

## 7. FILE LOG

### 7.1. Server Log (`server.log`)

**Format**:
```
2025/10/30 14:25:30.123456 Loaded 5 users, 3 groups, 120 messages
2025/10/30 14:25:35.234567 User registered: alice
2025/10/30 14:25:40.345678 User logged in: bob
2025/10/30 14:25:45.456789 User connected: alice
2025/10/30 14:26:00.567890 [PM] alice -> bob: Hello!
2025/10/30 14:26:10.678901 Group created: project-team with 1 members (creator: alice)
2025/10/30 14:26:15.789012 User bob joined group: project-team
2025/10/30 14:26:20.890123 [GROUP project-team] alice: Meeting at 3pm (to 1 members)
2025/10/30 14:30:00.901234 User disconnected: bob
```

**Nội dung log**:
- Startup: Load data từ file
- Authentication: Register, Login
- Connection: User connect/disconnect
- Messages: Private và Group messages
- Group operations: Create, Join
- Errors: Connection errors, offline users

### 7.2. Client Log (`client.log`)

**Format**:
```
2025/10/30 14:25:35.123456 [alice] Client starting for user: alice
2025/10/30 14:25:36.567890 [bob] Client starting for user: bob
2025/10/30 14:26:00.345678 [alice] Sent private message to bob: Hello!
2025/10/30 14:26:00.456789 [bob] Received PM from alice: Hello!
2025/10/30 14:26:05.567890 [bob] Sent private message to alice: Hi Alice!
2025/10/30 14:26:05.678901 [alice] Received PM from bob: Hi Alice!
2025/10/30 14:26:10.789012 [charlie] Client starting for user: charlie
2025/10/30 14:26:15.890123 [alice] Creating group: project-team
2025/10/30 14:26:15.901234 [alice] Group created successfully: project-team
2025/10/30 14:26:20.012345 [bob] Joining group: project-team
2025/10/30 14:26:20.123456 [bob] Joined group successfully: project-team
2025/10/30 14:26:25.234567 [charlie] Joining group: project-team
2025/10/30 14:26:25.345678 [charlie] Joined group successfully: project-team
2025/10/30 14:26:30.456789 [alice] Sent group message to project-team: Meeting at 3pm
2025/10/30 14:26:30.567890 [bob] Received group message in project-team from alice: Meeting at 3pm
2025/10/30 14:26:30.678901 [charlie] Received group message in project-team from alice: Meeting at 3pm
2025/10/30 14:26:35.789012 [bob] Sent group message to project-team: Got it!
2025/10/30 14:26:35.890123 [alice] Received group message in project-team from bob: Got it!
2025/10/30 14:26:35.901234 [charlie] Received group message in project-team from bob: Got it!
```

**Nội dung log**:
- Connection establishment
- Message sending
- Group operations
- Errors

---

## 8. VIDEO DEMO

 **Link video demo**: [Thêm link YouTube/Google Drive ở đây]
---

## 9. ĐÁNH GIÁ VÀ PHÂN CÔNG

### 9.1. Phân công đồ án

| Tính năng | Trạng thái | Phân công |
|---------|-----------|---------|
| **Server** |||
| Đăng ký user | Hoàn thành | Trọng Nghĩa |
| Danh sách online users | Hoàn thành | Quốc Phong |
| Tạo nhóm chat | Hoàn thành | Quốc Phong |
| Broadcast trong nhóm | Hoàn thành | Quốc Phong |
| Chat riêng 1-1 | Hoàn thành | Quốc Phong |
| **Client** |||
| Đăng ký/Đăng nhập | Hoàn thành | Trọng Nghĩa |
| Gửi private message | Hoàn thành | Quốc Phong |
| Gửi group message | Hoàn thành | Quốc Phong |
| Join group | Hoàn thành | Quốc Phong |
| **Khác** |||
| Persistent storage | Hoàn thành | Trọng Nghĩa |
| Log files | Hoàn thành | Trọng Nghĩa |

### 9.2. Đánh giá

Đồ án đã hoàn thành đầy đủ các yêu cầu:
- Sử dụng gRPC cho inter-process communication
- Implement đầy đủ tính năng chat riêng và chat nhóm
- Hỗ trợ 5+ concurrent users
- Có persistent storage
- Có log files chi tiết
- Code clean, dễ maintain

Qua đồ án này, nhóm đã:
- Nắm vững cách sử dụng gRPC framework
- Hiểu rõ về Unary và Streaming RPCs
- Áp dụng concurrent programming với goroutines
- Xử lý client-server communication patterns
- Implement persistent storage và logging

---

## 10. PHỤ LỤC

### 10.1. Cấu trúc data.json

```json
{
  "users": [
    {
      "username": "alice",
      "password": "123456"
    },
    {
      "username": "bob",
      "password": "123456"
    }
  ],
  "groups": [
    {
      "name": "project-team",
      "members": ["alice", "bob", "charlie"]
    },
    {
      "name": "general",
      "members": ["alice", "bob", "charlie", "diana", "eve"]
    }
  ],
  "messages": [
    {
      "from": "alice",
      "to": "bob",
      "type": "private",
      "text": "Hello Bob!",
      "timestamp": 1730280000
    },
    {
      "from": "alice",
      "to": "project-team",
      "type": "group",
      "text": "Meeting at 3pm",
      "timestamp": 1730280060
    }
  ]
}
```

### 10.2. Sample proto file

```protobuf
syntax = "proto3";

package chat;
option go_package = "./proto";

service ChatService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc CreateGroup(CreateGroupRequest) returns (CreateGroupResponse);
  rpc JoinGroup(JoinGroupRequest) returns (JoinGroupResponse);
  rpc ListUsers(Empty) returns (ListUsersResponse);
  rpc GetUserGroups(GetUserGroupsRequest) returns (GetUserGroupsResponse);
  rpc ChatStream(stream ChatMessage) returns (stream ChatMessage);
}

message Empty {}

message RegisterRequest {
  string username = 1;
  string password = 2;
}

message RegisterResponse {
  bool ok = 1;
  string message = 2;
}

message LoginRequest {
  string username = 1;
  string password = 2;
}

message LoginResponse {
  bool ok = 1;
  string message = 2;
}

message ChatMessage {
  string from = 1;
  string to = 2;
  string type = 3;
  string text = 4;
  int64 timestamp = 5;
}

message CreateGroupRequest {
  string group_name = 1;
  repeated string members = 2;
}

message CreateGroupResponse {
  bool ok = 1;
  string message = 2;
}

message JoinGroupRequest {
  string group_name = 1;
  string username = 2;
}

message JoinGroupResponse {
  bool ok = 1;
  string message = 2;
}

message UserInfo {
  string username = 1;
}

message ListUsersResponse {
  repeated UserInfo users = 1;
}

message GroupInfo {
  string name = 1;
  repeated string members = 2;
}

message GetUserGroupsRequest {
  string username = 1;
}

message GetUserGroupsResponse {
  repeated GroupInfo groups = 1;
}
```

### 10.3. References

- [gRPC Official Documentation](https://grpc.io/docs/)
- [Protocol Buffers Guide](https://protobuf.dev/)
- [Go gRPC Tutorial](https://grpc.io/docs/languages/go/quickstart/)
- [Effective Go](https://go.dev/doc/effective_go)

---