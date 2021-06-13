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
		assert.Equal(t, responseUser.Rank, user.Rank)
	}

	req = httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("user_guid")
	c.SetParamValues("/" + "there-is-no-such-user-id")
	fmt.Println("We expect an error here")
	if assert.NoError(t, app.GetUserProile(c)) {
		assert.Equal(t, http.StatusNotFound, rec.Code)
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
