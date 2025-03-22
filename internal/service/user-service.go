// internal/service/user.go
package service

import (
	"context"
	"errors"
	"regexp"

	"github.com/codersaadi/go-micro/internal/models"
	repository "github.com/codersaadi/go-micro/internal/respository"
	"github.com/codersaadi/go-micro/pkg/micro"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailExists        = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type UserService interface {
	RegisterUser(ctx context.Context, params RegisterParams) (*models.User, error)
	GetUserByID(ctx context.Context, id int32) (*models.User, error)
	UpdateUser(ctx context.Context, params UpdateParams) (*models.User, error)
	DeleteUser(ctx context.Context, id int32) error
	Authenticate(ctx context.Context, email, password string) (*models.User, error)
}

type userService struct {
	repo   repository.UserRepository
	logger micro.Logger
}

func NewUserService(repo repository.UserRepository, logger micro.Logger) UserService {
	return &userService{
		repo:   repo,
		logger: logger.With(zap.String("component", "user-service")),
	}
}

type RegisterParams struct {
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type UpdateParams struct {
	ID       int32   `json:"-"`
	Name     *string `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Email    *string `json:"email,omitempty" validate:"omitempty,email"`
	Password *string `json:"password,omitempty" validate:"omitempty,min=8,max=72"`
}

func (s *userService) RegisterUser(ctx context.Context, params RegisterParams) (*models.User, error) {
	const cost = bcrypt.DefaultCost
	logger := s.logger.With(
		micro.MethodField("RegisterUser"),
		micro.EmailField(params.Email),
	)

	// Validate password strength
	if err := validatePassword(params.Password); err != nil {
		logger.Warn("password validation failed")
		return nil, err
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(params.Password), cost)
	if err != nil {
		logger.Error("failed to hash password", micro.ErrorField(err))
		return nil, micro.ErrInternalServer
	}

	// Create user in repository
	user, err := s.repo.CreateUser(ctx, models.CreateUserParams{
		Name:     params.Name,
		Email:    params.Email,
		Password: string(hashedPassword),
	})

	if err != nil {
		if errors.Is(err, repository.ErrEmailExists) {
			return nil, ErrEmailExists
		}
		logger.Error("failed to create user", micro.ErrorField(err))
		return nil, micro.ErrInternalServer
	}

	logger.Info("user registered successfully", micro.UserIDField(user.ID))
	return user, nil
}

func (s *userService) GetUserByID(ctx context.Context, id int32) (*models.User, error) {
	logger := s.logger.With(
		micro.MethodField("GetUserByID"),
		micro.UserIDField(id),
	)

	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		logger.Error("failed to retrieve user", micro.ErrorField(err))
		return nil, micro.ErrInternalServer
	}

	return user, nil
}

func (s *userService) UpdateUser(ctx context.Context, params UpdateParams) (*models.User, error) {
	logger := s.logger.With(
		micro.MethodField("UpdateUser"),
		micro.UserIDField(params.ID),
	)

	updateParams := models.UpdateUserParams{ID: params.ID}

	if params.Name != nil {
		updateParams.Name = *params.Name
	}

	if params.Email != nil {
		updateParams.Email = *params.Email
	}

	if params.Password != nil {
		if err := validatePassword(*params.Password); err != nil {
			return nil, err
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*params.Password), bcrypt.DefaultCost)
		if err != nil {
			logger.Error("failed to hash password", micro.ErrorField(err))
			return nil, micro.ErrInternalServer
		}
		updateParams.Password = string(hashedPassword)
	}

	user, err := s.repo.UpdateUser(ctx, updateParams)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		if errors.Is(err, repository.ErrEmailExists) {
			return nil, ErrEmailExists
		}
		logger.Error("failed to update user", micro.ErrorField(err))
		return nil, micro.ErrInternalServer
	}

	logger.Info("user updated successfully")
	return user, nil
}

func (s *userService) DeleteUser(ctx context.Context, id int32) error {
	logger := s.logger.With(
		micro.MethodField("DeleteUser"),
		micro.UserIDField(id),
	)

	if err := s.repo.DeleteUser(ctx, id); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		logger.Error("failed to delete user", micro.ErrorField(err))
		return micro.ErrInternalServer
	}

	logger.Info("user deleted successfully")
	return nil
}

func (s *userService) Authenticate(ctx context.Context, email, password string) (*models.User, error) {
	logger := s.logger.With(
		micro.MethodField("Authenticate"),
		micro.EmailField(email),
	)
	if !isValidEmail(email) {
		return nil, micro.NewAPIError(403, "invalid email data")
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		logger.Error("failed to retrieve user", micro.ErrorField(err))
		return nil, micro.ErrInternalServer
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		logger.Warn("invalid password attempt")
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	return nil
}

// Helper function for email validation
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
