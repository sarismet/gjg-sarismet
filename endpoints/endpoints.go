package endpoints

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	mu         sync.Mutex // we can define this in redis struct as well
	RedisDB    *db.RedisDatabase
	SQLDB      *db.SQLDatabase
	syncNeeded bool // we can also define this in redis struct as well
}

func (app *App) Checking(l *pq.Listener) {
	fmt.Println("I am checking wheter sync is needed")
	if app.syncNeeded && !app.SQLDB.SyncNeed {
		fmt.Println("Sql is right but Redis is not right")
		isSuccess := true // we define this since if there is an error
		// we do not return since we want to continue sync as much as we can
		// if this is false then in the next round we execute its related if statement again

		var rows *sql.Rows
		var rowCount int
		userSql := "select * from (select *, rank() over (order by points desc) as rank from users) t;"
		var err error
		rows, err = app.SQLDB.SqlClient.Query(userSql)
		if err != nil {
			isSuccess = false
		}
		_ = rows
		app.SQLDB.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = $1", "general").Scan(&rowCount)

		if rowCount == 0 || rows == nil {
			isSuccess = false
		}

		defer rows.Close()
		users := make([]db.User, rowCount)
		var countryRowCounts map[string]int = make(map[string]int)
		index := 0
		for rows.Next() {
			err := rows.Scan(&users[index].User_Id, &users[index].Display_Name, &users[index].Points, &users[index].Country, &users[index].Rank)
			if err != nil {
				fmt.Println("Failed to execute query: ", err)
			}
			countryRowCounts[users[index].Country] += 1
			users[index].Rank = users[index].Rank - 1
			userMember := &redis.Z{
				Member: users[index].User_Id,
				Score:  float64(users[index].Points),
			}
			pipe := app.RedisDB.Client.TxPipeline()
			pipe.ZAdd(db.Ctx, "leaderboard", userMember)
			_, err = pipe.Exec(db.Ctx)
			if err != nil {
				fmt.Printf("failed due to %s ... \n", err.Error())
				isSuccess = false
			}
			userJson, err := json.Marshal(users[index])
			if err != nil {
				fmt.Printf("failed due to %s ... \n", err.Error())
				isSuccess = false
			}
			err = app.RedisDB.Client.Set(Ctx, users[index].User_Id, userJson, 0).Err()
			if err != nil {
				fmt.Printf("failed due to %s ... \n", err.Error())
				isSuccess = false
			}
			index = index + 1
		}
		var totalCountryCount int = 0
		for countryName, countryCount := range countryRowCounts {
			totalCountryCount += countryCount
			app.RedisDB.Client.Set(Ctx, countryName, countryCount, 0)
		}
		app.RedisDB.Client.Set(Ctx, "totalUserNumber", totalCountryCount, 0)
		if isSuccess {
			app.syncNeeded = false
		}

	} else if !app.syncNeeded && app.SQLDB.SyncNeed {
		fmt.Println("Sql is not right but Redis is right")
		isSuccess := true
		users, _ := app.RedisDB.GetLeaderboard("", true)
		var countryRowCounts map[string]int = make(map[string]int)
		for _, user := range users {
			updateUser := "UPDATE users SET Points = Points + $2, Timestamp = $3 WHERE User_Id = $1"
			res2, _ := app.SQLDB.SqlClient.Exec(updateUser, user.User_Id, user.Points, user.Timestamp)
			countryRowCounts[user.Country] += 1
			if res2 != nil {
				affectedrows, _ := res2.RowsAffected()
				if affectedrows == 0 {
					insertCountryNumberDB := `INSERT INTO  Users (User_Id, Display_Name, Points, Country, Timestamp) values($1, $2, $3, $4, $5);`
					_, err := app.SQLDB.SqlClient.Exec(insertCountryNumberDB, user.User_Id, user.Points, user.Country, user.Timestamp)
					if err != nil {
						isSuccess = false
					}
				}
			}

		}

		var totalCountryCount int = 0
		for countryName, countryCount := range countryRowCounts {
			totalCountryCount += countryCount
			cts := countryName + "_timestamp"

			secs, _ := strconv.Atoi(app.RedisDB.Client.Get(Ctx, cts).Val())
			updateCountryCount := `UPDATE CountryNumberSizes SET size = $3, timestamp = $2 WHERE code = $1;`
			res2, _ := app.SQLDB.SqlClient.Exec(updateCountryCount, countryName, secs, countryCount)
			if res2 != nil {
				affectedrows, _ := res2.RowsAffected()
				if affectedrows == 0 {
					insertCountryNumberDB := `INSERT INTO CountryNumberSizes (code, size, timestamp) values($1, $2, $3);`
					_, err := app.SQLDB.SqlClient.Exec(insertCountryNumberDB, countryName, countryCount, secs)
					if err != nil {
						isSuccess = false
					}
				}
			}
		}
		tsecs, _ := strconv.Atoi(app.RedisDB.Client.Get(Ctx, "totalUserNumber_timestamp").Val())
		updateGeneralCount := `UPDATE CountryNumberSizes SET size = $3, timestamp = $2 WHERE code = $1;`
		res3, _ := app.SQLDB.SqlClient.Exec(updateGeneralCount, "general", tsecs, totalCountryCount)
		if res3 != nil {
			affectedrows, _ := res3.RowsAffected()
			if affectedrows == 0 {
				insertGeneralNumberDB := `INSERT INTO CountryNumberSizes (code, size, timestamp) values($1, $2, $3);`
				_, err := app.SQLDB.SqlClient.Exec(insertGeneralNumberDB, "general", totalCountryCount, tsecs)
				if err != nil {
					isSuccess = false
				}
			}
		}
		if isSuccess {
			app.SQLDB.SyncNeed = false
		}
	} else if app.syncNeeded && app.SQLDB.SyncNeed {
		fmt.Println("Both Sql and Redis are not right")
		var users map[string]db.User = make(map[string]db.User)
		redisUsers, sizeRedis := app.RedisDB.GetLeaderboard("", true)
		sqlUsers, sizeSql := app.SQLDB.GetAllUser("")

		index := 0
		commonSize := 0
		if sizeRedis <= sizeSql {
			commonSize = sizeRedis
		} else {
			commonSize = sizeSql
		}
		for index < commonSize {
			if redisUsers[index].User_Id == sqlUsers[index].User_Id {
				if redisUsers[index].Timestamp > sqlUsers[index].Timestamp {
					users[redisUsers[index].User_Id] = redisUsers[index]
				} else {
					users[redisUsers[index].User_Id] = sqlUsers[index]
				}

			} else {
				if users[redisUsers[index].User_Id].User_Id == "" {
					users[redisUsers[index].User_Id] = redisUsers[index]
				} else if users[redisUsers[index].User_Id].Timestamp < redisUsers[index].Timestamp {
					users[redisUsers[index].User_Id] = redisUsers[index]
				}
				if users[sqlUsers[index].User_Id].User_Id == "" {
					users[sqlUsers[index].User_Id] = sqlUsers[index]
				} else if users[sqlUsers[index].User_Id].Timestamp < sqlUsers[index].Timestamp {
					users[sqlUsers[index].User_Id] = sqlUsers[index]
				}

			}

			index++
		}
		if (sizeRedis - commonSize) > 0 {
			index := 0
			extraSize := sizeRedis - commonSize
			for index < extraSize {
				if users[sqlUsers[index].User_Id].User_Id == "" {
					users[sqlUsers[index].User_Id] = sqlUsers[index]
				} else if users[sqlUsers[index].User_Id].Timestamp < sqlUsers[index].Timestamp {
					users[sqlUsers[index].User_Id] = sqlUsers[index]
				}
			}
		} else {
			index := 0
			extraSize := sizeSql - commonSize
			for index < extraSize {
				if users[redisUsers[index].User_Id].User_Id == "" {
					users[redisUsers[index].User_Id] = redisUsers[index]
				} else if users[redisUsers[index].User_Id].Timestamp < redisUsers[index].Timestamp {
					users[redisUsers[index].User_Id] = redisUsers[index]
				}
			}
		}
	} else {
		fmt.Println("No need to sync databases")
	}

}

func (app *App) Sync(l *pq.Listener) {
	for {
		app.SQLDB.Sqlmu.Lock()
		app.mu.Lock()
		app.Checking(l)
		app.mu.Unlock()
		app.SQLDB.Sqlmu.Unlock()
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
		log.Fatalln("Error as creating Sql tables", err)
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
	users, size = app.RedisDB.GetLeaderboard(countryCode, false)
	is_Redis_empty := false
	if size == -1 {
		users, _ = app.SQLDB.GetAllUser(countryCode)
	} else {
		if users == nil {
			fmt.Println("fail to get from redis trying to get from sql")
			users, _ = app.SQLDB.GetAllUser(countryCode)
			if users != nil {
				is_Redis_empty = true
			}
		}
	}

	if is_Redis_empty {
		for _, user := range users {
			fmt.Println("saving to redis with go keyword")
			go app.RedisDB.SaveUser(&user)
		}
	}

	app.mu.Unlock()
	if users == nil {
		users = make([]db.User, 0)
	}
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
	user.Timestamp = 0
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

	if multipleUsers.Count > 0 {
		err := app.SQLDB.SaveMultipleUser(&multipleUsers.Users)
		if err != nil {
			log.Println("An error in save users in sql", err)
		}
		go func(users *[]db.User) {
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
			multipleUsers.Users[index].Timestamp = 0
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
