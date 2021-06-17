## Run Locally

Clone the project in your go path

```bash
  cd ~/go/src/github.com/
  git clone https://github.com/sarismet/gjg-sarismet
```

Go to the project directory

```bash
  cd gjg-sarismet
```
Make sure 

```
  GO111MODULE="auto"
```
To make GO111MODULE="auto" run

```
  export GO111MODULE="auto"
```

Install dependencies

```bash
  go get github.com/google/uuid
  go get github.com/labstack/echo
  go get github.com/lib/pq
  go get github.com/go-redis/redis
  go get github.com/alicebob/miniredis
  go get github.com/stretchr/testify
```
Please make sure that you have docker in your machine.  
After that run these commands.
```
  docker run --name postgresql-container -p 5432:5432 -e POSTGRES_PASSWORD=123 -d postgres
  docker exec -it [container-id] bash
  psql -h localhost -p 5432 -U postgres -W
```
If you want to see the container id, you can basically run docker ps to get the running containers. When you connect to your database you can see postgres default database by typing \l.  

To install and run redis in your machine, type the command in your terminal.  
It install the redis image if you do not have and run it in port 6379
```
  docker run --name=rediboard -p 6379:6379 redis
```



There are two types of starting the server. There are a synchronization mechanism  
between Redis and Sql. If you want to trigger sysynchronization type 

```bash
  go run main.go -sync -i [int-number]
```
or If you do not want to trigger then type
```bash
  go run main.go
```

## Notes
- When we want to run a operation such as creating multiple a user, we always do it in Redis if the the number of the user we want to create is not huge. After the creating users in Redis is done we create a goroutine to handle the same operation using postgresql. However if the Redis fails then we make the program do the operation in postgresql. This approach is applied in every endpoints.
- I implemented a synchronization mechanism so that if there is an error while saving or updating a user in Redis or Sql. You need to spesify whether or not you want to run the synchronization mechanism as you run the program. If it is triggered then the program create a goroutine which checks if there is sync needed. If the program face an issue in a Redis operation, then the mechanism will try to copy the information of the user from postgresql to Redis. If there is an error faced as saving or updating in sql then the synchronization mechanism copy the information from Redis to Sql. If we have the same user information in both Sql and Redis and the program always determine the correct user info considering the timestamp. The bigger timestamp would have the priority.
- Using Postgresql rather than Redis has some pros such as selecting multiple row in Postgresql is much faster than Redis so if the size of the user in the system is bigger than 1000 then we search the user in Posgresql. I tested this with 100.000 users in my localhost and getting the user from Resis lasts 6min 15 sec but getting them from postgresql just last 1min 12sec. For creating multiple user and geting the leaderboard with more than 1000 user operations we must use postgresql instead of Redis. However if we just want to run one operation such as geting user or creating user then using redis is much more faster.
- If you have remote database host or you can run redis, postgresql and our project in docker compose then
I left a Dockerfile for this project.
- If you want to submit a score for a user who is not really exists then the program does not update the scores or create new user but in the response code it is shown as the users with the wrong user ids are updated. I used the same mode that I created as reading the request as returning respond. I did this since mreating and other model can slow down the program. 
- The user ids are unique.

## Locks
- Since we synchronize our databases we had to used locks. Golang as its mutex and when we adding, updating or geting from a database we lock the database first so that when the synchronization runs it does not get the wrong version of the database.
- We also used goroutine to make some asynchronous operation so these operation can access and change the same user information. In order to prohibit that condition we had to use lock as well.

## Important Rules
- Ranks star with 0


#### Get user

Request  

```
```http
  GET http://localhost:8000/user/profile/{user_id} or 18.117.96.165:8000/user/profile/{user_id}
```

| Parameter | Type     | Description                |
| :-------- | :------- | :------------------------- |
| `user_id` | `string` | the user id of the user |

Response  

```
{
    "user_id": "520c00a9-5b86-4b23-b2f8-77e339f887a0",
    "display_name": "ismet",
    "points": 0,
    "rank": 0,
    "country": "tr"
}
```

#### Get leaderboard
 
Request  

```http
  GET http://localhost:8000/leaderboard or 18.117.96.165:8000/leaderboard
```

| Parameter | Type     | Description                |
| :-------- | :------- | :------------------------- |
| `-` | `-` | List the leaderboard. |

Response  

```
[
    {
        "rank": 0,
        "points": 100,
        "display_name": "ismet",
        "country": "tr"
    },
    {
        "rank": 1,
        "points": 50,
        "display_name": "jhon",
        "country": "eu"
    },
    {
        "rank": 2,
        "points": 30,
        "display_name": "elif",
        "country": "tr"
    }
]

#### Get leaderboard with country iso code

Request  

```
```http
  GET http://localhost:8000/leaderboard/tr or 18.117.96.165:8000/leaderboard/tr
```

| Parameter | Type     | Description                |
| :-------- | :------- | :------------------------- |
| `country_iso_code` | `string` | the iso code of the country |

Response  

```
[
    {
        "rank": 0,
        "points": 100,
        "display_name": "ismet",
        "country": "tr"
    },
    {
        "rank": 2,
        "points": 300,
        "display_name": "elif",
        "country": "tr"
    }
]
```

#### Create an user 

Request  

```http
  POST http://localhost:8000/user/create or 18.117.96.165:8000/user/create
``` 

| Body | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `display_name`      | `string` | **Required**. name of the user |
| `country`      | `string` | **Required**. the country iso code of the user |

Response 

```
{
    "user_id": "fda12554-c1ef-4205-8722-913596d2fbe4",
    "display_name": "ismet",
    "points": 0,
    "rank": 0
}
```

#### Create multiple users 

Request  

```http
  POST http://localhost:8000/user/create_multiple or 18.117.96.165:8000/user/create_multiple
```

| Body | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `count`      | `int ` | **Required**. the size of the users array|
| `{display_name:string,country: string}` | `array` | **Required**. display_name and country of the users |

Response  

```
{
    "user_id": "fda12554-c1ef-4205-8722-913596d2fbe4",
    "display_name": "ismet",
    "points": 0,
    "rank": 0
},
{
    "user_id": "90fc2588-7361-4c36-890f-b7c5debd9499",
    "display_name": "john",
    "points": 0,
    "rank": 0
}
```

#### Submit the score

Request  

```http
  POST http://localhost:8000/submit/score or 18.117.96.165:8000/submit/score
```

| Body | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `score_worth`      | `float or int ` | **Required**. the point to submit |
| `user_id`      | `string` | **Required**. the id of the user |

Response  

```
{
    "score_worth": 101,
    "user_id": "1021278d-4756-426f-b923-9a9e9cd93349",
    "timestamp": 1623000201
}
```

#### Submit multiple score

Request  

```http
  POST http://localhost:8000/score/submit_multiple or 18.117.96.165:8000/submit/submit_multiple
```

Response  

| Body | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `count`      | `int ` | **Required**. the size of the scores array|
| `{score_worth: float or int`, `user_id: string}`     | `array ` | **Required**. the point to submit |

```
{
    "score_worth": 101,
    "user_id": "1021278d-4756-426f-b923-9a9e9cd93349",
    "timestamp": 1623000498
},
{
    "score_worth": 201,
    "user_id": "b6c93b8c-aa7e-4b76-9eb7-b06fea707e8b",
    "timestamp": 1623000498
}
```

  
## How we can improve it

```
- When the program is started we try to connect both redis and sql.
  We can create connection pools for both databases to use a connect from the pool
  However, it is not implemented yet.
```
```
- If some users have the same points then the order in postgres database and redis can be different.
  We order the same users in postgres database by considering their record date however in Redis
  there is no rule to order users with the same point. We can define a rule for them in Redis.
```

  
## Tech Stack
**Server:** Golang, Echo
```
  I had made a research and found that the best framework which is good at scalability is Echo for Golang so I decided to use Echo even though Gin is much more popular and Fiber is much more faster then Echo.
  Golang has goroutine which makes the program faster and it was the one of the reason why I choose golang. 
```

**Database:** Postgresql, Redis
```
  I felt that we need to design a caching mechanism to make the process faster. When we post a request
  we first store it in Redis and then store it in Postgresql by asynchronous operaion using a goroutine. That way, when we want to get an user we first look at the Redis and if we do not found then we look at Postgresql which is much slower operation. However, if we want to get huge number of users like 50.000 then we first searcg them in Postgresql since sql operation with huge number of rows is faster.

  Postgresql uses more than one GPU core to execute the operation so it can be more attractive to run complext queries.
  ```
## Running Tests

I implemented tests for every endpoint and there is another test file for all the endpoints run sequentially.

To run the endpoints one by one
```bash
  go test ./tests/. -v
```

To run all the tests sequentially
```bash
  go test ./tests/alltest/. -v
```
