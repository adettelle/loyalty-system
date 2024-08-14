package mware

import (
	"log"
	"net/http"
	"strings"

	"github.com/adettelle/loyalty-system/pkg/mware/security"
)

// AuthMwr добавляет аутентификацию пользователя и возвращает новый http.Handler
func AuthMwr(h http.HandlerFunc) http.HandlerFunc {
	authFn := func(w http.ResponseWriter, r *http.Request) {
		// получаем http header вида 'Bearer {jwt}'
		authHeaderValue := r.Header.Get("Authorization")
		log.Println("authHeaderValue:", authHeaderValue)
		if authHeaderValue == "" {
			w.WriteHeader(http.StatusUnauthorized) // пользователь не аутентифицирован
			return
		}

		// проверяем доступы
		if authHeaderValue != "" {
			bearerToken := strings.Split(authHeaderValue, " ")
			log.Println("bearerToken:", bearerToken[1])
			if len(bearerToken) == 2 {
				login, ok := security.VerifyToken(bearerToken[1])
				if !ok {
					w.WriteHeader(http.StatusUnauthorized) // пользователь не аутентифицирован
					return
				} else {
					r.Header.Set("x-user", login)
					h.ServeHTTP(w, r) // передали следующей функции, которую мы обрамляем middleware'ом
					return
				}
			}
		}

	}
	// возвращаем функционально расширенный хендлер
	return http.HandlerFunc(authFn)
}
