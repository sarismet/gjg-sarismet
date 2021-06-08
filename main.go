package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gjg-sarismet/db"

	"github.com/google/uuid"
	"github.com/labstack/echo"
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
	var lusers []db.LeaderBoardRespond
	countryCode := c.Param("country_iso_code")
	if countryCode != "" {
		countryCode = countryCode[1:]
	}
	var users []db.User
	var size int
	users, size = RedisDB.GetLeaderboard(countryCode)
	is_Redis_empty := false
	if users == nil {
		fmt.Println("fail to get from redis trying to get from redis")
		var err error
		users, err = SQLDB.GetAllUser(countryCode)
		if err != nil {
			if err.Error() == "rowCount is zero" {
				return c.JSON(http.StatusOK, lusers)
			}
			return c.String(http.StatusNotFound, err.Error())
		}
		is_Redis_empty = true
		size = len(users)
	}
	fmt.Printf("size %d", size)
	lusers = make([]db.LeaderBoardRespond, size)
	for index, user := range users {
		if is_Redis_empty {
			RedisDB.SaveUser(&user)
		}
		lusers[index] = db.LeaderBoardRespond{
			Rank: user.Rank, Points: user.Points, Display_Name: user.Display_Name, Country: user.Country,
		}
	}
	return c.JSON(http.StatusOK, lusers)
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

func createMultipleUsers(c echo.Context) error {

	defer c.Request().Body.Close()
	multipleUsers := &db.MultipleUsers{}
	if err := c.Bind(multipleUsers); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	for index := range multipleUsers.Users {
		multipleUsers.Users[index].User_Id = uuid.New().String()
		multipleUsers.Users[index].Points = 0
		multipleUsers.Users[index].Rank = -1
		_, err := RedisDB.SaveUser(&multipleUsers.Users[index])
		country := multipleUsers.Users[index].Country
		if err != nil {
			log.Printf("An error in save user has occurred %s tring to save on sql \n", err)
			sqlerr := SQLDB.SaveUser(&multipleUsers.Users[index], country)
			if sqlerr != nil {
				return c.String(http.StatusInternalServerError, "An error comes up as saving user in both database!")
			}
			return c.String(http.StatusInternalServerError, "An error comes up as saving user in redis but stored in sql!")
		}
		multipleUsers.Users[index].Country = ""
	}
	return c.JSON(http.StatusOK, multipleUsers.Users)
}

func getUserProile(c echo.Context) error {

	user_guid := c.Param("user_guid")
	user_guid = user_guid[1:]
	user, err := RedisDB.GetUser(user_guid)
	if err != nil {
		fmt.Printf("Error as getting user from Redis %s", err)
		user, err = SQLDB.GetUser(user_guid)
		if err != nil {
			fmt.Printf("Error as getting user from SQL %s", err)
			errs := fmt.Sprintf("Error as getting user from SQL %s", err.Error())
			return c.String(http.StatusNotFound, errs)
		}
		RedisDB.SaveUser(&user)
	}
	return c.JSON(http.StatusOK, user)
}

func scoreSubmit(c echo.Context) error {

	score := &db.Score{}
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
		return c.String(http.StatusOK, "error as submiting score in redis")
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

	for index := range multipleScores.Scores {
		user, _ := RedisDB.GetUser(multipleScores.Scores[index].User_Id)
		user.Points = user.Points + multipleScores.Scores[index].Score_worth
		multipleScores.Scores[index].Score_worth = user.Points
		timestamp, err := RedisDB.SaveUser(&user)
		multipleScores.Scores[index].Timestamp = timestamp
		if err != nil {
			return c.String(http.StatusOK, "error as submiting score in redis")
		} else {
			go SQLDB.SubmitScore(multipleScores.Scores[index].User_Id, multipleScores.Scores[index].Score_worth)
		}
	}

	return c.JSON(http.StatusOK, multipleScores.Scores)
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

	e.Start("127.0.0.1:8000")
}
