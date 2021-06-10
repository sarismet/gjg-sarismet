package endpoints

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gjg-sarismet/db"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/lib/pq"
)

var (
	Ctx = context.TODO()
)

type App struct {
	mu         sync.Mutex
	RedisDB    *db.RedisDatabase
	SQLDB      *db.SQLDatabase
	syncNeeded bool
}

func (app *App) Checking(l *pq.Listener) {
	fmt.Println("I am checking")
	app.SQLDB.Sqlmu.Lock()
	var rows *sql.Rows
	var rowCount int
	userSql := "select * from (select *, rank() over (order by points desc) as rank from users) t;"
	var err error
	rows, err = app.SQLDB.SqlClient.Query(userSql)
	if err != nil {
		fmt.Println("Failed to execute query: ", err)
		return
	}
	_ = rows
	app.SQLDB.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = $1", "general").Scan(&rowCount)
	fmt.Printf("Round count is %d\n", rowCount)

	if rowCount == 0 {
		fmt.Printf("Round count is %d Returning... \n", 0)
		return
	}

	if rows == nil {
		fmt.Println("rows pointer is nil Returning... ")
		return
	}
	app.SQLDB.Sqlmu.Unlock()

	defer rows.Close()
	users := make([]db.User, rowCount)
	index := 0
	for rows.Next() {
		err := rows.Scan(&users[index].User_Id, &users[index].Display_Name, &users[index].Points, &users[index].Country, &users[index].Rank)
		if err != nil {
			fmt.Println("Failed to execute query: ", err)
		}
		users[index].Rank = users[index].Rank - 1
		// TODO : SET REDIS HERE FOR THIS USER
		userMember := &redis.Z{
			Member: users[index].User_Id,
			Score:  float64(users[index].Points),
		}
		pipe := app.RedisDB.Client.TxPipeline()
		pipe.ZAdd(db.Ctx, "leaderboard", userMember)
		_, err = pipe.Exec(db.Ctx)
		if err != nil {
			fmt.Printf("failed due to %s Returning... \n", err.Error())
			return
		}
		userJson, err := json.Marshal(users[index])
		if err != nil {
			fmt.Printf("failed due to %s Returning... \n", err.Error())
			return
		}
		err = app.RedisDB.Client.Set(Ctx, users[index].User_Id, userJson, 0).Err()
		if err != nil {
			fmt.Printf("failed due to %s Returning... \n", err.Error())
			return
		}
		index = index + 1
	}
}

func (app *App) Sync(l *pq.Listener) {
	for {
		if app.syncNeeded {
			app.mu.Lock()
			app.Checking(l)
			app.mu.Unlock()
		}
		time.Sleep(30 * time.Second)
	}
}

func Init() {
	var err error
	app := App{}
	RedisDB, err := db.NewRedisDatabase()
	if err != nil || RedisDB == nil {
		log.Fatal("Error as conencting to Redis")
	}
	size := RedisDB.Client.Get(Ctx, "size")
	if size.Val() == "" {
		RedisDB.Client.Set(Ctx, "leaderboardsize", 0, 0)
	}
	SQLDB, psqlInfo, err := db.NewSqlDatabase()
	if err != nil {
		log.Fatal("Error as conencting to Sql")
	}

	err = SQLDB.CreateTableNotExists()
	if err != nil {
		log.Fatal("Error as creating Sql tables")
	}

	app.syncNeeded = false

	reportProblem := func(et pq.ListenerEventType, err error) {
		if err != nil {
			fmt.Println(err)
		}
	}

	listener := pq.NewListener(psqlInfo, 1*time.Second, 2*time.Minute, reportProblem)

	go app.Sync(listener)

	app.RedisDB = RedisDB
	app.SQLDB = SQLDB

	e := echo.New()

	e.GET("/", app.Hello)

	e.GET("/leaderboard:country_iso_code", app.GetLeaderBoard)
	e.GET("/leaderboard", app.GetLeaderBoard)

	e.POST("/user/create", app.CreateUser)
	e.POST("/user/create_multiple", app.CreateMultipleUsers)
	e.GET("/user/profile:user_guid", app.GetUserProile)

	e.POST("/score/submit", app.ScoreSubmit)
	e.POST("/score/submit_multiple", app.ScoreSubmitMultiple)

	e.Start(":8000")
}

func (app *App) Hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello from server side")
}

func (app *App) GetLeaderBoard(c echo.Context) error {

	countryCode := c.Param("country_iso_code")
	if countryCode != "" {
		countryCode = countryCode[1:]
	}
	var users []db.User
	var size int
	app.mu.Lock()
	users, size = app.RedisDB.GetLeaderboard(countryCode)
	is_Redis_empty := false
	if size == -1 {
		fmt.Println("-1-1-1-1-1-1-1-1-1")
		users, size = app.SQLDB.GetAllUser(countryCode)
	} else {
		if users == nil {
			fmt.Println("fail to get from redis trying to get from sql")
			users, size = app.SQLDB.GetAllUser(countryCode)
			if users != nil {
				is_Redis_empty = true
			}
		}
	}

	fmt.Printf("size %d", size)
	if is_Redis_empty {
		for _, user := range users {
			fmt.Println("saving to redis with go keyword")
			go app.RedisDB.SaveUser(&user)
		}
	}

	app.mu.Unlock()
	return c.JSON(http.StatusOK, users)
}

func (app *App) CreateUser(c echo.Context) error {

	user := &db.User{}
	defer c.Request().Body.Close()
	err := c.Bind(user)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	user.User_Id = uuid.New().String()
	user.Points = 0
	user.Rank = -1
	app.mu.Lock()
	_, err = app.RedisDB.SaveUser(user)
	country := user.Country
	if err != nil {

		log.Printf("An error in save user has occurred %s tring to save on sql \n", err)
		sqlerr := app.SQLDB.SaveUser(user, country)
		if sqlerr != nil {
			app.mu.Unlock()
			return c.String(http.StatusInternalServerError, "An error comes up as saving user in both database!")
		}
		app.syncNeeded = true
		log.Println("An error comes up as saving user in redis but stored in sql!")
	} else {
		// here we use go since we managed to save user in redis and we can keep going without waiting for sql to be saved
		go app.SQLDB.SaveUser(user, country)
	}
	//this is done since we do not show the country iso code of user in response. We can use string interface as response but
	//the order would be aphetically and in project pdf response fields is not ordered aphetically
	user.Country = ""
	app.mu.Unlock()
	return c.JSON(http.StatusCreated, user)
}

func (app *App) CreateMultipleUsers(c echo.Context) error {

	defer c.Request().Body.Close()
	multipleUsers := &db.MultipleUsers{}
	if err := c.Bind(multipleUsers); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	app.mu.Lock()

	if multipleUsers.Count > 1000 {
		fmt.Println(" multipleUsers.Count > 1000 ")
		err := app.SQLDB.SaveMultipleUser(&multipleUsers.Users)
		if err != nil {
			log.Println("An error in save users in sql", err)
		}
		go func(users *[]db.User) {
			fmt.Println(" GO FUNC ")
			size := len(*users)
			index := 0
			for index < size {
				multipleUsers.Users[index].User_Id = uuid.New().String()
				multipleUsers.Users[index].Points = 0
				multipleUsers.Users[index].Rank = -1
				_, err := app.RedisDB.SaveUser(&multipleUsers.Users[index])
				if err != nil {
					log.Printf("An error in save user has occurred %s tring to make syncNeeded true \n", err)
					app.syncNeeded = true
				}
				index++
			}
		}(&multipleUsers.Users)
	} else {
		for index := range multipleUsers.Users {
			multipleUsers.Users[index].User_Id = uuid.New().String()
			multipleUsers.Users[index].Points = 0
			multipleUsers.Users[index].Rank = -1

			_, err := app.RedisDB.SaveUser(&multipleUsers.Users[index])
			country := multipleUsers.Users[index].Country
			if err != nil {
				log.Printf("An error in save user has occurred %s tring to save on sql \n", err)
				sqlerr := app.SQLDB.SaveUser(&multipleUsers.Users[index], country)
				if sqlerr != nil {
					app.mu.Unlock()
					return c.String(http.StatusInternalServerError, "An error comes up as saving user in both database!")
				}
				log.Println("An error comes up as saving user in redis but stored in sql!")
				app.syncNeeded = true
			} else {
				go app.SQLDB.SaveUser(&multipleUsers.Users[index], country)
			}
			multipleUsers.Users[index].Country = ""
		}
	}

	app.mu.Unlock()
	return c.JSON(http.StatusCreated, multipleUsers.Users)
}

func (app *App) GetUserProile(c echo.Context) error {

	user_guid := c.Param("user_guid")
	user_guid = user_guid[1:]
	app.mu.Lock()
	user, err := app.RedisDB.GetUser(user_guid)
	if err != nil {
		fmt.Printf("Error as getting user from Redis %s", err)
		user, err = app.SQLDB.GetUser(user_guid)
		if err != nil {
			fmt.Printf("Error as getting user from SQL %s", err)
			errs := fmt.Sprintf("Error as getting user from SQL %s", err.Error())
			app.mu.Unlock()
			return c.String(http.StatusNotFound, errs)
		}
		app.RedisDB.SaveUser(&user)
	}
	app.mu.Unlock()
	user.User_Id = ""
	return c.JSON(http.StatusOK, user)
}

func (app *App) ScoreSubmit(c echo.Context) error {

	score := &db.Score{}
	defer c.Request().Body.Close()
	err := c.Bind(score)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	app.mu.Lock()
	user, _ := app.RedisDB.GetUser(score.User_Id)
	user.Points = user.Points + score.Score_worth
	score.Score_worth = user.Points
	score.Timestamp, err = app.RedisDB.SaveUser(&user)
	if err != nil {
		fmt.Println("error as saving to redis ", err)
		err = app.SQLDB.SubmitScore(score.User_Id, score.Score_worth)
		if err != nil {
			app.mu.Unlock()
			return c.String(http.StatusBadRequest, "error as submiting score in both redis and sql")
		}
		app.syncNeeded = true
		app.mu.Unlock()
		return c.String(http.StatusOK, "error as submiting score in redis")
	} else {
		go app.SQLDB.SubmitScore(score.User_Id, score.Score_worth)
	}
	app.mu.Unlock()
	return c.JSON(http.StatusOK, score)
}

func (app *App) ScoreSubmitMultiple(c echo.Context) error {
	defer c.Request().Body.Close()
	multipleScores := &db.MultipleScores{}
	if err := c.Bind(multipleScores); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	app.mu.Lock()
	for index := range multipleScores.Scores {
		user, _ := app.RedisDB.GetUser(multipleScores.Scores[index].User_Id)
		user.Points = user.Points + multipleScores.Scores[index].Score_worth
		multipleScores.Scores[index].Score_worth = user.Points
		timestamp, err := app.RedisDB.SaveUser(&user)
		multipleScores.Scores[index].Timestamp = timestamp
		if err != nil {
			err = app.SQLDB.SubmitScore(multipleScores.Scores[index].User_Id, multipleScores.Scores[index].Score_worth)
			if err != nil {
				app.mu.Unlock()
				return c.String(http.StatusBadRequest, "error as submiting score in both redis and sql")
			}
			app.syncNeeded = true
		} else {
			go app.SQLDB.SubmitScore(multipleScores.Scores[index].User_Id, multipleScores.Scores[index].Score_worth)
		}
	}
	app.mu.Unlock()
	return c.JSON(http.StatusOK, multipleScores.Scores)
}
