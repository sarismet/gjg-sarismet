package alltest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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

var (
	app   endpoints.App
	user  *db.User
	users []*db.User
	e     *echo.Echo
)

func TestMain(m *testing.M) {
	app = endpoints.App{}
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
	code := m.Run()
	os.Exit(code)
}

func TestCreateUser(t *testing.T) {
	userJSON := `{"display_name":"Snow","country":"na"}`
	e = echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if assert.NoError(t, app.CreateUser(c)) {
		json.Unmarshal(rec.Body.Bytes(), &user)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "Snow", user.Display_Name)
	}
}

func TestCreateMultipleUsers(t *testing.T) {
	userJSON := `{"count":2,"users":[{"display_name":"ahmet","country":"tr"},{"display_name":"ismet","country":"tr"}]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if assert.NoError(t, app.CreateMultipleUsers(c)) {
		json.Unmarshal(rec.Body.Bytes(), &users)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "ahmet", users[0].Display_Name)
	}
}

func TestScoreSubmit(t *testing.T) {
	userJSON := fmt.Sprintf(`{"score_worth":100,"user_id":"%s"}`, user.User_Id)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if assert.NoError(t, app.ScoreSubmit(c)) {
		var responseScore db.Score
		json.Unmarshal(rec.Body.Bytes(), &responseScore)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, responseScore.Score_worth, float64(100))
		assert.Equal(t, responseScore.User_Id, user.User_Id)
	}
}

func TestScoreSubmitMultiple(t *testing.T) {
	userJSON := fmt.Sprintf(`{"count":2,"scores":[{"score_worth":1000,"user_id":"%s"},{"score_worth":900,"user_id":"%s"}]}`, users[0].User_Id, users[1].User_Id)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if assert.NoError(t, app.ScoreSubmitMultiple(c)) {
		var responseScores []db.Score
		json.Unmarshal(rec.Body.Bytes(), &responseScores)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, responseScores[0].Score_worth, float64(1000))
		assert.Equal(t, responseScores[1].Score_worth, float64(900))
		assert.Equal(t, responseScores[0].User_Id, users[0].User_Id)
		assert.Equal(t, responseScores[1].User_Id, users[1].User_Id)
	}
}

func TestGetLeaderBoard(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if assert.NoError(t, app.GetLeaderBoard(c)) {
		var responseUsers []db.LeaderBoardRespond
		json.Unmarshal(rec.Body.Bytes(), &responseUsers)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, responseUsers[0].Display_Name, users[0].Display_Name)
		assert.Equal(t, responseUsers[1].Display_Name, users[1].Display_Name)
	}
}

func TestGetLeaderBoardWithCountryIsoCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("country_iso_code")
	c.SetParamValues("/na")
	if assert.NoError(t, app.GetLeaderBoard(c)) {
		var responseUsers []db.LeaderBoardRespond
		json.Unmarshal(rec.Body.Bytes(), &responseUsers)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, responseUsers[0].Display_Name, user.Display_Name)
	}
}

func TestGetUserProile(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("user_guid")
	c.SetParamValues("/" + users[0].User_Id)

	if assert.NoError(t, app.GetUserProile(c)) {
		var responseUser db.User
		json.Unmarshal(rec.Body.Bytes(), &responseUser)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, responseUser.Display_Name, users[0].Display_Name)
	}
}
