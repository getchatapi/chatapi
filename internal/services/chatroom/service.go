package chatroom

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/google/uuid"
	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/repository"
)

// Service handles chatroom operations
type Service struct {
	repo repository.RoomRepository
}

// NewService creates a new chatroom service
func NewService(repo repository.RoomRepository) *Service {
	return &Service{repo: repo}
}

// CreateRoom creates a new chatroom
func (s *Service) CreateRoom(tenantID string, req *models.CreateRoomRequest) (*models.Room, error) {
	var room *models.Room
	var err error

	switch req.Type {
	case "dm":
		room, err = s.createDMRoom(tenantID, req)
	case "group", "channel":
		room, err = s.createGroupRoom(tenantID, req)
	default:
		return nil, fmt.Errorf("invalid room type: %s", req.Type)
	}

	if err != nil {
		return nil, err
	}

	// Add members to the room
	if err := s.repo.AddMembers(tenantID, room.RoomID, req.Members); err != nil {
		return nil, fmt.Errorf("failed to add members: %w", err)
	}

	slog.Info("Created room", "tenant_id", tenantID, "room_id", room.RoomID, "type", req.Type)
	return room, nil
}

// createDMRoom creates a DM room with deterministic unique_key
func (s *Service) createDMRoom(tenantID string, req *models.CreateRoomRequest) (*models.Room, error) {
	if len(req.Members) != 2 {
		return nil, fmt.Errorf("DM rooms must have exactly 2 members")
	}

	// Sort members for deterministic key
	members := make([]string, len(req.Members))
	copy(members, req.Members)
	sort.Strings(members)

	uniqueKey := fmt.Sprintf("dm:%s:%s", members[0], members[1])

	// Check if DM already exists
	existing, err := s.repo.GetByUniqueKey(tenantID, uniqueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing DM: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	// Create new DM room
	room := &models.Room{
		RoomID:    generateRoomID(),
		TenantID:  tenantID,
		Type:      "dm",
		UniqueKey: uniqueKey,
		Name:      req.Name,
		Metadata:  req.Metadata,
		LastSeq:   0,
	}

	if err := s.repo.Create(tenantID, room); err != nil {
		return nil, fmt.Errorf("failed to create DM room: %w", err)
	}

	return room, nil
}

// createGroupRoom creates a group or channel room
func (s *Service) createGroupRoom(tenantID string, req *models.CreateRoomRequest) (*models.Room, error) {
	if len(req.Members) < 2 {
		return nil, fmt.Errorf("group/channel rooms must have at least 2 members")
	}

	room := &models.Room{
		RoomID:   generateRoomID(),
		TenantID: tenantID,
		Type:     req.Type,
		Name:     req.Name,
		Metadata: req.Metadata,
		LastSeq:  0,
	}

	if err := s.repo.Create(tenantID, room); err != nil {
		return nil, fmt.Errorf("failed to create group room: %w", err)
	}

	return room, nil
}

// GetRoom retrieves a room by ID
func (s *Service) GetRoom(tenantID, roomID string) (*models.Room, error) {
	return s.repo.GetByID(tenantID, roomID)
}

// GetRoomMembers retrieves all members of a room
func (s *Service) GetRoomMembers(tenantID, roomID string) ([]*models.RoomMember, error) {
	return s.repo.GetMembers(tenantID, roomID)
}

// AddMember adds a single member to a room
func (s *Service) AddMember(tenantID, roomID, userID string) error {
	if err := s.repo.AddMember(tenantID, roomID, userID); err != nil {
		return err
	}
	slog.Info("Added member to room", "tenant_id", tenantID, "room_id", roomID, "user_id", userID)
	return nil
}

// RemoveMember removes a member from a room
func (s *Service) RemoveMember(tenantID, roomID, userID string) error {
	if err := s.repo.RemoveMember(tenantID, roomID, userID); err != nil {
		return err
	}
	slog.Info("Removed member from room", "tenant_id", tenantID, "room_id", roomID, "user_id", userID)
	return nil
}

// GetUserRooms returns all rooms that a user is a member of
func (s *Service) GetUserRooms(tenantID, userID string) ([]*models.Room, error) {
	return s.repo.GetUserRooms(tenantID, userID)
}

// UpdateRoom updates a room's name and/or metadata. Nil pointer fields are left unchanged.
func (s *Service) UpdateRoom(tenantID, roomID string, req *models.UpdateRoomRequest) (*models.Room, error) {
	if req.Name == nil && req.Metadata == nil {
		return s.GetRoom(tenantID, roomID)
	}

	if err := s.repo.Update(tenantID, roomID, req); err != nil {
		return nil, err
	}

	slog.Info("Updated room", "tenant_id", tenantID, "room_id", roomID)
	return s.GetRoom(tenantID, roomID)
}

// generateRoomID generates a unique room ID
func generateRoomID() string {
	return uuid.New().String()
}
