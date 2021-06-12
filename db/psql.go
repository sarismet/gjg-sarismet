package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type SQLDatabase struct {
	SyncNeed  bool
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

	return &SQLDatabase{
		SqlClient: db,
	}, psqlInfo, nil
}

func (db *SQLDatabase) GetAllUser(countryName string) ([]User, int) {

	var rows *sql.Rows
	var rowCount int
	db.Sqlmu.Lock()
	if countryName == "" {
		userSql := "select * from (select User_Id, Display_Name, Points, Country, rank() over (order by points desc) as rank from users) t;"
		var err error
		rows, err = db.SqlClient.Query(userSql)
		if err != nil {
			log.Println("Failed to execute query: ", err)
			db.Sqlmu.Unlock()
			return nil, 0
		}
		_ = rows
		db.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = $1", "general").Scan(&rowCount)

	} else {
		userSql := "select * from (select User_Id, Display_Name, Points, Country, rank() over (order by points desc) as rank from users) t where Country = " + "'" + countryName + "';"
		var err error
		rows, err = db.SqlClient.Query(userSql)
		if err != nil {
			log.Println("Failed to execute query: ", err)
			db.Sqlmu.Unlock()
			return nil, 0
		}
		_ = rows
		db.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = $1", countryName).Scan(&rowCount)
	}
	db.Sqlmu.Unlock()

	if rowCount == 0 || rows == nil {
		return nil, 0
	}

	defer rows.Close()
	users := make([]User, rowCount)
	index := 0
	for rows.Next() {

		err := rows.Scan(&users[index].User_Id, &users[index].Display_Name, &users[index].Points, &users[index].Country, &users[index].Rank)
		if err != nil {
			log.Println("Failed to execute query: ", err)
		}
		users[index].User_Id = ""
		users[index].Rank = users[index].Rank - 1

		index = index + 1
	}

	return users, rowCount
}
func (db *SQLDatabase) GetUser(user_guid string) (User, error) {
	db.Sqlmu.Lock()
	var user User
	userSql := "select User_Id, Display_Name, Points, Country from (select User_Id, Display_Name, Points, Country, rank() over (order by points desc) as rank from users) t where User_Id = " + "'" + user_guid + "';"

	err := db.SqlClient.QueryRow(userSql).Scan(&user.User_Id, &user.Display_Name, &user.Points, &user.Country, &user.Rank)

	if err != nil {
		db.Sqlmu.Unlock()
		return user, err
	}
	db.Sqlmu.Unlock()
	user.Rank = user.Rank - 1
	return user, err
}

func (db *SQLDatabase) SaveMultipleUser(users *[]User) error {
	db.Sqlmu.Lock()
	now := time.Now()
	secs := now.Unix()
	var countrySize map[string]int = make(map[string]int)
	size := len(*users)
	updateGeneralDB := `UPDATE CountryNumberSizes SET size = size + $2, timestamp = $3 WHERE code = $1;`
	res2, _ := db.SqlClient.Exec(updateGeneralDB, "general", size, secs)
	var generalNo int = 0
	if res2 != nil {
		affectedrows, _ := res2.RowsAffected()
		if affectedrows == 0 {
			insertCountryNumberDB := `INSERT INTO CountryNumberSizes (code, size, timestamp) values($1, $2, $3);`
			_, err := db.SqlClient.Exec(insertCountryNumberDB, "general", size, secs)
			if err != nil {
				db.Sqlmu.Unlock()
				db.SyncNeed = true
				return err
			}
		} else {
			db.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = $1", "general").Scan(&generalNo)
		}
	}

	index := 0
	insertDB := "INSERT INTO  Users (User_Id, Display_Name, Points, Country, Timestamp) values "

	for index < size-1 {
		country := (*users)[index].Country
		insertDB += fmt.Sprintf("('%s', '%s', %f, '%s', '%d'),", uuid.New().String(), (*users)[index].Display_Name, 0.0, country, secs)
		countrySize[country] += 1
		(*users)[index].Timestamp = 0
		(*users)[index].Rank = generalNo + index + 1
		index++
	}

	country := (*users)[size-1].Country
	insertDB += fmt.Sprintf("('%s', '%s', %f, '%s', '%d')", uuid.New().String(), (*users)[size-1].Display_Name, 0.0, country, secs)
	countrySize[country] += 1
	(*users)[size-1].Timestamp = 0

	insertDB = insertDB + ";"
	res3, _ := db.SqlClient.Exec(insertDB)

	if res3 != nil {
		affectedrows, _ := res3.RowsAffected()
		if affectedrows == 0 {
			db.Sqlmu.Unlock()
			db.SyncNeed = true
			return errors.New("can't")
		}

	}

	for iso_code, size := range countrySize {

		updateDB := `UPDATE CountryNumberSizes SET size = size + $2, timestamp = $3 WHERE code = $1;`
		res, _ := db.SqlClient.Exec(updateDB, iso_code, size, secs)

		if res != nil {
			affectedrows, _ := res.RowsAffected()
			if affectedrows == 0 {
				insertCountryNumberDB := `INSERT INTO CountryNumberSizes (code, size, timestamp) values($1, $2, $3);`
				_, err := db.SqlClient.Exec(insertCountryNumberDB, iso_code, size, secs)
				if err != nil {
					db.Sqlmu.Unlock()
					db.SyncNeed = true
					return err
				}
			}
		}
	}

	db.Sqlmu.Unlock()
	return nil
}

func (db *SQLDatabase) SaveUser(user *User, country string) error {
	db.Sqlmu.Lock()
	now := time.Now()
	secs := now.Unix()
	updateDB := `UPDATE CountryNumberSizes SET size = size + 1, timestamp = $2 WHERE code = $1;`
	res, _ := db.SqlClient.Exec(updateDB, country, secs)

	if res != nil {
		affectedrows, _ := res.RowsAffected()
		if affectedrows == 0 {
			insertCountryNumberDB := `INSERT INTO CountryNumberSizes (code, size, timestamp) values($1, $2, $3);`
			_, err := db.SqlClient.Exec(insertCountryNumberDB, country, 1, secs)
			if err != nil {
				db.Sqlmu.Unlock()
				db.SyncNeed = true
				return err
			}
		}
	}
	var generalNo int = 0
	updateGeneralDB := `UPDATE CountryNumberSizes SET size = size + 1, timestamp = $2 WHERE code = $1;`
	res2, _ := db.SqlClient.Exec(updateGeneralDB, "general", secs)
	if res2 != nil {
		affectedrows, _ := res2.RowsAffected()
		if affectedrows == 0 {
			insertCountryNumberDB := `INSERT INTO CountryNumberSizes (code, size, timestamp) values($1, $2, $3);`
			_, err := db.SqlClient.Exec(insertCountryNumberDB, "general", 1, secs)
			if err != nil {
				db.Sqlmu.Unlock()
				db.SyncNeed = true
				return err
			}
		} else {
			db.SqlClient.QueryRow("SELECT size FROM CountryNumberSizes WHERE code = $1", "general").Scan(&generalNo)
		}
	}

	insertDB := `INSERT INTO  Users (User_Id, Display_Name, Points, Country, Timestamp) values($1, $2, $3, $4, $5);`
	res3, _ := db.SqlClient.Exec(insertDB, user.User_Id, user.Display_Name, user.Points, country, secs)

	if res3 != nil {
		affectedrows, _ := res3.RowsAffected()
		if affectedrows == 0 {
			db.Sqlmu.Unlock()
			db.SyncNeed = true
			return errors.New("can't")
		}
	}
	user.Rank = generalNo + 1
	db.Sqlmu.Unlock()

	return nil
}

func (db *SQLDatabase) SubmitScore(user_guid string, score float64) error {
	db.Sqlmu.Lock()
	now := time.Now()
	secs := now.Unix()
	userSql := "UPDATE users SET Points = Points + $2, Timestamp = $3 WHERE User_Id = $1"
	_, err := db.SqlClient.Exec(userSql, user_guid, score, secs)
	if err != nil {
		log.Println("Failed to execute query: ", err)
		db.Sqlmu.Unlock()
		db.SyncNeed = true
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
		Timestamp INT NOT NULL,
        PRIMARY KEY (User_Id));`
	_, err := db.SqlClient.Exec(createDB)
	if err != nil {
		return err
	}
	createCountryDB := `CREATE TABLE IF NOT EXISTS CountryNumberSizes(
        code VARCHAR(30) UNIQUE NOT NULL,
        size INT  NOT NULL,
		timestamp INT NOT NULL,
        PRIMARY KEY (code));`
	_, err = db.SqlClient.Exec(createCountryDB)
	if err != nil {
		return err
	}

	return nil
}
