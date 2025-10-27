package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware is a middleware for handling CORS
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := getValidOrigin(c)

		// Set CORS headers
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, PATCH, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, ngrok-skip-browser-warning")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // Cache for 24 hours

		// Handle preflight requests
		if strings.ToUpper(c.Request.Method) == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// getValidOrigin determines the appropriate origin
func getValidOrigin(c *gin.Context) string {
	origin := c.GetHeader("Origin")
	remoteIP := c.ClientIP()

	if origin == "" {
		if isValidIP(remoteIP) {
			origin = c.GetHeader("Referer")
			if origin == "" {
				origin = c.GetHeader("Host")
			}
		} else {
			blockRequest(c)
			return ""
		}
	}

	if origin == "" {
		return "*"
	}
	return origin
}

// isValidIP checks if the request comes from a trusted IP
//
//goland:noinspection GoUnusedParameter
func isValidIP(ip string) bool {
	// TODO Implement specific IP validation
	// return strings.Contains(ip, "127.0.0.1") || strings.HasPrefix(ip, "192.168.")
	return true
}

// blockRequest denies access to unauthorized requests
func blockRequest(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden: Unauthorized request"})
}
