package services

import (
	"rental-server/config"
	"rental-server/models"
	"rental-server/utils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewAuthService(db *gorm.DB, cfg *config.Config) *AuthService {
	return &AuthService{DB: db, Cfg: cfg}
}

func (s *AuthService) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := s.DB.Where("username = ?", username).First(&user).Error
	return &user, err
}

func (s *AuthService) GetUserByID(id uint) (*models.User, error) {
	var user models.User
	err := s.DB.First(&user, id).Error
	return &user, err
}

func (s *AuthService) CreateUser(user *models.User) error {
	return s.DB.Create(user).Error
}

func (s *AuthService) UpdateUser(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}

func (s *AuthService) DeleteUser(id uint) error {
	return s.DB.Delete(&models.User{}, id).Error
}

func (s *AuthService) ListUsers() ([]models.User, error) {
	var users []models.User
	err := s.DB.Find(&users).Error
	return users, err
}

func (s *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func (s *AuthService) CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	buildingID := uint(0)
	if user.BuildingID != nil {
		buildingID = *user.BuildingID
	}
	return utils.GenerateToken(user.ID, user.Username, user.Role, s.Cfg.JWTSecret, buildingID)
}

func (s *AuthService) GenerateRefreshToken(user *models.User) (string, error) {
	buildingID := uint(0)
	if user.BuildingID != nil {
		buildingID = *user.BuildingID
	}
	return utils.GenerateRefreshToken(user.ID, user.Username, user.Role, s.Cfg.JWTSecret, buildingID)
}

func (s *AuthService) GetBuildingByID(id uint) (*models.Building, error) {
	var building models.Building
	err := s.DB.First(&building, id).Error
	return &building, err
}

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
