package middleware

import (
	"log"
	"math"
	"strings"

	"elderly-fitness/config"
	"elderly-fitness/internal/handler"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims JWT声明
type JWTClaims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// AuthMiddleware JWT认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return authMiddleware("user")
}

// AdminAuthMiddleware 仅允许管理员访问管理接口。
func AdminAuthMiddleware() gin.HandlerFunc {
	return authMiddleware("admin")
}

// UploadAuthMiddleware 允许用户和管理员上传文件。
func UploadAuthMiddleware() gin.HandlerFunc {
	return authMiddleware("upload")
}

func authMiddleware(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, handler.Response{
				Code:    401,
				Message: "未登录",
			})
			c.Abort()
			return
		}

		// Bearer token格式
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(401, handler.Response{
				Code:    401,
				Message: "Token格式错误",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 解析token (使用 MapClaims 支持多种格式)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.AppConfig.JWT.Secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

		if err != nil || !token.Valid {
			c.JSON(401, handler.Response{
				Code:    401,
				Message: "Token无效或已过期",
			})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(401, handler.Response{
				Code:    401,
				Message: "Token解析失败",
			})
			c.Abort()
			return
		}

		uid, hasUser := claims["user_id"].(float64)
		aid, hasAdmin := claims["admin_id"].(float64)
		isAdmin := hasAdmin && !hasUser && claims["type"] == "admin"
		isUser := hasUser && !hasAdmin
		if (!isUser && !isAdmin) || (role == "user" && !isUser) || (role == "admin" && !isAdmin) {
			c.AbortWithStatusJSON(403, handler.Response{Code: 403, Message: "无权访问"})
			return
		}
		id := uid
		if isAdmin {
			id = aid
		}
		if id <= 0 || id != math.Trunc(id) || id >= math.Exp2(64) {
			c.AbortWithStatusJSON(401, handler.Response{Code: 401, Message: "Token格式无效"})
			return
		}
		userID := uint(id)

		// 将用户ID存入上下文
		c.Set("userID", userID)
		log.Printf("[Auth] %s %s userID=%d", c.Request.Method, c.Request.URL.Path, userID)
		c.Next()
	}
}

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
