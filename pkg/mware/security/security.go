package security

import (
	"crypto/sha256"
	"encoding/hex"
	"log"

	"github.com/adettelle/loyalty-system/internal/gophermart/model"
	"github.com/golang-jwt/jwt/v5"
)

// VerifyUser — функция, которая выполняет аутентификацию и авторизацию пользователя
// user — логин пользователя, pass — пароль, permission — необходимая привилегия.
// если пользователь ввел правильные данные, и у него есть необходимая привилегия — возвращаем true, иначе — false
func VerifyUser(login string, pass string, gmStorage *model.GophermartStorage) bool {
	// получаем хеш пароля
	hashedPassword := sha256.Sum256([]byte(pass))
	hashStringPassword := hex.EncodeToString(hashedPassword[:])
	log.Println(hashStringPassword)

	// проверяем введенные данные
	cust, err := gmStorage.GetCustomerByLogin(login)
	if err != nil {
		log.Printf("Error in authorization %s", cust.Login)
		return false
	}

	log.Println(cust.Password)
	return cust.Password == hashStringPassword
}

// VerifyToken — функция, которая выполняет аутентификацию и авторизацию пользователя.
// token — JWT пользователя.
// если у пользователь ввел правильные данные, и у него есть необходимая привилегия -
// возвращаем true и логин пользователя, иначе - false
func VerifyToken(secret []byte, token string) (string, bool) {
	jwtToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		return secret, nil
	})

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

func GenerateJwtToken(secret []byte, userLogin string) (string, error) {
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
