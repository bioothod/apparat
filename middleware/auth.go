package middleware

import (
	"github.com/bioothod/apparat/services/auth"
	"github.com/gin-gonic/gin"
	"net/http"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := auth.CheckAuthCookie(c.Request)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H {
				"error": err,
			})
			return
		}

		c.Next()
	}
}
