package handlers

import (
	"encoding/json"
	"errors"
	"file-sharing-server/models"
	"file-sharing-server/services"
	"net/http"
	"strings"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthUserResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

type RegisterResponse struct {
	Message string           `json:"message"`
	User    AuthUserResponse `json:"user"`
}

type LoginResponse struct {
	Message string           `json:"message"`
	Token   string           `json:"token"`
	User    AuthUserResponse `json:"user"`
}

// Register godoc
//
//	@Summary	Register a new user
//	@Description	Creates a user account and stores the password using Argon2id.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RegisterRequest	true	"Registration details"
//	@Success		201		{object}	RegisterResponse
//	@Failure		400		{object}	map[string]string	"Invalid registration input"
//	@Failure		409		{object}	map[string]string	"Email already registered"
//	@Failure		500		{object}	map[string]string	"Registration failed"
//	@Router			/api/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid Request Body",
		})
		return
	}

	user, err := h.authService.Register(
		r.Context(),
		req.Name,
		req.Email,
		req.Password,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidInput):
			writeAuthJSON(w, http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		case errors.Is(err, services.ErrEmailAlreadyExists):
			writeAuthJSON(w, http.StatusConflict, map[string]string{
				"error": "Email already registered",
			})
		default:
			writeAuthJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Failed to Register user",
			})
		}
		return
	}
	writeAuthJSON(w, http.StatusCreated, map[string]any{
		"message": "Registration successful",
		"user":    toAuthUserResponse(user),
	})
}

// Login godoc
//
//	@Summary	Authenticate a user
//	@Description	Verifies email and password and returns a JWT access token.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequest	true	"Login details"
//	@Success		200		{object}	LoginResponse
//	@Failure		400		{object}	map[string]string	"Email and password are required"
//	@Failure		401		{object}	map[string]string	"Invalid email or password"
//	@Failure		500		{object}	map[string]string	"Login failed"
//	@Router			/api/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request Body",
		})
		return
	}

	if strings.TrimSpace(req.Email) == "" || req.Password == "" {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Email and password are required",
		})
		return
	}

	token, user, err := h.authService.Login(
		r.Context(),
		req.Email,
		req.Password,
	)

	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			writeAuthJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to login",
		})
		return
	}

	writeAuthJSON(w, http.StatusOK, map[string]any{
		"message": "Login successful",
		"token":   token,
		"user":    toAuthUserResponse(user),
	})
}

func toAuthUserResponse(user *models.User) AuthUserResponse {
	return AuthUserResponse{
		ID:        user.ID.Hex(),
		Name:      user.Name,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func writeAuthJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
