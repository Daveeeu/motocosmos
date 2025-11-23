package models

import "time"

type FriendRequestStatus string

const (
	FriendRequestStatusPending  FriendRequestStatus = "pending"
	FriendRequestStatusAccepted FriendRequestStatus = "accepted"
	FriendRequestStatusRejected FriendRequestStatus = "rejected"
)

type FriendRequest struct {
	ID         uint                `json:"id" gorm:"primaryKey"`
	SenderID   string              `json:"sender_id" gorm:"not null;size:191"`
	ReceiverID string              `json:"receiver_id" gorm:"not null;size:191"`
	Status     FriendRequestStatus `json:"status" gorm:"not null;default:'pending';size:20"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`

	Sender   User `json:"sender" gorm:"foreignKey:SenderID"`
	Receiver User `json:"receiver" gorm:"foreignKey:ReceiverID"`
}

type Friendship struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	User1ID   string    `json:"user1_id" gorm:"not null;size:191"`
	User2ID   string    `json:"user2_id" gorm:"not null;size:191"`
	CreatedAt time.Time `json:"created_at"`

	User1 User `json:"user1" gorm:"foreignKey:User1ID"`
	User2 User `json:"user2" gorm:"foreignKey:User2ID"`
}