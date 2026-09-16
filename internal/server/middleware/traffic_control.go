package middleware

import (
	"github.com/bestruirui/octopus/internal/op"
	"github.com/bestruirui/octopus/internal/server/resp"
	"github.com/gin-gonic/gin"
)

func TrafficControl() gin.HandlerFunc {
	return func(c *gin.Context) {
		decision := op.TrafficControlCheckIP(c.ClientIP())
		if decision.Blocked {
			c.Header("X-Traffic-Control-Rule", decision.RuleName)
			resp.Error(c, decision.StatusCode, decision.Message)
			c.Abort()
			return
		}
		c.Next()
	}
}
