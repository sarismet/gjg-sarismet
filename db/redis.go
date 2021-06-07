package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
)

type RedisDatabase struct {
	Client *redis.Client
}

var (
	Ctx = context.TODO()
)

func (db *RedisDatabase) GetLeaderboard(countryName string) []User {
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
		fmt.Printf("arraySize %d\n", arraysize)
	} else {
		fmt.Println("Country Name is empty")
		totalUserVal := db.Client.Get(Ctx, "totalUserNumber").Val()
		if totalUserVal == "" {
			fmt.Println("However we cannot get total users")
			db.Client.Set(Ctx, "totalUserNumber", 0, 0)
			return nil
		}
		arraysize, _ = strconv.Atoi(totalUserVal)
	}
	fmt.Printf("arraySize %d\n", arraysize)
	users := make([]User, arraysize)
	for rank, member := range scores.Val() {
		var tempUsers User
		val, err := db.Client.Get(Ctx, member.Member.(string)).Result()
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
					users[rank] = tempUsers
				}
			} else {
				users[rank] = tempUsers
			}
		}
	}
	return users
}

func (db *RedisDatabase) SaveUser(user *User) (int64, error) {
	// FROM HERE
	userMember := &redis.Z{
		Member: user.User_Id,
		Score:  float64(user.Points),
	}
	pipe := db.Client.TxPipeline()
	pipe.ZAdd(Ctx, "leaderboard", userMember)
	rank := pipe.ZRevRank(Ctx, "leaderboard", user.User_Id)
	_, err := pipe.Exec(Ctx)
	if err != nil {
		return 0, err
	}
	// TO HERE REFERENCE: https://blog.logrocket.com/how-to-use-redis-as-a-database-with-go-redis/
	now := time.Now()
	secs := now.Unix()
	user.Rank = int(rank.Val())
	userInRedis := db.Client.Get(Ctx, user.User_Id).Val()
	is_user_present := false
	if userInRedis != "" {
		is_user_present = true
	}
	countrySizeVal := db.Client.Get(Ctx, user.Country).Val()
	if countrySizeVal == "" {
		db.Client.Set(Ctx, user.Country, 1, 0)
	} else if !is_user_present {
		size, _ := strconv.Atoi(countrySizeVal)
		db.Client.Set(Ctx, user.Country, size+1, 0)
	}

	totalUserNumberSizeVal := db.Client.Get(Ctx, "totalUserNumber").Val()
	if totalUserNumberSizeVal == "" {
		db.Client.Set(Ctx, "totalUserNumber", 1, 0)
	} else if !is_user_present {
		size, _ := strconv.Atoi(totalUserNumberSizeVal)
		db.Client.Set(Ctx, "totalUserNumber", size+1, 0)
	}
	userJson, err := json.Marshal(user)
	if err != nil {
		return 0, err
	}
	err = db.Client.Set(Ctx, user.User_Id, userJson, 0).Err()
	if err != nil {
		return 0, err
	}
	return secs, nil
}

func (db *RedisDatabase) GetUser(user_guid string) (User, error) {
	var user User
	val, err := db.Client.Get(Ctx, user_guid).Result()
	rank := db.Client.ZRevRank(Ctx, "leaderboard", user.User_Id)
	if err == nil {
		json.Unmarshal([]byte(val), &user)
		user.Rank = int(rank.Val())
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
