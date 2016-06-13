package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/context"
)

func ClearGorillaContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		context.Clear(c.Request)
	}
}


