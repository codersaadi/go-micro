package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/codersaadi/go-micro/internal/models"
	"github.com/codersaadi/go-micro/pkg/micro"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgxpool"

	"go.uber.org/zap"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrEmailExists  = errors.New("email already exists")
	ErrInvalidInput = errors.New("invalid input")
)

type UserRepository interface {
	CreateUser(ctx context.Context, params models.CreateUserParams) (*models.User, error)
	GetUserByID(ctx context.Context, id int32) (*models.User, error)
	UpdateUser(ctx context.Context, params models.UpdateUserParams) (*models.User, error)
	DeleteUser(ctx context.Context, id int32) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
}

type userRepo struct {
	pool    *pgxpool.Pool
	queries *models.Queries
	logger  micro.Logger
}

func NewUserRepository(pool *pgxpool.Pool, logger micro.Logger) UserRepository {
	return &userRepo{
		pool:    pool,
		queries: models.New(pool),
		logger:  logger.With(zap.String("component", "user-repository")),
	}
}

func (r *userRepo) CreateUser(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
	logger := r.logger.With(
		zap.String("method", "CreateUser"),
		zap.Any("params", params),
	)

	user, err := r.queries.CreateUser(ctx, params)
	if err != nil {
		if isDuplicateKeyError(err) {
			logger.Warn("duplicate email attempt")
			return nil, ErrEmailExists
		}
		logger.Error("failed to create user", zap.Error(err))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	logger.Info("user created successfully")
	return &user, nil
}

func (r *userRepo) GetUserByID(ctx context.Context, id int32) (*models.User, error) {
	logger := r.logger.With(
		zap.String("method", "GetUserByID"),
		zap.Int32("user_id", id),
	)

	user, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("user not found")
			return nil, ErrUserNotFound
		}
		logger.Error("failed to get user", zap.Error(err))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}
func (r *userRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	logger := r.logger.With(
		zap.String("method", "GetUserByID"),
		zap.String("email", email),
	)

	user, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("user not found")
			return nil, ErrUserNotFound
		}
		logger.Error("failed to get user", zap.Error(err))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (r *userRepo) UpdateUser(ctx context.Context, params models.UpdateUserParams) (*models.User, error) {
	logger := r.logger.With(
		zap.String("method", "UpdateUser"),
		zap.Any("params", params),
	)

	user, err := r.queries.UpdateUser(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("user not found for update")
			return nil, ErrUserNotFound
		}
		if isDuplicateKeyError(err) {
			logger.Warn("duplicate email attempt in updint64ate")
			return nil, ErrEmailExists
		}
		logger.Error("failed to update user", zap.Error(err))
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	logger.Info("user updated successfully")
	return &user, nil
}

func (r *userRepo) DeleteUser(ctx context.Context, id int32) error {
	logger := r.logger.With(
		zap.String("method", "DeleteUser"),
		zap.Int32("user_id", id),
	)

	err := r.queries.DeleteUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("user not found for deletion")
			return ErrUserNotFound
		}
		logger.Error("failed to delete user", zap.Error(err))
		return fmt.Errorf("failed to delete user: %w", err)
	}

	logger.Info("user deleted successfully")
	return nil
}

func isDuplicateKeyError(err error) bool {
	var pgErr *pgx.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
