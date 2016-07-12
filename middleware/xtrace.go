package middleware

import (
	"github.com/gin-gonic/gin"
	"math/rand"
)

const letterBytes = "0123456789abcdef"
const XRequestHeader = "X-Request"

func XTrace() gin.HandlerFunc {
	return func(c *gin.Context) {
		xreq := c.Request.Header.Get(XRequestHeader)
		if xreq == "" {
			xb := make([]byte, 16)
			for i := range xb {
				xb[i] = letterBytes[rand.Int63() % int64(len(letterBytes))]
			}

			c.Request.Header.Set(XRequestHeader, string(xb))
		}

		c.Next()
	}
}
