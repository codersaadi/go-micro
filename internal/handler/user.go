package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/codersaadi/go-micro/internal/service"
	"github.com/codersaadi/go-micro/pkg/micro"
)

// Example Handlers
type UserHandler struct {
	service service.UserService
	app     *micro.App
}

func NewUserHandler(app *micro.App, service service.UserService) *UserHandler {
	return &UserHandler{
		service: service,
		app:     app,
	}
}

func (h *UserHandler) Register(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var params service.RegisterParams
	if err := h.app.Decode(r, &params); err != nil {
		return err
	}
	user, err := h.service.RegisterUser(ctx, params)
	if err != nil {
		return err
	}

	return h.app.JSON(w, http.StatusCreated, map[string]interface{}{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}

func (h *UserHandler) Login(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var credentials struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := h.app.Decode(r, &credentials); err != nil {
		return err
	}

	user, err := h.service.Authenticate(ctx, credentials.Email, credentials.Password)
	if err != nil {
		return micro.NewAPIError(http.StatusUnauthorized, "invalid credentials")
	}

	return h.app.JSON(w, http.StatusOK, map[string]interface{}{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}

// internal/handler/user.go

func (h *UserHandler) GetUser(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	userID, err := h.app.URLParamInt(r, "id")
	if err != nil {
		return micro.NewAPIError(http.StatusBadRequest, "invalid user ID")
	}

	user, err := h.service.GetUserByID(ctx, int32(userID))
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return micro.NewAPIError(http.StatusNotFound, "user not found")
		}
		return micro.NewAPIError(http.StatusInternalServerError, "failed to retrieve user")
	}

	return h.app.JSON(w, http.StatusOK, map[string]interface{}{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}

func (h *UserHandler) UpdateUser(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	userID, err := h.app.URLParamInt(r, "id")
	if err != nil {
		return micro.NewAPIError(http.StatusBadRequest, "invalid user ID")
	}

	var params service.UpdateParams
	if err := h.app.Decode(r, &params); err != nil {
		return err
	}

	params.ID = int32(userID)
	user, err := h.service.UpdateUser(ctx, params)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			return micro.NewAPIError(http.StatusNotFound, "user not found")
		case errors.Is(err, service.ErrEmailExists):
			return micro.NewAPIError(http.StatusConflict, "email already exists")
		default:
			return micro.NewAPIError(http.StatusInternalServerError, "failed to update user")
		}
	}

	return h.app.JSON(w, http.StatusOK, map[string]interface{}{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}

func (h *UserHandler) DeleteUser(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	userID, err := h.app.URLParamInt(r, "id")
	if err != nil {
		return micro.NewAPIError(http.StatusBadRequest, "invalid user ID")
	}

	if err := h.service.DeleteUser(ctx, int32(userID)); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return micro.NewAPIError(http.StatusNotFound, "user not found")
		}
		return micro.NewAPIError(http.StatusInternalServerError, "failed to delete user")
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
