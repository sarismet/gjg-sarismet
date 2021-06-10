package tests

import (
	"encoding/json"
	"fmt"
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

func TestMultipleSubmitScore(t *testing.T) {
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

	e := echo.New()
	var users []*db.User

	userJSON := `{"count":2,"users":[{"display_name":"ismet","country":"tr"},{"display_name":"ahmet","country":"tr"}]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if assert.NoError(t, app.CreateMultipleUsers(c)) {
		json.Unmarshal(rec.Body.Bytes(), &users)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "ismet", users[0].Display_Name)
	}

	userJSON = fmt.Sprintf(`{"count":2,"scores":[{"score_worth":1000.0,"user_id":"%s"},{"score_worth":100.0,"user_id":"%s"}]}`, users[1].User_Id, users[0].User_Id)
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	if assert.NoError(t, app.ScoreSubmitMultiple(c)) {
		var responseScores []db.Score
		json.Unmarshal(rec.Body.Bytes(), &responseScores)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, responseScores[1].User_Id, users[0].User_Id)
		assert.Equal(t, responseScores[0].User_Id, users[1].User_Id)
	}

}
