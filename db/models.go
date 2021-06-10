package db

type User struct {
	User_Id      string  `json:"user_id,omitempty" bson:"user_id,omitempty"`
	Display_Name string  `json:"display_name" bson:"display_name"`
	Points       float64 `json:"points" bson:"points"`
	Rank         int     `json:"rank" bson:"rank"`
	Country      string  `json:"country,omitempty" bson:"country,omitempty"`
}

type LeaderBoardRespond struct {
	Rank         int     `json:"rank" bson:"rank"`
	Points       float64 `json:"points" bson:"points"`
	Display_Name string  `json:"display_name" bson:"display_name"`
	Country      string  `json:"country" bson:"country"`
}

type Score struct {
	Score_worth float64 `json:"score_worth" bson:"score_worth"`
	User_Id     string  `json:"user_id" bson:"user_id"`
	Timestamp   int64   `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
}

type MultipleUsers struct {
	Count int    `json:"count" bson:"count"`
	Users []User `json:"users" bson:"users"`
}

type MultipleScores struct {
	Count  int     `json:"count" bson:"count"`
	Scores []Score `json:"scores" bson:"scores"`
}
