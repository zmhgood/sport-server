package service

import (
	"errors"
	"time"

	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AdminService struct {
	adminRepo *repository.AdminRepository
	userRepo  *repository.UserRepository
}

func NewAdminService(adminRepo *repository.AdminRepository, userRepo *repository.UserRepository) *AdminService {
	return &AdminService{
		adminRepo: adminRepo,
		userRepo:  userRepo,
	}
}

// AdminLoginRequest 管理员登录请求
type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AdminLogin 管理员登录
func (s *AdminService) AdminLogin(req *AdminLoginRequest) (*model.Admin, string, error) {
	// 查找管理员
	admin, err := s.adminRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, "", errors.New("用户名或密码错误")
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.Password)); err != nil {
		return nil, "", errors.New("用户名或密码错误")
	}

	// 检查状态
	if admin.Status != 1 {
		return nil, "", errors.New("账号已被禁用")
	}

	// 生成JWT
	token, err := s.generateToken(admin.ID)
	if err != nil {
		return nil, "", errors.New("生成token失败")
	}

	return admin, token, nil
}

// generateToken 生成JWT token
func (s *AdminService) generateToken(adminID uint) (string, error) {
	claims := jwt.MapClaims{
		"admin_id": adminID,
		"type":     "admin",
		"exp":      time.Now().Add(config.AppConfig.JWT.ExpireTime).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWT.Secret))
}

// GetAdminByID 根据ID获取管理员
func (s *AdminService) GetAdminByID(id uint) (*model.Admin, error) {
	return s.adminRepo.FindByID(id)
}

// CreateAdmin 创建管理员（初始化用）
func (s *AdminService) CreateAdmin(username, password, nickname string) error {
	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &model.Admin{
		Username: username,
		Password: string(hashedPassword),
		Nickname: nickname,
		Role:     "admin",
		Status:   1,
	}

	return s.adminRepo.Create(admin)
}
