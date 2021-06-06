package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

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

	user := &db.User{}
	defer c.Request().Body.Close()
	err := c.Bind(user)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	user.User_Id = uuid.New().String()
	user.Points = 0
	user.Rank = -1
	_, err = RedisDB.SaveUser(user)
	country := user.Country
	if err != nil {
		log.Printf("An error in save user has occurred %s tring to save on sql \n", err)
		sqlerr := SQLDB.SaveUser(user, country)
		if sqlerr != nil {
			return c.String(http.StatusInternalServerError, "An error comes up as saving user in both database!")
		}
		return c.String(http.StatusInternalServerError, "An error comes up as saving user in redis but stored in sql!")
	} else {
		// here we use go since we managed to save user in redis and we can keep going without waiting for sql to be saved
		go SQLDB.SaveUser(user, country)
	}
	//this is done since we do not show the country iso code of user in response. We can use string interface as response but
	//the order would be aphetically and in project pdf response fields is not ordered aphetically
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
	err := c.Bind(score)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	user, _ := RedisDB.GetUser(score.User_Id)
	user.Points = user.Points + score.Score_worth
	score.Score_worth = user.Points
	score.Timestamp, err = RedisDB.SaveUser(&user)
	if err != nil {
		return c.String(http.StatusOK, "error as submiting score in both redis")
	} else {
		go SQLDB.SubmitScore(score.User_Id, score.Score_worth)
	}
	return c.JSON(http.StatusOK, score)
}

func scoreSubmitMultiple(c echo.Context) error {

	defer c.Request().Body.Close()
	multipleScores := &db.MultipleScores{}
	if err := c.Bind(multipleScores); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	for key, score := range multipleScores.Scores {
		user, _ := RedisDB.GetUser(score.User_Id)
		user.Points = user.Points + score.Score_worth
		score.Score_worth = user.Points
		timestamp, err := RedisDB.SaveUser(&user)
		multipleScores.Scores[key].Timestamp = timestamp
		if err != nil {
			return c.String(http.StatusOK, "error as submiting score in redis")
		} else {
			go SQLDB.SubmitScore(score.User_Id, score.Score_worth)
		}
	}

	return c.JSON(http.StatusOK, multipleScores.Scores)
}

func createMultipleUsers(c echo.Context) error {

	defer c.Request().Body.Close()
	multipleUsers := &db.MultipleUsers{}
	if err := c.Bind(multipleUsers); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	for index, user := range multipleUsers.Users {
		user.User_Id = uuid.New().String()
		user.Points = 0
		user.Rank = -1
		_, err := RedisDB.SaveUser(&user)
		country := user.Country
		if err != nil {
			log.Printf("An error in save user has occurred %s tring to save on sql \n", err)
			sqlerr := SQLDB.SaveUser(&user, country)
			if sqlerr != nil {
				return c.String(http.StatusInternalServerError, "An error comes up as saving user in both database!")
			}
			return c.String(http.StatusInternalServerError, "An error comes up as saving user in redis but stored in sql!")
		} else {
			go SQLDB.SaveUser(&user, country)
		}
		multipleUsers.Users[index].Country = ""
	}
	return c.JSON(http.StatusOK, multipleUsers.Users)
}

func main() {

	var err error
	RedisDB, err = db.NewRedisDatabase()
	if err != nil || RedisDB == nil {
		fmt.Printf("Error as conencting to Redis %s", err)
	}
	size := RedisDB.Client.Get(Ctx, "size")
	if size.Val() == "" {
		RedisDB.Client.Set(Ctx, "leaderboardsize", 0, 0)
	}
	SQLDB, err = db.NewSqlDatabase()
	if err != nil {
		fmt.Printf("Error as conencting to Sql %s", err)
	}
	err = SQLDB.CreateTableNotExists()
	if err != nil {
		fmt.Printf("Error as creating Sql tables %s", err)
	}
	e := echo.New()
	e.GET("/", hello)

	e.GET("/leaderboard:country_iso_code", getLeaderBoard)
	e.GET("/leaderboard", getLeaderBoard)

	e.POST("/user/create", createUser)
	e.POST("/user/create_multiple", createMultipleUsers)
	e.GET("/user/profile:user_guid", getUserProile)

	e.POST("/score/submit", scoreSubmit)
	e.POST("/score/submit_multiple", scoreSubmitMultiple)

	e.Start(":8000")
}
