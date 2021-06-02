package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/sarismet/ismet/db"
)

var (
	Ctx     = context.TODO()
	RedisDB *db.RedisDatabase
	SQLDB   *db.SQLDatabase
)

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello from server side")
}

func getLeaderBoard(c echo.Context) error {

	countryCode := c.Param("country_iso_code")

	if countryCode != "" {
		countryCode = countryCode[1:]
	}
	fmt.Printf("country code is %s", countryCode)

	users := RedisDB.GetLeaderboard(countryCode)

	if users == nil {
		fmt.Println("fail to get from redis trying to get from sql")
		users = SQLDB.GetAllUser(countryCode)
		if users == nil {
			fmt.Println("fail to get from both redis and sql")
		}
	}

	return c.JSON(http.StatusOK, users)
}

func createUser(c echo.Context) error {

	user := db.User{}

	defer c.Request().Body.Close()

	b, err := ioutil.ReadAll(c.Request().Body)

	if err != nil {
		log.Printf(" Failed to read the request %s", err)
		return c.String(http.StatusInternalServerError, "I made a mistake I could not read the request sorry")
	}

	err = json.Unmarshal(b, &user)

	if err != nil {
		log.Printf("An error has occurred as marhalling %s", err)
		return c.String(http.StatusInternalServerError, "An error comes up!")
	}

	user.User_Id = uuid.New().String()
	user.Points = 0
	user.Rank = -1

	if RedisDB == nil {
		fmt.Println("BOSSSSSADASDASDASDS")
	}

	err = RedisDB.SaveUser(&user)

	if err != nil {
		log.Printf("An error in save user has occurred %s tring to save on sql \n", err)
		sqlerr := SQLDB.SaveUser(user)
		if sqlerr != nil {
			return c.String(http.StatusInternalServerError, "An error comes up as saving user in both database!")
		}
		return c.String(http.StatusInternalServerError, "An error comes up as saving user in redis but stored in sql!")

	} else {
		go SQLDB.SaveUser(user)
	}

	fmt.Println("DEVAMKEE")

	log.Printf("this is your cat: %#v", user)

	user.Country = ""

	return c.JSON(http.StatusOK, user)

}

func getUserProile(c echo.Context) error {

	user_guid := c.Param("user_guid")

	user, err := RedisDB.GetUser(user_guid)

	if err != nil {
		user, err = SQLDB.GetUser(user_guid)
		if err != nil {
			fmt.Printf("Error as getting user from SQL %s", err)
		}
	}

	return c.JSON(http.StatusOK, user)
}

func scoreSubmit(c echo.Context) error {

	score := db.Score{}

	defer c.Request().Body.Close()

	b, err := ioutil.ReadAll(c.Request().Body)

	if err != nil {
		log.Printf(" Failed to read the request %s", err)
		return c.String(http.StatusInternalServerError, "I made a mistake I could not read the request sorry")
	}

	err = json.Unmarshal(b, &score)

	if err != nil {
		log.Printf("An error has occurred as marhalling %s", err)
		return c.String(http.StatusInternalServerError, "An error comes up!")
	}
	now := time.Now()
	secs := now.Unix()
	score.Timestamp = secs

	user, _ := RedisDB.GetUser(score.User_Id)

	fmt.Printf("user point before %f", user.Points)
	fmt.Printf("score worth is %f", score.Score_worth)

	user.Points = user.Points + score.Score_worth
	score.Score_worth = user.Points

	fmt.Printf("user point after %f", user.Points)

	err = RedisDB.SaveUser(&user)

	if err != nil {

		err = SQLDB.SubmitScore(score.User_Id)

		if err != nil {
			return c.String(http.StatusOK, "error as submiting score in both redis and sql")
		}
	} else {
		go SQLDB.SubmitScore(score.User_Id)
	}

	return c.JSON(http.StatusOK, user)
}

func main() {
	fmt.Println("Hello")
	db.Helllo()
	db.Psql()

	var err error

	RedisDB, err = db.NewRedisDatabase()

	if err != nil {
		fmt.Printf("Error as conencting to Redis %s", err)
	}

	RedisDB.Client.Set(Ctx, "language", "Go", 0)
	language := RedisDB.Client.Get(Ctx, "language")
	year := RedisDB.Client.Get(Ctx, "year")

	SQLDB, err = db.NewSqlDatabase()

	if err != nil {
		fmt.Printf("Error as conencting to Sql %s", err)
	}

	res := SQLDB.CreateTableNotExists()

	if res == nil {
		fmt.Println("SUCCESS TO CREATE TABLE")
	}

	fmt.Println(language.Val()) // "Go"
	fmt.Println(year.Val())     // ""

	e := echo.New()
	e.GET("/", hello)

	e.GET("/leaderboard:country_iso_code", getLeaderBoard)
	e.GET("/leaderboard", getLeaderBoard)

	e.POST("/user/create", createUser)
	e.GET("/user/profile:user_guid", getUserProile)

	e.POST("/score/submit", scoreSubmit)

	e.Start(":8000")
}
