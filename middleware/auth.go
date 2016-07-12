package middleware

import (
	"fmt"
	"github.com/bioothod/apparat/services/auth"
	"github.com/bioothod/apparat/services/common"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"net/http"
)

func AuthRequired(auth_url string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(auth.CookieName)
		if err != nil {
			glog.Errorf("could not get cookie '%s': %v\n", auth.CookieName, err)
			estr := fmt.Sprintf("could not get cookie '%s' from request, error: %v", auth.CookieName, err)
			common.NewErrorString(c, "auth", estr)
			c.JSON(http.StatusForbidden, gin.H {
				"operation": "auth",
				"error": estr,
			})
			c.Abort()
			return
		}

		ac, err := auth.CheckCookieWeb(auth_url, cookie)
		if err != nil {
			glog.Errorf("cookie check has failed: %v\n", err)
			estr := fmt.Sprintf("cookie check has failed: %v", err)
			common.NewErrorString(c, "auth", estr)
			c.JSON(http.StatusForbidden, gin.H {
				"operation": "auth",
				"error": estr,
			})
			c.Abort()
			return
		}

		c.Set("username", ac.Username)
		c.Next()
	}
}

