package middleware

import (
	"strconv"

	"elderly-fitness/internal/handler"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"elderly-fitness/pkg/database"

	"github.com/gin-gonic/gin"
)

// FamilyAccessMiddleware 读取家庭资料前核验当前用户的成员关系。
func FamilyAccessMiddleware(goalResource bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawID := c.Param("id")
		if rawID == "" {
			rawID = c.Query("family_id")
		}
		id, err := strconv.ParseUint(rawID, 10, 32)
		if err != nil || id == 0 {
			c.AbortWithStatusJSON(400, handler.Response{Code: 400, Message: "参数错误"})
			return
		}
		familyID := uint(id)
		if goalResource {
			var goal model.DailyGoal
			if err := database.DB.Select("family_id").First(&goal, id).Error; err != nil {
				c.AbortWithStatusJSON(404, handler.Response{Code: 404, Message: "目标不存在"})
				return
			}
			familyID = goal.FamilyID
		}
		userID := c.GetUint("userID")
		member, err := repository.NewFamilyRepository(database.DB).IsMember(familyID, userID)
		if err != nil {
			c.AbortWithStatusJSON(500, handler.Response{Code: 500, Message: "权限校验失败"})
			return
		}
		if !member {
			c.AbortWithStatusJSON(403, handler.Response{Code: 403, Message: "无权访问该家庭"})
			return
		}
		c.Next()
	}
}
