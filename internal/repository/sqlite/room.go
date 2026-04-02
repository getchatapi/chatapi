package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/hastenr/chatapi/internal/models"
)

// SQLiteRoomRepository implements repository.RoomRepository using SQLite.
type SQLiteRoomRepository struct {
	db *sql.DB
}

// NewRoomRepository creates a new SQLiteRoomRepository.
func NewRoomRepository(db *sql.DB) *SQLiteRoomRepository {
	return &SQLiteRoomRepository{db: db}
}

func scanRoom(row interface {
	Scan(dest ...interface{}) error
}) (*models.Room, error) {
	var room models.Room
	var uniqueKey, name, metadata sql.NullString
	err := row.Scan(
		&room.RoomID,
		&room.TenantID,
		&room.Type,
		&uniqueKey,
		&name,
		&room.LastSeq,
		&metadata,
		&room.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	room.UniqueKey = uniqueKey.String
	room.Name = name.String
	room.Metadata = metadata.String
	return &room, nil
}

// GetByID retrieves a room by tenant and room ID.
func (r *SQLiteRoomRepository) GetByID(tenantID, roomID string) (*models.Room, error) {
	query := `
		SELECT room_id, tenant_id, type, unique_key, name, last_seq, metadata, created_at
		FROM rooms
		WHERE tenant_id = ? AND room_id = ?
	`
	row := r.db.QueryRow(query, tenantID, roomID)
	room, err := scanRoom(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("room not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}
	return room, nil
}

// GetByUniqueKey retrieves a room by unique key. Returns nil, nil if not found.
func (r *SQLiteRoomRepository) GetByUniqueKey(tenantID, uniqueKey string) (*models.Room, error) {
	query := `
		SELECT room_id, tenant_id, type, unique_key, name, last_seq, metadata, created_at
		FROM rooms
		WHERE tenant_id = ? AND unique_key = ?
	`
	row := r.db.QueryRow(query, tenantID, uniqueKey)
	room, err := scanRoom(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room by unique key: %w", err)
	}
	return room, nil
}

// Create inserts a new room record.
func (r *SQLiteRoomRepository) Create(tenantID string, room *models.Room) error {
	if room.UniqueKey != "" {
		query := `
			INSERT INTO rooms (room_id, tenant_id, type, unique_key, name, metadata, last_seq)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		_, err := r.db.Exec(query, room.RoomID, room.TenantID, room.Type, room.UniqueKey, room.Name, room.Metadata, room.LastSeq)
		if err != nil {
			return fmt.Errorf("failed to create room: %w", err)
		}
	} else {
		query := `
			INSERT INTO rooms (room_id, tenant_id, type, name, metadata, last_seq)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		_, err := r.db.Exec(query, room.RoomID, room.TenantID, room.Type, room.Name, room.Metadata, room.LastSeq)
		if err != nil {
			return fmt.Errorf("failed to create room: %w", err)
		}
	}
	return nil
}

// Update updates a room's name and/or metadata.
func (r *SQLiteRoomRepository) Update(tenantID, roomID string, req *models.UpdateRoomRequest) error {
	var setParts []string
	var args []interface{}

	if req.Name != nil {
		setParts = append(setParts, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Metadata != nil {
		setParts = append(setParts, "metadata = ?")
		args = append(args, *req.Metadata)
	}
	if len(setParts) == 0 {
		return nil
	}

	args = append(args, tenantID, roomID)
	query := "UPDATE rooms SET " + strings.Join(setParts, ", ") + " WHERE tenant_id = ? AND room_id = ?"

	result, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update room: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("room not found")
	}
	return nil
}

// GetUserRooms returns all rooms that a user is a member of.
func (r *SQLiteRoomRepository) GetUserRooms(tenantID, userID string) ([]*models.Room, error) {
	query := `
		SELECT r.room_id, r.tenant_id, r.type, r.unique_key, r.name, r.last_seq, r.metadata, r.created_at
		FROM rooms r
		JOIN room_members rm ON r.room_id = rm.chatroom_id AND r.tenant_id = rm.tenant_id
		WHERE r.tenant_id = ? AND rm.user_id = ?
		ORDER BY r.created_at DESC
	`

	rows, err := r.db.Query(query, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user rooms: %w", err)
	}
	defer rows.Close()

	var rooms []*models.Room
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan room: %w", err)
		}
		rooms = append(rooms, room)
	}

	return rooms, rows.Err()
}

// AddMember adds a single member to a room.
func (r *SQLiteRoomRepository) AddMember(tenantID, roomID, userID string) error {
	query := `
		INSERT INTO room_members (chatroom_id, tenant_id, user_id, role)
		VALUES (?, ?, ?, 'member')
	`
	_, err := r.db.Exec(query, roomID, tenantID, userID)
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}
	return nil
}

// AddMembers adds multiple members to a room using a transaction with INSERT OR IGNORE.
func (r *SQLiteRoomRepository) AddMembers(tenantID, roomID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT OR IGNORE INTO room_members (chatroom_id, tenant_id, user_id, role)
		VALUES (?, ?, ?, 'member')
	`

	for _, userID := range userIDs {
		_, err = tx.Exec(query, roomID, tenantID, userID)
		if err != nil {
			return fmt.Errorf("failed to add member %s: %w", userID, err)
		}
	}

	return tx.Commit()
}

// RemoveMember removes a member from a room.
func (r *SQLiteRoomRepository) RemoveMember(tenantID, roomID, userID string) error {
	query := `
		DELETE FROM room_members
		WHERE tenant_id = ? AND chatroom_id = ? AND user_id = ?
	`

	result, err := r.db.Exec(query, tenantID, roomID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("member not found in room")
	}
	return nil
}

// GetMembers retrieves all members of a room.
func (r *SQLiteRoomRepository) GetMembers(tenantID, roomID string) ([]*models.RoomMember, error) {
	query := `
		SELECT chatroom_id, tenant_id, user_id, role, joined_at
		FROM room_members
		WHERE tenant_id = ? AND chatroom_id = ?
		ORDER BY joined_at
	`

	rows, err := r.db.Query(query, tenantID, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to get room members: %w", err)
	}
	defer rows.Close()

	var members []*models.RoomMember
	for rows.Next() {
		var member models.RoomMember
		err := rows.Scan(
			&member.ChatroomID,
			&member.TenantID,
			&member.UserID,
			&member.Role,
			&member.JoinedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		members = append(members, &member)
	}

	return members, nil
}

// GetMemberIDs retrieves the list of user IDs who are members of a room.
func (r *SQLiteRoomRepository) GetMemberIDs(tenantID, roomID string) ([]string, error) {
	query := `
		SELECT user_id
		FROM room_members
		WHERE tenant_id = ? AND chatroom_id = ?
	`

	rows, err := r.db.Query(query, tenantID, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		members = append(members, userID)
	}

	return members, rows.Err()
}
