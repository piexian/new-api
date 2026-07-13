package middleware

import (
	"net/http"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func Gzip() gin.HandlerFunc {
	handler := gzip.Gzip(gzip.DefaultCompression)
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodHead {
			c.Next()
			return
		}
		handler(c)
	}
}
