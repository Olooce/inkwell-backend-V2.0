package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	Log "inkwell-backend-V2.0/pkg/logging"

	"github.com/gin-gonic/gin"
)

// sensitiveHeaders is the set of header keys whose values must never appear in logs.
var sensitiveHeaders = map[string]bool{
	"authorization": true,
	"cookie":        true,
	"set-cookie":    true,
	"x-api-key":     true,
}

// sensitiveBodyFields is the set of JSON field names whose values must never appear in logs.
var sensitiveBodyFields = map[string]bool{
	"password":     true,
	"token":        true,
	"access_token": true,
	"ssn":          true,
}

// redactHeaderValue returns "<redacted>" for known-sensitive header keys,
// otherwise returns the original value unchanged.
func redactHeaderValue(key, value string) string {
	if sensitiveHeaders[strings.ToLower(key)] {
		return "<redacted>"
	}
	return value
}

// sanitizeBody scrubs sensitive field values from a JSON body.
// Non-JSON bodies are returned as-is; invalid JSON after scrubbing falls back to the original.
func sanitizeBody(body []byte) []byte {
	if !json.Valid(body) {
		return body
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	sanitizeMap(m)
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}

func sanitizeMap(m map[string]interface{}) {
	for k, v := range m {
		if sensitiveBodyFields[strings.ToLower(k)] {
			m[k] = "<redacted>"
			continue
		}
		// Recurse into nested objects.
		if nested, ok := v.(map[string]interface{}); ok {
			sanitizeMap(nested)
		}
	}
}

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
				b.WriteString("    - " + k + ": " + redactHeaderValue(k, v) + "\n")
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
			b.WriteString(formatBody(sanitizeBody(bodyBytes)))
		}

		Log.Debug("%s", b.String())
		c.Next()
	}
}

const maxBodyLen = 1000

func formatBody(body []byte) string {
	if len(body) > maxBodyLen {
		return "    " + string(body[:maxBodyLen]) + "...(truncated)\n"
	}

	if json.Valid(body) {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, body, "    ", "  "); err == nil {
			s := pretty.String()
			if len(s) > maxBodyLen {
				s = s[:maxBodyLen] + "...(truncated)"
			}
			return "    " + s + "\n"
		}
	}

	// Non-JSON fallback.
	return "    " + string(body) + "\n"
}
