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
	const (
		host         = "0.0.0.0"
		port         = 5432
		databaseuser = "postgres"
		password     = "123"
		dbname       = "postgres"
	)
	mainDBconnection := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, databaseuser, password, dbname)

	sqldb, err := sql.Open("postgres", mainDBconnection)
	if err != nil {
		log.Fatal(err)
	}
	dbName := "testdb"
	_, err = sqldb.Exec("create database " + dbName + ";")
	if err != nil {
		log.Fatal(err)
	}
	testDBconnection := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, databaseuser, password, dbName)
	testdb, err := sql.Open("postgres", testDBconnection)
	if err != nil {
		log.Fatal(err)
	}
	SQLDB := &db.SQLDatabase{
		SqlClient: testdb,
	}
	app.RedisDB = RedisDB
	app.SQLDB = SQLDB
	err = app.SQLDB.CreateTableNotExists()
	if err != nil {
		log.Fatalf("Error as creating Sql tables %s", err)
	}

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
	db.Redismutex.Lock()
	app.RedisDB.Client.FlushAll(db.Ctx)
	db.Redismutex.Unlock()
	db.Sqlmutex.Lock()
	app.SQLDB.SqlClient.Close()
	db.Sqlmutex.Unlock()

	if err != nil {
		log.Fatal(err)
	}
	_, err = sqldb.Exec("DROP DATABASE IF EXISTS " + dbName + ";")
	if err != nil {
		log.Fatal(err)
	}
	sqldb.Close()
}
