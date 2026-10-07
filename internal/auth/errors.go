package auth

import (
	"github.com/gin-gonic/gin"
)

// Error writes the design §8 error envelope.
func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

// ErrorDetails writes the §8 envelope with a details list.
func ErrorDetails(c *gin.Context, status int, code, message string, details []string) {
	c.JSON(status, gin.H{"error": gin.H{
		"code": code, "message": message, "details": details,
	}})
}
