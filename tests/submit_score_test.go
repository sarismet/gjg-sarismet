package tests

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/gjg-sarismet/db"
	"github.com/gjg-sarismet/endpoints"
	"github.com/go-redis/redis"
	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
)

func TestSubmitScore(t *testing.T) {
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
	const (
		host         = "localhost"
		port         = 5432
		databaseuser = "postgres"
		password     = "123"
	)

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s sslmode=disable",
		host, port, databaseuser, password)

	sqldb, err := sql.Open("postgres", psqlInfo)

	if err != nil {
		log.Fatal(err)
	}

	dbName := "testdb"
	_, err = sqldb.Exec("create database " + dbName + ";")
	if err != nil {
		//handle the error
		log.Fatal(err)
	}

	SQLDB := &db.SQLDatabase{
		SqlClient: sqldb,
	}

	app.RedisDB = RedisDB
	app.SQLDB = SQLDB
	err = app.SQLDB.CreateTableNotExists()
	if err != nil {
		log.Fatalf("Error as creating Sql tables %s", err)
	}
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

	userJSON = fmt.Sprintf(`{"score_worth":100,"user_id":"%s"}`, user.User_Id)
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	if assert.NoError(t, app.ScoreSubmit(c)) {
		var responseScore db.Score
		json.Unmarshal(rec.Body.Bytes(), &responseScore)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, responseScore.Score_worth, float64(100))
		assert.Equal(t, responseScore.User_Id, user.User_Id)
	}

	_, err = sqldb.Exec("DROP DATABASE IF EXISTS " + dbName + ";")
	if err != nil {
		//handle the error
		log.Fatal(err)
	}

}
