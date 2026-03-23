package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Authorization")
		if c.Request.Method == "OPTIONS" { c.AbortWithStatus(204); return }
		c.Next()
	}
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "다시 로그인해 주세요"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		jwtSecret := os.Getenv("SUPABASE_JWT_SECRET")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "다시 로그인해 주세요"})
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok { c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "권한 오류"}); return }
		if uid, ok := claims["sub"].(string); ok { c.Set("user_id", uid) }
		if appMeta, ok := claims["app_metadata"].(map[string]interface{}); ok {
			if orgID, ok := appMeta["org_id"].(string); ok { c.Set("org_id", orgID) }
			if pr, ok := appMeta["primary_role"].(string); ok { c.Set("primary_role", pr) }
		}
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		pr, _ := c.Get("primary_role")
		role, _ := pr.(string)
		for _, r := range roles { if role == r { c.Next(); return } }
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "권한이 없어요"})
	}
}

func GetOrgID(c *gin.Context) string {
	v, _ := c.Get("org_id"); s, _ := v.(string); return s
}

func GetUserID(c *gin.Context) string {
	v, _ := c.Get("user_id"); s, _ := v.(string); return s
}
