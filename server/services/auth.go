// 认证服务，提供用户认证、密码哈希和令牌管理
package services

import (
	"rental-server/config"
	"rental-server/models"
	"rental-server/utils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService 认证服务
type AuthService struct {
	DB  *gorm.DB
	Cfg *config.Config
}

// NewAuthService 创建认证服务实例
func NewAuthService(db *gorm.DB, cfg *config.Config) *AuthService {
	return &AuthService{DB: db, Cfg: cfg}
}

// GetUserByUsername 根据用户名查找用户
func (s *AuthService) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := s.DB.Where("username = ?", username).First(&user).Error
	return &user, err
}

// GetUserByID 根据ID查找用户
func (s *AuthService) GetUserByID(id uint) (*models.User, error) {
	var user models.User
	err := s.DB.First(&user, id).Error
	return &user, err
}

// CreateUser 创建新用户
func (s *AuthService) CreateUser(user *models.User) error {
	return s.DB.Create(user).Error
}

// UpdateUser 更新用户信息
func (s *AuthService) UpdateUser(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteUser 删除用户
func (s *AuthService) DeleteUser(id uint) error {
	return s.DB.Delete(&models.User{}, id).Error
}

// ListUsers 获取所有用户列表
func (s *AuthService) ListUsers() ([]models.User, error) {
	var users []models.User
	err := s.DB.Find(&users).Error
	return users, err
}

// HashPassword 对密码进行bcrypt哈希
func (s *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPassword 验证密码是否匹配
func (s *AuthService) CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// GenerateToken 生成JWT访问令牌
func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	buildingID := uint(0)
	if user.BuildingID != nil {
		buildingID = *user.BuildingID
	}
	return utils.GenerateToken(user.ID, user.Username, user.Role, s.Cfg.JWTSecret, buildingID)
}

// GenerateRefreshToken 生成JWT刷新令牌
func (s *AuthService) GenerateRefreshToken(user *models.User) (string, error) {
	buildingID := uint(0)
	if user.BuildingID != nil {
		buildingID = *user.BuildingID
	}
	return utils.GenerateRefreshToken(user.ID, user.Username, user.Role, s.Cfg.JWTSecret, buildingID)
}

// GetBuildingByID 根据ID查找楼栋
func (s *AuthService) GetBuildingByID(id uint) (*models.Building, error) {
	var building models.Building
	err := s.DB.First(&building, id).Error
	return &building, err
}

// IsBuildingExpired 检查楼栋是否已过期
func (s *AuthService) IsBuildingExpired(building *models.Building) bool {
	if building.ExpiredAt == "" {
		return false
	}
	expDate, err := utils.ParseDate(building.ExpiredAt)
	if err != nil {
		return false
	}
	return utils.Now().After(expDate)
}
