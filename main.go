package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"os"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

var messages []string
var rsakeys map[string]*rsa.PublicKey

func Messages(c *gin.Context) {
	if Verify(c) {
		message := c.PostForm("message")
		messages = append(messages, message)
		c.JSON(http.StatusOK, gin.H{"messages": messages})
	}
}

func Verify(c *gin.Context) bool {
	isValid := false
	errorMessage := ""
	tokenString := c.Request.Header.Get("Authorization")
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return rsakeys[token.Header["kid"].(string)], nil
		})
		if err != nil {
			errorMessage = err.Error()
		} else if !token.Valid {
			errorMessage = "Invalid token"
		} else if token.Header["alg"] == nil {
			errorMessage = "alg must be defined"
		} else if token.Claims.(jwt.MapClaims)["aud"] != "api://default" {
			errorMessage = "Invalid aud"
		} else if !strings.Contains(token.Claims.(jwt.MapClaims)["iss"].(string), os.Getenv("OKTA_DOMAIN")) {
			errorMessage = "Invalid iss"
		} else {
			isValid = true
		}
		if !isValid {
			c.String(http.StatusForbidden, errorMessage)
		}
	} else {
		c.String(http.StatusUnauthorized, "Unauthorized")
	}
	return isValid
}

func GetPublicKeys() {
	rsakeys = make(map[string]*rsa.PublicKey)
	var body map[string]interface{}
	uri := "https://" + os.Getenv("OKTA_DOMAIN") + "/oauth2/default/v1/keys"
	resp, _ := http.Get(uri)
	json.NewDecoder(resp.Body).Decode(&body)
	for _, bodykey := range body["keys"].([]interface{}) {
		key := bodykey.(map[string]interface{})
		kid := key["kid"].(string)
		rsakey := new(rsa.PublicKey)
		number, _ := base64.RawURLEncoding.DecodeString(key["n"].(string))
		rsakey.N = new(big.Int).SetBytes(number)
		rsakey.E = 65537
		rsakeys[kid] = rsakey
	}
}

func main() {
	GetPublicKeys()
	r := gin.Default()
	r.Use(static.Serve("/", static.LocalFile("./client", false)))
	r.POST("/api/messages", Messages)
	r.Run()
}
