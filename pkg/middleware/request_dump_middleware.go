package middleware

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
	"inkwell-backend-V2.0/pkg/logging"
)

func RequestDumpMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		logger.Debug(
			"[Request]\n"+
				"\tMethod: %s\n"+
				"\tURL: %s\n"+
				"\tHeaders: %v\n"+
				"\tParams: %v\n"+
				"\tBody: %s",
			c.Request.Method,
			c.Request.URL.String(),
			c.Request.Header,
			c.Params,
			string(bodyBytes),
		)

		c.Next()
	}
}
