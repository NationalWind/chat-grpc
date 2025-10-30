package database

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// User model for GORM
type User struct {
	ID          uint      `gorm:"primaryKey"`
	Username    string    `gorm:"uniqueIndex;size:50;not null"`
	Password    string    `gorm:"size:255;not null"` // Hashed password
	DisplayName string    `gorm:"size:100"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	LastSeen    time.Time `gorm:"autoUpdateTime"`
	IsOnline    bool      `gorm:"default:false;index"`
}

// TableName specifies the table name
func (User) TableName() string {
	return "users"
}

// Group model for GORM
type Group struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"uniqueIndex;size:100;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName specifies the table name
func (Group) TableName() string {
	return "groups"
}

// GroupMember model for GORM (many-to-many relationship)
type GroupMember struct {
	ID       uint      `gorm:"primaryKey"`
	GroupID  uint      `gorm:"not null;index"`
	Username string    `gorm:"size:50;not null;index"`
	JoinedAt time.Time `gorm:"autoCreateTime"`
	Group    Group     `gorm:"foreignKey:GroupID"`
}

// TableName specifies the table name
func (GroupMember) TableName() string {
	return "group_members"
}

// Message model for GORM
type Message struct {
	ID          uint      `gorm:"primaryKey"`
	FromUser    string    `gorm:"size:50;not null;index"`
	ToTarget    string    `gorm:"size:100;not null;index"` // username for private, group name for group
	MessageType string    `gorm:"size:20;not null;index"`  // 'private' or 'group'
	Text        string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

// TableName specifies the table name
func (Message) TableName() string {
	return "messages"
}

// Database connection
type DB struct {
	*gorm.DB
}

// Config for database connection
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// Default config for local development
func DefaultConfig() Config {
	return Config{
		Host:     "localhost",
		Port:     5430,
		User:     "chatapp",
		Password: "chatapp123",
		DBName:   "chatapp",
		SSLMode:  "disable",
	}
}

// Connect to database
func Connect(cfg Config) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto migrate the schema
	if err := db.AutoMigrate(&User{}, &Group{}, &GroupMember{}, &Message{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Enable pg_trgm extension for fuzzy search
	db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm")

	// Create GIN index for fuzzy search
	db.Exec("CREATE INDEX IF NOT EXISTS idx_users_username_trgm ON users USING gin(username gin_trgm_ops)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_users_display_name_trgm ON users USING gin(display_name gin_trgm_ops)")

	log.Println("Database connected successfully")
	return &DB{db}, nil
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword checks if the provided password matches the hashed password
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CreateUser creates a new user with hashed password
func (db *DB) CreateUser(username, password string) (*User, error) {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &User{
		Username:    username,
		Password:    hashedPassword,
		DisplayName: username, // Default display name is username
		IsOnline:    false,
	}

	result := db.Create(user)
	if result.Error != nil {
		return nil, result.Error
	}

	return user, nil
}

// AuthenticateUser checks username and password
func (db *DB) AuthenticateUser(username, password string) (*User, error) {
	var user User
	result := db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}

	if !CheckPassword(password, user.Password) {
		return nil, fmt.Errorf("invalid password")
	}

	return &user, nil
}

// GetUserByUsername gets a user by username
func (db *DB) GetUserByUsername(username string) (*User, error) {
	var user User
	result := db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// UpdateUserOnlineStatus updates user online status
func (db *DB) UpdateUserOnlineStatus(username string, isOnline bool) error {
	return db.Model(&User{}).
		Where("username = ?", username).
		Updates(map[string]interface{}{
			"is_online": isOnline,
			"last_seen": time.Now(),
		}).Error
}

// SearchUsers performs fuzzy search for users
// Returns users matching the search query (case-insensitive, partial match)
func (db *DB) SearchUsers(query string, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 20
	}

	var users []User

	// Use PostgreSQL similarity and ILIKE for fuzzy search
	// similarity() requires pg_trgm extension
	err := db.Raw(`
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
	`, query, query, "%"+query+"%", "%"+query+"%", query, query, limit).Scan(&users).Error

	if err != nil {
		return nil, err
	}

	return users, nil
}

// GetAllOnlineUsers returns all currently online users
func (db *DB) GetAllOnlineUsers() ([]User, error) {
	var users []User
	result := db.Where("is_online = ?", true).Order("username ASC").Find(&users)
	if result.Error != nil {
		return nil, result.Error
	}
	return users, nil
}

// GetAllUsers returns all users
func (db *DB) GetAllUsers() ([]User, error) {
	var users []User
	result := db.Order("username ASC").Find(&users)
	if result.Error != nil {
		return nil, result.Error
	}
	return users, nil
}

// UserExists checks if a user exists
func (db *DB) UserExists(username string) (bool, error) {
	var count int64
	result := db.Model(&User{}).Where("username = ?", username).Count(&count)
	if result.Error != nil {
		return false, result.Error
	}
	return count > 0, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// ========== GROUP FUNCTIONS ==========

// CreateGroup creates a new group
func (db *DB) CreateGroup(groupName string) (*Group, error) {
	group := &Group{
		Name: groupName,
	}

	result := db.Create(group)
	if result.Error != nil {
		return nil, result.Error
	}

	return group, nil
}

// GetGroupByName gets a group by name
func (db *DB) GetGroupByName(name string) (*Group, error) {
	var group Group
	result := db.Where("name = ?", name).First(&group)
	if result.Error != nil {
		return nil, result.Error
	}
	return &group, nil
}

// GroupExists checks if a group exists
func (db *DB) GroupExists(name string) (bool, error) {
	var count int64
	result := db.Model(&Group{}).Where("name = ?", name).Count(&count)
	if result.Error != nil {
		return false, result.Error
	}
	return count > 0, nil
}

// AddGroupMember adds a user to a group
func (db *DB) AddGroupMember(groupName, username string) error {
	// Get group ID
	group, err := db.GetGroupByName(groupName)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	// Check if member already exists
	var count int64
	db.Model(&GroupMember{}).Where("group_id = ? AND username = ?", group.ID, username).Count(&count)
	if count > 0 {
		return nil // Already a member
	}

	member := &GroupMember{
		GroupID:  group.ID,
		Username: username,
	}

	return db.Create(member).Error
}

// GetGroupMembers gets all members of a group
func (db *DB) GetGroupMembers(groupName string) ([]string, error) {
	group, err := db.GetGroupByName(groupName)
	if err != nil {
		return nil, err
	}

	var members []GroupMember
	result := db.Where("group_id = ?", group.ID).Find(&members)
	if result.Error != nil {
		return nil, result.Error
	}

	usernames := make([]string, len(members))
	for i, m := range members {
		usernames[i] = m.Username
	}

	return usernames, nil
}

// GetUserGroups gets all groups that a user is a member of
func (db *DB) GetUserGroups(username string) ([]Group, error) {
	var groups []Group

	result := db.Raw(`
		SELECT g.id, g.name, g.created_at
		FROM groups g
		INNER JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.username = ?
		ORDER BY g.name ASC
	`, username).Scan(&groups)

	if result.Error != nil {
		return nil, result.Error
	}

	return groups, nil
}

// ========== MESSAGE FUNCTIONS ==========

// SaveMessage saves a message to the database
func (db *DB) SaveMessage(fromUser, toTarget, messageType, text string) error {
	message := &Message{
		FromUser:    fromUser,
		ToTarget:    toTarget,
		MessageType: messageType,
		Text:        text,
	}

	return db.Create(message).Error
}

// GetPrivateMessages gets private messages between two users
func (db *DB) GetPrivateMessages(user1, user2 string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}

	var messages []Message
	result := db.Where(
		"(from_user = ? AND to_target = ? AND message_type = 'private') OR (from_user = ? AND to_target = ? AND message_type = 'private')",
		user1, user2, user2, user1,
	).Order("created_at DESC").Limit(limit).Find(&messages)

	if result.Error != nil {
		return nil, result.Error
	}

	return messages, nil
}

// GetGroupMessages gets messages for a group
func (db *DB) GetGroupMessages(groupName string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}

	var messages []Message
	result := db.Where("to_target = ? AND message_type = 'group'", groupName).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages)

	if result.Error != nil {
		return nil, result.Error
	}

	return messages, nil
}
