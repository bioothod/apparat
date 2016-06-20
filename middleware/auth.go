package middleware

import (
	"fmt"
	"github.com/bioothod/apparat/services/auth"
	"github.com/gin-gonic/gin"
	"net/http"
)

func AuthRequired(auth_url string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(auth.CookieName)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H {
				"error": fmt.Errorf("could not get cookie '%s' from request, error: %v", auth.CookieName, err),
			})
			return
		}

		ac, err := auth.CheckCookieWeb(auth_url, cookie)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H {
				"error": fmt.Errorf("cookie check has failed: %v", err),
			})
			return
		}

		c.Set("username", ac.Username)
		c.Next()
	}
}

