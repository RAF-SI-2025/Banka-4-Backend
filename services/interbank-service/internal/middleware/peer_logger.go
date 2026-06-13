package middleware

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type captureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *captureWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// PeerMessageLogger logs every inbound peer message with its request body and
// response body. Must run after APIKeyAuth so PeerContextKey is already set.
func PeerMessageLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		peerRouting, _ := c.Get(PeerContextKey)

		var reqBody []byte
		if c.Request.Body != nil {
			reqBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		cw := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = cw

		c.Next()

		zap.L().Info("interbank inbound",
			zap.Any("peer", peerRouting),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.ByteString("request", reqBody),
			zap.Int("status", c.Writer.Status()),
			zap.ByteString("response", cw.body.Bytes()),
		)
	}
}
