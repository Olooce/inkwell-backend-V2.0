package controller

import (
	"inkwell-backend-V2.0/internal/service"

	"net/http"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	UserService service.UserService
}

func NewUserController(userService service.UserService) *UserController {
	return &UserController{UserService: userService}
}

// GetCurrentUser returns the signed-in user's own profile.
func (uc *UserController) GetCurrentUser(c *gin.Context) {
	uid, ok := c.Get("user_id")
	id, isUint := uid.(uint)
	if !ok || !isUint {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}
	user, err := uc.UserService.GetUser(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}
