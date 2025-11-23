package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
	"strconv"
)

type FriendController struct {
	db                     *gorm.DB
	notificationController *NotificationController
}

func NewFriendController(db *gorm.DB, notificationController *NotificationController) *FriendController {
	return &FriendController{
		db:                     db,
		notificationController: notificationController,
	}
}

func (fc *FriendController) SendFriendRequest(c *gin.Context) {
	senderID := c.GetString("user_id")
	receiverID := c.Param("user_id")

	if senderID == receiverID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot send friend request to yourself"})
		return
	}

	var receiver models.User
	if err := fc.db.First(&receiver, "id = ?", receiverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if fc.areFriends(senderID, receiverID) {
		c.JSON(http.StatusConflict, gin.H{"error": "Already friends with this user"})
		return
	}

	var existingRequest models.FriendRequest
	err := fc.db.Where("((sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)) AND status = ?",
		senderID, receiverID, receiverID, senderID, models.FriendRequestStatusPending).First(&existingRequest).Error

	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Friend request already exists"})
		return
	}

	friendRequest := models.FriendRequest{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Status:     models.FriendRequestStatusPending,
	}

	if err := fc.db.Create(&friendRequest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send friend request"})
		return
	}

	if err := fc.notificationController.CreateFollowNotification(senderID, receiverID); err != nil {
		fmt.Printf("Failed to create friend request notification: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend request sent successfully"})
}

func (fc *FriendController) AcceptFriendRequest(c *gin.Context) {
	userID := c.GetString("user_id")
	requestIDStr := c.Param("request_id")

	requestID, err := strconv.ParseUint(requestIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	var friendRequest models.FriendRequest
	if err := fc.db.First(&friendRequest, "id = ? AND receiver_id = ? AND status = ?",
		uint(requestID), userID, models.FriendRequestStatusPending).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Friend request not found"})
		return
	}

	err = fc.db.Transaction(func(tx *gorm.DB) error {
		friendRequest.Status = models.FriendRequestStatusAccepted
		if err := tx.Save(&friendRequest).Error; err != nil {
			return err
		}

		user1ID, user2ID := friendRequest.SenderID, friendRequest.ReceiverID
		if user1ID > user2ID {
			user1ID, user2ID = user2ID, user1ID
		}

		friendship := models.Friendship{
			User1ID: user1ID,
			User2ID: user2ID,
		}

		if err := tx.Create(&friendship).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept friend request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend request accepted successfully"})
}

func (fc *FriendController) RejectFriendRequest(c *gin.Context) {
	userID := c.GetString("user_id")
	requestIDStr := c.Param("request_id")

	requestID, err := strconv.ParseUint(requestIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	var friendRequest models.FriendRequest
	if err := fc.db.First(&friendRequest, "id = ? AND receiver_id = ? AND status = ?",
		uint(requestID), userID, models.FriendRequestStatusPending).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Friend request not found"})
		return
	}

	friendRequest.Status = models.FriendRequestStatusRejected
	if err := fc.db.Save(&friendRequest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject friend request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend request rejected successfully"})
}

func (fc *FriendController) RemoveFriend(c *gin.Context) {
	userID := c.GetString("user_id")
	friendID := c.Param("user_id")

	if userID == friendID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid operation"})
		return
	}

	user1ID, user2ID := userID, friendID
	if user1ID > user2ID {
		user1ID, user2ID = user2ID, user1ID
	}

	var friendship models.Friendship
	if err := fc.db.Where("user1_id = ? AND user2_id = ?", user1ID, user2ID).First(&friendship).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Friendship not found"})
		return
	}

	if err := fc.db.Delete(&friendship).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove friend"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend removed successfully"})
}

func (fc *FriendController) GetFriends(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	var friendships []models.Friendship
	if err := fc.db.Where("user1_id = ? OR user2_id = ?", userID, userID).
		Offset(offset).Limit(limit).Find(&friendships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch friends"})
		return
	}

	friendIDs := make([]string, 0, len(friendships))
	for _, friendship := range friendships {
		if friendship.User1ID == userID {
			friendIDs = append(friendIDs, friendship.User2ID)
		} else {
			friendIDs = append(friendIDs, friendship.User1ID)
		}
	}

	if len(friendIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"friends": []models.User{}})
		return
	}

	var friends []models.User
	if err := fc.db.Where("id IN ?", friendIDs).Find(&friends).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch friend details"})
		return
	}

	for i := range friends {
		friends[i].Password = ""
	}

	c.JSON(http.StatusOK, gin.H{"friends": friends})
}

func (fc *FriendController) GetPendingRequests(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	var requests []models.FriendRequest
	if err := fc.db.Preload("Sender").Where("receiver_id = ? AND status = ?", userID, models.FriendRequestStatusPending).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch friend requests"})
		return
	}

	for i := range requests {
		requests[i].Sender.Password = ""
	}

	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

func (fc *FriendController) GetSentRequests(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	var requests []models.FriendRequest
	if err := fc.db.Preload("Receiver").Where("sender_id = ? AND status = ?", userID, models.FriendRequestStatusPending).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sent requests"})
		return
	}

	for i := range requests {
		requests[i].Receiver.Password = ""
	}

	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

func (fc *FriendController) areFriends(user1ID, user2ID string) bool {
	if user1ID > user2ID {
		user1ID, user2ID = user2ID, user1ID
	}

	var friendship models.Friendship
	err := fc.db.Where("user1_id = ? AND user2_id = ?", user1ID, user2ID).First(&friendship).Error
	return err == nil
}

func (fc *FriendController) GetFriendshipStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	targetUserID := c.Param("user_id")

	if userID == targetUserID {
		c.JSON(http.StatusOK, gin.H{
			"is_friend":          false,
			"has_pending_sent":   false,
			"has_pending_received": false,
		})
		return
	}

	// Check if friends
	isFriend := fc.areFriends(userID, targetUserID)

	// Check for pending requests
	var sentRequest models.FriendRequest
	hasPendingSent := false
	if err := fc.db.Where("sender_id = ? AND receiver_id = ? AND status = ?",
		userID, targetUserID, models.FriendRequestStatusPending).First(&sentRequest).Error; err == nil {
		hasPendingSent = true
	}

	var receivedRequest models.FriendRequest
	hasPendingReceived := false
	if err := fc.db.Where("sender_id = ? AND receiver_id = ? AND status = ?",
		targetUserID, userID, models.FriendRequestStatusPending).First(&receivedRequest).Error; err == nil {
		hasPendingReceived = true
	}

	c.JSON(http.StatusOK, gin.H{
		"is_friend":              isFriend,
		"has_pending_sent":       hasPendingSent,
		"has_pending_received":   hasPendingReceived,
		"sent_request_id":        sentRequest.ID,
		"received_request_id":    receivedRequest.ID,
	})
}