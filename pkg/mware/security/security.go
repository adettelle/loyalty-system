package security

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"

	"github.com/golang-jwt/jwt/v5"
)

var (
	secret = []byte("my_secret_key")
)

type UserPassword struct {
	login string
	pwd   string
}

// VerifyUser — функция, которая выполняет аутентификацию и авторизацию пользователя
// user — логин пользователя, pass — пароль, permission — необходимая привилегия.
// если пользователь ввел правильные данные, и у него есть необходимая привилегия — возвращаем true, иначе — false
func VerifyUser(login string, pass string, db *sql.DB, ctx context.Context) bool {
	// получаем хеш пароля
	hashedPassword := sha256.Sum256([]byte(pass))
	hashStringPassword := hex.EncodeToString(hashedPassword[:])
	log.Println(hashStringPassword)

	// проверяем введенные данные
	sqlSt := `select login, password from customer where login = $1;`
	row := db.QueryRowContext(ctx, sqlSt, login)

	var sqlStPassword UserPassword

	err := row.Scan(&sqlStPassword.login, &sqlStPassword.pwd)
	if err != nil {
		log.Printf("Error in authorization %s", login)
		return false
	}
	return true
}

// VerifyToken — функция, которая выполняет аутентификацию и авторизацию пользователя.
// token — JWT пользователя.
// если у пользователь ввел правильные данные, и у него есть необходимая привилегия -
// возвращаем true и логин пользователя, иначе - false
func VerifyToken(token string) (string, bool) {
	log.Println("in VerifyToken")
	jwtToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		return secret, nil
	})

	log.Println("jwtToken:", jwtToken)

	if err != nil {
		log.Printf("Failed to parse token: %s\n", err)
		return "", false
	}
	if !jwtToken.Valid {
		return "", false
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", false
	}

	loginRaw, ok := claims["login"]
	log.Println("claims[login]:", claims["login"])
	if !ok {
		return "", false
	}

	login, ok := loginRaw.(string)
	if !ok {
		return "", false
	}
	log.Println("login:", login)
	return login, true
}

func GenerateJwtToken(userLogin string) (string, error) {
	// создаём payload
	claims := jwt.MapClaims{
		"login": userLogin,
	}

	// создаём jwt и указываем payload
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// получаем подписанный токен
	signedToken, err := jwtToken.SignedString(secret)
	if err != nil {
		log.Printf("failed to sign jwt: %s\n", err)
		return "", err
	}
	log.Println("Result token: " + signedToken) // eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJsb2dpbiI6Iml2YW5AeWEucnUifQ.aFyFmyfsB_-cfzyBIUSUJqLOmgkUDOg_SvQUckpQCfo
	return signedToken, nil
}
