package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

type SQLDatabase struct {
	SqlClient *sql.DB
}

func Psql() {
	fmt.Println("Hello Psql")
}

func NewSqlDatabase() (*SQLDatabase, error) {
	connStr := ""
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		log.Fatal(err)
	}
	return &SQLDatabase{
		SqlClient: db,
	}, nil
}

func (db *SQLDatabase) GetAllUser(countryName string) []LeaderBoardRespond {
	var users []LeaderBoardRespond

	var rows sql.Rows
	if countryName == "" {
		userSql := "SELECT User_Id, Display_Name, Points, Rank, Country FROM users ORDER BY Points DESC;"
		rows, err := db.SqlClient.Query(userSql)
		if err != nil {
			log.Fatal("Failed to execute query: ", err)
		}
		_ = rows

	} else {
		userSql := "SELECT User_Id, Display_Name, Points, Rank, Country FROM users WHERE Country = $1  ORDER BY Points DESC;"
		rows, err := db.SqlClient.Query(userSql, countryName)
		if err != nil {
			log.Fatal("Failed to execute query: ", err)
		}
		_ = rows
	}

	defer rows.Close()
	for rows.Next() {
		var user LeaderBoardRespond
		err := rows.Scan(&user.Rank, &user.Points, &user.Display_Name, &user.Country)
		if err != nil {
			log.Fatal("Failed to execute query: ", err)
		}
		users = append(users, user)
	}

	return users
}
func (db *SQLDatabase) GetUser(user_guid string) (User, error) {
	var user User
	userSql := "SELECT User_Id, Display_Name, Points, Rank, Country FROM users WHERE User_Id = $1"

	err := db.SqlClient.QueryRow(userSql, user_guid).Scan(&user.User_Id, &user.Display_Name, &user.Points, &user.Rank, &user.Country)
	if err != nil {
		log.Fatal("Failed to execute query: ", err)
	}

	return user, err
}
func (db *SQLDatabase) SaveUser(user User) error {

	fmt.Println("SQLDatabase SaveUser")

	insertDB := `INSERT INTO  Users (User_Id, Display_Name, Points, Rank, Country) values($1, $2, $3, $4, $5);`
	_, err := db.SqlClient.Exec(insertDB, user.User_Id, user.Display_Name, user.Points, user.Rank, user.Country)

	if err != nil {
		fmt.Printf("There is an err in sql database save %s", err)
		return err
	}
	return err
}

func (db *SQLDatabase) SubmitScore(user_guid string) error {
	userSql := "UPDATE users SET Points = Points + 1 WHERE User_Id = $1"

	_, err := db.SqlClient.Exec(userSql, user_guid)
	if err != nil {
		log.Fatal("Failed to execute query: ", err)
		return err
	}

	return nil
}

func (db *SQLDatabase) CreateTableNotExists() error {
	createDB := `CREATE TABLE IF NOT EXISTS Users(
        User_Id VARCHAR(100) UNIQUE NOT NULL,
        Display_Name VARCHAR(100) NOT NULL,
        Points FLOAT NOT NULL,
        Rank INT  NOT NULL,
        Country VARCHAR(10) NOT NULL,
        PRIMARY KEY (User_Id));`
	_, err := db.SqlClient.Exec(createDB)

	if err != nil {
		fmt.Printf("There is an err in sql database save %s", err)
		return err
	}

	return nil
}
