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
7. [Testing](#7-testing)
8. [File log](#8-file-log)
9. [Video demo](#9-video-demo)
10. [Đánh giá và phân công](#10-đánh-giá-và-phân-công)
11. [Phụ lục](#11-phụ-lục)

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
- Lưu trữ dữ liệu persistent với PostgreSQL database
- Tìm kiếm người dùng với fuzzy search (pg_trgm)
- Bảo mật password với bcrypt hashing
- Theo dõi trạng thái online/offline của người dùng

### 1.3. Công nghệ sử dụng
- **Ngôn ngữ**: Go (Golang)
- **Framework gRPC**: google.golang.org/grpc
- **Protocol**: Protocol Buffers (protobuf)
- **Database**: PostgreSQL 13+
- **ORM**: GORM (Go Object Relational Mapper)
- **Authentication**: bcrypt password hashing
- **Search**: PostgreSQL pg_trgm extension (Fuzzy Search)

---

## 2. YÊU CẦU HỆ THỐNG

### 2.1. Phần mềm cần thiết
```
- Go version 1.20 trở lên
- PostgreSQL 13+ (với pg_trgm extension)
- Protocol Buffer Compiler (protoc)
- Go plugins cho protoc:
  + protoc-gen-go
  + protoc-gen-go-grpc
```

### 2.2. Cài đặt PostgreSQL -- need install docker
```bash
docker compose up -d 
```

### 2.3. Cài đặt Go dependencies
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
│   └── main.go             # Server implementation
│   └── server.log          # Server log file (optional)
├── client/
│   └── main.go             # Client implementation
│   └── client.log          # Client log file (optional)
├── database/
│   └── database.go         # Database layer với GORM
├── go.mod
├── go.sum
└── README.md               # Document
```

### 3.2. Compile proto file
```bash
protoc --go_out=. --go-grpc_out=. proto/chat.proto
```

### 3.3. Chạy Server
```bash
# Đảm bảo PostgreSQL đang chạy và database đã được tạo
# Server sẽ tự động migrate schema khi khởi động

# Terminal 1: Server
cd server
go run main.go

# Hoặc build binary
go build -o server-bin main.go
./server-bin
```

### 3.4. Chạy Client (ít nhất 5 clients)
```bash
# Terminal 2: Client 1 (Alice)
cd client
go run main.go

# Terminal 3: Client 2 (Bob)
go run main.go

# Terminal 4: Client 3 (Charlie)
go run main.go

# Terminal 5: Client 4 (Diana)
go run main.go

# Terminal 6: Client 5 (Eve)
go run main.go

# Hoặc build binary
go build -o client-bin main.go
./client-bin
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
  rpc SearchUsers(SearchUsersRequest) returns (SearchUsersResponse);
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
}

// Database models (GORM)
type User struct {
    ID          uint      `gorm:"primaryKey"`
    Username    string    `gorm:"uniqueIndex;size:50;not null"`
    Password    string    `gorm:"size:255;not null"` // bcrypt hashed
    DisplayName string    `gorm:"size:100"`
    CreatedAt   time.Time
    LastSeen    time.Time
    IsOnline    bool      `gorm:"default:false;index"`
}

type Group struct {
    ID        uint      `gorm:"primaryKey"`
    Name      string    `gorm:"uniqueIndex;size:100;not null"`
    CreatedAt time.Time
}

type GroupMember struct {
    ID       uint      `gorm:"primaryKey"`
    GroupID  uint      `gorm:"not null;index"`
    Username string    `gorm:"size:50;not null;index"`
    JoinedAt time.Time
}

type Message struct {
    ID          uint      `gorm:"primaryKey"`
    FromUser    string    `gorm:"size:50;not null;index"`
    ToTarget    string    `gorm:"size:100;not null;index"`
    MessageType string    `gorm:"size:20;not null;index"`
    Text        string    `gorm:"type:text;not null"`
    CreatedAt   time.Time
}
```

**Xử lý đồng thời**:
- Sử dụng `sync.RWMutex` để đảm bảo thread-safety
- Mỗi client connection có goroutine riêng để nhận/gửi messages
- Channels được sử dụng cho communication giữa goroutines

**Database Layer**:
- Sử dụng GORM làm ORM layer
- Auto-migration schema khi khởi động server
- Connection pooling được quản lý bởi GORM
- Prepared statements cho an toàn trước SQL injection
- Indexes được tạo cho các trường thường xuyên query (username, is_online, group_id)
- GIN index (pg_trgm) cho fuzzy search hiệu suất cao

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
- Kiểm tra username trùng lặp trong database
- Password được hash bằng bcrypt (chi phí = 10)
- Lưu trữ persistent vào PostgreSQL database
- Tự động tạo DisplayName = Username khi đăng ký

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

### 6.5. Tìm kiếm người dùng (Fuzzy Search)

**Cú pháp**:
```bash
/search <query>
```

**Ví dụ**:
```bash
# Tìm kiếm với từ khóa
/search alice

Search results for 'alice' (3 users):
  - alice (alice) [online] (you)
  - alice2023 (Alice Smith) [offline]
  - alicia (Alicia Johnson) [online]

# Tìm kiếm với tên gần giống
/search bob

Search results for 'bob' (2 users):
  - bob (bob) [online]
  - bobby (Bobby Tables) [offline]
```

**Tính năng**:
- **Fuzzy matching** sử dụng PostgreSQL pg_trgm extension
- Tìm kiếm trên cả `username` và `display_name`
- Case-insensitive (không phân biệt hoa thường)
- Partial matching (khớp một phần chuỗi)
- Similarity score threshold = 0.3
- Hiển thị trạng thái online/offline
- Mặc định giới hạn 20 kết quả (có thể điều chỉnh)
- Sắp xếp theo độ tương đồng (similarity score)

**Chi tiết kỹ thuật**:
```sql
-- Fuzzy search query sử dụng trong database layer
SELECT DISTINCT ON (username)
    id, username, display_name, created_at, last_seen, is_online,
    GREATEST(
        similarity(LOWER(username), LOWER(?)),
        similarity(LOWER(COALESCE(display_name, '')), LOWER(?))
    ) as similarity_score
FROM users
WHERE
    LOWER(username) LIKE LOWER(?) OR
    LOWER(display_name) LIKE LOWER(?) OR
    similarity(LOWER(username), LOWER(?)) > 0.3 OR
    similarity(LOWER(COALESCE(display_name, '')), LOWER(?)) > 0.3
ORDER BY username, similarity_score DESC, is_online DESC
LIMIT ?
```

### 6.6. Danh sách commands

| Command | Mô tả |
|---------|-------|
| `/pm <user> <message>` | Gửi tin nhắn riêng |
| `/group <group> <message>` | Gửi tin nhắn nhóm |
| `/create_group <group>` | Tạo nhóm mới |
| `/join_group <group>` | Tham gia nhóm |
| `/my_groups` | Xem nhóm đã join |
| `/list_users` | Xem users online |
| `/search <query>` | Tìm kiếm người dùng (fuzzy search) |

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

 **Link video demo**: [https://drive.google.com/file/d/1-j-xqcE8_6X7jowlN5yobDyoB92go3sD/view?usp=sharing]
---

## 9. ĐÁNH GIÁ VÀ PHÂN CÔNG

### 9.1. Phân công đồ án

| Tính năng | Trạng thái | Phân công |
|---------|-----------|---------|
| **Server** |||
| Đăng ký user với bcrypt | Hoàn thành | Trọng Nghĩa |
| Danh sách online users | Hoàn thành | Quốc Phong |
| Tạo nhóm chat | Hoàn thành | Quốc Phong |
| Broadcast trong nhóm | Hoàn thành | Quốc Phong |
| Chat riêng 1-1 | Hoàn thành | Quốc Phong |
| Fuzzy search users | Hoàn thành | Trọng Nghĩa |
| **Client** |||
| Đăng ký/Đăng nhập | Hoàn thành | Quốc Phong |
| Gửi private message | Hoàn thành | Quốc Phong |
| Gửi group message | Hoàn thành | Quốc Phong |
| Join group | Hoàn thành | Quốc Phong |
| Search command | Hoàn thành | Trọng Nghĩa |
| **Database & Security** |||
| PostgreSQL integration | Hoàn thành | Trọng Nghĩa |
| GORM ORM layer | Hoàn thành | Trọng Nghĩa |
| Password hashing (bcrypt) | Hoàn thành | Trọng Nghĩa |
| Fuzzy search (pg_trgm) | Hoàn thành | Trọng Nghĩa |
| User online status | Hoàn thành | Trọng Nghĩa |
| **Khác** |||
| Log files | Hoàn thành | Quốc Phong |

### 9.2. Đánh giá

Đồ án đã hoàn thành đầy đủ các yêu cầu:
- ✅ Sử dụng gRPC cho inter-process communication
- ✅ Implement đầy đủ tính năng chat riêng và chat nhóm
- ✅ Hỗ trợ 5+ concurrent users
- ✅ Có persistent storage với PostgreSQL database
- ✅ Có log files chi tiết (optional, có thể bật/tắt)
- ✅ Code clean, dễ maintain với separation of concerns

**Điểm nổi bật**:
- 🔐 **Security**: Sử dụng bcrypt để hash password với cost factor = 10
- 🔍 **Advanced Search**: Fuzzy search với PostgreSQL pg_trgm extension
- 💾 **Database Design**: Schema được normalize với proper indexes
- 📊 **ORM Integration**: Sử dụng GORM với auto-migration
- 🟢 **Real-time Status**: Theo dõi trạng thái online/offline
- ⚡ **Performance**: GIN indexes cho full-text search hiệu suất cao

Qua đồ án này, nhóm đã:
- Nắm vững cách sử dụng gRPC framework (Unary và Streaming RPCs)
- Hiểu rõ về bidirectional streaming communication
- Áp dụng concurrent programming với goroutines và channels
- Xử lý client-server communication patterns
- Implement persistent storage với PostgreSQL và GORM ORM
- Áp dụng security best practices (bcrypt, prepared statements)
- Tối ưu database với indexes và full-text search

---

## 10. PHỤ LỤC

### 10.1. Database Schema (PostgreSQL)

**Bảng users**:
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,  -- bcrypt hashed
    display_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_online BOOLEAN DEFAULT FALSE
);

-- Indexes for performance
CREATE INDEX idx_users_is_online ON users(is_online);
CREATE INDEX idx_users_username_trgm ON users USING gin(username gin_trgm_ops);
CREATE INDEX idx_users_display_name_trgm ON users USING gin(display_name gin_trgm_ops);
```

**Bảng groups**:
```sql
CREATE TABLE groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Bảng group_members** (Many-to-Many):
```sql
CREATE TABLE group_members (
    id SERIAL PRIMARY KEY,
    group_id INTEGER NOT NULL REFERENCES groups(id),
    username VARCHAR(50) NOT NULL,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_group_members_group_id ON group_members(group_id);
CREATE INDEX idx_group_members_username ON group_members(username);
```

**Bảng messages**:
```sql
CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    from_user VARCHAR(50) NOT NULL,
    to_target VARCHAR(100) NOT NULL,  -- username or group name
    message_type VARCHAR(20) NOT NULL,  -- 'private' or 'group'
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_from_user ON messages(from_user);
CREATE INDEX idx_messages_to_target ON messages(to_target);
CREATE INDEX idx_messages_type ON messages(message_type);
```

**Sample data**:
```sql
-- Users (passwords are bcrypt hashed)
INSERT INTO users (username, password, display_name, is_online) VALUES
  ('alice', '$2a$10$...hashed...', 'alice', true),
  ('bob', '$2a$10$...hashed...', 'bob', true);

-- Groups
INSERT INTO groups (name) VALUES ('project-team'), ('general');

-- Group members
INSERT INTO group_members (group_id, username) VALUES
  (1, 'alice'), (1, 'bob'), (1, 'charlie'),
  (2, 'alice'), (2, 'bob'), (2, 'charlie'), (2, 'diana'), (2, 'eve');

-- Messages
INSERT INTO messages (from_user, to_target, message_type, text) VALUES
  ('alice', 'bob', 'private', 'Hello Bob!'),
  ('alice', 'project-team', 'group', 'Meeting at 3pm');
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
  string display_name = 2;
  bool is_online = 3;
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

message SearchUsersRequest {
  string query = 1;
  int32 limit = 2;  // optional, default 20
}

message SearchUsersResponse {
  repeated UserInfo users = 1;
}
```

### 10.3. Cấu hình Database

**File**: `database/database.go` - Config struct

```go
type Config struct {
    Host     string  // Default: "localhost"
    Port     int     // Default: 5430
    User     string  // Default: "chatapp"
    Password string  // Default: "chatapp123"
    DBName   string  // Default: "chatapp"
    SSLMode  string  // Default: "disable"
}
```

**Customize database config** (nếu cần):
```go
// Trong server/main.go
cfg := database.Config{
    Host:     "your-host",
    Port:     5432,
    User:     "your-user",
    Password: "your-password",
    DBName:   "your-dbname",
    SSLMode:  "require",  // enable SSL nếu production
}
db, err := database.Connect(cfg)
```

### 10.4. References

- [gRPC Official Documentation](https://grpc.io/docs/)
- [Protocol Buffers Guide](https://protobuf.dev/)
- [Go gRPC Tutorial](https://grpc.io/docs/languages/go/quickstart/)
- [Effective Go](https://go.dev/doc/effective_go)
- [GORM Documentation](https://gorm.io/docs/)
- [PostgreSQL pg_trgm](https://www.postgresql.org/docs/current/pgtrgm.html)
- [bcrypt Password Hashing](https://pkg.go.dev/golang.org/x/crypto/bcrypt)

---