package userdao

import (
	"context"
	"errors"
	"time"

	"vesko/auth"

	"gorm.io/gorm"
)

type UserModel struct {
	ID             uint   `gorm:"primaryKey;autoIncrement"`
	Username       string `gorm:"size:255;uniqueIndex;not null"`
	PasswordHash   string `gorm:"size:255;not null"`
	Email          string `gorm:"size:255;uniqueIndex;not null"`
	Mobile         string `gorm:"size:20;uniqueIndex;not null"`
	MobileVerified bool   `gorm:"not null;default:false"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

type PostgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) AutoMigrate(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&UserModel{})
}

func (r *PostgresRepository) GetUserDetailsByUsername(ctx context.Context, username string) (auth.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, err
	}

	return toDomainUser(model), nil
}

func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (auth.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, err
	}

	return toDomainUser(model), nil
}

func (r *PostgresRepository) GetUserByMobile(ctx context.Context, mobile string) (auth.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("mobile = ?", mobile).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, err
	}

	return toDomainUser(model), nil
}

func (r *PostgresRepository) RegisterUser(ctx context.Context, user auth.User) (auth.User, error) {
	model := UserModel{
		Username:       user.Username,
		PasswordHash:   user.PasswordHash,
		Email:          user.Email,
		Mobile:         user.Mobile,
		MobileVerified: user.MobileVerified,
	}

	err := r.db.WithContext(ctx).Create(&model).Error
	if err == nil {
		return toDomainUser(model), nil
	}

	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return auth.User{}, auth.ErrUserAlreadyExists
	}

	var existing UserModel
	switch {
	case r.db.WithContext(ctx).Where("username = ?", user.Username).First(&existing).Error == nil:
		return auth.User{}, auth.ErrUsernameAlreadyExists
	case r.db.WithContext(ctx).Where("email = ?", user.Email).First(&existing).Error == nil:
		return auth.User{}, auth.ErrEmailAlreadyExists
	case r.db.WithContext(ctx).Where("mobile = ?", user.Mobile).First(&existing).Error == nil:
		return auth.User{}, auth.ErrMobileAlreadyExists
	}

	return auth.User{}, err
}

func toDomainUser(model UserModel) auth.User {
	return auth.User{
		ID:             model.ID,
		Username:       model.Username,
		PasswordHash:   model.PasswordHash,
		Email:          model.Email,
		Mobile:         model.Mobile,
		MobileVerified: model.MobileVerified,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}
