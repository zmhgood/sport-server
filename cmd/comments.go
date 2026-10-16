package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Keep legacy URLs closed even for old clients with valid login credentials.
// No repository access: existing comments remain available only to admins.
func registerDisabledCommentRoutes(api *gin.RouterGroup) {
	disabled := func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusGone, gin.H{"code": http.StatusGone, "message": "评论功能已停用"})
	}
	api.GET("/comments", disabled)
	api.POST("/comments", disabled)
	api.DELETE("/comments/:id", disabled)
	api.POST("/comments/:id/like", disabled)
	api.GET("/user/comments", disabled)
}
