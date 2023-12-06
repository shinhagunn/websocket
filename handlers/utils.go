package handlers

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/shinhagunn/websocket/config"
	"github.com/shinhagunn/websocket/pkg/auth"
)

const prefix = "Bearer "

func token(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(string(authHeader), prefix) {
		return ""
	}

	return authHeader[len(prefix):]
}

func authHandler(h http.HandlerFunc, key *rsa.PublicKey, mustAuth bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// auth, err := auth.ParseAndValidate(token(r), key)
		// if err != nil && mustAuth {
		// 	w.WriteHeader(http.StatusUnauthorized)
		// 	return
		// }

		// if err == nil {
		// 	r.Header.Set("JwtUID", auth.UID)
		// 	r.Header.Set("JwtRole", auth.Role)
		// } else {
		// 	r.Header.Del("JwtUID")
		// 	r.Header.Del("JwtRole")
		// }
		h(w, r)
	}
}

func getPublicKey(config *config.Config) (pub *rsa.PublicKey, err error) {
	ks := auth.KeyStore{}
	encPem := config.JWTPublicKey
	ks.LoadPublicKeyFromString(encPem)

	if err != nil {
		return nil, err
	}
	if ks.PublicKey == nil {
		return nil, fmt.Errorf("failed")
	}
	return ks.PublicKey, nil
}

func checkSameOrigin(origins string) func(r *http.Request) bool {
	if origins == "" {
		return func(r *http.Request) bool {
			return true
		}
	}

	hosts := []string{}

	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if strings.HasPrefix(o, "http://") || strings.HasPrefix(o, "https://") {
			u, err := url.Parse(o)
			if err != nil || u.Host == "" {
				panic("Failed to parse url in API_CORS_ORIGINS: " + o)
			}
			hosts = append(hosts, u.Host)
		} else {
			hosts = append(hosts, o)
		}
	}

	return func(r *http.Request) bool {
		origin := r.Header["Origin"]
		if len(origin) == 0 {
			return true
		}
		u, err := url.Parse(origin[0])
		if err != nil {
			return false
		}

		for _, host := range hosts {
			if strings.EqualFold(u.Host, host) {
				return true
			}
		}
		return false
	}
}
