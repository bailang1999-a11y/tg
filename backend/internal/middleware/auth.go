package middleware

import (
	"net/http"
	"strings"

	"codex3/backend/internal/database"
	"codex3/backend/internal/models"
	"codex3/backend/internal/services"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

const claimsKey = "claims"

func Auth(auth *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenString := ""
		if strings.HasPrefix(header, "Bearer ") {
			tokenString = strings.TrimPrefix(header, "Bearer ")
		}
		if tokenString == "" {
			tokenString = c.Query("access_token")
		}
		if tokenString == "" {
			utils.Fail(c, http.StatusUnauthorized, "缺少授权 Token")
			c.Abort()
			return
		}
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			utils.Fail(c, http.StatusUnauthorized, "Token 无效或已过期")
			c.Abort()
			return
		}
		c.Set(claimsKey, claims)
		c.Request = c.Request.WithContext(database.WithTenant(c.Request.Context(), claims.TenantID))
		c.Next()
	}
}

func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := CurrentClaims(c)
		if claims == nil || claims.Role != role {
			utils.Fail(c, http.StatusForbidden, "权限不足")
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return RequireRole(models.RoleAdmin)
}

func CurrentClaims(c *gin.Context) *services.Claims {
	value, ok := c.Get(claimsKey)
	if !ok {
		return nil
	}
	claims, _ := value.(*services.Claims)
	return claims
}
