package handler

import (
	"errors"
	"net/http"
	"strconv"

	"elderly-fitness/internal/service"
	"github.com/gin-gonic/gin"
)

type RenameFamilyRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *FamilyHandler) RenameFamily(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, Response{Code: 401, Message: "请先登录"})
		return
	}
	familyID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || familyID == 0 {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "家庭ID无效"})
		return
	}
	var input RenameFamilyRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "请输入有效的家庭名称"})
		return
	}
	name, err := h.familyService.RenameFamily(uint(familyID), userID, input.Name)
	if err != nil {
		status, message := http.StatusInternalServerError, "修改家庭名称失败，请重试"
		switch {
		case errors.Is(err, service.ErrFamilyNameInvalid):
			status, message = http.StatusBadRequest, err.Error()
		case errors.Is(err, service.ErrFamilyRenameForbidden):
			status, message = http.StatusForbidden, err.Error()
		case errors.Is(err, service.ErrFamilyRenameNotFound):
			status, message = http.StatusNotFound, err.Error()
		}
		c.JSON(status, Response{Code: status, Message: message})
		return
	}
	c.JSON(http.StatusOK, Response{Code: 0, Message: "名称已修改", Data: gin.H{"id": familyID, "name": name}})
}
