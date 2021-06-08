package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

type SQLDatabase struct {
	SqlClient *sql.DB
}

func NewSqlDatabase() (*SQLDatabase, error) {
	const (
		host     = "0.0.0.0"
		port     = 5432
		user     = "postgres"
		password = "123"
		dbname   = "postgres"
	)
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db, err := sql.Open("postgres", psqlInfo)

	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &SQLDatabase{
		SqlClient: db,
	}, nil
}

func (db *SQLDatabase) GetAllUser(countryName string) ([]User, error) {

	var rows sql.Rows
	var rowCount int = 0
	if countryName == "" {
		userSql := "select * from (select *, rank() over (order by points desc) as rank from users) t;"
		rows, err := db.SqlClient.Query(userSql)
		if err != nil {
			log.Fatal("Failed to execute query: ", err)
			return nil, err
		}
		_ = rows
		db.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = general").Scan(&rowCount)

	} else {
		userSql := "select * from (select *, rank() over (order by points desc) as rank from users) t where Country = $1;"
		rows, err := db.SqlClient.Query(userSql, countryName)
		if err != nil {
			log.Fatal("Failed to execute query: ", err)
			return nil, err
		}
		_ = rows
		db.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = $1", countryName).Scan(&rowCount)
	}
	if rowCount == 0 {
		return nil, errors.New("rowCount is zero")
	}

	defer rows.Close()
	users := make([]User, rowCount)
	index := 0
	for rows.Next() {
		var user User
		err := rows.Scan(&user.User_Id, &user.Rank, &user.Points, &user.Display_Name, &user.Country)
		if err != nil {
			log.Fatal("Failed to execute query: ", err)
		}
		users[index] = user
		index = index + 1
	}

	return users, nil
}
func (db *SQLDatabase) GetUser(user_guid string) (User, error) {
	var user User
	userSql := "select * from (select *, rank() over (order by points desc) as rank from users) t where display_name = $1;"

	err := db.SqlClient.QueryRow(userSql, user_guid).Scan(&user.User_Id, &user.Display_Name, &user.Points, &user.Country, &user.Rank)
	if err != nil {
		log.Fatal("Failed to execute query: ", err)
	}

	return user, err
}
func (db *SQLDatabase) SaveUser(user *User, country string) error {

	updateDB := `UPDATE CountryNumberSizes SET size = size + 1 WHERE code = $1;`
	res, _ := db.SqlClient.Exec(updateDB, country)

	if res != nil {
		affectedrows, _ := res.RowsAffected()
		if affectedrows == 0 {
			insertCountryNumberDB := `INSERT INTO CountryNumberSizes (code, size) values($1, $2);`
			_, err := db.SqlClient.Exec(insertCountryNumberDB, country, 1)
			if err != nil {
				return err
			}
		}
	}

	updateGeneralDB := `UPDATE CountryNumberSizes SET size = size + 1 WHERE code = $1;`
	res2, _ := db.SqlClient.Exec(updateGeneralDB, "general")
	if res2 != nil {
		affectedrows, _ := res2.RowsAffected()
		if affectedrows == 0 {
			insertCountryNumberDB := `INSERT INTO CountryNumberSizes (code, size) values($1, $2);`
			_, err := db.SqlClient.Exec(insertCountryNumberDB, "general", 1)
			if err != nil {
				return err
			}
		}
	}

	insertDB := `INSERT INTO  Users (User_Id, Display_Name, Points, Country) values($1, $2, $3, $4);`
	res3, _ := db.SqlClient.Exec(insertDB, user.User_Id, user.Display_Name, user.Points, country)

	if res3 != nil {
		affectedrows, _ := res3.RowsAffected()
		if affectedrows == 0 {
			return errors.New("can't work with 42")
		}
	}

	return nil
}

func (db *SQLDatabase) SubmitScore(user_guid string, score float64) error {
	userSql := "UPDATE users SET Points = Points + $2 WHERE User_Id = $1"
	_, err := db.SqlClient.Exec(userSql, user_guid, score)
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
        Country VARCHAR(10) NOT NULL,
        PRIMARY KEY (User_Id));`
	_, err := db.SqlClient.Exec(createDB)
	if err != nil {
		return err
	}
	createCountryDB := `CREATE TABLE IF NOT EXISTS CountryNumberSizes(
        code VARCHAR(30) UNIQUE NOT NULL,
        size INT  NOT NULL,
        PRIMARY KEY (code));`
	_, err = db.SqlClient.Exec(createCountryDB)
	if err != nil {
		return err
	}

	return nil
}
