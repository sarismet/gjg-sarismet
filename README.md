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
```

Start the server

```bash
  go run main.go
```

  
## API Reference

#### Get leaderboard

```http
  GET http://localhost:8000/leaderboard or 18.117.96.165:8000/leaderboard
```

| Parameter | Type     | Description                |
| :-------- | :------- | :------------------------- |
| `-` | `-` | List the leaderboard. |

Leaderboard
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
```
```http
  GET http://localhost:8000/leaderboard/tr or 18.117.96.165:8000/leaderboard/tr
```

| Parameter | Type     | Description                |
| :-------- | :------- | :------------------------- |
| `country_iso_code` | `string` | List the leaderboard. |

Leaderboard
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
        "points": 30,
        "display_name": "elif",
        "country": "tr"
    }
]
```

#### Create an user 

```http
  POST http://localhost:8000/user/create or 18.117.96.165:8000/user/create
```

| Body | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `display_name`      | `string` | **Required**. name of the user |
| `country`      | `string` | **Required**. the country iso code of the user |

```
{
    "user_id": "fda12554-c1ef-4205-8722-913596d2fbe4",
    "display_name": "ismet",
    "points": 0,
    "rank": 0
}
```

#### Create multiple users 

```http
  POST http://localhost:8000/user/create_multiple or 18.117.96.165:8000/user/create_multiple
```

| Body | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `count`      | `int ` | **Required**. the size of the users array|
| `{display_name:string,country: string}` | `array` | **Required**. display_name and country of the users |


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

```http
  POST http://localhost:8000/submit/score or 18.117.96.165:8000/submit/score
```

| Body | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `score_worth`      | `float or int ` | **Required**. the point to submit |
| `user_id`      | `string` | **Required**. the id of the user |

```
{
    "score_worth": 101,
    "user_id": "1021278d-4756-426f-b923-9a9e9cd93349",
    "timestamp": 1623000201
}
```

#### Submit multiple score

```http
  POST http://localhost:8000/score/submit_multiple or 18.117.96.165:8000/submit/submit_multiple
```

| Body | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `count`      | `int ` | **Required**. the size of the scores array|
| `{score_worth: float or int`, `user_id: string}`     | `array ` | **Required**. the point to submit |

```
{
    "score_worth": 101,
    "user_id": "1021278d-4756-426f-b923-9a9e9cd93349",
    "timestamp": 1623000201
},
{
    "score_worth": 201,
    "user_id": "b6c93b8c-aa7e-4b76-9eb7-b06fea707e8b",
    "timestamp": 1623000498
}
```

  
## Optimizations

```
- When the program is started we try to connect both redis and sql.
  We can create connection pools for both databases to use a connect from the pool
  However, it is not implemented yet.
```
```
- We need to establish synchronization between Redis and Postgresql database.
  If there is an connection error in Redis database we store our values in Postgresql.
  When we get the connection of Redis and we request a get method then the response would
  not be the same as the response we would get from Postgresql.
```
  
## Tech Stack
**Server:** Golang, Echo
```
  I had made a research and found that the best framework which is good at scalability is Echo for Golang so
  so I decided to use Echo even though Gin is much more popular and Fiber is much more faster then Echo.
```

**Database:** Postgresql, Redis
```
  I felt that we need to design a caching mechanism to make the process faster. When we post a request
  we first store in Redis and then store it in Postgresql by asynchronous operaion. That way, when we want to get
  a user we first look at the Redis and if we do not found then we look at Postgresql which is much slower operation.
```
## Running Tests

To run tests, run the following command

```bash
  go run test
```
  
