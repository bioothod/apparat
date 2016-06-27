package middleware

import (
	"fmt"
	"github.com/bioothod/apparat/services/auth"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"net/http"
)

func AuthRequired(auth_url string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(auth.CookieName)
		if err != nil {
			glog.Errorf("could not get cookie '%s': %v\n", auth.CookieName, err)
			c.JSON(http.StatusForbidden, gin.H {
				"operation": "auth",
				"error": fmt.Sprintf("could not get cookie '%s' from request, error: %v", auth.CookieName, err),
			})
			c.Abort()
			return
		}

		ac, err := auth.CheckCookieWeb(auth_url, cookie)
		if err != nil {
			glog.Errorf("cookie check has failed: %v\n", err)
			c.JSON(http.StatusForbidden, gin.H {
				"operation": "auth",
				"error": fmt.Sprintf("cookie check has failed: %v", err),
			})
			c.Abort()
			return
		}

		c.Set("username", ac.Username)
		c.Next()
	}
}

