package handler

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/service"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

type BannerHandler struct{ service *service.BannerService }

func NewBannerHandler(s *service.BannerService) *BannerHandler { return &BannerHandler{service: s} }
func (h *BannerHandler) respond(c *gin.Context, public bool) {
	result, err := h.service.Get(public)
	if err != nil {
		c.JSON(500, Response{Code: 500, Message: "轮播图加载失败，请重试"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, Response{Code: 0, Message: "success", Data: result})
}
func (h *BannerHandler) Public(c *gin.Context) { h.respond(c, true) }
func (h *BannerHandler) Admin(c *gin.Context)  { h.respond(c, false) }
func (h *BannerHandler) Save(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var input struct {
		Items *[]model.Banner `json:"items"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Items == nil {
		c.JSON(400, Response{Code: 400, Message: "请提供有效的轮播图列表"})
		return
	}
	if err := h.service.Save(*input.Items); err != nil {
		if errors.Is(err, service.ErrInvalidBanners) {
			c.JSON(400, Response{Code: 400, Message: err.Error()})
		} else {
			c.JSON(500, Response{Code: 500, Message: "轮播图保存失败，请重试"})
		}
		return
	}
	c.JSON(200, Response{Code: 0, Message: "轮播图已保存"})
}
