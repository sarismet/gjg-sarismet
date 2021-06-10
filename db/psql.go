package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"

	_ "github.com/lib/pq"
)

type SQLDatabase struct {
	Sqlmu     sync.Mutex
	SqlClient *sql.DB
}

func NewSqlDatabase() (*SQLDatabase, string, error) {
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
		return nil, "", err

	}

	if err != nil {
		log.Fatal(err)
		return nil, "", err
	}
	return &SQLDatabase{
		SqlClient: db,
	}, psqlInfo, nil
}

func (db *SQLDatabase) GetAllUser(countryName string) ([]User, int) {

	var rows *sql.Rows
	var rowCount int
	db.Sqlmu.Lock()
	if countryName == "" {
		userSql := "select * from (select *, rank() over (order by points desc) as rank from users) t;"
		var err error
		rows, err = db.SqlClient.Query(userSql)
		if err != nil {
			log.Fatal("Failed to execute query: ", err)
			db.Sqlmu.Unlock()
			return nil, 0
		}
		_ = rows
		db.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = $1", "general").Scan(&rowCount)
		fmt.Printf("Round count is %d", rowCount)

	} else {
		userSql := "select * from (select *, rank() over (order by points desc) as rank from users) t where Country = " + "'" + countryName + "';"
		var err error
		rows, err = db.SqlClient.Query(userSql)
		if err != nil {
			log.Fatal("Failed to execute query: ", err)
			db.Sqlmu.Unlock()
			return nil, 0
		}
		_ = rows
		db.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = $1", countryName).Scan(&rowCount)
		fmt.Printf("Round count asdasdasd is %d", rowCount)
	}
	db.Sqlmu.Unlock()

	if rowCount == 0 || rows == nil {
		return nil, 0
	}

	defer rows.Close()
	users := make([]User, rowCount)
	index := 0
	for rows.Next() {
		var user User
		err := rows.Scan(&user.User_Id, &user.Display_Name, &user.Points, &user.Country, &user.Rank)
		if err != nil {
			log.Fatal("Failed to execute query: ", err)
		}
		user.Rank = user.Rank - 1
		users[index] = user
		index = index + 1
	}

	return users, rowCount
}
func (db *SQLDatabase) GetUser(user_guid string) (User, error) {
	db.Sqlmu.Lock()
	var user User
	userSql := "select * from (select *, rank() over (order by points desc) as rank from users) t where User_Id = " + "'" + user_guid + "';"

	fmt.Printf("userSql %s\n", userSql)
	fmt.Printf("user_guid %s\n", user_guid)
	err := db.SqlClient.QueryRow(userSql).Scan(&user.User_Id, &user.Display_Name, &user.Points, &user.Country, &user.Rank)

	if err != nil {
		db.Sqlmu.Unlock()
		return user, err
	}
	db.Sqlmu.Unlock()
	user.Rank = user.Rank - 1
	return user, err
}
func (db *SQLDatabase) SaveUser(user *User, country string) error {
	db.Sqlmu.Lock()
	updateDB := `UPDATE CountryNumberSizes SET size = size + 1 WHERE code = $1;`
	res, _ := db.SqlClient.Exec(updateDB, country)

	if res != nil {
		affectedrows, _ := res.RowsAffected()
		if affectedrows == 0 {
			insertCountryNumberDB := `INSERT INTO CountryNumberSizes (code, size) values($1, $2);`
			_, err := db.SqlClient.Exec(insertCountryNumberDB, country, 1)
			if err != nil {
				db.Sqlmu.Unlock()
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
				db.Sqlmu.Unlock()
				return err
			}
		}
	}

	insertDB := `INSERT INTO  Users (User_Id, Display_Name, Points, Country) values($1, $2, $3, $4);`
	res3, _ := db.SqlClient.Exec(insertDB, user.User_Id, user.Display_Name, user.Points, country)

	if res3 != nil {
		affectedrows, _ := res3.RowsAffected()
		if affectedrows == 0 {
			db.Sqlmu.Unlock()
			return errors.New("can't")
		}
	}
	db.Sqlmu.Unlock()

	return nil
}

func (db *SQLDatabase) SubmitScore(user_guid string, score float64) error {
	db.Sqlmu.Lock()
	userSql := "UPDATE users SET Points = Points + $2 WHERE User_Id = $1"
	_, err := db.SqlClient.Exec(userSql, user_guid, score)
	if err != nil {
		log.Fatal("Failed to execute query: ", err)
		db.Sqlmu.Unlock()
		return err
	}
	db.Sqlmu.Unlock()
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
