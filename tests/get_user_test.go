package tests

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis"
	"github.com/gjg-sarismet/db"
	"github.com/gjg-sarismet/endpoints"
	"github.com/go-redis/redis"
	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
)

func TestGetUser(t *testing.T) {
	app := endpoints.App{}
	mr, err := miniredis.Run()
	if err != nil {
		log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	newRedisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	if newRedisClient == nil {
		log.Fatalf("RedisDB is nil")
	}
	RedisDB := &db.RedisDatabase{
		Client: newRedisClient,
	}
	mockDB, _, err := sqlmock.New()
	if mockDB == nil {
		log.Fatalf("db is nil")
	}
	SQLDB := &db.SQLDatabase{
		SqlClient: mockDB,
	}
	if err != nil {
		log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	app.RedisDB = RedisDB
	app.SQLDB = SQLDB
	var user *db.User

	userJSON := `{"display_name":"Snow","country":"na"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if assert.NoError(t, app.CreateUser(c)) {
		json.Unmarshal(rec.Body.Bytes(), &user)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "Snow", user.Display_Name)
	}

	req = httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("user_guid")
	c.SetParamValues("/" + user.User_Id)
	if assert.NoError(t, app.GetUserProile(c)) {
		var responseUser db.User
		json.Unmarshal(rec.Body.Bytes(), &responseUser)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, responseUser.Points, float64(0))
		assert.Equal(t, responseUser.User_Id, user.User_Id)
		assert.Equal(t, responseUser.Rank, user.Rank)
	}

}
