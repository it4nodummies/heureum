package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"
)

type OAuthUserInfo struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	ProviderID  string `json:"provider_id"`
}

type OAuthProvider interface {
	GetName() string
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error)
}

type GenericOAuthProvider struct {
	name        string
	config      *oauth2.Config
	userInfoURL string
}

func (p *GenericOAuthProvider) GetName() string { return p.name }

func (p *GenericOAuthProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *GenericOAuthProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.config.Exchange(ctx, code)
}

func (p *GenericOAuthProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := p.config.Client(ctx, token)
	resp, err := client.Get(p.userInfoURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &OAuthUserInfo{
		Email:       getStr(data, "email"),
		Username:    getStr(data, "username"),
		DisplayName: getStr(data, "name"),
		AvatarURL:   getStr(data, "avatar_url"),
		ProviderID:  fmt.Sprintf("%s:%v", p.name, data["id"]),
	}, nil
}

func getStr(data map[string]interface{}, key string) string {
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

func NewOAuthProvider(name, clientID, clientSecret, redirectURL, authURL, tokenURL, userInfoURL string, scopes []string) OAuthProvider {
	return &GenericOAuthProvider{
		name: name,
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: tokenURL,
			},
		},
		userInfoURL: userInfoURL,
	}
}
