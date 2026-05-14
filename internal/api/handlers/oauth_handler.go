package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/user"
)

type OAuthHandler struct {
	DB        *gorm.DB
	Secret    string
	BaseURL   string
	Providers map[string]auth.OAuthProvider
	States    map[string]string
}

func NewOAuthHandler(db *gorm.DB, secret, baseURL string) *OAuthHandler {
	return &OAuthHandler{
		DB:        db,
		Secret:    secret,
		BaseURL:   baseURL,
		Providers: make(map[string]auth.OAuthProvider),
		States:    make(map[string]string),
	}
}

func (h *OAuthHandler) AddProvider(provider auth.OAuthProvider) {
	h.Providers[provider.GetName()] = provider
}

func (h *OAuthHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	p, ok := h.Providers[providerName]
	if !ok {
		http.Error(w, `{"error":"unsupported provider"}`, http.StatusBadRequest)
		return
	}
	stateBytes := make([]byte, 16)
	rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)
	h.States[state] = providerName
	http.Redirect(w, r, p.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	p, ok := h.Providers[providerName]
	if !ok {
		http.Error(w, `{"error":"unsupported provider"}`, http.StatusBadRequest)
		return
	}
	state := r.URL.Query().Get("state")
	if _, ok := h.States[state]; !ok {
		http.Error(w, `{"error":"invalid state"}`, http.StatusBadRequest)
		return
	}
	delete(h.States, state)
	code := r.URL.Query().Get("code")
	token, err := p.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, `{"error":"failed to exchange token"}`, http.StatusInternalServerError)
		return
	}
	info, err := p.GetUserInfo(r.Context(), token)
	if err != nil {
		http.Error(w, `{"error":"failed to get user info"}`, http.StatusInternalServerError)
		return
	}
	var u user.User
	if err := h.DB.Where("email = ?", info.Email).First(&u).Error; err != nil {
		u = user.User{
			Email:       info.Email,
			Username:    info.Username,
			DisplayName: info.DisplayName,
			AvatarURL:   info.AvatarURL,
			IsActive:    true,
		}
		u.ID = uuid.New().String()
		h.DB.Create(&u)
	}
	jwtToken, _ := auth.GenerateToken(h.Secret, u.ID, 24*time.Hour)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": jwtToken, "provider": providerName})
}
