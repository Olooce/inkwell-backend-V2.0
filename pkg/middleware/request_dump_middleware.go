package middleware

import (
	"bytes"
	"encoding/json"
	"io"

	Log "inkwell-backend-V2.0/pkg/logging"

	"github.com/gin-gonic/gin"
)

func RequestDumpMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var b bytes.Buffer

		b.WriteString("\n[Request]\n")

		b.WriteString("  Method: " + c.Request.Method + "\n")
		b.WriteString("  URL: " + c.Request.URL.String() + "\n")

		b.WriteString("  Headers:\n")
		for k, vals := range c.Request.Header {
			for _, v := range vals {
				b.WriteString("    - " + k + ": " + v + "\n")
			}
		}

		if len(c.Params) > 0 {
			b.WriteString("  Params:\n")
			for _, p := range c.Params {
				b.WriteString("    - " + p.Key + ": " + p.Value + "\n")
			}
		}

		if len(bodyBytes) > 0 {
			b.WriteString("  Body:\n")
			b.WriteString(formatBody(bodyBytes))
		}

		Log.Debug("%s", b.String())

		c.Next()
	}
}

func formatBody(body []byte) string {
	var pretty bytes.Buffer

func formatBody(body []byte) string {
	var pretty bytes.Buffer

	const max = 1000
	if len(body) > max {
		return "    " + string(body[:max]) + "...(truncated)\n"
	}

	if json.Valid(body) {
		if err := json.Indent(&pretty, body, "    ", "  "); err == nil {
			return pretty.String() + "\n"
		}
	}

	// fallback
	return "    " + string(body) + "\n"
}

	// fallback (truncate long bodies)
	const max = 1000
	if len(body) > max {
		return "    " + string(body[:max]) + "...(truncated)\n"
	}

	return "    " + string(body) + "\n"
}
