package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/go-redis/redis"
)

type RedisDatabase struct {
	Client *redis.Client
}

var (
	Ctx = context.TODO()
)

func (db *RedisDatabase) GetLeaderboard(countryName string) []LeaderBoardRespond {
	scores := db.Client.ZRevRangeWithScores(Ctx, "leaderboard", 0, -1)
	if scores == nil {
		return nil
	}

	var arraysize int
	if countryName != "" {
		fmt.Println("Country Name is not empty")
		countrySizeVal := db.Client.Get(Ctx, countryName).Val()
		if countrySizeVal == "" {
			fmt.Println("However we cannot find any size of this country")
			db.Client.Set(Ctx, countryName, 0, 0)
			return nil
		}
		arraysize, _ = strconv.Atoi(countrySizeVal)
	}

	users := make([]LeaderBoardRespond, arraysize)

	for rank, member := range scores.Val() {
		var tempUsers LeaderBoardRespond

		val, err := db.Client.Get(Ctx, member.Member.(string)).Result()
		//fmt.Print("val is ")
		//fmt.Println(val)
		if err == nil {
			json.Unmarshal([]byte(val), &tempUsers)

			if tempUsers.Rank != rank {
				tempUsers.Rank = rank

				var ttuser User
				ttuser.Rank = rank
				json.Unmarshal([]byte(val), &ttuser)

				userJson, _ := json.Marshal(ttuser)

				db.Client.Set(Ctx, ttuser.User_Id, userJson, 0)
			}
			if countryName != "" {
				if tempUsers.Country == countryName {
					users = append(users, tempUsers)
				}
			} else {
				users = append(users, tempUsers)
			}
		}
	}
	return users
}

func (db *RedisDatabase) SaveUser(user *User) error {

	userMember := &redis.Z{
		Member: user.User_Id,
		Score:  float64(user.Points),
	}
	pipe := db.Client.TxPipeline()
	pipe.ZAdd(Ctx, "leaderboard", userMember)
	rank := pipe.ZRevRank(Ctx, "leaderboard", user.User_Id)
	_, err := pipe.Exec(Ctx)
	if err != nil {
		return err
	}

	fmt.Println(rank.Val(), err)
	user.Rank = int(rank.Val())
	fmt.Println("Rank is ")
	fmt.Println(user.Rank)

	countrySizeVal := db.Client.Get(Ctx, user.Country).Val()
	if countrySizeVal == "" {
		fmt.Println("However we cannot find any size of this country")
		db.Client.Set(Ctx, user.Country, 1, 0)
	} else {
		size, _ := strconv.Atoi(countrySizeVal)
		db.Client.Set(Ctx, user.Country, size+1, 0)
	}

	userJson, err := json.Marshal(user)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return err
	}

	fmt.Printf("userJson is %s ", userJson)

	err = db.Client.Set(Ctx, user.User_Id, userJson, 0).Err()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return nil
}

func (db *RedisDatabase) GetUser(user_guid string) (User, error) {
	var user User

	val, err := db.Client.Get(Ctx, user_guid).Result()
	if err == nil {
		json.Unmarshal([]byte(val), &user)
	}
	return user, err
}

func NewRedisDatabase() (*RedisDatabase, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if err := client.Ping(Ctx).Err(); err != nil {
		return nil, err
	}
	return &RedisDatabase{
		Client: client,
	}, nil
}

func Helllo() {
	fmt.Println("Hello")
}
