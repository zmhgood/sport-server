package service

import (
	"encoding/json"
	"errors"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo   *repository.UserRepository
	smsService *SMSService
}

func NewAuthService(userRepo *repository.UserRepository, smsService *SMSService) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		smsService: smsService,
	}
}

// WeChatLoginResult 微信登录结果
type WeChatLoginResult struct {
	Token    string      `json:"token"`
	UserInfo *model.User `json:"userInfo"`
}

// WeChatLogin 微信登录
func (s *AuthService) WeChatLogin(code string) (*WeChatLoginResult, error) {
	// 调用微信接口获取openid
	openID, _, err := s.getWeChatOpenID(code)
	if err != nil {
		return nil, err
	}

	// 查找或创建用户
	user, err := s.userRepo.FindByOpenID(openID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 用户不存在，创建新用户
		user = &model.User{
			OpenID: openID,
		}
		if err = s.userRepo.Create(user); err != nil {
			if e, ok := err.(*driver.MySQLError); ok && e.Number == 1062 {
				user, err = s.userRepo.FindByOpenID(openID)
			}
			if err != nil {
				return nil, errors.New("创建用户失败")
			}
		}
	}

	if err != nil {
		return nil, errors.New("用户读取失败，请重试")
	}
	// 生成JWT token
	token, err := s.generateToken(user.ID)
	if err != nil {
		return nil, errors.New("生成token失败")
	}

	return &WeChatLoginResult{
		Token:    token,
		UserInfo: user,
	}, nil
}

// getWeChatOpenID 获取微信openid
func (s *AuthService) getWeChatOpenID(code string) (string, string, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		config.AppConfig.WeChat.AppID,
		config.AppConfig.WeChat.AppSecret,
		code,
	)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(url)
	if err != nil {
		return "", "", errors.New("请求微信登录服务失败")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var result struct {
		OpenID     string `json:"openid"`
		SessionKey string `json:"session_key"`
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", err
	}

	if result.ErrCode != 0 {
		return "", "", fmt.Errorf("微信登录失败: %s", result.ErrMsg)
	}

	if result.OpenID == "" {
		return "", "", errors.New("微信登录响应无效")
	}
	return result.OpenID, result.SessionKey, nil
}

// generateToken 生成JWT token
func (s *AuthService) generateToken(userID uint) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(config.AppConfig.JWT.ExpireTime).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWT.Secret))
}

// GetUserByID 根据ID获取用户
func (s *AuthService) GetUserByID(id uint) (*model.User, error) {
	return s.userRepo.FindByID(id)
}

// UserInfoUpdate 用户信息更新
type UserInfoUpdate struct {
	NickName     *string  `json:"nick_name"`
	AvatarURL    *string  `json:"avatar_url"`
	Gender       *int     `json:"gender"`
	Age          *int     `json:"age"`
	Height       *int     `json:"height"`
	Weight       *float64 `json:"weight"`
	Phone        *string  `json:"phone"`
	HealthStatus *string  `json:"health_status"`
}

func (s *AuthService) UpdateUserInfo(userID uint, update *UserInfoUpdate) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if update.NickName != nil {
		name := strings.TrimSpace(*update.NickName)
		if utf8.RuneCountInString(name) > 64 {
			return errors.New("昵称太长")
		}
		if name != "" {
			user.NickName = name
		}
	}
	if update.AvatarURL != nil && *update.AvatarURL != "" {
		user.AvatarURL = *update.AvatarURL
	}
	if update.Gender != nil {
		if *update.Gender < 0 || *update.Gender > 2 {
			return errors.New("性别参数无效")
		}
		user.Gender = *update.Gender
	}
	if update.Age != nil {
		if *update.Age < 0 || *update.Age > 150 {
			return errors.New("年龄参数无效")
		}
		user.Age = *update.Age
	}
	if update.Height != nil {
		if *update.Height != 0 && (*update.Height < 50 || *update.Height > 250) {
			return errors.New("身高参数无效")
		}
		user.Height = *update.Height
	}
	if update.Weight != nil {
		if *update.Weight != 0 && (*update.Weight < 20 || *update.Weight > 300) {
			return errors.New("体重参数无效")
		}
		user.Weight = *update.Weight
	}
	if update.Phone != nil && *update.Phone != "" {
		user.Phone = *update.Phone
	}
	if update.HealthStatus != nil {
		if utf8.RuneCountInString(*update.HealthStatus) > 2000 {
			return errors.New("健康状况内容太长")
		}
		user.HealthStatus = *update.HealthStatus
	}
	return s.userRepo.Update(user)
}

// SMSLoginResult 短信登录结果
type SMSLoginResult struct {
	Token    string      `json:"token"`
	UserInfo *model.User `json:"userInfo"`
	IsNew    bool        `json:"is_new"` // 是否新用户
}

// SMSLogin 短信验证码登录
func (s *AuthService) SMSLogin(phone, code string) (*SMSLoginResult, error) {
	// 验证验证码
	valid, err := s.smsService.VerifyCode(phone, code, "login")
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errors.New("验证码无效")
	}

	// 查找或创建用户
	user, err := s.userRepo.FindByPhone(phone)
	isNew := false
	if err != nil {
		// 用户不存在，创建新用户
		isNew = true
		user = &model.User{
			Phone: phone,
		}
		if err := s.userRepo.Create(user); err != nil {
			return nil, errors.New("创建用户失败")
		}
	}

	// 生成JWT token
	token, err := s.generateToken(user.ID)
	if err != nil {
		return nil, errors.New("生成token失败")
	}

	return &SMSLoginResult{
		Token:    token,
		UserInfo: user,
		IsNew:    isNew,
	}, nil
}

// HashPassword 哈希密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
